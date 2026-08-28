package collector

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/guohuachan/ZLMediaKit_exporter/internal/zlmapi"
)

var (
	networkThreadsTotal      = newMetricDescr(SubsystemNetworkThreads, "total", "Total number of network threads", []string{})
	networkThreadsLoadTotal  = newMetricDescr(SubsystemNetworkThreads, "load_total", "Total of network threads load", []string{})
	networkThreadsDelayTotal = newMetricDescr(SubsystemNetworkThreads, "delay_total", "Total of network threads delay", []string{})

	workThreadsTotal      = newMetricDescr(SubsystemWorkThreads, "total", "Total number of work threads", []string{})
	workThreadsLoadTotal  = newMetricDescr(SubsystemWorkThreads, "load_total", "Total of work threads load", []string{})
	workThreadsDelayTotal = newMetricDescr(SubsystemWorkThreads, "delay_total", "Total of work threads delay", []string{})
)

// threadsCollector serves both the network (event poller) and the work thread
// pools; the two endpoints return the same shape.
type threadsCollector struct {
	endpoint  string
	totalDesc *prometheus.Desc
	loadDesc  *prometheus.Desc
	delayDesc *prometheus.Desc
}

func networkThreadsCollector() threadsCollector {
	return threadsCollector{
		endpoint:  zlmapi.EndpointGetThreadsLoad,
		totalDesc: networkThreadsTotal,
		loadDesc:  networkThreadsLoadTotal,
		delayDesc: networkThreadsDelayTotal,
	}
}

func workThreadsCollector() threadsCollector {
	return threadsCollector{
		endpoint:  zlmapi.EndpointGetWorkThreadsLoad,
		totalDesc: workThreadsTotal,
		loadDesc:  workThreadsLoadTotal,
		delayDesc: workThreadsDelayTotal,
	}
}

func (c threadsCollector) Endpoint() string { return c.endpoint }

func (c threadsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.totalDesc
	ch <- c.loadDesc
	ch <- c.delayDesc
}

func (c threadsCollector) Collect(ctx context.Context, client *zlmapi.Client, ch chan<- prometheus.Metric) error {
	data, err := zlmapi.Get[[]zlmapi.ThreadLoad](ctx, client, c.endpoint)
	if err != nil {
		return err
	}

	var loadTotal, delayTotal, total float64
	for _, thread := range data {
		loadTotal += thread.Load
		delayTotal += thread.Delay
		total++
	}
	ch <- prometheus.MustNewConstMetric(c.totalDesc, prometheus.GaugeValue, total)
	ch <- prometheus.MustNewConstMetric(c.loadDesc, prometheus.GaugeValue, loadTotal)
	ch <- prometheus.MustNewConstMetric(c.delayDesc, prometheus.GaugeValue, delayTotal)
	return nil
}
