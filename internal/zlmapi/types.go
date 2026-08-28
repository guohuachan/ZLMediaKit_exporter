package zlmapi

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
}

// RtpServer is one entry of index/api/listRtpServer.
type RtpServer struct {
	Port     string `json:"port"`
	StreamID string `json:"stream_id"`
}
