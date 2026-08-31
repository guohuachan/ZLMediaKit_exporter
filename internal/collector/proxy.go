package collector

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/guohuachan/ZLMediaKit_exporter/internal/zlmapi"
)

// Proxy metric label sets. "key" is ZLMediaKit's own handle for the proxy and
// is what the delStreamProxy API takes, so it is worth carrying.
var (
	proxyLabels = []string{"key", "vhost", "app", "stream"}

	streamProxyInfo = newMetricDescr(SubsystemStreamProxy, "info", "Stream pull proxy information",
		[]string{"key", "vhost", "app", "stream", "url", "status_str"})
	streamProxyStatus = newMetricDescr(SubsystemStreamProxy, "status",
		"Stream pull proxy status as reported by ZLMediaKit (0: success)", proxyLabels)
	streamProxyLiveSeconds = newMetricDescr(SubsystemStreamProxy, "live_seconds",
		"Seconds the stream pull proxy has been alive", proxyLabels)
	streamProxyRePullTotal = newMetricDescr(SubsystemStreamProxy, "repull_total",
		"Number of times the stream pull proxy reconnected", proxyLabels)
	streamProxyReaders = newMetricDescr(SubsystemStreamProxy, "total_readers",
		"Number of readers of the proxied stream", proxyLabels)
	streamProxyBytesPerSecond = newMetricDescr(SubsystemStreamProxy, "bytes_per_second",
		"Current receive rate of the stream pull proxy in bytes per second", proxyLabels)
	streamProxyBytesTotal = newMetricDescr(SubsystemStreamProxy, "bytes_total",
		"Total bytes received by the stream pull proxy", proxyLabels)
	streamProxies = newMetricDescr("", "stream_proxies", "Number of stream pull proxies", nil)

	streamPusherInfo = newMetricDescr(SubsystemStreamPusher, "info", "Stream push proxy information",
		[]string{"key", "vhost", "app", "stream", "url"})
	streamPusherStatus = newMetricDescr(SubsystemStreamPusher, "status",
		"Stream push proxy status as reported by ZLMediaKit (0: success)", proxyLabels)
	streamPusherLiveSeconds = newMetricDescr(SubsystemStreamPusher, "live_seconds",
		"Seconds the stream push proxy has been alive", proxyLabels)
	streamPusherRePublishTotal = newMetricDescr(SubsystemStreamPusher, "republish_total",
		"Number of times the stream push proxy reconnected", proxyLabels)
	streamPusherBytesPerSecond = newMetricDescr(SubsystemStreamPusher, "bytes_per_second",
		"Current send rate of the stream push proxy in bytes per second", proxyLabels)
	streamPusherBytesTotal = newMetricDescr(SubsystemStreamPusher, "bytes_total",
		"Total bytes sent by the stream push proxy", proxyLabels)
	streamPushers = newMetricDescr("", "stream_pushers", "Number of stream push proxies", nil)
)

type streamProxyCollector struct{}

func (streamProxyCollector) Name() string     { return "stream_proxy" }
func (streamProxyCollector) Endpoint() string { return zlmapi.EndpointListStreamProxy }

func (streamProxyCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{
		streamProxyInfo, streamProxyStatus, streamProxyLiveSeconds, streamProxyRePullTotal,
		streamProxyReaders, streamProxyBytesPerSecond, streamProxyBytesTotal, streamProxies,
	} {
		ch <- d
	}
}

func (streamProxyCollector) Collect(ctx context.Context, client *zlmapi.Client, ch chan<- prometheus.Metric) error {
	data, err := zlmapi.Get[[]zlmapi.StreamProxy](ctx, client, zlmapi.EndpointListStreamProxy)
	if err != nil {
		return err
	}

	for _, p := range data {
		labels := []string{p.Key, p.Src.Vhost, p.Src.App, p.Src.Stream}

		ch <- prometheus.MustNewConstMetric(streamProxyInfo, prometheus.GaugeValue, 1,
			p.Key, p.Src.Vhost, p.Src.App, p.Src.Stream, p.URL, p.StatusStr)
		ch <- prometheus.MustNewConstMetric(streamProxyStatus, prometheus.GaugeValue, p.Status, labels...)
		ch <- prometheus.MustNewConstMetric(streamProxyLiveSeconds, prometheus.GaugeValue, p.LiveSecs, labels...)
		ch <- prometheus.MustNewConstMetric(streamProxyRePullTotal, prometheus.CounterValue, p.RePullCount, labels...)
		ch <- prometheus.MustNewConstMetric(streamProxyReaders, prometheus.GaugeValue, p.TotalReaderCount, labels...)
		ch <- prometheus.MustNewConstMetric(streamProxyBytesPerSecond, prometheus.GaugeValue, p.BytesSpeed, labels...)
		ch <- prometheus.MustNewConstMetric(streamProxyBytesTotal, prometheus.CounterValue, p.TotalBytes, labels...)
	}
	ch <- prometheus.MustNewConstMetric(streamProxies, prometheus.GaugeValue, float64(len(data)))
	return nil
}

type streamPusherCollector struct{}

func (streamPusherCollector) Name() string     { return "stream_pusher" }
func (streamPusherCollector) Endpoint() string { return zlmapi.EndpointListStreamPusherProxy }

func (streamPusherCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{
		streamPusherInfo, streamPusherStatus, streamPusherLiveSeconds, streamPusherRePublishTotal,
		streamPusherBytesPerSecond, streamPusherBytesTotal, streamPushers,
	} {
		ch <- d
	}
}

func (streamPusherCollector) Collect(ctx context.Context, client *zlmapi.Client, ch chan<- prometheus.Metric) error {
	data, err := zlmapi.Get[[]zlmapi.StreamPusherProxy](ctx, client, zlmapi.EndpointListStreamPusherProxy)
	if err != nil {
		return err
	}

	for _, p := range data {
		labels := []string{p.Key, p.Src.Vhost, p.Src.App, p.Src.Stream}

		ch <- prometheus.MustNewConstMetric(streamPusherInfo, prometheus.GaugeValue, 1,
			p.Key, p.Src.Vhost, p.Src.App, p.Src.Stream, p.URL)
		ch <- prometheus.MustNewConstMetric(streamPusherStatus, prometheus.GaugeValue, p.Status, labels...)
		ch <- prometheus.MustNewConstMetric(streamPusherLiveSeconds, prometheus.GaugeValue, p.LiveSecs, labels...)
		ch <- prometheus.MustNewConstMetric(streamPusherRePublishTotal, prometheus.CounterValue, p.RePublishCount, labels...)
		ch <- prometheus.MustNewConstMetric(streamPusherBytesPerSecond, prometheus.GaugeValue, p.BytesSpeed, labels...)
		ch <- prometheus.MustNewConstMetric(streamPusherBytesTotal, prometheus.CounterValue, p.TotalBytes, labels...)
	}
	ch <- prometheus.MustNewConstMetric(streamPushers, prometheus.GaugeValue, float64(len(data)))
	return nil
}
