// Package collector turns ZLMediaKit API responses into Prometheus metrics.
package collector

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/guohuachan/ZLMediaKit_exporter/internal/zlmapi"
)

// Metric namespace and per-endpoint subsystems. An empty subsystem yields a
// name directly under the namespace, e.g. zlm_streams.
const (
	Namespace              = "zlm"
	SubsystemVersion       = "version"
	SubsystemAPI           = "api"
	SubsystemExporter      = "exporter"
	SubsystemNetworkThread = "network_thread"
	SubsystemWorkThread    = "work_thread"
	SubsystemStatistics    = "statistics"
	SubsystemSession       = "session"
	SubsystemStream        = "stream"
	SubsystemStreamProxy   = "stream_proxy"
	SubsystemStreamPusher  = "stream_pusher"
	SubsystemRtp           = "rtp"
)

// defaultScrapeTimeout bounds a single /metrics scrape across all collectors.
const defaultScrapeTimeout = 12 * time.Second

// A Collector scrapes one ZLMediaKit API endpoint and turns it into metrics.
type Collector interface {
	// Name identifies the collector in the exporter's own metrics and in
	// --zlm.disable-collectors.
	Name() string
	// Endpoint is the API path this collector scrapes.
	Endpoint() string
	Describe(ch chan<- *prometheus.Desc)
	Collect(ctx context.Context, client *zlmapi.Client, ch chan<- prometheus.Metric) error
}

// Config selects which collectors run and how much detail they emit. The zero
// value is the shipped default: no per-endpoint API status, no per-session
// series, tracks enabled, nothing disabled.
type Config struct {
	// ExposeAPIStatus enables zlm_api_status, which emits one constant series
	// per ZLMediaKit endpoint (around seventy of them).
	ExposeAPIStatus bool

	// ExposeSessionInfo enables the per-connection zlm_session_info series.
	// zlm_sessions carries the aggregate either way.
	ExposeSessionInfo bool

	// DisableStreamTracks drops the per-track metrics.
	DisableStreamTracks bool

	// DisabledCollectors names collectors to skip entirely.
	DisabledCollectors []string
}

// Exporter collects every ZLMediaKit metric for a single server.
type Exporter struct {
	client     *zlmapi.Client
	collectors []Collector
	log        *slog.Logger
	mutex      sync.RWMutex

	scrapeTimeout time.Duration

	up               prometheus.Gauge
	totalScrapes     prometheus.Counter
	scrapeErrors     *prometheus.CounterVec
	scrapeDuration   *prometheus.GaugeVec
	collectorSuccess *prometheus.GaugeVec
}

// New returns an Exporter scraping the ZLMediaKit API server at uri.
func New(uri, secret string, logger *slog.Logger, options zlmapi.Options, cfg Config) (*Exporter, error) {
	client, err := zlmapi.NewClient(uri, secret, options)
	if err != nil {
		return nil, err
	}

	e := &Exporter{
		client:        client,
		log:           logger,
		scrapeTimeout: defaultScrapeTimeout,
		collectors:    enabledCollectors(cfg),

		up: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "up",
			Help:      "Was the last scrape of ZLMediaKit successful.",
		}),

		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: SubsystemExporter,
			Name:      "scrapes_total",
			Help:      "Current total ZLMediaKit scrapes.",
		}),

		scrapeErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: Namespace,
			Subsystem: SubsystemExporter,
			Name:      "scrape_errors_total",
			Help:      "Number of errors while scraping ZLMediaKit, per collector.",
		}, []string{"collector"}),

		scrapeDuration: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: Namespace,
			Subsystem: SubsystemExporter,
			Name:      "scrape_duration_seconds",
			Help:      "Duration of the last scrape, per collector.",
		}, []string{"collector"}),

		collectorSuccess: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: Namespace,
			Subsystem: SubsystemExporter,
			Name:      "collector_success",
			Help:      "Whether the last scrape of a collector succeeded (1: yes, 0: no).",
		}, []string{"collector"}),
	}

	// Materialise one series per collector so a rate() or an alert has a
	// baseline instead of no data until the first failure.
	for _, c := range e.collectors {
		e.scrapeErrors.WithLabelValues(c.Name())
	}
	return e, nil
}

// enabledCollectors applies cfg to the full collector set.
func enabledCollectors(cfg Config) []Collector {
	all := []Collector{
		versionCollector{},
		apiStatusCollector{},
		networkThreadsCollector(),
		workThreadsCollector(),
		statisticsCollector{},
		sessionCollector{exposeInfo: cfg.ExposeSessionInfo},
		streamCollector{exposeTracks: !cfg.DisableStreamTracks},
		streamProxyCollector{},
		streamPusherCollector{},
		rtpCollector{},
	}

	disabled := make(map[string]bool, len(cfg.DisabledCollectors)+1)
	for _, name := range cfg.DisabledCollectors {
		if name = strings.TrimSpace(name); name != "" {
			disabled[name] = true
		}
	}
	if !cfg.ExposeAPIStatus {
		disabled[apiStatusCollector{}.Name()] = true
	}

	enabled := make([]Collector, 0, len(all))
	for _, c := range all {
		if !disabled[c.Name()] {
			enabled = append(enabled, c)
		}
	}
	return enabled
}

// Describe implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	for _, c := range e.collectors {
		c.Describe(ch)
	}
	ch <- e.up.Desc()
	ch <- e.totalScrapes.Desc()
	e.scrapeErrors.Describe(ch)
	e.scrapeDuration.Describe(ch)
	e.collectorSuccess.Describe(ch)
}

// Collect implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	up := e.scrape(ch)
	ch <- prometheus.MustNewConstMetric(e.up.Desc(), prometheus.GaugeValue, up)
	ch <- e.totalScrapes
	e.scrapeErrors.Collect(ch)
	e.scrapeDuration.Collect(ch)
	e.collectorSuccess.Collect(ch)
}

func (e *Exporter) scrape(ch chan<- prometheus.Metric) (up float64) {
	e.totalScrapes.Inc()

	ctx, cancel := context.WithTimeout(context.Background(), e.scrapeTimeout)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(len(e.collectors))
	for _, c := range e.collectors {
		go func(c Collector) {
			defer wg.Done()

			start := time.Now()
			err := c.Collect(ctx, e.client, ch)
			e.scrapeDuration.WithLabelValues(c.Name()).Set(time.Since(start).Seconds())

			success := 1.0
			if err != nil {
				success = 0
				e.scrapeErrors.WithLabelValues(c.Name()).Inc()
				e.log.Error("error collecting metrics", "collector", c.Name(), "endpoint", c.Endpoint(), "err", err)
			}
			e.collectorSuccess.WithLabelValues(c.Name()).Set(success)
		}(c)
	}

	// Wait unconditionally. Returning at the deadline would leave collectors
	// running and sending on a metric channel that Collect has already closed.
	wg.Wait()

	if err := ctx.Err(); err != nil {
		e.log.Error("scrape timeout", "err", err)
		return 0
	}
	return 1
}

// newMetricDescr builds a metric descriptor under the exporter's namespace.
// An empty subsystem places the metric directly under the namespace.
func newMetricDescr(subsystem, metricName, docString string, labels []string) *prometheus.Desc {
	return prometheus.NewDesc(prometheus.BuildFQName(Namespace, subsystem, metricName), docString, labels, nil)
}

// mustNewConstMetric accepts either a float64 or a numeric string. A string
// that does not parse yields the value 1, which keeps info-style metrics
// usable. Any other type yields an invalid metric rather than nil, so that
// sending the result on the collection channel can never panic.
func mustNewConstMetric(desc *prometheus.Desc, valueType prometheus.ValueType, value interface{}, labelValues ...string) prometheus.Metric {
	switch vt := value.(type) {
	case float64:
		return prometheus.MustNewConstMetric(desc, valueType, vt, labelValues...)
	case string:
		valueFloat, err := strconv.ParseFloat(vt, 64)
		if err == nil {
			return prometheus.MustNewConstMetric(desc, valueType, valueFloat, labelValues...)
		}
		return prometheus.MustNewConstMetric(desc, valueType, 1, labelValues...)
	default:
		return prometheus.NewInvalidMetric(desc, fmt.Errorf("unsupported metric value type %T", value))
	}
}

// millisecondsToSeconds converts a ZLMediaKit duration to the Prometheus base
// unit.
func millisecondsToSeconds(ms float64) float64 { return ms / 1000 }
