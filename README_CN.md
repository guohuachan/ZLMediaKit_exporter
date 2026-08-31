# ZLMediaKit Prometheus Exporter

![zlm_exporter](https://socialify.git.ci/guohuachan/ZLMediaKit_exporter/image?language=1&owner=1&name=1&stargazers=1&theme=Light)

[English](./README.md) | 简体中文

[ZLMediaKit](https://github.com/ZLMediaKit/ZLMediaKit) 指标的 Prometheus exporter，使用 Go 语言编写。

通过 ZLMediaKit 的 API 收集指标，并暴露为 Prometheus 指标。

> **从 1.0 之前的版本升级？** 指标名称与 TLS 参数均有变化，详见 [MIGRATION.md](./MIGRATION.md)。

[![Go Report Card](https://goreportcard.com/badge/github.com/guohuachan/ZLMediaKit_exporter)](https://goreportcard.com/report/github.com/guohuachan/ZLMediaKit_exporter)
[![](https://img.shields.io/badge/license-MIT-green.svg)](https://github.com/guohuachan/ZLMediaKit_exporter/blob/master/LICENSE)
[![](https://img.shields.io/badge/language-golang-red.svg)](https://en.cppreference.com/)
[![](https://img.shields.io/badge/PRs-welcome-yellow.svg)](hhttps://github.com/guohuachan/ZLMediaKit_exporter/pulls)

## Grafana DEMO
![Grafana DEMO](./www/grafana_demo.png)


## Workflow

![workflow](./www/workflow.png)
## Usage

### Prerequisites

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'zlm_exporter'
    static_configs:
      - targets: ['<zlm_exporter_host>:9101']
```

### Docker

```shell
## 拉取镜像或者构建镜像
docker pull zlmexporter/zlmexporter:latest
# OR
make build-docker

## 运行容器
docker run --rm --name zlm_exporter -p 9101:9101 \
  -e ZLM_API_URL=<zlmediakit_api_uri> \
  -e ZLM_API_SECRET=<zlmediakit_api_secret> \
  zlmexporter/zlmexporter:latest

## 获取指标
curl http://localhost:9101/metrics
```

### 源代码
```shell
git clone https://github.com/guohuachan/ZLMediaKit_exporter
cd ZLMediaKit_exporter
## 构建
make build
## 运行
./zlm_exporter --zlm.api-url=<zlmediakit_api_uri> --zlm.secret=<zlmediakit_api_secret>

## 获取指标
curl http://localhost:9101/metrics
```

## 命令行参数

| 参数 | 环境变量 | 说明 |
|---|---|---|
| `zlm.api-url` | `ZLM_API_URL` | ZLMediaKit API 地址，默认 `http://127.0.0.1` |
| `zlm.secret` | `ZLM_API_SECRET` | 访问 ZLMediaKit API 的 secret |
| `zlm.tls-insecure-skip-verify` | `ZLM_EXPORTER_TLS_INSECURE_SKIP_VERIFY` | 不校验 ZLMediaKit 的 TLS 证书，默认 `false` |
| `zlm.expose-api-status` | `ZLM_EXPORTER_EXPOSE_API_STATUS` | 输出 `zlm_api_status`（每个 API 端点一条恒为 1 的序列），默认 `false` |
| `zlm.expose-session-info` | `ZLM_EXPORTER_EXPOSE_SESSION_INFO` | 输出 `zlm_session_info`（每条连接一条序列），默认 `false` |
| `zlm.expose-stream-tracks` | `ZLM_EXPORTER_EXPOSE_STREAM_TRACKS` | 输出流的 track 明细指标，默认 `true` |
| `zlm.disable-collectors` | `ZLM_EXPORTER_DISABLE_COLLECTORS` | 逗号分隔，整体停用指定 collector，如 `stream_proxy,stream_pusher` |
| `web.listen-address` | `ZLM_EXPORTER_TELEMETRY_ADDRESS` | 指标监听地址，默认 `:9101` |
| `web.telemetry-path` | `ZLM_EXPORTER_TELEMETRY_PATH` | 指标路径，默认 `/metrics` |
| `web.timeout` | `ZLM_EXPORTER_TIMEOUT` | 请求 ZLMediaKit 的超时时间，默认 `15s` |
| `web.metric-only` | `ZLM_EXPORTER_METRIC_ONLY` | 只输出 ZLMediaKit 指标，不带 Go 运行时指标，默认 `true` |

`zlm.disable-collectors` 可用的 collector 名称：`version`、`api`、
`network_threads`、`work_threads`、`statistics`、`session`、`stream`、
`stream_proxy`、`stream_pusher`、`rtp`。

> 从 1.0 之前的版本升级？`--web.ssl-verify` 已删除，TLS 证书默认开始校验。
> 详见 [MIGRATION.md](./MIGRATION.md)。

## 收集的指标

### Exporter 自身

| 指标 | 标签 | 说明 |
|---|---|---|
| `zlm_up` | | 最近一次抓取是否成功 |
| `zlm_exporter_scrapes_total` | | 累计抓取次数 |
| `zlm_exporter_scrape_errors_total` | collector | 各 collector 的抓取错误数 |
| `zlm_exporter_scrape_duration_seconds` | collector | 各 collector 最近一次抓取耗时 |
| `zlm_exporter_collector_success` | collector | 各 collector 最近一次抓取是否成功 |
| `zlm_exporter_build_info` | version, revision, branch, goversion, tags | exporter 构建信息 |

### 服务端

| 指标 | 标签 | 说明 |
|---|---|---|
| `zlm_version_info` | branch_name, build_time, commit_hash | ZLMediaKit 版本信息 |
| `zlm_api_status` | endpoint | 可用的 API 端点，需 `--zlm.expose-api-status` 开启 |

### 线程

| 指标 | 标签 | 说明 |
|---|---|---|
| `zlm_network_threads` | | 网络线程（event poller）数量 |
| `zlm_network_thread_load_percent` | index | 单个网络线程负载百分比 |
| `zlm_network_thread_delay_seconds` | index | 单个网络线程任务延迟 |
| `zlm_work_threads` | | 工作线程数量 |
| `zlm_work_thread_load_percent` | index | 单个工作线程负载百分比 |
| `zlm_work_thread_delay_seconds` | index | 单个工作线程任务延迟 |

### 对象统计

`zlm_statistics_buffer`、`zlm_statistics_buffer_like_string`、
`zlm_statistics_buffer_list`、`zlm_statistics_buffer_raw`、
`zlm_statistics_frame`、`zlm_statistics_frame_imp`、
`zlm_statistics_media_source`、`zlm_statistics_multi_media_source_muxer`、
`zlm_statistics_rtmp_packet`、`zlm_statistics_rtp_packet`、
`zlm_statistics_socket`、`zlm_statistics_tcp_client`、
`zlm_statistics_tcp_server`、`zlm_statistics_tcp_session`、
`zlm_statistics_udp_server`、`zlm_statistics_udp_session` —— ZLMediaKit
内部对象池的实时计数，无标签。

### 会话

| 指标 | 标签 | 说明 |
|---|---|---|
| `zlm_sessions` | typeid, type | 按会话类型与传输层协议聚合的会话数 |
| `zlm_session_info` | id, identifier, local_ip, local_port, peer_ip, peer_port, typeid, type | 单连接明细，需 `--zlm.expose-session-info` 开启 |

### 流

| 指标 | 标签 | 说明 |
|---|---|---|
| `zlm_streams` | | 源流数量 |
| `zlm_stream_info` | vhost, app, stream, schema, origin_type, origin_url | 流基本信息 |
| `zlm_stream_status` | vhost, app, stream, schema | 有数据流动时为 1 |
| `zlm_stream_readers` | vhost, app, stream, schema | 该协议下的观看者数 |
| `zlm_stream_total_readers` | vhost, app, stream | 跨全部协议的观看者数 |
| `zlm_stream_bytes_per_second` | vhost, app, stream, schema | 当前吞吐，单位字节/秒 |
| `zlm_stream_bytes_total` | vhost, app, stream, schema | 累计传输字节数 |
| `zlm_stream_alive_seconds` | vhost, app, stream, schema | 流存活秒数 |
| `zlm_stream_create_time_seconds` | vhost, app, stream, schema | 流创建时间的 Unix 时间戳 |
| `zlm_stream_recording` | vhost, app, stream, type | 录制状态，`type` 为 `mp4` 或 `hls` |

### 流 track 明细

track 属于源流、与协议（schema）无关，因此按流去重后只输出一次。
全部带标签 `vhost, app, stream, codec_type, codec_name, track_index`。
可用 `--zlm.expose-stream-tracks=false` 关闭。

| 指标 | 适用 | 说明 |
|---|---|---|
| `zlm_stream_track_ready` | 全部 | track 是否就绪 |
| `zlm_stream_track_frames_total` | 全部 | track 累计帧数 |
| `zlm_stream_track_duration_seconds` | 全部 | track 时长 |
| `zlm_stream_track_fps` | 视频 | 帧率 |
| `zlm_stream_track_width` | 视频 | 宽度（像素） |
| `zlm_stream_track_height` | 视频 | 高度（像素） |
| `zlm_stream_track_gop_size` | 视频 | GOP 大小（帧） |
| `zlm_stream_track_gop_interval_seconds` | 视频 | GOP 间隔 |
| `zlm_stream_track_key_frames_total` | 视频 | 关键帧数 |
| `zlm_stream_track_sample_rate` | 音频 | 采样率（Hz） |
| `zlm_stream_track_channels` | 音频 | 声道数 |
| `zlm_stream_track_sample_bit` | 音频 | 采样位深 |

### 代理

拉流代理（`listStreamProxy`）与推流代理（`listStreamPusherProxy`）。
所有单代理指标都带 `key, vhost, app, stream` 标签。

| 指标 | 说明 |
|---|---|
| `zlm_stream_proxies` | 拉流代理数量 |
| `zlm_stream_proxy_info` | 拉流代理信息，额外带 `url` 与 `status_str` |
| `zlm_stream_proxy_status` | ZLMediaKit 上报的状态，0 表示正常 |
| `zlm_stream_proxy_live_seconds` | 代理存活秒数 |
| `zlm_stream_proxy_repull_total` | 重连次数 |
| `zlm_stream_proxy_total_readers` | 被代理流的观看者数 |
| `zlm_stream_proxy_bytes_per_second` | 当前接收速率 |
| `zlm_stream_proxy_bytes_total` | 累计接收字节数 |
| `zlm_stream_pushers` | 推流代理数量 |
| `zlm_stream_pusher_info` | 推流代理信息，额外带 `url` |
| `zlm_stream_pusher_status` | ZLMediaKit 上报的状态，0 表示正常 |
| `zlm_stream_pusher_live_seconds` | 代理存活秒数 |
| `zlm_stream_pusher_republish_total` | 重连次数 |
| `zlm_stream_pusher_bytes_per_second` | 当前发送速率 |
| `zlm_stream_pusher_bytes_total` | 累计发送字节数 |

### RTP

| 指标 | 标签 | 说明 |
|---|---|---|
| `zlm_rtp_servers` | | RTP server 数量 |
| `zlm_rtp_server_info` | vhost, app, stream_id, port, ssrc, tcp_mode | RTP server 信息 |

<details>
<summary>指标输出示例</summary>

```
# HELP zlm_exporter_collector_success Whether the last scrape of a collector succeeded (1: yes, 0: no).
# TYPE zlm_exporter_collector_success gauge
zlm_exporter_collector_success{collector="network_threads"} 1
zlm_exporter_collector_success{collector="rtp"} 1
zlm_exporter_collector_success{collector="session"} 1
zlm_exporter_collector_success{collector="statistics"} 1
zlm_exporter_collector_success{collector="stream"} 1
zlm_exporter_collector_success{collector="stream_proxy"} 1
zlm_exporter_collector_success{collector="stream_pusher"} 1
zlm_exporter_collector_success{collector="version"} 1
zlm_exporter_collector_success{collector="work_threads"} 1
# HELP zlm_exporter_scrape_duration_seconds Duration of the last scrape, per collector.
# TYPE zlm_exporter_scrape_duration_seconds gauge
zlm_exporter_scrape_duration_seconds{collector="network_threads"} 0.0031
zlm_exporter_scrape_duration_seconds{collector="rtp"} 0.0031
zlm_exporter_scrape_duration_seconds{collector="session"} 0.0031
zlm_exporter_scrape_duration_seconds{collector="statistics"} 0.0031
zlm_exporter_scrape_duration_seconds{collector="stream"} 0.0031
zlm_exporter_scrape_duration_seconds{collector="stream_proxy"} 0.0031
zlm_exporter_scrape_duration_seconds{collector="stream_pusher"} 0.0031
zlm_exporter_scrape_duration_seconds{collector="version"} 0.0031
zlm_exporter_scrape_duration_seconds{collector="work_threads"} 0.0031
# HELP zlm_exporter_scrape_errors_total Number of errors while scraping ZLMediaKit, per collector.
# TYPE zlm_exporter_scrape_errors_total counter
zlm_exporter_scrape_errors_total{collector="network_threads"} 0
zlm_exporter_scrape_errors_total{collector="rtp"} 0
zlm_exporter_scrape_errors_total{collector="session"} 0
zlm_exporter_scrape_errors_total{collector="statistics"} 0
zlm_exporter_scrape_errors_total{collector="stream"} 0
zlm_exporter_scrape_errors_total{collector="stream_proxy"} 0
zlm_exporter_scrape_errors_total{collector="stream_pusher"} 0
zlm_exporter_scrape_errors_total{collector="version"} 0
zlm_exporter_scrape_errors_total{collector="work_threads"} 0
# HELP zlm_exporter_scrapes_total Current total ZLMediaKit scrapes.
# TYPE zlm_exporter_scrapes_total counter
zlm_exporter_scrapes_total 1
# HELP zlm_network_thread_delay_seconds Network thread task delay in seconds
# TYPE zlm_network_thread_delay_seconds gauge
zlm_network_thread_delay_seconds{index="0"} 0
zlm_network_thread_delay_seconds{index="1"} 0
zlm_network_thread_delay_seconds{index="2"} 0
zlm_network_thread_delay_seconds{index="3"} 0
zlm_network_thread_delay_seconds{index="4"} 0
zlm_network_thread_delay_seconds{index="5"} 0
zlm_network_thread_delay_seconds{index="6"} 0
zlm_network_thread_delay_seconds{index="7"} 0
# HELP zlm_network_thread_load_percent Network thread load in percent
# TYPE zlm_network_thread_load_percent gauge
zlm_network_thread_load_percent{index="0"} 0
zlm_network_thread_load_percent{index="1"} 0
zlm_network_thread_load_percent{index="2"} 0
zlm_network_thread_load_percent{index="3"} 0
zlm_network_thread_load_percent{index="4"} 0
zlm_network_thread_load_percent{index="5"} 0
zlm_network_thread_load_percent{index="6"} 0
zlm_network_thread_load_percent{index="7"} 0
# HELP zlm_network_threads Number of network (event poller) threads
# TYPE zlm_network_threads gauge
zlm_network_threads 8
# HELP zlm_rtp_server_info RTP server info
# TYPE zlm_rtp_server_info gauge
zlm_rtp_server_info{app="rtp",port="10000",ssrc="1234567890",stream_id="test_rtp",tcp_mode="0",vhost="__defaultVhost__"} 1
zlm_rtp_server_info{app="rtp",port="10002",ssrc="987654321",stream_id="other_rtp",tcp_mode="1",vhost="__defaultVhost__"} 1
# HELP zlm_rtp_servers Number of RTP servers
# TYPE zlm_rtp_servers gauge
zlm_rtp_servers 2
# HELP zlm_sessions Number of sessions, by session class and transport
# TYPE zlm_sessions gauge
zlm_sessions{type="tcp",typeid="mediakit::HttpSession"} 1
# HELP zlm_statistics_buffer Statistics buffer
# TYPE zlm_statistics_buffer gauge
zlm_statistics_buffer 10
# HELP zlm_statistics_buffer_like_string Statistics BufferLikeString
# TYPE zlm_statistics_buffer_like_string gauge
zlm_statistics_buffer_like_string 2
# HELP zlm_statistics_buffer_list Statistics BufferList
# TYPE zlm_statistics_buffer_list gauge
zlm_statistics_buffer_list 0
# HELP zlm_statistics_buffer_raw Statistics BufferRaw
# TYPE zlm_statistics_buffer_raw gauge
zlm_statistics_buffer_raw 8
# HELP zlm_statistics_frame Statistics Frame
# TYPE zlm_statistics_frame gauge
zlm_statistics_frame 0
# HELP zlm_statistics_frame_imp Statistics FrameImp
# TYPE zlm_statistics_frame_imp gauge
zlm_statistics_frame_imp 0
# HELP zlm_statistics_media_source Statistics MediaSource
# TYPE zlm_statistics_media_source gauge
zlm_statistics_media_source 1
# HELP zlm_statistics_multi_media_source_muxer Statistics MultiMediaSourceMuxer
# TYPE zlm_statistics_multi_media_source_muxer gauge
zlm_statistics_multi_media_source_muxer 0
# HELP zlm_statistics_rtmp_packet Statistics RtmpPacket
# TYPE zlm_statistics_rtmp_packet gauge
zlm_statistics_rtmp_packet 0
# HELP zlm_statistics_rtp_packet Statistics RtpPacket
# TYPE zlm_statistics_rtp_packet gauge
zlm_statistics_rtp_packet 0
# HELP zlm_statistics_socket Statistics Socket
# TYPE zlm_statistics_socket gauge
zlm_statistics_socket 58
# HELP zlm_statistics_tcp_client Statistics TcpClient
# TYPE zlm_statistics_tcp_client gauge
zlm_statistics_tcp_client 1
# HELP zlm_statistics_tcp_server Statistics TcpServer
# TYPE zlm_statistics_tcp_server gauge
zlm_statistics_tcp_server 43
# HELP zlm_statistics_tcp_session Statistics TcpSession
# TYPE zlm_statistics_tcp_session gauge
zlm_statistics_tcp_session 1
# HELP zlm_statistics_udp_server Statistics UdpServer
# TYPE zlm_statistics_udp_server gauge
zlm_statistics_udp_server 16
# HELP zlm_statistics_udp_session Statistics UdpSession
# TYPE zlm_statistics_udp_session gauge
zlm_statistics_udp_session 0
# HELP zlm_stream_alive_seconds Seconds the stream has been alive
# TYPE zlm_stream_alive_seconds gauge
zlm_stream_alive_seconds{app="live",schema="fmp4",stream="test",vhost="__defaultVhost__"} 7
zlm_stream_alive_seconds{app="live",schema="rtmp",stream="test",vhost="__defaultVhost__"} 7
zlm_stream_alive_seconds{app="live",schema="rtsp",stream="test",vhost="__defaultVhost__"} 7
zlm_stream_alive_seconds{app="live",schema="ts",stream="test",vhost="__defaultVhost__"} 7
# HELP zlm_stream_bytes_per_second Current stream throughput in bytes per second
# TYPE zlm_stream_bytes_per_second gauge
zlm_stream_bytes_per_second{app="live",schema="fmp4",stream="test",vhost="__defaultVhost__"} 31156
zlm_stream_bytes_per_second{app="live",schema="rtmp",stream="test",vhost="__defaultVhost__"} 20680
zlm_stream_bytes_per_second{app="live",schema="rtsp",stream="test",vhost="__defaultVhost__"} 20984
zlm_stream_bytes_per_second{app="live",schema="ts",stream="test",vhost="__defaultVhost__"} 28624
# HELP zlm_stream_bytes_total Total bytes transferred for the stream
# TYPE zlm_stream_bytes_total counter
zlm_stream_bytes_total{app="live",schema="fmp4",stream="test",vhost="__defaultVhost__"} 218094
zlm_stream_bytes_total{app="live",schema="rtmp",stream="test",vhost="__defaultVhost__"} 144761
zlm_stream_bytes_total{app="live",schema="rtsp",stream="test",vhost="__defaultVhost__"} 146891
zlm_stream_bytes_total{app="live",schema="ts",stream="test",vhost="__defaultVhost__"} 200368
# HELP zlm_stream_create_time_seconds Unix timestamp at which the stream was created
# TYPE zlm_stream_create_time_seconds gauge
zlm_stream_create_time_seconds{app="live",schema="fmp4",stream="test",vhost="__defaultVhost__"} 1.731424913e+09
zlm_stream_create_time_seconds{app="live",schema="rtmp",stream="test",vhost="__defaultVhost__"} 1.731424913e+09
zlm_stream_create_time_seconds{app="live",schema="rtsp",stream="test",vhost="__defaultVhost__"} 1.731424913e+09
zlm_stream_create_time_seconds{app="live",schema="ts",stream="test",vhost="__defaultVhost__"} 1.731424913e+09
# HELP zlm_stream_info Stream basic information
# TYPE zlm_stream_info gauge
zlm_stream_info{app="live",origin_type="rtsp_push",origin_url="rtsp://127.0.0.1:554/live/test",schema="fmp4",stream="test",vhost="__defaultVhost__"} 1
zlm_stream_info{app="live",origin_type="rtsp_push",origin_url="rtsp://127.0.0.1:554/live/test",schema="rtmp",stream="test",vhost="__defaultVhost__"} 1
zlm_stream_info{app="live",origin_type="rtsp_push",origin_url="rtsp://127.0.0.1:554/live/test",schema="rtsp",stream="test",vhost="__defaultVhost__"} 1
zlm_stream_info{app="live",origin_type="rtsp_push",origin_url="rtsp://127.0.0.1:554/live/test",schema="ts",stream="test",vhost="__defaultVhost__"} 1
# HELP zlm_stream_proxies Number of stream pull proxies
# TYPE zlm_stream_proxies gauge
zlm_stream_proxies 1
# HELP zlm_stream_proxy_bytes_per_second Current receive rate of the stream pull proxy in bytes per second
# TYPE zlm_stream_proxy_bytes_per_second gauge
zlm_stream_proxy_bytes_per_second{app="proxy",key="__defaultVhost__/proxy/camera1",stream="camera1",vhost="__defaultVhost__"} 20480
# HELP zlm_stream_proxy_bytes_total Total bytes received by the stream pull proxy
# TYPE zlm_stream_proxy_bytes_total counter
zlm_stream_proxy_bytes_total{app="proxy",key="__defaultVhost__/proxy/camera1",stream="camera1",vhost="__defaultVhost__"} 736280
# HELP zlm_stream_proxy_info Stream pull proxy information
# TYPE zlm_stream_proxy_info gauge
zlm_stream_proxy_info{app="proxy",key="__defaultVhost__/proxy/camera1",status_str="success",stream="camera1",url="rtsp://192.168.1.10:554/live/ch0",vhost="__defaultVhost__"} 1
# HELP zlm_stream_proxy_live_seconds Seconds the stream pull proxy has been alive
# TYPE zlm_stream_proxy_live_seconds gauge
zlm_stream_proxy_live_seconds{app="proxy",key="__defaultVhost__/proxy/camera1",stream="camera1",vhost="__defaultVhost__"} 3600
# HELP zlm_stream_proxy_repull_total Number of times the stream pull proxy reconnected
# TYPE zlm_stream_proxy_repull_total counter
zlm_stream_proxy_repull_total{app="proxy",key="__defaultVhost__/proxy/camera1",stream="camera1",vhost="__defaultVhost__"} 2
# HELP zlm_stream_proxy_status Stream pull proxy status as reported by ZLMediaKit (0: success)
# TYPE zlm_stream_proxy_status gauge
zlm_stream_proxy_status{app="proxy",key="__defaultVhost__/proxy/camera1",stream="camera1",vhost="__defaultVhost__"} 0
# HELP zlm_stream_proxy_total_readers Number of readers of the proxied stream
# TYPE zlm_stream_proxy_total_readers gauge
zlm_stream_proxy_total_readers{app="proxy",key="__defaultVhost__/proxy/camera1",stream="camera1",vhost="__defaultVhost__"} 3
# HELP zlm_stream_pusher_bytes_per_second Current send rate of the stream push proxy in bytes per second
# TYPE zlm_stream_pusher_bytes_per_second gauge
zlm_stream_pusher_bytes_per_second{app="live",key="__defaultVhost__/live/test",stream="test",vhost="__defaultVhost__"} 15360
# HELP zlm_stream_pusher_bytes_total Total bytes sent by the stream push proxy
# TYPE zlm_stream_pusher_bytes_total counter
zlm_stream_pusher_bytes_total{app="live",key="__defaultVhost__/live/test",stream="test",vhost="__defaultVhost__"} 276480
# HELP zlm_stream_pusher_info Stream push proxy information
# TYPE zlm_stream_pusher_info gauge
zlm_stream_pusher_info{app="live",key="__defaultVhost__/live/test",stream="test",url="rtmp://cdn.example.com/live/test",vhost="__defaultVhost__"} 1
# HELP zlm_stream_pusher_live_seconds Seconds the stream push proxy has been alive
# TYPE zlm_stream_pusher_live_seconds gauge
zlm_stream_pusher_live_seconds{app="live",key="__defaultVhost__/live/test",stream="test",vhost="__defaultVhost__"} 1800
# HELP zlm_stream_pusher_republish_total Number of times the stream push proxy reconnected
# TYPE zlm_stream_pusher_republish_total counter
zlm_stream_pusher_republish_total{app="live",key="__defaultVhost__/live/test",stream="test",vhost="__defaultVhost__"} 1
# HELP zlm_stream_pusher_status Stream push proxy status as reported by ZLMediaKit (0: success)
# TYPE zlm_stream_pusher_status gauge
zlm_stream_pusher_status{app="live",key="__defaultVhost__/live/test",stream="test",vhost="__defaultVhost__"} 0
# HELP zlm_stream_pushers Number of stream push proxies
# TYPE zlm_stream_pushers gauge
zlm_stream_pushers 1
# HELP zlm_stream_readers Number of readers of the stream
# TYPE zlm_stream_readers gauge
zlm_stream_readers{app="live",schema="fmp4",stream="test",vhost="__defaultVhost__"} 0
zlm_stream_readers{app="live",schema="rtmp",stream="test",vhost="__defaultVhost__"} 0
zlm_stream_readers{app="live",schema="rtsp",stream="test",vhost="__defaultVhost__"} 0
zlm_stream_readers{app="live",schema="ts",stream="test",vhost="__defaultVhost__"} 0
# HELP zlm_stream_recording Whether the stream is being recorded (1: recording, 0: not)
# TYPE zlm_stream_recording gauge
zlm_stream_recording{app="live",stream="test",type="hls",vhost="__defaultVhost__"} 1
zlm_stream_recording{app="live",stream="test",type="mp4",vhost="__defaultVhost__"} 0
# HELP zlm_stream_status Stream status (1: active with data flowing, 0: inactive)
# TYPE zlm_stream_status gauge
zlm_stream_status{app="live",schema="fmp4",stream="test",vhost="__defaultVhost__"} 1
zlm_stream_status{app="live",schema="rtmp",stream="test",vhost="__defaultVhost__"} 1
zlm_stream_status{app="live",schema="rtsp",stream="test",vhost="__defaultVhost__"} 1
zlm_stream_status{app="live",schema="ts",stream="test",vhost="__defaultVhost__"} 1
# HELP zlm_stream_total_readers Number of readers of the stream across all schemas
# TYPE zlm_stream_total_readers gauge
zlm_stream_total_readers{app="live",stream="test",vhost="__defaultVhost__"} 0
# HELP zlm_stream_track_channels Audio track channel count
# TYPE zlm_stream_track_channels gauge
zlm_stream_track_channels{app="live",codec_name="mpeg4-generic",codec_type="audio",stream="test",track_index="0",vhost="__defaultVhost__"} 1
# HELP zlm_stream_track_duration_seconds Track duration in seconds
# TYPE zlm_stream_track_duration_seconds gauge
zlm_stream_track_duration_seconds{app="live",codec_name="H264",codec_type="video",stream="test",track_index="1",vhost="__defaultVhost__"} 5.597
zlm_stream_track_duration_seconds{app="live",codec_name="mpeg4-generic",codec_type="audio",stream="test",track_index="0",vhost="__defaultVhost__"} 3.57
# HELP zlm_stream_track_fps Video track frames per second
# TYPE zlm_stream_track_fps gauge
zlm_stream_track_fps{app="live",codec_name="H264",codec_type="video",stream="test",track_index="1",vhost="__defaultVhost__"} 26
# HELP zlm_stream_track_frames_total Frames carried by the track
# TYPE zlm_stream_track_frames_total counter
zlm_stream_track_frames_total{app="live",codec_name="H264",codec_type="video",stream="test",track_index="1",vhost="__defaultVhost__"} 149
zlm_stream_track_frames_total{app="live",codec_name="mpeg4-generic",codec_type="audio",stream="test",track_index="0",vhost="__defaultVhost__"} 187
# HELP zlm_stream_track_gop_interval_seconds Video track GOP interval in seconds
# TYPE zlm_stream_track_gop_interval_seconds gauge
zlm_stream_track_gop_interval_seconds{app="live",codec_name="H264",codec_type="video",stream="test",track_index="1",vhost="__defaultVhost__"} 0.801
# HELP zlm_stream_track_gop_size Video track GOP size in frames
# TYPE zlm_stream_track_gop_size gauge
zlm_stream_track_gop_size{app="live",codec_name="H264",codec_type="video",stream="test",track_index="1",vhost="__defaultVhost__"} 21
# HELP zlm_stream_track_height Video track height in pixels
# TYPE zlm_stream_track_height gauge
zlm_stream_track_height{app="live",codec_name="H264",codec_type="video",stream="test",track_index="1",vhost="__defaultVhost__"} 960
# HELP zlm_stream_track_key_frames_total Video track key frames
# TYPE zlm_stream_track_key_frames_total counter
zlm_stream_track_key_frames_total{app="live",codec_name="H264",codec_type="video",stream="test",track_index="1",vhost="__defaultVhost__"} 2
# HELP zlm_stream_track_ready Whether the track is ready (1: ready, 0: not)
# TYPE zlm_stream_track_ready gauge
zlm_stream_track_ready{app="live",codec_name="H264",codec_type="video",stream="test",track_index="1",vhost="__defaultVhost__"} 1
zlm_stream_track_ready{app="live",codec_name="mpeg4-generic",codec_type="audio",stream="test",track_index="0",vhost="__defaultVhost__"} 1
# HELP zlm_stream_track_sample_bit Audio track sample bit depth
# TYPE zlm_stream_track_sample_bit gauge
zlm_stream_track_sample_bit{app="live",codec_name="mpeg4-generic",codec_type="audio",stream="test",track_index="0",vhost="__defaultVhost__"} 16
# HELP zlm_stream_track_sample_rate Audio track sample rate in hertz
# TYPE zlm_stream_track_sample_rate gauge
zlm_stream_track_sample_rate{app="live",codec_name="mpeg4-generic",codec_type="audio",stream="test",track_index="0",vhost="__defaultVhost__"} 44100
# HELP zlm_stream_track_width Video track width in pixels
# TYPE zlm_stream_track_width gauge
zlm_stream_track_width{app="live",codec_name="H264",codec_type="video",stream="test",track_index="1",vhost="__defaultVhost__"} 448
# HELP zlm_streams Number of source streams
# TYPE zlm_streams gauge
zlm_streams 1
# HELP zlm_up Was the last scrape of ZLMediaKit successful.
# TYPE zlm_up gauge
zlm_up 1
# HELP zlm_version_info ZLMediaKit version info.
# TYPE zlm_version_info gauge
zlm_version_info{branch_name="master",build_time="2024-06-11T21:28:30",commit_hash="c446f6b"} 1
# HELP zlm_work_thread_delay_seconds Work thread task delay in seconds
# TYPE zlm_work_thread_delay_seconds gauge
zlm_work_thread_delay_seconds{index="0"} 0.005
zlm_work_thread_delay_seconds{index="1"} 0.005
zlm_work_thread_delay_seconds{index="2"} 0.005
zlm_work_thread_delay_seconds{index="3"} 0.005
zlm_work_thread_delay_seconds{index="4"} 0.005
zlm_work_thread_delay_seconds{index="5"} 0.005
zlm_work_thread_delay_seconds{index="6"} 0.005
zlm_work_thread_delay_seconds{index="7"} 0.005
# HELP zlm_work_thread_load_percent Work thread load in percent
# TYPE zlm_work_thread_load_percent gauge
zlm_work_thread_load_percent{index="0"} 0
zlm_work_thread_load_percent{index="1"} 0
zlm_work_thread_load_percent{index="2"} 0
zlm_work_thread_load_percent{index="3"} 100
zlm_work_thread_load_percent{index="4"} 0
zlm_work_thread_load_percent{index="5"} 0
zlm_work_thread_load_percent{index="6"} 0
zlm_work_thread_load_percent{index="7"} 0
# HELP zlm_work_threads Number of work threads
# TYPE zlm_work_threads gauge
zlm_work_threads 8
```

</details>

## Roadmap

- [x] 添加 Git Action CI/CD，并触发 Docker 构建和推送到 Docker Hub
- [x] GA
- [x] 添加 Grafana 仪表板 / Prometheus 告警使用示例
- [x] 高基数指标改为可选（`--zlm.expose-*`、`--zlm.disable-collectors`）
- [x] 适配最新 ZLMediaKit：track 明细、录制状态、拉流/推流代理
- [ ] 添加更多测试

## 贡献和报告问题

欢迎～


## 致谢
[ZLMediaKit](https://github.com/ZLMediaKit/ZLMediaKit)

[JetBrains](https://www.jetbrains.com/)

[redis_exporter](https://github.com/oliver006/redis_exporter)

[haproxy_exporter](https://github.com/prometheus/haproxy_exporter)

[Prometheus](https://prometheus.io/)

[Cursor](https://www.cursor.com/)

JetBrains/Cursor 为编码提供了出色的工具。

大多数单元测试由 Cursor 自动生成。
