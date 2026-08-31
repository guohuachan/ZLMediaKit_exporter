package collector

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/guohuachan/ZLMediaKit_exporter/internal/zlmapi"
)

var (
	rtpServerInfo = newMetricDescr(SubsystemRtp, "server_info", "RTP server info",
		[]string{"vhost", "app", "stream_id", "port", "ssrc", "tcp_mode"})
	rtpServers = newMetricDescr("", "rtp_servers", "Number of RTP servers", nil)
)

type rtpCollector struct{}

func (rtpCollector) Name() string     { return "rtp" }
func (rtpCollector) Endpoint() string { return zlmapi.EndpointListRtpServer }

func (rtpCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- rtpServerInfo
	ch <- rtpServers
}

func (rtpCollector) Collect(ctx context.Context, client *zlmapi.Client, ch chan<- prometheus.Metric) error {
	data, err := zlmapi.Get[[]zlmapi.RtpServer](ctx, client, zlmapi.EndpointListRtpServer)
	if err != nil {
		return err
	}

	for _, s := range data {
		ch <- prometheus.MustNewConstMetric(rtpServerInfo, prometheus.GaugeValue, 1,
			s.Vhost, s.App, s.StreamID, s.Port.String(), s.SSRC.String(), s.TCPMode.String())
	}
	ch <- prometheus.MustNewConstMetric(rtpServers, prometheus.GaugeValue, float64(len(data)))
	return nil
}
