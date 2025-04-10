package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/assert"

	"github.com/gin-gonic/gin"
)

// most of unittest powered by cursor
// WIP
var (
	MockZlmAPIServerAddr    = "http://localhost:9999"
	MockZlmAPIServerSecret  = "test-secret"
	MockZlmAPIServerHandler = gin.Default()
)

func setup() {
	setupZlmApiServer()
}

func setupZlmApiServer() {
	r := MockZlmAPIServerHandler
	r.GET("index/api/version", func(c *gin.Context) {
		c.JSON(http.StatusOK, readTestData("version"))
	})

	r.GET("index/api/getApiList", func(c *gin.Context) {
		c.JSON(http.StatusOK, readTestData("getApiList"))
	})

	r.GET("index/api/getThreadsLoad", func(c *gin.Context) {
		c.JSON(http.StatusOK, readTestData("getThreadsLoad"))
	})

	r.GET("index/api/getWorkThreadsLoad", func(c *gin.Context) {
		c.JSON(http.StatusOK, readTestData("getWorkThreadsLoad"))
	})

	r.GET("index/api/getStatistic", func(c *gin.Context) {
		c.JSON(http.StatusOK, readTestData("getStatistic"))
	})

	r.GET("index/api/getServerConfig", func(c *gin.Context) {
		c.JSON(http.StatusOK, readTestData("getServerConfig"))
	})

	r.GET("index/api/getAllSession", func(c *gin.Context) {
		c.JSON(http.StatusOK, readTestData("getAllSession"))
	})

	r.GET("index/api/getMediaList", func(c *gin.Context) {
		c.JSON(http.StatusOK, readTestData("getMediaList"))
	})

	r.GET("index/api/listRtpServer", func(c *gin.Context) {
		c.JSON(http.StatusOK, readTestData("listRtpServer"))
	})

	go func() {
		err := r.Run(":9999")
		if err != nil {
			log.Fatal(err)
		}
	}()
	startTime := time.Now()
	timeout := 5 * time.Second
	for {
		resp, err := http.Get(MockZlmAPIServerAddr + "/index/api/version")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			break
		}

		if time.Since(startTime) > timeout {
			log.Fatalf("Mock ZLM API Server未能在%s内启动", timeout)
		}

		time.Sleep(100 * time.Millisecond)
	}
}

func setupTestServer(t *testing.T, endpoint string, response interface{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-secret", r.Header.Get("secret"))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
}

func setupExporter(t *testing.T, server *httptest.Server) *Exporter {
	exporter, err := NewExporter(server.URL, MockZlmAPIServerSecret, promslog.New(&promslog.Config{}), Options{})
	assert.NoError(t, err)
	return exporter
}

func readTestData(name string) map[string]any {
	file, err := os.ReadFile(fmt.Sprintf("testdata/api/%s.json", name))
	if err != nil {
		log.Fatal(err)
	}
	var fileJson map[string]any
	err = json.Unmarshal(file, &fileJson)
	if err != nil {
		log.Println(err)
	}
	return fileJson
}

func teardown() {
	scrapeErrors.Reset()
}

func TestMetricsDescribe(t *testing.T) {
	tests := []struct {
		name          string
		metricsCount  int
		includeUpDesc bool
	}{
		{
			name:          "verify all metrics",
			metricsCount:  len(metrics) + 2,
			includeUpDesc: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exporter, err := NewExporter("http://localhost", MockZlmAPIServerSecret, promslog.New(&promslog.Config{}), Options{})
			assert.NoError(t, err)

			ch := make(chan *prometheus.Desc, tt.metricsCount)
			done := make(chan bool)

			go func() {
				exporter.Describe(ch)
				close(ch)
				done <- true
			}()

			descriptions := make([]*prometheus.Desc, 0)
			for desc := range ch {
				descriptions = append(descriptions, desc)
			}
			<-done

			assert.Equal(t, tt.metricsCount, len(descriptions), "metrics description count not match")

			descMap := make(map[string]bool)
			for _, desc := range descriptions {
				descMap[desc.String()] = true
			}

			for _, metric := range metrics {
				assert.True(t, descMap[metric.String()], "missing metric description: %s", metric.String())
			}

			assert.True(t, descMap[exporter.up.Desc().String()], "missing up metric description")
			assert.True(t, descMap[exporter.totalScrapes.Desc().String()], "missing totalScrapes metric description")

			keyMetrics := []struct {
				name     string
				desc     *prometheus.Desc
				expected bool
			}{
				{"ZLMediaKitInfo", ZLMediaKitInfo, true},
				{"ApiStatus", ApiStatus, true},
				{"NetworkThreadsTotal", NetworkThreadsTotal, true},
				{"StreamsInfo", StreamsInfo, true},
				{"SessionInfo", SessionInfo, true},
				{"RtpServerInfo", RtpServerInfo, true},
			}

			for _, km := range keyMetrics {
				assert.True(t, descMap[km.desc.String()], "missing key metric description: %s", km.name)
			}
			teardown()
		})
	}
}

func TestMetricsCollect(t *testing.T) {
	setup()
	tests := []struct {
		name         string
		metricsCount int
	}{
		{
			name: "verify all metrics",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exporter, err := NewExporter(MockZlmAPIServerAddr, MockZlmAPIServerSecret, promslog.New(&promslog.Config{}), Options{})
			assert.NoError(t, err)

			ch := make(chan prometheus.Metric, tt.metricsCount)
			done := make(chan bool)

			timeout := time.After(10 * time.Second)

			go func() {
				exporter.Collect(ch)
				close(ch)
				done <- true
			}()

			select {
			case <-done:
			case <-timeout:
				t.Log("scrape timeout")
			}

			metrics := make([]prometheus.Metric, 0)
			for metric := range ch {
				metrics = append(metrics, metric)
			}

			if len(metrics) > 0 {
				t.Logf("scrape %d metrics", len(metrics))
			} else {
				t.Error("no metric")
			}
		})
	}
}

func TestFetchHTTPErrorHandling(t *testing.T) {
	tests := []struct {
		name          string
		responseCode  int
		responseBody  string
		expectedError bool
	}{
		{
			name:          "success response",
			responseCode:  http.StatusOK,
			responseBody:  `{"code": 0, "msg": "success", "data": {}}`,
			expectedError: false,
		},
		{
			name:          "invalid json response",
			responseCode:  http.StatusOK,
			responseBody:  `invalid json`,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.responseCode)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			options := Options{
				SSLVerify: true,
			}
			exporter, err := NewExporter(server.URL, MockZlmAPIServerSecret, promslog.New(&promslog.Config{}), options)
			assert.NoError(t, err)

			ch := make(chan prometheus.Metric, 1)
			endpoint := "test/endpoint"

			processFunc := func(closer io.ReadCloser) error {
				var result map[string]interface{}
				if err := json.NewDecoder(closer).Decode(&result); err != nil {
					return err
				}
				return nil
			}

			exporter.fetchHTTP(context.Background(), ch, endpoint, processFunc)

			errorCount := testutil.ToFloat64(scrapeErrors.WithLabelValues(endpoint))
			if tt.expectedError {
				assert.Greater(t, errorCount, float64(0), "expected error but not recorded")
			} else {
				assert.Equal(t, float64(0), errorCount, "unexpected error recorded")
			}
		})
	}
}

func TestMetricsRegistration(t *testing.T) {
	if ZLMediaKitInfo == nil {
		t.Error("ZLMediaKitInfo metric not initialized")
	}
	if ApiStatus == nil {
		t.Error("ApiStatus metric not initialized")
	}
	if NetworkThreadsTotal == nil {
		t.Error("NetworkThreadsTotal metric not initialized")
	}
	if NetworkThreadsLoadTotal == nil {
		t.Error("NetworkThreadsLoadTotal metric not initialized")
	}
	if WorkThreadsTotal == nil {
		t.Error("WorkThreadsTotal metric not initialized")
	}
	if WorkThreadsLoadTotal == nil {
		t.Error("WorkThreadsLoadTotal metric not initialized")
	}
	if StatisticsBuffer == nil {
		t.Error("StatisticsBuffer metric not initialized")
	}
	if StatisticsBufferLikeString == nil {
		t.Error("StatisticsBufferLikeString metric not initialized")
	}
	if StatisticsBufferList == nil {
		t.Error("StatisticsBufferList metric not initialized")
	}
	if StatisticsBufferRaw == nil {
		t.Error("StatisticsBufferRaw metric not initialized")
	}
	if StatisticsFrame == nil {
		t.Error("StatisticsFrame metric not initialized")
	}
	if StatisticsFrameImp == nil {
		t.Error("StatisticsFrameImp metric not initialized")
	}
	if StatisticsMediaSource == nil {
		t.Error("StatisticsMediaSource metric not initialized")
	}
	if StatisticsMultiMediaSourceMuxer == nil {
		t.Error("StatisticsMultiMediaSourceMuxer metric not initialized")
	}
	if StatisticsRtmpPacket == nil {
		t.Error("StatisticsRtmpPacket metric not initialized")
	}
	if StatisticsRtpPacket == nil {
		t.Error("StatisticsRtpPacket metric not initialized")
	}
	if StatisticsSocket == nil {
		t.Error("StatisticsSocket metric not initialized")
	}
	if StatisticsTcpClient == nil {
		t.Error("StatisticsTcpClient metric not initialized")
	}
	if StatisticsTcpServer == nil {
		t.Error("StatisticsTcpServer metric not initialized")
	}
	if StatisticsTcpSession == nil {
		t.Error("StatisticsTcpSession metric not initialized")
	}
	if StatisticsUdpServer == nil {
		t.Error("StatisticsUdpServer metric not initialized")
	}
	if StatisticsUdpSession == nil {
		t.Error("StatisticsUdpSession metric not initialized")
	}
	if StreamBitrate == nil {
		t.Error("StreamBandwidths metric not initialized")
	}
	if StreamsInfo == nil {
		t.Error("StreamsInfo metric not initialized")
	}
	if StreamReaderCount == nil {
		t.Error("StreamReaderCount metric not initialized")
	}
	if StreamTotalReaderCount == nil {
		t.Error("StreamTotalReaderCount metric not initialized")
	}
	if StreamaliveSecond == nil {
		t.Error("StreamaliveSecond metric not initialized")
	}
	if StreamCreateStamp == nil {
		t.Error("StreamCreateStamp metric not initialized")
	}

}

func TestNewExporter(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		secret      string
		shouldError bool
	}{
		{
			name:        "有效的URI和Secret",
			uri:         "http://localhost:8080",
			secret:      MockZlmAPIServerSecret,
			shouldError: false,
		},
		{
			name:        "空URI",
			uri:         "",
			secret:      MockZlmAPIServerSecret,
			shouldError: true,
		},
		{
			name:        "空Secret",
			uri:         "http://localhost:8080",
			secret:      "",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exporter, err := NewExporter(tt.uri, tt.secret, promslog.New(&promslog.Config{}), Options{})
			if tt.shouldError {
				assert.Error(t, err)
				assert.Nil(t, exporter)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, exporter)
			}
		})
		teardown()
	}
}

func TestExtractVersion(t *testing.T) {
	tests := []struct {
		name                      string
		mockResponse              map[string]interface{}
		expectedMetricsCount      int
		expectedScrapeErrorsCount float64
	}{
		{
			name: "success",
			mockResponse: map[string]interface{}{
				"code": 0,
				"msg":  "success",
				"data": map[string]interface{}{
					"branchName": "master",
					"buildTime":  "20220101",
					"commitHash": "abc123",
				},
			},
			expectedMetricsCount:      1,
			expectedScrapeErrorsCount: 0,
		},
		{
			name: "error-code-not-0",
			mockResponse: map[string]interface{}{
				"code": 1,
				"msg":  "error",
			},
			expectedMetricsCount:      0,
			expectedScrapeErrorsCount: 1,
		},
		{
			name: "error-decode-json",
			mockResponse: map[string]interface{}{
				"code": 0,
				"msg":  "success",
				"data": map[string]interface{}{
					"branchName": "master",
					"buildTime":  20220101,
					"commitHash": "abc123",
				},
			},
			expectedMetricsCount:      0,
			expectedScrapeErrorsCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := setupTestServer(t, ZlmAPIEndpointVersion, tt.mockResponse)
			defer server.Close()

			exporter := setupExporter(t, server)

			ch := make(chan prometheus.Metric, tt.expectedMetricsCount)
			done := make(chan bool)

			go func() {
				exporter.extractVersion(context.Background(), ch)
				close(ch)
				done <- true
			}()

			metrics := make([]prometheus.Metric, 0)
			for metric := range ch {
				metrics = append(metrics, metric)
			}
			<-done
			assert.Equal(t, tt.expectedScrapeErrorsCount, testutil.ToFloat64(scrapeErrors.WithLabelValues(ZlmAPIEndpointVersion)))
			assert.Equal(t, tt.expectedMetricsCount, len(metrics))
			teardown()
		})
	}
}

func TestExtractAPIStatus(t *testing.T) {
	tests := []struct {
		name                      string
		mockResponse              map[string]interface{}
		expectedMetricsCount      int
		expectedScrapeErrorsCount float64
	}{
		{
			name: "success",
			mockResponse: map[string]interface{}{
				"code": 0,
				"msg":  "success",
				"data": []string{
					"index/api/version",
					"index/api/getApiList",
				},
			},
			expectedMetricsCount:      2,
			expectedScrapeErrorsCount: 0,
		},
		{
			name: "error-code-not-0",
			mockResponse: map[string]interface{}{
				"code": 1,
				"msg":  "error",
			},
			expectedMetricsCount:      0,
			expectedScrapeErrorsCount: 1,
		},
		{
			name: "error-invalid-data-type",
			mockResponse: map[string]interface{}{
				"code": 0,
				"msg":  "success",
				"data": map[string]interface{}{
					"invalid": "data",
				},
			},
			expectedMetricsCount:      0,
			expectedScrapeErrorsCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := setupTestServer(t, ZlmAPIEndpointGetApiList, tt.mockResponse)
			defer server.Close()

			exporter := setupExporter(t, server)

			ch := make(chan prometheus.Metric, tt.expectedMetricsCount)
			done := make(chan bool)

			go func() {
				exporter.extractAPIStatus(context.Background(), ch)
				close(ch)
				done <- true
			}()

			metrics := make([]prometheus.Metric, 0)
			for metric := range ch {
				metrics = append(metrics, metric)
			}
			<-done

			assert.Equal(t, tt.expectedScrapeErrorsCount, testutil.ToFloat64(scrapeErrors.WithLabelValues(ZlmAPIEndpointGetApiList)))
			assert.Equal(t, tt.expectedMetricsCount, len(metrics))
			teardown()
		})
	}
}

func TestExtractNetworkThreads(t *testing.T) {
	tests := []struct {
		name                      string
		mockResponse              map[string]interface{}
		expectedMetricsCount      int
		expectedScrapeErrorsCount float64
	}{
		{
			name: "success",
			mockResponse: map[string]interface{}{
				"code": 0,
				"msg":  "success",
				"data": []map[string]interface{}{
					{
						"load":  100,
						"delay": 100,
					},
					{
						"load":  200,
						"delay": 200,
					},
				},
			},
			expectedMetricsCount:      3, // NetworkThreadsTotal + 2个线程的负载数据
			expectedScrapeErrorsCount: 0,
		},
		{
			name: "error-code-not-0",
			mockResponse: map[string]interface{}{
				"code": 1,
				"msg":  "error",
			},
			expectedMetricsCount:      0,
			expectedScrapeErrorsCount: 1,
		},
		{
			name: "error-invalid-data-structure",
			mockResponse: map[string]interface{}{
				"code": 0,
				"msg":  "success",
				"data": "invalid",
			},
			expectedMetricsCount:      0,
			expectedScrapeErrorsCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := setupTestServer(t, ZlmAPIEndpointGetNetworkThreads, tt.mockResponse)
			defer server.Close()

			exporter := setupExporter(t, server)

			ch := make(chan prometheus.Metric, tt.expectedMetricsCount)
			done := make(chan bool)

			go func() {
				exporter.extractNetworkThreads(context.Background(), ch)
				close(ch)
				done <- true
			}()

			metrics := make([]prometheus.Metric, 0)
			for metric := range ch {
				metrics = append(metrics, metric)
			}
			<-done

			assert.Equal(t, tt.expectedScrapeErrorsCount, testutil.ToFloat64(scrapeErrors.WithLabelValues(ZlmAPIEndpointGetNetworkThreads)))
			assert.Equal(t, tt.expectedMetricsCount, len(metrics))
			teardown()
		})
	}
}

func TestExtractWorkThreads(t *testing.T) {
	tests := []struct {
		name                      string
		mockResponse              map[string]interface{}
		expectedMetricsCount      int
		expectedScrapeErrorsCount float64
	}{
		{
			name: "success",
			mockResponse: map[string]interface{}{
				"code": 0,
				"msg":  "success",
				"data": []map[string]interface{}{
					{
						"load":  100,
						"delay": 100,
					},
					{
						"load":  200,
						"delay": 200,
					},
				},
			},
			expectedMetricsCount:      3, // WorkThreadsTotal + 2个工作线程的负载数据
			expectedScrapeErrorsCount: 0,
		},
		{
			name: "error-code-not-0",
			mockResponse: map[string]interface{}{
				"code": 1,
				"msg":  "error",
			},
			expectedMetricsCount:      0,
			expectedScrapeErrorsCount: 1,
		},
		{
			name: "error-invalid-data-structure",
			mockResponse: map[string]interface{}{
				"code": 0,
				"msg":  "success",
				"data": "invalid",
			},
			expectedMetricsCount:      0,
			expectedScrapeErrorsCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := setupTestServer(t, ZlmAPIEndpointGetWorkThreads, tt.mockResponse)
			defer server.Close()

			exporter := setupExporter(t, server)

			ch := make(chan prometheus.Metric, tt.expectedMetricsCount)
			done := make(chan bool)

			go func() {
				exporter.extractWorkThreads(context.Background(), ch)
				close(ch)
				done <- true
			}()

			metrics := make([]prometheus.Metric, 0)
			for metric := range ch {
				metrics = append(metrics, metric)
			}
			<-done

			assert.Equal(t, tt.expectedScrapeErrorsCount, testutil.ToFloat64(scrapeErrors.WithLabelValues(ZlmAPIEndpointGetWorkThreads)))
			assert.Equal(t, tt.expectedMetricsCount, len(metrics))
			scrapeErrors.Reset()
		})
	}
}

func TestExtractStatistics(t *testing.T) {
	mockResponse := ZLMAPIResponse[APIStatisticsObj]{
		Code: 0,
		Msg:  "success",
		Data: APIStatisticsObj{
			Buffer:                100,
			BufferLikeString:      100,
			BufferList:            100,
			BufferRaw:             100,
			Frame:                 100,
			FrameImp:              100,
			MediaSource:           100,
			MultiMediaSourceMuxer: 100,
			RtmpPacket:            100,
			RtpPacket:             100,
			Socket:                100,
			TcpClient:             100,
			TcpServer:             100,
			TcpSession:            100,
			UdpServer:             100,
			UdpSession:            100,
		},
	}
	server := setupTestServer(t, ZlmAPIEndpointGetStatistics, mockResponse)
	defer server.Close()

	exporter, err := NewExporter(server.URL, MockZlmAPIServerSecret, promslog.New(&promslog.Config{}), Options{})
	assert.NoError(t, err)

	ch := make(chan prometheus.Metric, 1)
	done := make(chan bool)

	go func() {
		exporter.extractStatistics(context.Background(), ch)
		close(ch)
		done <- true
	}()

	metrics := make([]prometheus.Metric, 0)
	for metric := range ch {
		metrics = append(metrics, metric)
	}
	<-done

	assert.Equal(t, 16, len(metrics))
	teardown()
}

func TestExtractSession(t *testing.T) {
	mockResponse := ZLMAPIResponse[APISessionObjs]{
		Code: 0,
		Msg:  "success",
		Data: APISessionObjs{
			APISessionObj{
				Id:         "1111",
				Identifier: "1111",
				LocalIp:    "127.0.0.1",
				LocalPort:  1111,
				PeerIp:     "127.0.0.1",
				PeerPort:   1111,
				TypeID:     "1111",
			},
			APISessionObj{
				Id:         "2222",
				Identifier: "2222",
				LocalIp:    "127.0.0.1",
				LocalPort:  2222,
				PeerIp:     "127.0.0.1",
				PeerPort:   2222,
				TypeID:     "2222",
			},
		},
	}
	server := setupTestServer(t, ZlmAPIEndpointGetAllSession, mockResponse)
	defer server.Close()

	exporter, err := NewExporter(server.URL, MockZlmAPIServerSecret, promslog.New(&promslog.Config{}), Options{})
	assert.NoError(t, err)

	ch := make(chan prometheus.Metric, 1)
	done := make(chan bool)

	go func() {
		exporter.extractSession(context.Background(), ch)
		close(ch)
		done <- true
	}()

	metrics := make([]prometheus.Metric, 0)
	for metric := range ch {
		metrics = append(metrics, metric)
	}
	<-done

	assert.Equal(t, 3, len(metrics))
	teardown()
}

func TestExtractStreamInfo(t *testing.T) {
	mockResponse := ZLMAPIResponse[APIStreamInfoObjs]{
		Code: 0,
		Msg:  "success",
		Data: APIStreamInfoObjs{
			APIStreamInfoObj{
				Stream:           "test1",
				Vhost:            "test1",
				App:              "test1",
				Schema:           "test1",
				AliveSecond:      100,
				BytesSpeed:       100,
				OriginType:       100,
				OriginTypeStr:    "test1",
				OriginUrl:        "test1",
				ReaderCount:      100,
				TotalReaderCount: 100,
			},
			APIStreamInfoObj{
				Stream:           "test2",
				Vhost:            "test2",
				App:              "test2",
				Schema:           "test2",
				AliveSecond:      200,
				BytesSpeed:       200,
				OriginType:       200,
				OriginTypeStr:    "test2",
				OriginUrl:        "test2",
				ReaderCount:      200,
				TotalReaderCount: 200,
			},
		},
	}
	server := setupTestServer(t, ZlmAPIEndpointGetMediaList, mockResponse)
	defer server.Close()

	exporter, err := NewExporter(server.URL, MockZlmAPIServerSecret, promslog.New(&promslog.Config{}), Options{})
	assert.NoError(t, err)

	ch := make(chan prometheus.Metric, 1)
	done := make(chan bool)

	go func() {
		exporter.extractStream(context.Background(), ch)
		close(ch)
		done <- true
	}()

	metrics := make([]prometheus.Metric, 0)
	for metric := range ch {
		metrics = append(metrics, metric)
	}
	<-done

	assert.Equal(t, 15, len(metrics))
	teardown()
}

func TestExtractRtpServer(t *testing.T) {
	mockResponse := ZLMAPIResponse[APIRtpServerObjs]{
		Code: 0,
		Msg:  "success",
		Data: APIRtpServerObjs{
			APIRtpServerObj{
				Port:     "1111",
				StreamID: "1111",
			},
			APIRtpServerObj{
				Port:     "2222",
				StreamID: "2222",
			},
		},
	}
	server := setupTestServer(t, ZlmAPIEndpointListRtpServer, mockResponse)
	defer server.Close()

	exporter, err := NewExporter(server.URL, MockZlmAPIServerSecret, promslog.New(&promslog.Config{}), Options{})
	assert.NoError(t, err)

	ch := make(chan prometheus.Metric, 1)
	done := make(chan bool)

	go func() {
		exporter.extractRtp(context.Background(), ch)
		close(ch)
		done <- true
	}()

	metrics := make([]prometheus.Metric, 0)
	for metric := range ch {
		metrics = append(metrics, metric)
	}
	<-done

	assert.Equal(t, 3, len(metrics))
	teardown()
}

func TestMustNewConstMetric(t *testing.T) {
	exporter, err := NewExporter("http://localhost", MockZlmAPIServerSecret, promslog.New(&promslog.Config{}), Options{})
	assert.NoError(t, err)

	tests := []struct {
		name        string
		value       interface{}
		shouldBeNil bool
	}{
		{
			name:        "float64",
			value:       float64(123.45),
			shouldBeNil: false,
		},
		{
			name:        "string",
			value:       "123.45",
			shouldBeNil: false,
		},
		{
			name:        "non-numeric string",
			value:       "abc",
			shouldBeNil: false,
		},
		{
			name:        "other type",
			value:       struct{}{},
			shouldBeNil: true,
		},
	}

	desc := prometheus.NewDesc("test_metric", "Test metric", []string{"label"}, nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := exporter.mustNewConstMetric(desc, prometheus.GaugeValue, tt.value, "test_label")
			if tt.shouldBeNil {
				assert.Nil(t, metric)
			} else {
				assert.NotNil(t, metric)
			}
		})
	}
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultVal   string
		envValue     string
		expectedVal  string
		shouldSetEnv bool
	}{
		{
			name:         "env exists",
			key:          "TEST_ENV_1",
			defaultVal:   "default",
			envValue:     "custom",
			expectedVal:  "custom",
			shouldSetEnv: true,
		},
		{
			name:         "env not exists",
			key:          "TEST_ENV_2",
			defaultVal:   "default",
			expectedVal:  "default",
			shouldSetEnv: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.shouldSetEnv {
				t.Setenv(tt.key, tt.envValue)
			}
			result := getEnv(tt.key, tt.defaultVal)
			assert.Equal(t, tt.expectedVal, result)
		})
	}
}

func TestGetEnvBool(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultVal   bool
		envValue     string
		expectedVal  bool
		shouldSetEnv bool
	}{
		{
			name:         "valid true",
			key:          "TEST_BOOL_1",
			defaultVal:   false,
			envValue:     "true",
			expectedVal:  true,
			shouldSetEnv: true,
		},
		{
			name:         "valid false",
			key:          "TEST_BOOL_2",
			defaultVal:   true,
			envValue:     "false",
			expectedVal:  false,
			shouldSetEnv: true,
		},
		{
			name:         "invalid bool",
			key:          "TEST_BOOL_3",
			defaultVal:   true,
			envValue:     "invalid",
			expectedVal:  true,
			shouldSetEnv: true,
		},
		{
			name:         "env not exists",
			key:          "TEST_BOOL_4",
			defaultVal:   true,
			expectedVal:  true,
			shouldSetEnv: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.shouldSetEnv {
				t.Setenv(tt.key, tt.envValue)
			}
			result := getEnvBool(tt.key, tt.defaultVal)
			assert.Equal(t, tt.expectedVal, result)
		})
	}
}

func TestMaskSecret(t *testing.T) {
	tests := []struct {
		name     string
		secret   string
		expected string
	}{
		{
			name:     "empty secret",
			secret:   "",
			expected: "<empty>",
		},
		{
			name:     "short secret",
			secret:   "1234",
			expected: "****",
		},
		{
			name:     "long secret",
			secret:   "1234567890",
			expected: "12****90",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskSecret(tt.secret)
			assert.Equal(t, tt.expected, result)
		})
	}
}
