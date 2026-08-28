// Command zlm_exporter exports ZLMediaKit metrics for Prometheus.
//
// See https://prometheus.io/docs/instrumenting/writing_exporters/
package main

import (
	"net/http"
	"os"
	"runtime"
	"strconv"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promslog"
	"github.com/prometheus/common/version"
	promweb "github.com/prometheus/exporter-toolkit/web"
	webflag "github.com/prometheus/exporter-toolkit/web/kingpinflag"

	"github.com/guohuachan/ZLMediaKit_exporter/internal/collector"
	"github.com/guohuachan/ZLMediaKit_exporter/internal/zlmapi"
)

// BuildVersion, BuildDate and BuildCommitSha are filled in by the build script.
var (
	BuildVersion   = "<<< filled in by build >>>"
	BuildDate      = "<<< filled in by build >>>"
	BuildCommitSha = "<<< filled in by build >>>"
)

func getEnv(key string, defaultVal string) string {
	if envVal, ok := os.LookupEnv(key); ok {
		return envVal
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if envVal, ok := os.LookupEnv(key); ok {
		envBool, err := strconv.ParseBool(envVal)
		if err == nil {
			return envBool
		}
	}
	return defaultVal
}

// maskSecret keeps enough of the secret to recognise it in a log line without
// disclosing it.
func maskSecret(secret string) string {
	if len(secret) == 0 {
		return "<empty>"
	}
	if len(secret) <= 4 {
		return "****"
	}
	return secret[:2] + "****" + secret[len(secret)-2:]
}

var (
	webFlagConfig = webflag.AddFlags(kingpin.CommandLine, getEnv("ZLM_EXPORTER_TELEMETRY_ADDRESS", ":9101"))
	webTimeout    = kingpin.Flag("web.timeout", "Timeout for connection to ZlMediaKit instance (default 15s).").
			Default(getEnv("ZLM_EXPORTER_TIMEOUT", "15s")).Duration()
	webSSLVerify = kingpin.Flag("web.ssl-verify", "Enable SSL verification(default true).").
			Default(getEnv("ZLM_EXPORTER_SSL_VERIFY", "true")).Bool()

	metricsPath = kingpin.Flag("web.telemetry-path",
		"Path under which to expose metrics (default /metrics)").
		Default(getEnv("ZLM_EXPORTER_TELEMETRY_PATH", "/metrics")).String()
	metricOnly = kingpin.Flag("web.metric-only",
		"Only export metrics, not other key-value metrics(default true).").
		Default(getEnv("ZLM_EXPORTER_METRIC_ONLY", "true")).Bool()

	zlmApiURL = kingpin.Flag("zlm.api-url",
		"URI on which to scrape ZlMediaKit metrics(ZlMediaKit apiServer url).").
		Default(getEnv("ZLM_API_URL", "http://127.0.0.1")).String()
	zlmApiSecret = kingpin.Flag("zlm.secret", "Secret for the access ZlMediaKit api(from ZLM_API_SECRET env or CLI flag).").
			PlaceHolder("<secret>").String()
)

func main() {
	kingpin.Version(version.Print("zlm_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	logger := promslog.New(&promslog.Config{})

	if *zlmApiSecret == "" {
		*zlmApiSecret = getEnv("ZLM_API_SECRET", "")
	}

	logger.Info("ZLMediaKit Metrics Exporter %s    build date: %s    sha1: %s    Go: %s    GOOS: %s    GOARCH: %s",
		BuildVersion, BuildDate, BuildCommitSha,
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH,
	)

	logger.Info("Configuration")
	logger.Info("web configuration",
		"timeout", *webTimeout,
		"ssl_verify", *webSSLVerify,
		"zlm_api_url", *zlmApiURL,
		"zlm_api_secret", maskSecret(*zlmApiSecret),
		"metrics_path", *metricsPath,
		"metrics_only", *metricOnly)

	exporter, err := collector.New(*zlmApiURL, *zlmApiSecret, logger, zlmapi.Options{SSLVerify: *webSSLVerify})
	if err != nil {
		logger.Error("failed to create new exporter", "error", err)
		os.Exit(1)
	}

	registry := prometheus.NewRegistry()
	if !*metricOnly {
		registry = prometheus.DefaultRegisterer.(*prometheus.Registry)
	}
	registry.MustRegister(exporter)
	http.Handle(*metricsPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		Timeout: *webTimeout,
	}))
	svr := &http.Server{}

	logger.Info("zlm_exporter started successfully, metrics available at", "metrics_path", *metricsPath)
	if err := promweb.ListenAndServe(svr, webFlagConfig, logger); err != nil {
		logger.Error("Error starting HTTP server", "error", err)
		os.Exit(1)
	}
}
