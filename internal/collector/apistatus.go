package collector

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/guohuachan/ZLMediaKit_exporter/internal/zlmapi"
)

var apiStatus = newMetricDescr(SubsystemAPI, "status", "The status of API endpoint", []string{"endpoint"})

type apiStatusCollector struct{}

func (apiStatusCollector) Endpoint() string { return zlmapi.EndpointGetAPIList }

func (apiStatusCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- apiStatus
}

func (apiStatusCollector) Collect(ctx context.Context, client *zlmapi.Client, ch chan<- prometheus.Metric) error {
	data, err := zlmapi.Get[[]string](ctx, client, zlmapi.EndpointGetAPIList)
	if err != nil {
		return err
	}
	for _, endpoint := range data {
		ch <- prometheus.MustNewConstMetric(apiStatus, prometheus.GaugeValue, 1, endpoint)
	}
	return nil
}
