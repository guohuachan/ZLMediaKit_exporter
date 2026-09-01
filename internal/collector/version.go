package collector

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/guohuachan/ZLMediaKit_exporter/internal/zlmapi"
)

var zlmediaKitInfo = newMetricDescr(SubsystemVersion, "info", "ZLMediaKit version info.",
	[]string{"branchName", "buildTime", "commitHash"})

type versionCollector struct{}

func (versionCollector) Endpoint() string { return zlmapi.EndpointVersion }

func (versionCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- zlmediaKitInfo
}

func (versionCollector) Collect(ctx context.Context, client *zlmapi.Client, ch chan<- prometheus.Metric) error {
	data, err := zlmapi.Get[zlmapi.Version](ctx, client, zlmapi.EndpointVersion)
	if err != nil {
		return err
	}
	ch <- prometheus.MustNewConstMetric(zlmediaKitInfo, prometheus.GaugeValue, 1,
		data.BranchName, data.BuildTime, data.CommitHash)
	return nil
}
