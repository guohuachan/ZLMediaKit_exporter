// Command zlm_exporter exports ZLMediaKit metrics for Prometheus.
//
// See https://prometheus.io/docs/instrumenting/writing_exporters/
package main

import (
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	versioncollector "github.com/prometheus/client_golang/prometheus/collectors/version"
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
	zlmTLSInsecureSkipVerify = kingpin.Flag("zlm.tls-insecure-skip-verify",
		"Do not verify the ZlMediaKit TLS certificate (default false).").
		Default(getEnv("ZLM_EXPORTER_TLS_INSECURE_SKIP_VERIFY", "false")).Bool()

	zlmExposeAPIStatus = kingpin.Flag("zlm.expose-api-status",
		"Expose zlm_api_status, one constant series per ZlMediaKit API endpoint (default false).").
		Default(getEnv("ZLM_EXPORTER_EXPOSE_API_STATUS", "false")).Bool()
	zlmExposeSessionInfo = kingpin.Flag("zlm.expose-session-info",
		"Expose zlm_session_info, one series per connection (default false). "+
			"zlm_sessions carries the aggregate either way.").
		Default(getEnv("ZLM_EXPORTER_EXPOSE_SESSION_INFO", "false")).Bool()
	zlmExposeStreamTracks = kingpin.Flag("zlm.expose-stream-tracks",
		"Expose per-track stream metrics such as fps and resolution (default true).").
		Default(getEnv("ZLM_EXPORTER_EXPOSE_STREAM_TRACKS", "true")).Bool()
	zlmDisableCollectors = kingpin.Flag("zlm.disable-collectors",
		"Comma-separated collectors to skip entirely, e.g. stream_proxy,stream_pusher.").
		Default(getEnv("ZLM_EXPORTER_DISABLE_COLLECTORS", "")).String()
)

// splitCollectors parses the comma-separated --zlm.disable-collectors value.
func splitCollectors(value string) []string {
	var names []string
	for _, name := range strings.Split(value, ",") {
		if name = strings.TrimSpace(name); name != "" {
			names = append(names, name)
		}
	}
	return names
}

// logBuildInfo records the exporter build and runtime details as structured
// fields. slog takes alternating key/value pairs, not a printf format string.
func logBuildInfo(logger *slog.Logger) {
	logger.Info("ZLMediaKit metrics exporter",
		"version", BuildVersion,
		"build_date", BuildDate,
		"commit_sha", BuildCommitSha,
		"go_version", runtime.Version(),
		"goos", runtime.GOOS,
		"goarch", runtime.GOARCH,
	)
}

func main() {
	kingpin.Version(version.Print("zlm_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	logger := promslog.New(&promslog.Config{})

	if *zlmApiSecret == "" {
		*zlmApiSecret = getEnv("ZLM_API_SECRET", "")
	}

	logBuildInfo(logger)

	disabledCollectors := splitCollectors(*zlmDisableCollectors)

	logger.Info("configuration",
		"timeout", *webTimeout,
		"tls_insecure_skip_verify", *zlmTLSInsecureSkipVerify,
		"zlm_api_url", *zlmApiURL,
		"zlm_api_secret", maskSecret(*zlmApiSecret),
		"metrics_path", *metricsPath,
		"metrics_only", *metricOnly,
		"expose_api_status", *zlmExposeAPIStatus,
		"expose_session_info", *zlmExposeSessionInfo,
		"expose_stream_tracks", *zlmExposeStreamTracks,
		"disabled_collectors", disabledCollectors)

	exporter, err := collector.New(*zlmApiURL, *zlmApiSecret, logger, zlmapi.Options{
		Timeout:            *webTimeout,
		InsecureSkipVerify: *zlmTLSInsecureSkipVerify,
	}, collector.Config{
		ExposeAPIStatus:     *zlmExposeAPIStatus,
		ExposeSessionInfo:   *zlmExposeSessionInfo,
		DisableStreamTracks: !*zlmExposeStreamTracks,
		DisabledCollectors:  disabledCollectors,
	})
	if err != nil {
		logger.Error("failed to create new exporter", "error", err)
		os.Exit(1)
	}

	registry := prometheus.NewRegistry()
	if !*metricOnly {
		registry = prometheus.DefaultRegisterer.(*prometheus.Registry)
	}
	registry.MustRegister(exporter, versioncollector.NewCollector("zlm_exporter"))
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
