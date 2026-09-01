package collector

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/guohuachan/ZLMediaKit_exporter/internal/zlmapi"
)

var (
	statisticsBuffer                = newMetricDescr(SubsystemStatistics, "buffer", "Statistics buffer", []string{})
	statisticsBufferLikeString      = newMetricDescr(SubsystemStatistics, "buffer_like_string", "Statistics BufferLikeString", []string{})
	statisticsBufferList            = newMetricDescr(SubsystemStatistics, "buffer_list", "Statistics BufferList", []string{})
	statisticsBufferRaw             = newMetricDescr(SubsystemStatistics, "buffer_raw", "Statistics BufferRaw", []string{})
	statisticsFrame                 = newMetricDescr(SubsystemStatistics, "frame", "Statistics Frame", []string{})
	statisticsFrameImp              = newMetricDescr(SubsystemStatistics, "frame_imp", "Statistics FrameImp", []string{})
	statisticsMediaSource           = newMetricDescr(SubsystemStatistics, "media_source", "Statistics MediaSource", []string{})
	statisticsMultiMediaSourceMuxer = newMetricDescr(SubsystemStatistics, "multi_media_source_muxer", "Statistics MultiMediaSourceMuxer", []string{})
	statisticsRtmpPacket            = newMetricDescr(SubsystemStatistics, "rtmp_packet", "Statistics RtmpPacket", []string{})
	statisticsRtpPacket             = newMetricDescr(SubsystemStatistics, "rtp_packet", "Statistics RtpPacket", []string{})
	statisticsSocket                = newMetricDescr(SubsystemStatistics, "socket", "Statistics Socket", []string{})
	statisticsTcpClient             = newMetricDescr(SubsystemStatistics, "tcp_client", "Statistics TcpClient", []string{})
	statisticsTcpServer             = newMetricDescr(SubsystemStatistics, "tcp_server", "Statistics TcpServer", []string{})
	statisticsTcpSession            = newMetricDescr(SubsystemStatistics, "tcp_session", "Statistics TcpSession", []string{})
	statisticsUdpServer             = newMetricDescr(SubsystemStatistics, "udp_server", "Statistics UdpServer", []string{})
	statisticsUdpSession            = newMetricDescr(SubsystemStatistics, "udp_session", "Statistics UdpSession", []string{})
)

type statisticsCollector struct{}

func (statisticsCollector) Endpoint() string { return zlmapi.EndpointGetStatistic }

func (statisticsCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{
		statisticsBuffer, statisticsBufferLikeString, statisticsBufferList, statisticsBufferRaw,
		statisticsFrame, statisticsFrameImp, statisticsMediaSource, statisticsMultiMediaSourceMuxer,
		statisticsRtmpPacket, statisticsRtpPacket, statisticsSocket, statisticsTcpClient,
		statisticsTcpServer, statisticsTcpSession, statisticsUdpServer, statisticsUdpSession,
	} {
		ch <- d
	}
}

func (statisticsCollector) Collect(ctx context.Context, client *zlmapi.Client, ch chan<- prometheus.Metric) error {
	data, err := zlmapi.Get[zlmapi.Statistics](ctx, client, zlmapi.EndpointGetStatistic)
	if err != nil {
		return err
	}

	for _, m := range []struct {
		desc  *prometheus.Desc
		value float64
	}{
		{statisticsBuffer, data.Buffer},
		{statisticsBufferLikeString, data.BufferLikeString},
		{statisticsBufferList, data.BufferList},
		{statisticsBufferRaw, data.BufferRaw},
		{statisticsFrame, data.Frame},
		{statisticsFrameImp, data.FrameImp},
		{statisticsMediaSource, data.MediaSource},
		{statisticsMultiMediaSourceMuxer, data.MultiMediaSourceMuxer},
		{statisticsRtmpPacket, data.RtmpPacket},
		{statisticsRtpPacket, data.RtpPacket},
		{statisticsSocket, data.Socket},
		{statisticsTcpClient, data.TcpClient},
		{statisticsTcpServer, data.TcpServer},
		{statisticsTcpSession, data.TcpSession},
		{statisticsUdpServer, data.UdpServer},
		{statisticsUdpSession, data.UdpSession},
	} {
		ch <- mustNewConstMetric(m.desc, prometheus.GaugeValue, m.value)
	}
	return nil
}
