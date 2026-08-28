// Package collector turns ZLMediaKit API responses into Prometheus metrics.
package collector

import (
	"context"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/guohuachan/ZLMediaKit_exporter/internal/zlmapi"
)

// Metric namespace and per-endpoint subsystems.
const (
	Namespace               = "zlm"
	SubsystemVersion        = "version"
	SubsystemAPI            = "api"
	SubsystemNetworkThreads = "network_threads"
	SubsystemWorkThreads    = "work_threads"
	SubsystemStatistics     = "statistics"
	SubsystemSession        = "session"
	SubsystemStream         = "stream"
	SubsystemRtp            = "rtp"
)

// scrapeTimeout bounds a single /metrics scrape across all collectors.
const scrapeTimeout = 12 * time.Second

// A Collector scrapes one ZLMediaKit API endpoint and turns it into metrics.
type Collector interface {
	// Endpoint is the API path this collector scrapes. It also labels the
	// scrape error counter.
	Endpoint() string
	Describe(ch chan<- *prometheus.Desc)
	Collect(ctx context.Context, client *zlmapi.Client, ch chan<- prometheus.Metric) error
}

// Exporter collects every ZLMediaKit metric for a single server.
type Exporter struct {
	client     *zlmapi.Client
	collectors []Collector
	log        *slog.Logger
	mutex      sync.RWMutex

	up           prometheus.Gauge
	totalScrapes prometheus.Counter
	scrapeErrors *prometheus.CounterVec
}

// New returns an Exporter scraping the ZLMediaKit API server at uri.
func New(uri, secret string, logger *slog.Logger, options zlmapi.Options) (*Exporter, error) {
	client, err := zlmapi.NewClient(uri, secret, options)
	if err != nil {
		return nil, err
	}

	return &Exporter{
		client: client,
		log:    logger,
		collectors: []Collector{
			versionCollector{},
			apiStatusCollector{},
			networkThreadsCollector(),
			workThreadsCollector(),
			statisticsCollector{},
			sessionCollector{},
			streamCollector{},
			rtpCollector{},
		},

		up: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "up",
			Help:      "Was the last scrape of ZLMediaKit successful.",
		}),

		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "exporter_scrapes_total",
			Help:      "Current total ZLMediaKit scrapes.",
		}),

		scrapeErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "scrape_errors_total",
			Help:      "Number of errors while scraping ZLMediaKit.",
		}, []string{"endpoint"}),
	}, nil
}

// Describe implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	for _, c := range e.collectors {
		c.Describe(ch)
	}
	ch <- e.up.Desc()
	ch <- e.totalScrapes.Desc()
}

// Collect implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	up := e.scrape(ch)
	ch <- prometheus.MustNewConstMetric(e.up.Desc(), prometheus.GaugeValue, up)
	ch <- e.totalScrapes
}

func (e *Exporter) scrape(ch chan<- prometheus.Metric) (up float64) {
	e.totalScrapes.Inc()

	ctx, cancel := context.WithTimeout(context.Background(), scrapeTimeout)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(len(e.collectors))
	for _, c := range e.collectors {
		go func(c Collector) {
			defer wg.Done()
			if err := c.Collect(ctx, e.client, ch); err != nil {
				e.scrapeErrors.WithLabelValues(c.Endpoint()).Inc()
				e.log.Error("error processing response", "endpoint", c.Endpoint(), "err", err)
			}
		}(c)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		e.log.Error("scrape timeout", "error", ctx.Err())
		return 0
	case <-done:
		return 1
	}
}

// newMetricDescr builds a metric descriptor under the exporter's namespace.
func newMetricDescr(subsystem, metricName, docString string, labels []string) *prometheus.Desc {
	return prometheus.NewDesc(prometheus.BuildFQName(Namespace, subsystem, metricName), docString, labels, nil)
}

// mustNewConstMetric accepts either a float64 or a numeric string. A string
// that does not parse yields the value 1, which keeps info-style metrics
// usable; any other type yields nil.
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
		return nil
	}
}
