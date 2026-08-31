package collector

import (
	"context"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/guohuachan/ZLMediaKit_exporter/internal/zlmapi"
)

var (
	networkThreads     = newMetricDescr("", "network_threads", "Number of network (event poller) threads", nil)
	networkThreadLoad  = newMetricDescr(SubsystemNetworkThread, "load_percent", "Network thread load in percent", []string{"index"})
	networkThreadDelay = newMetricDescr(SubsystemNetworkThread, "delay_seconds", "Network thread task delay in seconds", []string{"index"})

	workThreads     = newMetricDescr("", "work_threads", "Number of work threads", nil)
	workThreadLoad  = newMetricDescr(SubsystemWorkThread, "load_percent", "Work thread load in percent", []string{"index"})
	workThreadDelay = newMetricDescr(SubsystemWorkThread, "delay_seconds", "Work thread task delay in seconds", []string{"index"})
)

// threadsCollector serves both the network (event poller) and the work thread
// pools; the two endpoints return the same shape.
type threadsCollector struct {
	name      string
	endpoint  string
	countDesc *prometheus.Desc
	loadDesc  *prometheus.Desc
	delayDesc *prometheus.Desc
}

func networkThreadsCollector() threadsCollector {
	return threadsCollector{
		name:      "network_threads",
		endpoint:  zlmapi.EndpointGetThreadsLoad,
		countDesc: networkThreads,
		loadDesc:  networkThreadLoad,
		delayDesc: networkThreadDelay,
	}
}

func workThreadsCollector() threadsCollector {
	return threadsCollector{
		name:      "work_threads",
		endpoint:  zlmapi.EndpointGetWorkThreadsLoad,
		countDesc: workThreads,
		loadDesc:  workThreadLoad,
		delayDesc: workThreadDelay,
	}
}

func (c threadsCollector) Name() string     { return c.name }
func (c threadsCollector) Endpoint() string { return c.endpoint }

func (c threadsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.countDesc
	ch <- c.loadDesc
	ch <- c.delayDesc
}

// Collect reports each thread individually. The pool size tracks the CPU
// count, so the cardinality is bounded, and sum()/avg() recover the aggregates
// the exporter used to compute itself.
func (c threadsCollector) Collect(ctx context.Context, client *zlmapi.Client, ch chan<- prometheus.Metric) error {
	data, err := zlmapi.Get[[]zlmapi.ThreadLoad](ctx, client, c.endpoint)
	if err != nil {
		return err
	}

	for i, thread := range data {
		index := strconv.Itoa(i)
		ch <- prometheus.MustNewConstMetric(c.loadDesc, prometheus.GaugeValue, thread.Load, index)
		ch <- prometheus.MustNewConstMetric(c.delayDesc, prometheus.GaugeValue, millisecondsToSeconds(thread.Delay), index)
	}
	ch <- prometheus.MustNewConstMetric(c.countDesc, prometheus.GaugeValue, float64(len(data)))
	return nil
}
