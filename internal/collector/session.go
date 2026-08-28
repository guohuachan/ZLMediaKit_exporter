package collector

import (
	"context"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/guohuachan/ZLMediaKit_exporter/internal/zlmapi"
)

var (
	sessionInfo = newMetricDescr(SubsystemSession, "info", "Session info",
		[]string{"id", "identifier", "local_ip", "local_port", "peer_ip", "peer_port", "typeid", "type"})
	sessionTotal = newMetricDescr(SubsystemSession, "total", "Total number of sessions", []string{})
)

type sessionCollector struct{}

func (sessionCollector) Endpoint() string { return zlmapi.EndpointGetAllSession }

func (sessionCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- sessionInfo
	ch <- sessionTotal
}

func (sessionCollector) Collect(ctx context.Context, client *zlmapi.Client, ch chan<- prometheus.Metric) error {
	data, err := zlmapi.Get[[]zlmapi.Session](ctx, client, zlmapi.EndpointGetAllSession)
	if err != nil {
		return err
	}

	for _, s := range data {
		ch <- prometheus.MustNewConstMetric(sessionInfo, prometheus.GaugeValue, 1,
			s.ID, s.Identifier, s.LocalIP, strconv.Itoa(s.LocalPort), s.PeerIP, strconv.Itoa(s.PeerPort), s.TypeID, s.Type)
	}
	ch <- prometheus.MustNewConstMetric(sessionTotal, prometheus.GaugeValue, float64(len(data)))
	return nil
}
