package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"sync"
	"time"

	"context"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promslog"
	"github.com/prometheus/common/version"
	promweb "github.com/prometheus/exporter-toolkit/web"
	webflag "github.com/prometheus/exporter-toolkit/web/kingpinflag"
)

const (
	ZlmAPISuccessCode = 0
)

const (
	ZlmAPIEndpointVersion           = "index/api/version"
	ZlmAPIEndpointGetApiList        = "index/api/getApiList"
	ZlmAPIEndpointGetNetworkThreads = "index/api/getThreadsLoad"
	ZlmAPIEndpointGetWorkThreads    = "index/api/getWorkThreadsLoad"
	ZlmAPIEndpointGetStatistics     = "index/api/getStatistic"
	ZlmAPIEndpointGetAllSession     = "index/api/getAllSession"
	ZlmAPIEndpointGetMediaList      = "index/api/getMediaList"
	ZlmAPIEndpointListRtpServer     = "index/api/listRtpServer"
)

const (
	Namespace               = "zlm"
	SubsystemVersion        = "version"
	SubsystemApi            = "api"
	SubsystemNetworkThreads = "network_threads"
	SubsystemWorkThreads    = "work_threads"
	SubsystemStatistics     = "statistics"
	SubsystemSession        = "session"
	SubsystemStream         = "stream"
	SubsystemRtp            = "rtp"
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

var (
	/*
		BuildVersion, BuildDate, BuildCommitSha are filled in by the build script
	*/
	BuildVersion   = "<<< filled in by build >>>"
	BuildDate      = "<<< filled in by build >>>"
	BuildCommitSha = "<<< filled in by build >>>"
)

var metrics []*prometheus.Desc

var (
	scrapeErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "scrape_errors_total",
			Help:      "Number of errors while scraping ZLMediaKit.",
		},
		[]string{"endpoint"},
	)
)

var (
	ZLMediaKitInfo = newMetricDescr(Namespace, SubsystemVersion, "info", "ZLMediaKit version info.", []string{"branchName", "buildTime", "commitHash"})
	ApiStatus      = newMetricDescr(Namespace, SubsystemApi, "status", "The status of API endpoint", []string{"endpoint"})

	// network threads metric
	NetworkThreadsTotal      = newMetricDescr(Namespace, SubsystemNetworkThreads, "total", "Total number of network threads", []string{})
	NetworkThreadsLoadTotal  = newMetricDescr(Namespace, SubsystemNetworkThreads, "load_total", "Total of network threads load", []string{})
	NetworkThreadsDelayTotal = newMetricDescr(Namespace, SubsystemNetworkThreads, "delay_total", "Total of network threads delay", []string{})

	// work threads metrics
	WorkThreadsTotal      = newMetricDescr(Namespace, SubsystemWorkThreads, "total", "Total number of work threads", []string{})
	WorkThreadsLoadTotal  = newMetricDescr(Namespace, SubsystemWorkThreads, "load_total", "Total of work threads load", []string{})
	WorkThreadsDelayTotal = newMetricDescr(Namespace, SubsystemWorkThreads, "delay_total", "Total of work threads delay", []string{})

	// statistics metrics
	StatisticsBuffer                = newMetricDescr(Namespace, SubsystemStatistics, "buffer", "Statistics buffer", []string{})
	StatisticsBufferLikeString      = newMetricDescr(Namespace, SubsystemStatistics, "buffer_like_string", "Statistics BufferLikeString", []string{})
	StatisticsBufferList            = newMetricDescr(Namespace, SubsystemStatistics, "buffer_list", "Statistics BufferList", []string{})
	StatisticsBufferRaw             = newMetricDescr(Namespace, SubsystemStatistics, "buffer_raw", "Statistics BufferRaw", []string{})
	StatisticsFrame                 = newMetricDescr(Namespace, SubsystemStatistics, "frame", "Statistics Frame", []string{})
	StatisticsFrameImp              = newMetricDescr(Namespace, SubsystemStatistics, "frame_imp", "Statistics FrameImp", []string{})
	StatisticsMediaSource           = newMetricDescr(Namespace, SubsystemStatistics, "media_source", "Statistics MediaSource", []string{})
	StatisticsMultiMediaSourceMuxer = newMetricDescr(Namespace, SubsystemStatistics, "multi_media_source_muxer", "Statistics MultiMediaSourceMuxer", []string{})
	StatisticsRtmpPacket            = newMetricDescr(Namespace, SubsystemStatistics, "rtmp_packet", "Statistics RtmpPacket", []string{})
	StatisticsRtpPacket             = newMetricDescr(Namespace, SubsystemStatistics, "rtp_packet", "Statistics RtpPacket", []string{})
	StatisticsSocket                = newMetricDescr(Namespace, SubsystemStatistics, "socket", "Statistics Socket", []string{})
	StatisticsTcpClient             = newMetricDescr(Namespace, SubsystemStatistics, "tcp_client", "Statistics TcpClient", []string{})
	StatisticsTcpServer             = newMetricDescr(Namespace, SubsystemStatistics, "tcp_server", "Statistics TcpServer", []string{})
	StatisticsTcpSession            = newMetricDescr(Namespace, SubsystemStatistics, "tcp_session", "Statistics TcpSession", []string{})
	StatisticsUdpServer             = newMetricDescr(Namespace, SubsystemStatistics, "udp_server", "Statistics UdpServer", []string{})
	StatisticsUdpSession            = newMetricDescr(Namespace, SubsystemStatistics, "udp_session", "Statistics UdpSession", []string{})

	// session metrics
	SessionInfo  = newMetricDescr(Namespace, SubsystemSession, "info", "Session info", []string{"id", "identifier", "local_ip", "local_port", "peer_ip", "peer_port", "typeid"})
	SessionTotal = newMetricDescr(Namespace, SubsystemSession, "total", "Total number of sessions", []string{})

	// stream metrics
	StreamsInfo            = newMetricDescr(Namespace, SubsystemStream, "info", "Stream basic information", []string{"vhost", "app", "stream", "schema", "origin_type", "origin_url"})
	StreamStatus           = newMetricDescr(Namespace, SubsystemStream, "status", "Stream status (1: active with data flowing, 0: inactive)", []string{"vhost", "app", "stream", "schema"})
	StreamReaderCount      = newMetricDescr(Namespace, SubsystemStream, "reader_count", "Stream reader count", []string{"vhost", "app", "stream", "schema"})
	StreamTotalReaderCount = newMetricDescr(Namespace, SubsystemStream, "total_reader_count", "Total reader count across all schemas", []string{"vhost", "app", "stream"})
	StreamBitrate          = newMetricDescr(Namespace, SubsystemStream, "bitrate", "Stream bitrate", []string{"vhost", "app", "stream", "schema"})
	StreamaliveSecond      = newMetricDescr(Namespace, SubsystemStream, "alive_second", "Stream alive second", []string{"vhost", "app", "stream", "schema"})
	StreamCreateStamp      = newMetricDescr(Namespace, SubsystemStream, "create_stamp", "Stream create stamp", []string{"vhost", "app", "stream", "schema"})
	StreamTotal            = newMetricDescr(Namespace, SubsystemStream, "total", "Total number of streams", []string{})

	// rtp metrics
	RtpServerInfo  = newMetricDescr(Namespace, SubsystemRtp, "server_info", "RTP server info", []string{"port", "stream_id"})
	RtpServerTotal = newMetricDescr(Namespace, SubsystemRtp, "server_total", "Total number of RTP servers", []string{})
)

type Exporter struct {
	scrapeURI    string
	scrapeSecret string
	client       http.Client
	mutex        sync.RWMutex

	up                prometheus.Gauge
	totalScrapes      prometheus.Counter
	totalScrapeErrors prometheus.CounterVec
	log               *slog.Logger
	options           Options

	buildInfo BuildInfo
}

type Options struct {
	SSLVerify bool
}

type BuildInfo struct {
	Version   string
	CommitSha string
	Date      string
}

func NewExporter(uri string, secret string, logger *slog.Logger, options Options) (*Exporter, error) {
	if uri == "" {
		return nil, fmt.Errorf("ZlMediaKit API uri is required")
	}

	if secret == "" {
		return nil, fmt.Errorf("ZlMediaKit API secret is required")
	}

	client := http.Client{}

	if options.SSLVerify {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	exporter := &Exporter{
		scrapeURI:    uri,
		scrapeSecret: secret,
		client:       client,

		up: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "up",
			Help:      "Was the last scrape of ZLMediaKit successful.",
		}),

		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "exporter_scrapes_total",
			Help:      "Current total ZLMediaKit scrapes.",
		}),

		totalScrapeErrors: *prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "scrape_errors_total",
			Help:      "Number of errors while scraping ZLMediaKit.",
		}, []string{"endpoint"}),

		log: logger,

		buildInfo: BuildInfo{
			Version:   BuildVersion,
			CommitSha: BuildCommitSha,
			Date:      BuildDate,
		},

		options: options,
	}

	return exporter, nil
}

func newMetricDescr(namespace, subsystem, metricName, docString string, labels []string) *prometheus.Desc {
	newDesc := prometheus.NewDesc(prometheus.BuildFQName(namespace, subsystem, metricName), docString, labels, nil)
	metrics = append(metrics, newDesc)
	return newDesc
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	for _, metric := range metrics {
		ch <- metric
	}
	ch <- e.up.Desc()
	ch <- e.totalScrapes.Desc()
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	up := e.scrape(ch)
	ch <- prometheus.MustNewConstMetric(e.up.Desc(), prometheus.GaugeValue, up)
	ch <- e.totalScrapes
}

func (e *Exporter) scrape(ch chan<- prometheus.Metric) (up float64) {
	e.totalScrapes.Inc()

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	var wg sync.WaitGroup

	wg.Add(8)
	go func() {
		defer wg.Done()
		e.extractVersion(ctx, ch)
	}()
	go func() {
		defer wg.Done()
		e.extractAPIStatus(ctx, ch)
	}()
	go func() {
		defer wg.Done()
		e.extractNetworkThreads(ctx, ch)
	}()
	go func() {
		defer wg.Done()
		e.extractWorkThreads(ctx, ch)
	}()
	go func() {
		defer wg.Done()
		e.extractStatistics(ctx, ch)
	}()
	go func() {
		defer wg.Done()
		e.extractSession(ctx, ch)
	}()
	go func() {
		defer wg.Done()
		e.extractStream(ctx, ch)
	}()
	go func() {
		defer wg.Done()
		e.extractRtp(ctx, ch)
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		e.log.Error("scrape timeout", "error", ctx.Err())
		return 0
	case <-done:
		return 1
	}
}

type ZLMAPIResponseData interface {
	[]string | APIVersionObj | APINetworkThreadsObjs | APIWorkThreadsObjs |
		APIStreamInfoObjs | APIStatisticsObj | APISessionObjs | APIRtpServerObjs
}

type ZLMAPIResponse[T ZLMAPIResponseData] struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data T      `json:"data"`
}

func (e *Exporter) mustNewConstMetric(desc *prometheus.Desc, valueType prometheus.ValueType, value interface{}, labelValues ...string) prometheus.Metric {
	switch vt := value.(type) {
	case float64:
		return prometheus.MustNewConstMetric(desc, valueType, vt, labelValues...)
	case string:
		valueFloat, err := strconv.ParseFloat(vt, 64)
		if err == nil {
			return prometheus.MustNewConstMetric(desc, valueType, valueFloat, labelValues...)
		}
		return prometheus.MustNewConstMetric(desc, valueType, 1, labelValues...)
	default:
		return nil
	}
}

func (e *Exporter) processAPIResponse(endpoint string, body io.ReadCloser, result interface{}) error {
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(result); err != nil {
		return fmt.Errorf("error decoding JSON response from %s: %w", endpoint, err)
	}

	v := reflect.ValueOf(result).Elem()
	codeField := v.FieldByName("Code")

	if !codeField.IsValid() {
		return fmt.Errorf("response structure does not contain Code field")
	}

	code := codeField.Int()
	if code != int64(ZlmAPISuccessCode) {
		msgField := v.FieldByName("Msg")
		var msg string
		if msgField.IsValid() {
			msg = msgField.String()
		}
		return fmt.Errorf("unexpected API response code from %s: %d, reason: %s", endpoint, code, msg)
	}

	return nil
}

func (e *Exporter) fetchHTTP(ctx context.Context, ch chan<- prometheus.Metric, endpoint string, processFunc func(closer io.ReadCloser) error) {
	uri := fmt.Sprintf("%s/%s", e.scrapeURI, endpoint)
	parsedURL, err := url.Parse(uri)
	if err != nil {
		scrapeErrors.WithLabelValues(endpoint).Inc()
		e.log.Error("error parsing URL", "err", err)
		return
	}

	req := &http.Request{
		Method: http.MethodGet,
		URL:    parsedURL,
		Header: http.Header{
			"secret": []string{e.scrapeSecret},
		},
	}

	res, err := e.client.Do(req)
	if err != nil {
		scrapeErrors.WithLabelValues(endpoint).Inc()
		e.log.Error("error scraping ZLMediaKit", "err", err)
		return
	}
	defer res.Body.Close()

	if err = processFunc(res.Body); err != nil {
		scrapeErrors.WithLabelValues(endpoint).Inc()
		e.log.Error("error processing response", "err", err)
	}
}

type APIVersionObj struct {
	BranchName string `json:"branchName"`
	BuildTime  string `json:"buildTime"`
	CommitHash string `json:"commitHash"`
}

func (e *Exporter) extractVersion(ctx context.Context, ch chan<- prometheus.Metric) {
	processFunc := func(body io.ReadCloser) error {
		var apiResponse ZLMAPIResponse[APIVersionObj]
		if err := e.processAPIResponse(ZlmAPIEndpointVersion, body, &apiResponse); err != nil {
			return err
		}
		data := apiResponse.Data
		ch <- prometheus.MustNewConstMetric(ZLMediaKitInfo, prometheus.GaugeValue, 1, data.BranchName, data.BuildTime, data.CommitHash)
		return nil
	}
	e.fetchHTTP(ctx, ch, ZlmAPIEndpointVersion, processFunc)
}

func (e *Exporter) extractAPIStatus(ctx context.Context, ch chan<- prometheus.Metric) {
	processFunc := func(body io.ReadCloser) error {
		var apiResponse ZLMAPIResponse[[]string]

		if err := e.processAPIResponse(ZlmAPIEndpointGetApiList, body, &apiResponse); err != nil {
			return err
		}

		data := apiResponse.Data

		for _, endpoint := range data {
			ch <- prometheus.MustNewConstMetric(ApiStatus, prometheus.GaugeValue, 1, endpoint)
		}
		return nil
	}
	e.fetchHTTP(ctx, ch, ZlmAPIEndpointGetApiList, processFunc)
}

type APINetworkThreadsObj struct {
	Load  float64 `json:"load"`
	Delay float64 `json:"delay"`
}

type APINetworkThreadsObjs []APINetworkThreadsObj

func (e *Exporter) extractNetworkThreads(ctx context.Context, ch chan<- prometheus.Metric) {
	processFunc := func(body io.ReadCloser) error {
		var apiResponse ZLMAPIResponse[APINetworkThreadsObjs]
		if err := e.processAPIResponse(ZlmAPIEndpointGetNetworkThreads, body, &apiResponse); err != nil {
			return err
		}

		var loadTotal, delayTotal, total float64
		for _, data := range apiResponse.Data {
			loadTotal += data.Load
			delayTotal += data.Delay
			total++
		}
		ch <- prometheus.MustNewConstMetric(NetworkThreadsTotal, prometheus.GaugeValue, total)
		ch <- prometheus.MustNewConstMetric(NetworkThreadsLoadTotal, prometheus.GaugeValue, loadTotal)
		ch <- prometheus.MustNewConstMetric(NetworkThreadsDelayTotal, prometheus.GaugeValue, delayTotal)
		return nil
	}
	e.fetchHTTP(ctx, ch, ZlmAPIEndpointGetNetworkThreads, processFunc)
}

type APIWorkThreadsObj struct {
	Load  float64 `json:"load"`
	Delay float64 `json:"delay"`
}

type APIWorkThreadsObjs []APIWorkThreadsObj

func (e *Exporter) extractWorkThreads(ctx context.Context, ch chan<- prometheus.Metric) {
	processFunc := func(body io.ReadCloser) error {
		var apiResponse ZLMAPIResponse[APIWorkThreadsObjs]
		if err := e.processAPIResponse(ZlmAPIEndpointGetWorkThreads, body, &apiResponse); err != nil {
			return err
		}
		var loadTotal, delayTotal, total float64
		for _, data := range apiResponse.Data {
			loadTotal += data.Load
			delayTotal += data.Delay
			total++
		}
		ch <- prometheus.MustNewConstMetric(WorkThreadsTotal, prometheus.GaugeValue, total)
		ch <- prometheus.MustNewConstMetric(WorkThreadsLoadTotal, prometheus.GaugeValue, loadTotal)
		ch <- prometheus.MustNewConstMetric(WorkThreadsDelayTotal, prometheus.GaugeValue, delayTotal)
		return nil
	}
	e.fetchHTTP(ctx, ch, ZlmAPIEndpointGetWorkThreads, processFunc)
}

type APIStatisticsObj struct {
	Buffer                float64 `json:"Buffer"`
	BufferLikeString      float64 `json:"BufferLikeString"`
	BufferList            float64 `json:"BufferList"`
	BufferRaw             float64 `json:"BufferRaw"`
	Frame                 float64 `json:"Frame"`
	FrameImp              float64 `json:"FrameImp"`
	MediaSource           float64 `json:"MediaSource"`
	MultiMediaSourceMuxer float64 `json:"MultiMediaSourceMuxer"`
	RtmpPacket            float64 `json:"RtmpPacket"`
	RtpPacket             float64 `json:"RtpPacket"`
	Socket                float64 `json:"Socket"`
	TcpClient             float64 `json:"TcpClient"`
	TcpServer             float64 `json:"TcpServer"`
	TcpSession            float64 `json:"TcpSession"`
	UdpServer             float64 `json:"UdpServer"`
	UdpSession            float64 `json:"UdpSession"`
}

func (e *Exporter) extractStatistics(ctx context.Context, ch chan<- prometheus.Metric) {
	processFunc := func(body io.ReadCloser) error {
		var apiResponse ZLMAPIResponse[APIStatisticsObj]
		if err := e.processAPIResponse(ZlmAPIEndpointGetStatistics, body, &apiResponse); err != nil {
			return err
		}
		data := apiResponse.Data
		ch <- e.mustNewConstMetric(StatisticsBuffer, prometheus.GaugeValue, data.Buffer)
		ch <- e.mustNewConstMetric(StatisticsBufferLikeString, prometheus.GaugeValue, data.BufferLikeString)
		ch <- e.mustNewConstMetric(StatisticsBufferList, prometheus.GaugeValue, data.BufferList)
		ch <- e.mustNewConstMetric(StatisticsBufferRaw, prometheus.GaugeValue, data.BufferRaw)
		ch <- e.mustNewConstMetric(StatisticsFrame, prometheus.GaugeValue, data.Frame)
		ch <- e.mustNewConstMetric(StatisticsFrameImp, prometheus.GaugeValue, data.FrameImp)
		ch <- e.mustNewConstMetric(StatisticsMediaSource, prometheus.GaugeValue, data.MediaSource)
		ch <- e.mustNewConstMetric(StatisticsMultiMediaSourceMuxer, prometheus.GaugeValue, data.MultiMediaSourceMuxer)
		ch <- e.mustNewConstMetric(StatisticsRtmpPacket, prometheus.GaugeValue, data.RtmpPacket)
		ch <- e.mustNewConstMetric(StatisticsRtpPacket, prometheus.GaugeValue, data.RtpPacket)
		ch <- e.mustNewConstMetric(StatisticsSocket, prometheus.GaugeValue, data.Socket)
		ch <- e.mustNewConstMetric(StatisticsTcpClient, prometheus.GaugeValue, data.TcpClient)
		ch <- e.mustNewConstMetric(StatisticsTcpServer, prometheus.GaugeValue, data.TcpServer)
		ch <- e.mustNewConstMetric(StatisticsTcpSession, prometheus.GaugeValue, data.TcpSession)
		ch <- e.mustNewConstMetric(StatisticsUdpServer, prometheus.GaugeValue, data.UdpServer)
		ch <- e.mustNewConstMetric(StatisticsUdpSession, prometheus.GaugeValue, data.UdpSession)
		return nil
	}
	e.fetchHTTP(ctx, ch, ZlmAPIEndpointGetStatistics, processFunc)
}

type APISessionObj struct {
	Id         string `json:"id"`
	Identifier string `json:"identifier"`
	LocalIp    string `json:"local_ip"`
	LocalPort  int    `json:"local_port"`
	PeerIp     string `json:"peer_ip"`
	PeerPort   int    `json:"peer_port"`
	TypeID     string `json:"typeid"`
}

type APISessionObjs []APISessionObj

func (e *Exporter) extractSession(ctx context.Context, ch chan<- prometheus.Metric) {
	processFunc := func(body io.ReadCloser) error {
		var apiResponse ZLMAPIResponse[APISessionObjs]
		if err := e.processAPIResponse(ZlmAPIEndpointGetAllSession, body, &apiResponse); err != nil {
			return err
		}
		for _, v := range apiResponse.Data {
			id := v.Id
			identifier := v.Identifier
			localIP := v.LocalIp
			localPort := strconv.Itoa(v.LocalPort)
			peerIP := v.PeerIp
			peerPort := strconv.Itoa(v.PeerPort)
			typeID := v.TypeID
			ch <- prometheus.MustNewConstMetric(SessionInfo, prometheus.GaugeValue, 1, id, identifier, localIP, localPort, peerIP, peerPort, typeID)
		}
		ch <- prometheus.MustNewConstMetric(SessionTotal, prometheus.GaugeValue, float64(len(apiResponse.Data)))
		return nil
	}
	e.fetchHTTP(ctx, ch, ZlmAPIEndpointGetAllSession, processFunc)
}

type APIStreamInfoObj struct {
	AliveSecond      int     `json:"aliveSecond"`
	App              string  `json:"app"`
	BytesSpeed       float64 `json:"bytesSpeed"`
	CreateStamp      int     `json:"createStamp"`
	OriginType       int     `json:"originType"`
	OriginTypeStr    string  `json:"originTypeStr"`
	OriginUrl        string  `json:"originUrl"`
	ReaderCount      int     `json:"readerCount"`
	Schema           string  `json:"schema"`
	Stream           string  `json:"stream"`
	TotalReaderCount int     `json:"totalReaderCount"`
	Vhost            string  `json:"vhost"`
}

type APIStreamInfoObjs []APIStreamInfoObj

// Streams with the same stream name represent the same source stream,
// while schema indicates the specific protocol.
// ZLMediaKit automatically pushes the source stream to multiple protocols (schemas) by default.
func (e *Exporter) extractStream(ctx context.Context, ch chan<- prometheus.Metric) {
	processFunc := func(body io.ReadCloser) error {
		var apiResponse ZLMAPIResponse[APIStreamInfoObjs]
		if err := e.processAPIResponse(ZlmAPIEndpointGetMediaList, body, &apiResponse); err != nil {
			return err
		}

		uniqueStreamKeys := make(map[string]bool)
		for _, stream := range apiResponse.Data {
			streamKey := fmt.Sprintf("%s_%s_%s", stream.Vhost, stream.App, stream.Stream)

			if !uniqueStreamKeys[streamKey] {
				ch <- prometheus.MustNewConstMetric(StreamTotalReaderCount,
					prometheus.GaugeValue,
					float64(stream.TotalReaderCount),
					stream.App, stream.Stream, stream.Vhost)

				uniqueStreamKeys[streamKey] = true
			}

			// stream info
			ch <- prometheus.MustNewConstMetric(StreamsInfo, prometheus.GaugeValue,
				1, stream.Vhost, stream.App, stream.Stream, stream.Schema,
				stream.OriginTypeStr, stream.OriginUrl)

			// stream status
			status := 0.0
			if stream.BytesSpeed > 0 {
				status = 1.0
			}
			ch <- prometheus.MustNewConstMetric(StreamStatus, prometheus.GaugeValue,
				status, stream.Vhost, stream.App, stream.Stream, stream.Schema)

			// stream reader count
			ch <- prometheus.MustNewConstMetric(StreamReaderCount,
				prometheus.GaugeValue,
				float64(stream.ReaderCount),
				stream.Vhost, stream.App, stream.Stream, stream.Schema)

			// stream bitrate
			ch <- prometheus.MustNewConstMetric(StreamBitrate,
				prometheus.GaugeValue,
				stream.BytesSpeed,
				stream.Vhost, stream.App, stream.Stream, stream.Schema)

			// stream alive second
			ch <- prometheus.MustNewConstMetric(StreamaliveSecond,
				prometheus.GaugeValue,
				float64(stream.AliveSecond),
				stream.Vhost, stream.App, stream.Stream, stream.Schema)

			// stream create stamp
			ch <- prometheus.MustNewConstMetric(StreamCreateStamp,
				prometheus.GaugeValue,
				float64(stream.CreateStamp),
				stream.Vhost, stream.App, stream.Stream, stream.Schema)

			// todo: 增加一个zlm_stream_bytes 字段，表示流的总流量
		}

		// stream total
		ch <- prometheus.MustNewConstMetric(StreamTotal,
			prometheus.GaugeValue,
			float64(len(uniqueStreamKeys)))
		return nil
	}
	e.fetchHTTP(ctx, ch, ZlmAPIEndpointGetMediaList, processFunc)
}

type APIRtpServerObj struct {
	Port     string `json:"port"`
	StreamID string `json:"stream_id"`
}

type APIRtpServerObjs []APIRtpServerObj

func (e *Exporter) extractRtp(ctx context.Context, ch chan<- prometheus.Metric) {
	processFunc := func(body io.ReadCloser) error {
		var apiResponse ZLMAPIResponse[APIRtpServerObjs]
		if err := e.processAPIResponse(ZlmAPIEndpointListRtpServer, body, &apiResponse); err != nil {
			return err
		}
		for _, v := range apiResponse.Data {
			rtpPort := v.Port
			streamID := v.StreamID
			ch <- prometheus.MustNewConstMetric(RtpServerInfo, prometheus.GaugeValue, 1, rtpPort, streamID)
		}
		ch <- prometheus.MustNewConstMetric(RtpServerTotal, prometheus.GaugeValue, float64(len(apiResponse.Data)))
		return nil
	}
	e.fetchHTTP(ctx, ch, ZlmAPIEndpointListRtpServer, processFunc)
}

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

// doc: https://prometheus.io/docs/instrumenting/writing_exporters/
// todo: --disable-exporting-key-values
func main() {
	kingpin.Version(version.Print("zlm_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
	promslogConfig := &promslog.Config{}
	logger := promslog.New(promslogConfig)

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

	option := Options{
		SSLVerify: *webSSLVerify,
	}

	exporter, err := NewExporter(*zlmApiURL, *zlmApiSecret, logger, option)
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
