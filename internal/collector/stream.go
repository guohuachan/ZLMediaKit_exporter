package collector

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/guohuachan/ZLMediaKit_exporter/internal/zlmapi"
)

var (
	streamsInfo = newMetricDescr(SubsystemStream, "info", "Stream basic information",
		[]string{"vhost", "app", "stream", "schema", "origin_type", "origin_url"})
	streamStatus = newMetricDescr(SubsystemStream, "status", "Stream status (1: active with data flowing, 0: inactive)",
		[]string{"vhost", "app", "stream", "schema"})
	streamReaderCount = newMetricDescr(SubsystemStream, "reader_count", "Stream reader count",
		[]string{"vhost", "app", "stream", "schema"})
	streamTotalReaderCount = newMetricDescr(SubsystemStream, "total_reader_count", "Total reader count across all schemas",
		[]string{"vhost", "app", "stream"})
	streamBitrate = newMetricDescr(SubsystemStream, "bitrate", "Stream bitrate",
		[]string{"vhost", "app", "stream", "schema"})
	streamAliveSecond = newMetricDescr(SubsystemStream, "alive_second", "Stream alive second",
		[]string{"vhost", "app", "stream", "schema"})
	streamCreateStamp = newMetricDescr(SubsystemStream, "create_stamp", "Stream create stamp",
		[]string{"vhost", "app", "stream", "schema"})
	streamTotal = newMetricDescr(SubsystemStream, "total", "Total number of streams", []string{})
)

type streamCollector struct{}

func (streamCollector) Endpoint() string { return zlmapi.EndpointGetMediaList }

func (streamCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{
		streamsInfo, streamStatus, streamReaderCount, streamTotalReaderCount,
		streamBitrate, streamAliveSecond, streamCreateStamp, streamTotal,
	} {
		ch <- d
	}
}

// Collect reports one series per schema. Streams sharing a name are the same
// source stream: ZLMediaKit republishes each source under several protocols
// (schemas) by default, so only the per-stream totals are de-duplicated.
func (streamCollector) Collect(ctx context.Context, client *zlmapi.Client, ch chan<- prometheus.Metric) error {
	data, err := zlmapi.Get[[]zlmapi.StreamInfo](ctx, client, zlmapi.EndpointGetMediaList)
	if err != nil {
		return err
	}

	uniqueStreamKeys := make(map[string]bool)
	for _, stream := range data {
		streamKey := fmt.Sprintf("%s_%s_%s", stream.Vhost, stream.App, stream.Stream)
		if !uniqueStreamKeys[streamKey] {
			ch <- prometheus.MustNewConstMetric(streamTotalReaderCount, prometheus.GaugeValue,
				float64(stream.TotalReaderCount),
				stream.Vhost, stream.App, stream.Stream)
			uniqueStreamKeys[streamKey] = true
		}

		ch <- prometheus.MustNewConstMetric(streamsInfo, prometheus.GaugeValue, 1,
			stream.Vhost, stream.App, stream.Stream, stream.Schema, stream.OriginTypeStr, stream.OriginURL)

		status := 0.0
		if stream.BytesSpeed > 0 {
			status = 1.0
		}
		ch <- prometheus.MustNewConstMetric(streamStatus, prometheus.GaugeValue, status,
			stream.Vhost, stream.App, stream.Stream, stream.Schema)

		ch <- prometheus.MustNewConstMetric(streamReaderCount, prometheus.GaugeValue,
			float64(stream.ReaderCount), stream.Vhost, stream.App, stream.Stream, stream.Schema)

		ch <- prometheus.MustNewConstMetric(streamBitrate, prometheus.GaugeValue,
			stream.BytesSpeed, stream.Vhost, stream.App, stream.Stream, stream.Schema)

		ch <- prometheus.MustNewConstMetric(streamAliveSecond, prometheus.GaugeValue,
			float64(stream.AliveSecond), stream.Vhost, stream.App, stream.Stream, stream.Schema)

		ch <- prometheus.MustNewConstMetric(streamCreateStamp, prometheus.GaugeValue,
			float64(stream.CreateStamp), stream.Vhost, stream.App, stream.Stream, stream.Schema)
	}

	ch <- prometheus.MustNewConstMetric(streamTotal, prometheus.GaugeValue, float64(len(uniqueStreamKeys)))
	return nil
}
