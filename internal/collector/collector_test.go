package collector

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guohuachan/ZLMediaKit_exporter/internal/zlmapi"
)

const testSecret = "test-secret"

// fixtureNames are the testdata/api files served by the mock ZLMediaKit server.
var fixtureNames = []string{
	"version",
	"getApiList",
	"getThreadsLoad",
	"getWorkThreadsLoad",
	"getStatistic",
	"getServerConfig",
	"getAllSession",
	"getMediaList",
	"listRtpServer",
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", "api", name+".json"))
	require.NoError(t, err)
	return body
}

// newMockZLMServer serves every fixture and rejects requests without the secret.
func newMockZLMServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	for _, name := range fixtureNames {
		payload := readFixture(t, name)
		mux.HandleFunc("/index/api/"+name, func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("secret") != testSecret {
				http.Error(w, "incorrect secret", http.StatusForbidden)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(payload)
		})
	}

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// newStaticServer answers every path with the same body.
func newStaticServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func gatherText(t *testing.T, reg prometheus.Gatherer) string {
	t.Helper()

	srv := httptest.NewServer(promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(body)
}

// TestExporterGolden pins the full exposition output produced against the
// fixtures. Run with UPDATE_GOLDEN=1 to rewrite the expectation.
func TestExporterGolden(t *testing.T) {
	srv := newMockZLMServer(t)

	exporter, err := New(srv.URL, testSecret, testLogger(), zlmapi.Options{})
	require.NoError(t, err)

	reg := prometheus.NewRegistry()
	reg.MustRegister(exporter)

	got := gatherText(t, reg)

	golden := filepath.Join("testdata", "expected_metrics.prom")
	if os.Getenv("UPDATE_GOLDEN") != "" {
		require.NoError(t, os.WriteFile(golden, []byte(got), 0o600))
	}

	want, err := os.ReadFile(golden)
	require.NoError(t, err)
	assert.Equal(t, string(want), got)
}

func TestExporterDescribe(t *testing.T) {
	exporter, err := New("http://localhost", testSecret, testLogger(), zlmapi.Options{})
	require.NoError(t, err)

	ch := make(chan *prometheus.Desc, 256)
	exporter.Describe(ch)
	close(ch)

	described := make(map[string]bool)
	for desc := range ch {
		described[desc.String()] = true
	}

	for _, desc := range []*prometheus.Desc{
		zlmediaKitInfo, apiStatus,
		networkThreadsTotal, networkThreadsLoadTotal, networkThreadsDelayTotal,
		workThreadsTotal, workThreadsLoadTotal, workThreadsDelayTotal,
		statisticsBuffer, statisticsUdpSession,
		sessionInfo, sessionTotal,
		streamsInfo, streamStatus, streamReaderCount, streamTotalReaderCount,
		streamBitrate, streamAliveSecond, streamCreateStamp, streamTotal,
		rtpServerInfo, rtpServerTotal,
	} {
		assert.True(t, described[desc.String()], "missing metric description: %s", desc)
	}

	assert.True(t, described[exporter.up.Desc().String()], "missing up metric description")
	assert.True(t, described[exporter.totalScrapes.Desc().String()], "missing totalScrapes metric description")
}

// TestExporterReportsUp documents that zlm_up reflects whether the scrape ran
// to completion, not whether every endpoint answered successfully.
func TestExporterReportsUp(t *testing.T) {
	tests := []struct {
		name   string
		server func(*testing.T) *httptest.Server
		wantUp string
	}{
		{
			name:   "healthy server",
			server: newMockZLMServer,
			wantUp: "zlm_up 1",
		},
		{
			name:   "every endpoint failing",
			server: func(t *testing.T) *httptest.Server { return newStaticServer(t, `{"code":1,"msg":"error"}`) },
			wantUp: "zlm_up 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := tt.server(t)

			exporter, err := New(srv.URL, testSecret, testLogger(), zlmapi.Options{})
			require.NoError(t, err)

			reg := prometheus.NewRegistry()
			reg.MustRegister(exporter)

			assert.Contains(t, gatherText(t, reg), tt.wantUp)
		})
	}
}

// TestCollectorsRejectBadResponses covers every collector against the response
// shapes ZLMediaKit can return on failure.
func TestCollectorsRejectBadResponses(t *testing.T) {
	bodies := map[string]string{
		"non-zero code":      `{"code":1,"msg":"error"}`,
		"invalid json":       `invalid json`,
		"data type mismatch": `{"code":0,"msg":"success","data":"invalid"}`,
	}

	exporter, err := New("http://localhost", testSecret, testLogger(), zlmapi.Options{})
	require.NoError(t, err)

	for _, c := range exporter.collectors {
		for name, body := range bodies {
			t.Run(c.Endpoint()+"/"+name, func(t *testing.T) {
				srv := newStaticServer(t, body)

				client, err := zlmapi.NewClient(srv.URL, testSecret, zlmapi.Options{})
				require.NoError(t, err)

				ch := make(chan prometheus.Metric, 64)
				err = c.Collect(context.Background(), client, ch)
				close(ch)

				assert.Error(t, err)
				assert.Empty(t, collectAll(ch), "a failing collector must not emit metrics")
			})
		}
	}
}

func TestScrapeCountsErrorsPerEndpoint(t *testing.T) {
	srv := newStaticServer(t, `{"code":1,"msg":"error"}`)

	exporter, err := New(srv.URL, testSecret, testLogger(), zlmapi.Options{})
	require.NoError(t, err)

	ch := make(chan prometheus.Metric, 256)
	go func() {
		exporter.Collect(ch)
		close(ch)
	}()
	collectAll(ch)

	for _, c := range exporter.collectors {
		assert.Equal(t, 1.0, testutil.ToFloat64(exporter.scrapeErrors.WithLabelValues(c.Endpoint())),
			"endpoint %s should have recorded one scrape error", c.Endpoint())
	}
}

func TestMustNewConstMetric(t *testing.T) {
	desc := prometheus.NewDesc("test_metric", "Test metric", []string{"label"}, nil)

	tests := []struct {
		name        string
		value       interface{}
		shouldBeNil bool
	}{
		{name: "float64", value: float64(123.45)},
		{name: "numeric string", value: "123.45"},
		{name: "non-numeric string", value: "abc"},
		{name: "unsupported type", value: struct{}{}, shouldBeNil: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := mustNewConstMetric(desc, prometheus.GaugeValue, tt.value, "test_label")
			if tt.shouldBeNil {
				assert.Nil(t, metric)
				return
			}
			assert.NotNil(t, metric)
		})
	}
}

// TestGoldenHasNoDuplicateHelp guards against two collectors registering the
// same metric name with different help text.
func TestGoldenHasNoDuplicateHelp(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "expected_metrics.prom"))
	require.NoError(t, err)

	seen := make(map[string]bool)
	for _, line := range strings.Split(string(body), "\n") {
		if !strings.HasPrefix(line, "# HELP ") {
			continue
		}
		name := strings.Fields(line)[2]
		assert.False(t, seen[name], "duplicate HELP for %s", name)
		seen[name] = true
	}
}

func collectAll(ch <-chan prometheus.Metric) []prometheus.Metric {
	var out []prometheus.Metric
	for m := range ch {
		out = append(out, m)
	}
	return out
}
