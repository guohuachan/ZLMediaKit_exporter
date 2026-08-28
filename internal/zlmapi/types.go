package zlmapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

// FlexInt decodes an integer that ZLMediaKit has emitted both as a JSON number
// and as a quoted string across releases (listRtpServer's port, for example).
type FlexInt int64

// UnmarshalJSON implements json.Unmarshaler.
func (f *FlexInt) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*f = 0
		return nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		if s == "" {
			*f = 0
			return nil
		}
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return fmt.Errorf("parsing %q as integer: %w", s, err)
		}
		*f = FlexInt(v)
		return nil
	}

	var v int64
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*f = FlexInt(v)
	return nil
}

// String renders the value for use as a Prometheus label.
func (f FlexInt) String() string { return strconv.FormatInt(int64(f), 10) }

// Track types as defined by ZLMediaKit's TrackType enum.
const (
	TrackVideo = 0
	TrackAudio = 1
	TrackTitle = 2
)

// Version is the payload of index/api/version.
type Version struct {
	BranchName string `json:"branchName"`
	BuildTime  string `json:"buildTime"`
	CommitHash string `json:"commitHash"`
}

// ThreadLoad is one entry of index/api/getThreadsLoad and
// index/api/getWorkThreadsLoad.
type ThreadLoad struct {
	Load  float64 `json:"load"`
	Delay float64 `json:"delay"`
}

// Statistics is the payload of index/api/getStatistic: live counts of
// ZLMediaKit's internal object pools.
type Statistics struct {
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

// Session is one entry of index/api/getAllSession.
type Session struct {
	ID         string `json:"id"`
	Identifier string `json:"identifier"`
	LocalIP    string `json:"local_ip"`
	LocalPort  int    `json:"local_port"`
	PeerIP     string `json:"peer_ip"`
	PeerPort   int    `json:"peer_port"`
	TypeID     string `json:"typeid"`
	// Type is "tcp" or "udp". Older builds omit it.
	Type string `json:"type"`
}

// Track describes one elementary stream of a media source. Which fields are
// populated depends on CodecType.
type Track struct {
	CodecID   int    `json:"codec_id"`
	CodecName string `json:"codec_id_name"`
	CodecType int    `json:"codec_type"`
	Ready     bool   `json:"ready"`

	Frames   float64 `json:"frames"`
	Duration float64 `json:"duration"`

	// Audio only.
	SampleRate float64 `json:"sample_rate"`
	Channels   float64 `json:"channels"`
	SampleBit  float64 `json:"sample_bit"`

	// Video only.
	Width         float64 `json:"width"`
	Height        float64 `json:"height"`
	FPS           float64 `json:"fps"`
	GopSize       float64 `json:"gop_size"`
	GopIntervalMS float64 `json:"gop_interval_ms"`
	KeyFrames     float64 `json:"key_frames"`
}

// MediaTuple identifies a stream. It is nested under "src" in the proxy APIs.
type MediaTuple struct {
	Vhost  string `json:"vhost"`
	App    string `json:"app"`
	Stream string `json:"stream"`
	Params string `json:"params"`
}

// StreamProxy is one entry of index/api/listStreamProxy: a stream ZLMediaKit
// pulls from an upstream source.
type StreamProxy struct {
	Key              string     `json:"key"`
	URL              string     `json:"url"`
	Status           float64    `json:"status"`
	StatusStr        string     `json:"status_str"`
	LiveSecs         float64    `json:"liveSecs"`
	RePullCount      float64    `json:"rePullCount"`
	TotalReaderCount float64    `json:"totalReaderCount"`
	BytesSpeed       float64    `json:"bytesSpeed"`
	TotalBytes       float64    `json:"totalBytes"`
	Src              MediaTuple `json:"src"`
}

// StreamPusherProxy is one entry of index/api/listStreamPusherProxy: a stream
// ZLMediaKit pushes to an upstream destination.
type StreamPusherProxy struct {
	Key            string     `json:"key"`
	URL            string     `json:"url"`
	Status         float64    `json:"status"`
	LiveSecs       float64    `json:"liveSecs"`
	RePublishCount float64    `json:"rePublishCount"`
	BytesSpeed     float64    `json:"bytesSpeed"`
	TotalBytes     float64    `json:"totalBytes"`
	Src            MediaTuple `json:"src"`
}

// StreamInfo is one entry of index/api/getMediaList. A source stream is
// published under several schemas, so one stream yields one entry per schema.
type StreamInfo struct {
	AliveSecond      int     `json:"aliveSecond"`
	App              string  `json:"app"`
	BytesSpeed       float64 `json:"bytesSpeed"`
	CreateStamp      int     `json:"createStamp"`
	OriginType       int     `json:"originType"`
	OriginTypeStr    string  `json:"originTypeStr"`
	OriginURL        string  `json:"originUrl"`
	ReaderCount      int     `json:"readerCount"`
	Schema           string  `json:"schema"`
	Stream           string  `json:"stream"`
	TotalReaderCount int     `json:"totalReaderCount"`
	Vhost            string  `json:"vhost"`

	// Added by newer ZLMediaKit builds.
	TotalBytes     float64 `json:"totalBytes"`
	IsRecordingMP4 bool    `json:"isRecordingMP4"`
	IsRecordingHLS bool    `json:"isRecordingHLS"`
	Tracks         []Track `json:"tracks"`
}

// RtpServer is one entry of index/api/listRtpServer. Master reports the port
// as a number and adds the media tuple, ssrc and tcp mode; older builds sent
// only a quoted port and the stream id.
type RtpServer struct {
	Vhost     string  `json:"vhost"`
	App       string  `json:"app"`
	StreamID  string  `json:"stream_id"`
	Port      FlexInt `json:"port"`
	SSRC      FlexInt `json:"ssrc"`
	TCPMode   FlexInt `json:"tcp_mode"`
	OnlyTrack FlexInt `json:"only_track"`
}
