package collector

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guohuachan/ZLMediaKit_exporter/internal/zlmapi"
)

const testSecret = "test-secret"

// allCollectorsConfig enables everything, so tests that sweep the collector
// set cover the opt-in ones too.
var allCollectorsConfig = Config{ExposeAPIStatus: true, ExposeSessionInfo: true}

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
	"listStreamProxy",
	"listStreamPusherProxy",
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// readFixture reads name from the first directory that provides it, so a
// partial fixture set can override the default one.
func readFixture(t *testing.T, name string, dirs ...string) []byte {
	t.Helper()

	for _, dir := range dirs {
		body, err := os.ReadFile(filepath.Join("testdata", dir, name+".json"))
		if err == nil {
			return body
		}
		require.ErrorIs(t, err, os.ErrNotExist)
	}
	t.Fatalf("fixture %q not found in %v", name, dirs)
	return nil
}

// newMockZLMServer serves the current ZLMediaKit fixtures and rejects requests
// without the secret.
func newMockZLMServer(t *testing.T) *httptest.Server {
	t.Helper()
	return newMockZLMServerFrom(t, "api")
}

// newLegacyMockZLMServer serves the response shapes of older ZLMediaKit builds,
// falling back to the current fixtures for endpoints that did not change.
func newLegacyMockZLMServer(t *testing.T) *httptest.Server {
	t.Helper()
	return newMockZLMServerFrom(t, "api-legacy", "api")
}

func newMockZLMServerFrom(t *testing.T, dirs ...string) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	for _, name := range fixtureNames {
		payload := readFixture(t, name, dirs...)
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

	exporter, err := New(srv.URL, testSecret, testLogger(), zlmapi.Options{}, Config{})
	require.NoError(t, err)

	reg := prometheus.NewRegistry()
	reg.MustRegister(exporter)

	got := normalizeVolatile(gatherText(t, reg))

	golden := filepath.Join("testdata", "expected_metrics.prom")
	if os.Getenv("UPDATE_GOLDEN") != "" {
		require.NoError(t, os.WriteFile(golden, []byte(got), 0o600))
	}

	want, err := os.ReadFile(golden)
	require.NoError(t, err)
	assert.Equal(t, string(want), got)
}

// scrapeDurationValue matches the timing value the golden cannot pin down.
var scrapeDurationValue = regexp.MustCompile(`(zlm_exporter_scrape_duration_seconds\{[^}]*\}) [0-9.e+-]+`)

// normalizeVolatile replaces measured durations with a placeholder so the
// golden file compares the metric set rather than the clock.
func normalizeVolatile(exposition string) string {
	return scrapeDurationValue.ReplaceAllString(exposition, "$1 <duration>")
}

func TestExporterDescribe(t *testing.T) {
	exporter, err := New("http://localhost", testSecret, testLogger(), zlmapi.Options{}, allCollectorsConfig)
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
		networkThreads, networkThreadLoad, networkThreadDelay,
		workThreads, workThreadLoad, workThreadDelay,
		statisticsBuffer, statisticsUdpSession,
		sessionInfo, sessions,
		streamsInfo, streamStatus, streamReaders, streamTotalReaders,
		streamBytesPerSecond, streamAliveSeconds, streamCreateTime, streams,
		rtpServerInfo, rtpServers,
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

			exporter, err := New(srv.URL, testSecret, testLogger(), zlmapi.Options{}, allCollectorsConfig)
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

	exporter, err := New("http://localhost", testSecret, testLogger(), zlmapi.Options{}, allCollectorsConfig)
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

	exporter, err := New(srv.URL, testSecret, testLogger(), zlmapi.Options{}, allCollectorsConfig)
	require.NoError(t, err)

	ch := make(chan prometheus.Metric, 256)
	go func() {
		exporter.Collect(ch)
		close(ch)
	}()
	collectAll(ch)

	for _, c := range exporter.collectors {
		assert.Equal(t, 1.0, testutil.ToFloat64(exporter.scrapeErrors.WithLabelValues(c.Name())),
			"collector %s should have recorded one scrape error", c.Name())
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
		{name: "unsupported type", value: struct{}{}},
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

// collectorAdapter exposes one Collector as a prometheus.Collector so the
// testutil comparison helpers can be used on it.
type collectorAdapter struct {
	collector Collector
	client    *zlmapi.Client
}

func (a collectorAdapter) Describe(ch chan<- *prometheus.Desc) { a.collector.Describe(ch) }

func (a collectorAdapter) Collect(ch chan<- prometheus.Metric) {
	_ = a.collector.Collect(context.Background(), a.client, ch)
}

func newAdapter(t *testing.T, c Collector, url string) collectorAdapter {
	t.Helper()
	client, err := zlmapi.NewClient(url, testSecret, zlmapi.Options{})
	require.NoError(t, err)
	return collectorAdapter{collector: c, client: client}
}

// Bug: the label values were passed as (app, stream, vhost) while the
// descriptor declares (vhost, app, stream), scrambling every series.
func TestStreamTotalReaderCountLabelsMatchDescriptor(t *testing.T) {
	srv := newMockZLMServer(t)

	const expected = `
# HELP zlm_stream_total_readers Number of readers of the stream across all schemas
# TYPE zlm_stream_total_readers gauge
zlm_stream_total_readers{app="live",stream="test",vhost="__defaultVhost__"} 0
`
	err := testutil.CollectAndCompare(
		newAdapter(t, streamCollector{exposeTracks: true}, srv.URL),
		strings.NewReader(expected),
		"zlm_stream_total_readers",
	)
	assert.NoError(t, err)
}

// stallingCollector runs past the scrape deadline without watching the
// context, the way a collector blocked inside a slow syscall would.
type stallingCollector struct {
	delay    time.Duration
	started  atomic.Int64
	finished atomic.Int64
}

func (*stallingCollector) Name() string                     { return "stalling" }
func (*stallingCollector) Endpoint() string                 { return "test/stalling" }
func (*stallingCollector) Describe(chan<- *prometheus.Desc) {}

func (c *stallingCollector) Collect(context.Context, *zlmapi.Client, chan<- prometheus.Metric) error {
	c.started.Add(1)
	time.Sleep(c.delay)
	c.finished.Add(1)
	return nil
}

// Bug: on timeout scrape() returned while its goroutines were still running,
// so a late send raced with the closing of the metric channel.
func TestCollectWaitsForEveryCollector(t *testing.T) {
	exporter, err := New("http://127.0.0.1:1", testSecret, testLogger(), zlmapi.Options{}, allCollectorsConfig)
	require.NoError(t, err)

	stalling := &stallingCollector{delay: 300 * time.Millisecond}
	exporter.collectors = []Collector{stalling}
	exporter.scrapeTimeout = 50 * time.Millisecond

	ch := make(chan prometheus.Metric, 16)
	exporter.Collect(ch)
	close(ch)

	assert.Equal(t, stalling.started.Load(), stalling.finished.Load(),
		"Collect returned while a collector was still running")
}

// gatherCollectorText renders one collector's output as exposition text.
func gatherCollectorText(t *testing.T, c Collector, url string) string {
	t.Helper()

	reg := prometheus.NewRegistry()
	reg.MustRegister(newAdapter(t, c, url))
	return gatherText(t, reg)
}

// ZLMediaKit master returns listRtpServer's port as a number and adds the
// media tuple, ssrc and tcp mode; the old string-typed port broke decoding.
func TestRtpCollectorReadsMasterShape(t *testing.T) {
	out := gatherCollectorText(t, rtpCollector{}, newMockZLMServer(t).URL)

	assert.Contains(t, out, `zlm_rtp_server_info{app="rtp",port="10000",ssrc="1234567890",stream_id="test_rtp",tcp_mode="0",vhost="__defaultVhost__"} 1`)
	assert.Contains(t, out, `zlm_rtp_server_info{app="rtp",port="10002",ssrc="987654321",stream_id="other_rtp",tcp_mode="1",vhost="__defaultVhost__"} 1`)
	assert.Contains(t, out, "zlm_rtp_servers 2")
}

func TestStreamCollectorReportsTotalBytes(t *testing.T) {
	out := gatherCollectorText(t, streamCollector{exposeTracks: true}, newMockZLMServer(t).URL)

	assert.Contains(t, out, `zlm_stream_bytes_total{app="live",schema="ts",stream="test",vhost="__defaultVhost__"} 200368`)
	assert.Contains(t, out, `zlm_stream_bytes_total{app="live",schema="rtmp",stream="test",vhost="__defaultVhost__"} 144761`)
}

// Recording state belongs to the source stream, not to a schema, so it is
// reported once per stream.
func TestStreamCollectorReportsRecordingState(t *testing.T) {
	out := gatherCollectorText(t, streamCollector{exposeTracks: true}, newMockZLMServer(t).URL)

	assert.Contains(t, out, `zlm_stream_recording{app="live",stream="test",type="hls",vhost="__defaultVhost__"} 1`)
	assert.Contains(t, out, `zlm_stream_recording{app="live",stream="test",type="mp4",vhost="__defaultVhost__"} 0`)
}

// Tracks describe the source stream, so they must not be multiplied by the
// number of schemas the stream is republished under.
func TestStreamCollectorReportsTracksOncePerStream(t *testing.T) {
	out := gatherCollectorText(t, streamCollector{exposeTracks: true}, newMockZLMServer(t).URL)

	assert.Contains(t, out, `zlm_stream_track_fps{app="live",codec_name="H264",codec_type="video",stream="test",track_index="1",vhost="__defaultVhost__"} 26`)
	assert.Contains(t, out, `zlm_stream_track_width{app="live",codec_name="H264",codec_type="video",stream="test",track_index="1",vhost="__defaultVhost__"} 448`)
	assert.Contains(t, out, `zlm_stream_track_height{app="live",codec_name="H264",codec_type="video",stream="test",track_index="1",vhost="__defaultVhost__"} 960`)
	assert.Contains(t, out, `zlm_stream_track_gop_size{app="live",codec_name="H264",codec_type="video",stream="test",track_index="1",vhost="__defaultVhost__"} 21`)
	assert.Contains(t, out, `zlm_stream_track_sample_rate{app="live",codec_name="mpeg4-generic",codec_type="audio",stream="test",track_index="0",vhost="__defaultVhost__"} 44100`)
	assert.Contains(t, out, `zlm_stream_track_channels{app="live",codec_name="mpeg4-generic",codec_type="audio",stream="test",track_index="0",vhost="__defaultVhost__"} 1`)

	assert.Equal(t, 1, strings.Count(out, "zlm_stream_track_fps{"), "tracks must be reported once per stream, not per schema")
}

func TestSessionCollectorReportsTransportType(t *testing.T) {
	out := gatherCollectorText(t, sessionCollector{exposeInfo: true}, newMockZLMServer(t).URL)

	assert.Contains(t, out, `type="tcp"`)
}

func TestStreamProxyCollector(t *testing.T) {
	out := gatherCollectorText(t, streamProxyCollector{}, newMockZLMServer(t).URL)

	assert.Contains(t, out, `zlm_stream_proxy_info{app="proxy",key="__defaultVhost__/proxy/camera1",status_str="success",stream="camera1",url="rtsp://192.168.1.10:554/live/ch0",vhost="__defaultVhost__"} 1`)
	assert.Contains(t, out, `zlm_stream_proxy_status{app="proxy",key="__defaultVhost__/proxy/camera1",stream="camera1",vhost="__defaultVhost__"} 0`)
	assert.Contains(t, out, `zlm_stream_proxy_live_seconds{app="proxy",key="__defaultVhost__/proxy/camera1",stream="camera1",vhost="__defaultVhost__"} 3600`)
	assert.Contains(t, out, `zlm_stream_proxy_repull_total{app="proxy",key="__defaultVhost__/proxy/camera1",stream="camera1",vhost="__defaultVhost__"} 2`)
	assert.Contains(t, out, `zlm_stream_proxy_bytes_per_second{app="proxy",key="__defaultVhost__/proxy/camera1",stream="camera1",vhost="__defaultVhost__"} 20480`)
	assert.Contains(t, out, `zlm_stream_proxy_bytes_total{app="proxy",key="__defaultVhost__/proxy/camera1",stream="camera1",vhost="__defaultVhost__"} 736280`)
	assert.Contains(t, out, "zlm_stream_proxies 1")
}

func TestStreamPusherProxyCollector(t *testing.T) {
	out := gatherCollectorText(t, streamPusherCollector{}, newMockZLMServer(t).URL)

	assert.Contains(t, out, `zlm_stream_pusher_info{app="live",key="__defaultVhost__/live/test",stream="test",url="rtmp://cdn.example.com/live/test",vhost="__defaultVhost__"} 1`)
	assert.Contains(t, out, `zlm_stream_pusher_republish_total{app="live",key="__defaultVhost__/live/test",stream="test",vhost="__defaultVhost__"} 1`)
	assert.Contains(t, out, `zlm_stream_pusher_bytes_total{app="live",key="__defaultVhost__/live/test",stream="test",vhost="__defaultVhost__"} 276480`)
	assert.Contains(t, out, "zlm_stream_pushers 1")
}

// Older ZLMediaKit builds omit the newer fields and quoted the RTP port.
// Scraping them must still succeed rather than fail to decode.
func TestCollectorsAcceptLegacyResponses(t *testing.T) {
	srv := newLegacyMockZLMServer(t)

	exporter, err := New(srv.URL, testSecret, testLogger(), zlmapi.Options{}, allCollectorsConfig)
	require.NoError(t, err)

	for _, c := range exporter.collectors {
		t.Run(c.Endpoint(), func(t *testing.T) {
			client, err := zlmapi.NewClient(srv.URL, testSecret, zlmapi.Options{})
			require.NoError(t, err)

			ch := make(chan prometheus.Metric, 256)
			err = c.Collect(context.Background(), client, ch)
			close(ch)
			collectAll(ch)

			assert.NoError(t, err)
		})
	}

	out := gatherCollectorText(t, rtpCollector{}, srv.URL)
	assert.Contains(t, out, `port="10000"`, "a string-typed port must still decode")
}

// TestMetricsFollowPrometheusNaming keeps the exposed names within the
// Prometheus conventions: base units, _total reserved for counters.
func TestMetricsFollowPrometheusNaming(t *testing.T) {
	srv := newMockZLMServer(t)

	exporter, err := New(srv.URL, testSecret, testLogger(), zlmapi.Options{}, allCollectorsConfig)
	require.NoError(t, err)

	problems, err := testutil.CollectAndLint(exporter)
	require.NoError(t, err)

	for _, p := range problems {
		t.Errorf("%s: %s", p.Metric, p.Text)
	}
}

func newTestExporter(t *testing.T, url string, cfg Config) *Exporter {
	t.Helper()
	exporter, err := New(url, testSecret, testLogger(), zlmapi.Options{}, cfg)
	require.NoError(t, err)
	return exporter
}

func gatherExporterText(t *testing.T, url string, cfg Config) string {
	t.Helper()

	reg := prometheus.NewRegistry()
	reg.MustRegister(newTestExporter(t, url, cfg))
	return gatherText(t, reg)
}

// zlm_api_status emits one constant series per ZLMediaKit endpoint (~70) and
// carries no signal, so it must be opt-in.
func TestAPIStatusIsOptIn(t *testing.T) {
	url := newMockZLMServer(t).URL

	assert.NotContains(t, gatherExporterText(t, url, Config{}), "zlm_api_status")
	assert.Contains(t, gatherExporterText(t, url, Config{ExposeAPIStatus: true}), "zlm_api_status")
}

// Per-session series are one time series per connection; the aggregate is
// always available, the detail is opt-in.
func TestSessionInfoIsOptInButAggregateIsAlways(t *testing.T) {
	url := newMockZLMServer(t).URL

	out := gatherExporterText(t, url, Config{})
	assert.NotContains(t, out, "zlm_session_info")
	assert.Contains(t, out, `zlm_sessions{type="tcp",typeid="mediakit::HttpSession"} 1`)

	assert.Contains(t, gatherExporterText(t, url, Config{ExposeSessionInfo: true}), "zlm_session_info")
}

func TestStreamTracksCanBeDisabled(t *testing.T) {
	url := newMockZLMServer(t).URL

	assert.Contains(t, gatherExporterText(t, url, Config{}), "zlm_stream_track_fps")
	assert.NotContains(t, gatherExporterText(t, url, Config{DisableStreamTracks: true}), "zlm_stream_track_fps")
}

func TestCollectorsCanBeDisabledByName(t *testing.T) {
	url := newMockZLMServer(t).URL

	out := gatherExporterText(t, url, Config{DisabledCollectors: []string{"stream_proxy", "stream_pusher"}})
	assert.NotContains(t, out, "zlm_stream_proxy")
	assert.NotContains(t, out, "zlm_stream_pusher")
	assert.Contains(t, out, "zlm_streams", "unrelated collectors must stay enabled")
}

func TestExporterReportsPerCollectorScrapeHealth(t *testing.T) {
	out := gatherExporterText(t, newMockZLMServer(t).URL, Config{})

	assert.Contains(t, out, `zlm_exporter_collector_success{collector="stream"} 1`)
	assert.Contains(t, out, `zlm_exporter_scrape_errors_total{collector="stream"} 0`)
	assert.Contains(t, out, `zlm_exporter_scrape_duration_seconds{collector="stream"}`)

	failing := gatherExporterText(t, newStaticServer(t, `{"code":1,"msg":"error"}`).URL, Config{})
	assert.Contains(t, failing, `zlm_exporter_collector_success{collector="stream"} 0`)
	assert.Contains(t, failing, `zlm_exporter_scrape_errors_total{collector="stream"} 1`)
}

// Thread load and delay are reported per thread; the old *_load_total and
// *_delay_total aggregates are recoverable with sum().
func TestThreadsReportedIndividually(t *testing.T) {
	out := gatherExporterText(t, newMockZLMServer(t).URL, Config{})

	assert.Contains(t, out, "zlm_network_threads 8")
	assert.Contains(t, out, "zlm_work_threads 8")
	assert.Contains(t, out, `zlm_work_thread_load_percent{index="3"} 100`)
	assert.Contains(t, out, `zlm_work_thread_delay_seconds{index="3"} 0.005`)
	assert.NotContains(t, out, "zlm_network_threads_load_total")
	assert.NotContains(t, out, "zlm_work_threads_delay_total")
}

func TestTrackTypeName(t *testing.T) {
	tests := []struct {
		name      string
		codecType int
		want      string
	}{
		{name: "video", codecType: zlmapi.TrackVideo, want: "video"},
		{name: "audio", codecType: zlmapi.TrackAudio, want: "audio"},
		{name: "title", codecType: zlmapi.TrackTitle, want: "title"},
		{name: "unknown positive", codecType: 99, want: "unknown"},
		{name: "invalid", codecType: -1, want: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, trackTypeName(tt.codecType))
		})
	}
}

func TestNewRejectsIncompleteConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		uri    string
		secret string
	}{
		{name: "empty uri", uri: "", secret: testSecret},
		{name: "empty secret", uri: "http://localhost", secret: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exporter, err := New(tt.uri, tt.secret, testLogger(), zlmapi.Options{}, Config{})
			assert.Error(t, err)
			assert.Nil(t, exporter)
		})
	}
}
