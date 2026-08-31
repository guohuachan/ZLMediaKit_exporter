package collector

import (
	"context"
	"fmt"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/guohuachan/ZLMediaKit_exporter/internal/zlmapi"
)

var (
	streamsInfo = newMetricDescr(SubsystemStream, "info", "Stream basic information",
		[]string{"vhost", "app", "stream", "schema", "origin_type", "origin_url"})
	streamStatus = newMetricDescr(SubsystemStream, "status", "Stream status (1: active with data flowing, 0: inactive)",
		[]string{"vhost", "app", "stream", "schema"})
	streamReaders = newMetricDescr(SubsystemStream, "readers", "Number of readers of the stream",
		[]string{"vhost", "app", "stream", "schema"})
	streamTotalReaders = newMetricDescr(SubsystemStream, "total_readers", "Number of readers of the stream across all schemas",
		[]string{"vhost", "app", "stream"})
	streamBytesPerSecond = newMetricDescr(SubsystemStream, "bytes_per_second", "Current stream throughput in bytes per second",
		[]string{"vhost", "app", "stream", "schema"})
	streamAliveSeconds = newMetricDescr(SubsystemStream, "alive_seconds", "Seconds the stream has been alive",
		[]string{"vhost", "app", "stream", "schema"})
	streamCreateTime = newMetricDescr(SubsystemStream, "create_time_seconds", "Unix timestamp at which the stream was created",
		[]string{"vhost", "app", "stream", "schema"})
	streamBytesTotal = newMetricDescr(SubsystemStream, "bytes_total", "Total bytes transferred for the stream",
		[]string{"vhost", "app", "stream", "schema"})
	streamRecording = newMetricDescr(SubsystemStream, "recording",
		"Whether the stream is being recorded (1: recording, 0: not)",
		[]string{"vhost", "app", "stream", "type"})
	streams = newMetricDescr("", "streams", "Number of source streams", nil)
)

// trackLabels identify one elementary stream of a media source.
var trackLabels = []string{"vhost", "app", "stream", "codec_type", "codec_name", "track_index"}

var (
	streamTrackReady      = newMetricDescr(SubsystemStream, "track_ready", "Whether the track is ready (1: ready, 0: not)", trackLabels)
	streamTrackFrames     = newMetricDescr(SubsystemStream, "track_frames_total", "Frames carried by the track", trackLabels)
	streamTrackDuration   = newMetricDescr(SubsystemStream, "track_duration_seconds", "Track duration in seconds", trackLabels)
	streamTrackFPS        = newMetricDescr(SubsystemStream, "track_fps", "Video track frames per second", trackLabels)
	streamTrackWidth      = newMetricDescr(SubsystemStream, "track_width", "Video track width in pixels", trackLabels)
	streamTrackHeight     = newMetricDescr(SubsystemStream, "track_height", "Video track height in pixels", trackLabels)
	streamTrackGopSize    = newMetricDescr(SubsystemStream, "track_gop_size", "Video track GOP size in frames", trackLabels)
	streamTrackGopSeconds = newMetricDescr(SubsystemStream, "track_gop_interval_seconds", "Video track GOP interval in seconds", trackLabels)
	streamTrackKeyFrames  = newMetricDescr(SubsystemStream, "track_key_frames_total", "Video track key frames", trackLabels)
	streamTrackSampleRate = newMetricDescr(SubsystemStream, "track_sample_rate", "Audio track sample rate in hertz", trackLabels)
	streamTrackChannels   = newMetricDescr(SubsystemStream, "track_channels", "Audio track channel count", trackLabels)
	streamTrackSampleBit  = newMetricDescr(SubsystemStream, "track_sample_bit", "Audio track sample bit depth", trackLabels)
)

// trackTypeName renders ZLMediaKit's numeric TrackType as a label value.
func trackTypeName(codecType int) string {
	switch codecType {
	case zlmapi.TrackVideo:
		return "video"
	case zlmapi.TrackAudio:
		return "audio"
	case zlmapi.TrackTitle:
		return "title"
	default:
		return "unknown"
	}
}

// streamCollector reports one series per schema. Track metrics describe the
// source stream, so they are reported once per stream rather than per schema.
type streamCollector struct {
	exposeTracks bool
}

func (streamCollector) Name() string     { return "stream" }
func (streamCollector) Endpoint() string { return zlmapi.EndpointGetMediaList }

func (c streamCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{
		streamsInfo, streamStatus, streamReaders, streamTotalReaders,
		streamBytesPerSecond, streamAliveSeconds, streamCreateTime, streamBytesTotal,
		streamRecording, streams,
	} {
		ch <- d
	}

	if !c.exposeTracks {
		return
	}
	for _, d := range []*prometheus.Desc{
		streamTrackReady, streamTrackFrames, streamTrackDuration,
		streamTrackFPS, streamTrackWidth, streamTrackHeight,
		streamTrackGopSize, streamTrackGopSeconds, streamTrackKeyFrames,
		streamTrackSampleRate, streamTrackChannels, streamTrackSampleBit,
	} {
		ch <- d
	}
}

// Collect reports one series per schema. Streams sharing a name are the same
// source stream: ZLMediaKit republishes each source under several protocols
// (schemas) by default, so only the per-stream totals are de-duplicated.
func (c streamCollector) Collect(ctx context.Context, client *zlmapi.Client, ch chan<- prometheus.Metric) error {
	data, err := zlmapi.Get[[]zlmapi.StreamInfo](ctx, client, zlmapi.EndpointGetMediaList)
	if err != nil {
		return err
	}

	uniqueStreamKeys := make(map[string]bool)
	for _, stream := range data {
		streamKey := fmt.Sprintf("%s_%s_%s", stream.Vhost, stream.App, stream.Stream)
		if !uniqueStreamKeys[streamKey] {
			ch <- prometheus.MustNewConstMetric(streamTotalReaders, prometheus.GaugeValue,
				float64(stream.TotalReaderCount),
				stream.Vhost, stream.App, stream.Stream)
			// Recording state and tracks describe the source stream, not the
			// schema it is republished under, so they are reported once.
			emitRecording(ch, stream)
			if c.exposeTracks {
				emitTracks(ch, stream)
			}

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

		ch <- prometheus.MustNewConstMetric(streamReaders, prometheus.GaugeValue,
			float64(stream.ReaderCount), stream.Vhost, stream.App, stream.Stream, stream.Schema)

		ch <- prometheus.MustNewConstMetric(streamBytesPerSecond, prometheus.GaugeValue,
			stream.BytesSpeed, stream.Vhost, stream.App, stream.Stream, stream.Schema)

		ch <- prometheus.MustNewConstMetric(streamAliveSeconds, prometheus.GaugeValue,
			float64(stream.AliveSecond), stream.Vhost, stream.App, stream.Stream, stream.Schema)

		ch <- prometheus.MustNewConstMetric(streamCreateTime, prometheus.GaugeValue,
			float64(stream.CreateStamp), stream.Vhost, stream.App, stream.Stream, stream.Schema)

		ch <- prometheus.MustNewConstMetric(streamBytesTotal, prometheus.CounterValue,
			stream.TotalBytes, stream.Vhost, stream.App, stream.Stream, stream.Schema)
	}

	ch <- prometheus.MustNewConstMetric(streams, prometheus.GaugeValue, float64(len(uniqueStreamKeys)))
	return nil
}

func emitRecording(ch chan<- prometheus.Metric, stream zlmapi.StreamInfo) {
	for _, r := range []struct {
		kind   string
		active bool
	}{
		{"mp4", stream.IsRecordingMP4},
		{"hls", stream.IsRecordingHLS},
	} {
		value := 0.0
		if r.active {
			value = 1.0
		}
		ch <- prometheus.MustNewConstMetric(streamRecording, prometheus.GaugeValue, value,
			stream.Vhost, stream.App, stream.Stream, r.kind)
	}
}

func emitTracks(ch chan<- prometheus.Metric, stream zlmapi.StreamInfo) {
	for i, track := range stream.Tracks {
		labels := []string{
			stream.Vhost, stream.App, stream.Stream,
			trackTypeName(track.CodecType), track.CodecName, strconv.Itoa(i),
		}

		ready := 0.0
		if track.Ready {
			ready = 1.0
		}
		ch <- prometheus.MustNewConstMetric(streamTrackReady, prometheus.GaugeValue, ready, labels...)
		ch <- prometheus.MustNewConstMetric(streamTrackFrames, prometheus.CounterValue, track.Frames, labels...)
		ch <- prometheus.MustNewConstMetric(streamTrackDuration, prometheus.GaugeValue, millisecondsToSeconds(track.Duration), labels...)

		switch track.CodecType {
		case zlmapi.TrackVideo:
			ch <- prometheus.MustNewConstMetric(streamTrackFPS, prometheus.GaugeValue, track.FPS, labels...)
			ch <- prometheus.MustNewConstMetric(streamTrackWidth, prometheus.GaugeValue, track.Width, labels...)
			ch <- prometheus.MustNewConstMetric(streamTrackHeight, prometheus.GaugeValue, track.Height, labels...)
			ch <- prometheus.MustNewConstMetric(streamTrackGopSize, prometheus.GaugeValue, track.GopSize, labels...)
			ch <- prometheus.MustNewConstMetric(streamTrackGopSeconds, prometheus.GaugeValue, millisecondsToSeconds(track.GopIntervalMS), labels...)
			ch <- prometheus.MustNewConstMetric(streamTrackKeyFrames, prometheus.CounterValue, track.KeyFrames, labels...)
		case zlmapi.TrackAudio:
			ch <- prometheus.MustNewConstMetric(streamTrackSampleRate, prometheus.GaugeValue, track.SampleRate, labels...)
			ch <- prometheus.MustNewConstMetric(streamTrackChannels, prometheus.GaugeValue, track.Channels, labels...)
			ch <- prometheus.MustNewConstMetric(streamTrackSampleBit, prometheus.GaugeValue, track.SampleBit, labels...)
		}
	}
}
