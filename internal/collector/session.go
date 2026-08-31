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
	sessions = newMetricDescr("", "sessions", "Number of sessions, by session class and transport", []string{"typeid", "type"})
)

// sessionCollector reports the session count grouped by class and transport.
// The per-connection detail is one time series per connection, so it is only
// emitted when explicitly asked for.
type sessionCollector struct {
	exposeInfo bool
}

func (sessionCollector) Name() string     { return "session" }
func (sessionCollector) Endpoint() string { return zlmapi.EndpointGetAllSession }

func (c sessionCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- sessions
	if c.exposeInfo {
		ch <- sessionInfo
	}
}

func (c sessionCollector) Collect(ctx context.Context, client *zlmapi.Client, ch chan<- prometheus.Metric) error {
	data, err := zlmapi.Get[[]zlmapi.Session](ctx, client, zlmapi.EndpointGetAllSession)
	if err != nil {
		return err
	}

	type sessionKey struct{ typeID, transport string }
	counts := make(map[sessionKey]int, len(data))

	for _, s := range data {
		counts[sessionKey{s.TypeID, s.Type}]++
		if c.exposeInfo {
			ch <- prometheus.MustNewConstMetric(sessionInfo, prometheus.GaugeValue, 1,
				s.ID, s.Identifier, s.LocalIP, strconv.Itoa(s.LocalPort),
				s.PeerIP, strconv.Itoa(s.PeerPort), s.TypeID, s.Type)
		}
	}

	for key, count := range counts {
		ch <- prometheus.MustNewConstMetric(sessions, prometheus.GaugeValue, float64(count), key.typeID, key.transport)
	}
	return nil
}
