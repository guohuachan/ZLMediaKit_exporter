# ZLMediaKit Exporter 现代化改造设计

- 日期：2026-08-28
- 状态：已确认，实施中
- 目标版本：v1.0.0（含破坏性变更）

## 背景

Exporter 最后一次实质更新停留在 2025 年初，对照 ZLMediaKit `master`
（`server/WebApi.cpp` / `server/WebApi.h`）已经出现三类问题：

1. **真 bug**：TLS 校验语义反转、指标标签错位、抓取错误计数器从未注册、
   HTTP 请求未携带 context、超时后 goroutine 仍写 channel。
2. **与新版 API 不兼容**：`listRtpServer` 的 `port` 在 master 中是数字，
   exporter 声明为 `string`，导致整个 RTP 采集 JSON 解析失败。
3. **可运维性**：高基数指标默认开启、指标命名不符合 Prometheus 规范、
   依赖停滞一年以上。

## 决策

| 议题 | 决定 |
|---|---|
| 向后兼容 | 允许破坏，发 v1.0.0 并提供迁移文档 |
| 高基数指标 | `zlm_session_info` / `zlm_api_status` 默认关闭，flag 开启 |
| 范围 | 工程基线 + 重构 + bug 修复 + 新版适配 + 命名治理 + 文档 |
| 代码结构 | 拆分为 `internal/zlmapi` + `internal/collector` |
| 多实例抓取 | **不做**，保持单实例模型 |

## 架构

```
main.go                     flag 解析 + web server
internal/zlmapi/
  client.go                 HTTP client：context 传递、超时、secret、错误封装
  types.go                  与 ZLM master 对齐的响应结构体
internal/collector/
  collector.go              Collector 接口 + Exporter 聚合、up、scrape_duration
  version.go  threads.go  statistics.go  session.go
  stream.go   rtp.go      proxy.go      apistatus.go
```

`Collector` 接口：

```go
type Collector interface {
    Name() string
    Describe(ch chan<- *prometheus.Desc)
    Collect(ctx context.Context, c *zlmapi.Client, ch chan<- prometheus.Metric) error
}
```

Exporter 用 `errgroup` 并发执行全部 collector，并**无条件 `Wait()`**。
超时由每个 HTTP 请求的 context 负责，保证所有 goroutine 在 `Collect` 返回前结束，
消除「向已关闭 channel 写入」的 panic 风险。

## 指标变更

### 改名

| 旧 | 新 | 理由 |
|---|---|---|
| `zlm_stream_bitrate` | `zlm_stream_bytes_per_second` | 源字段 `bytesSpeed` 单位是 B/s |
| `zlm_stream_alive_second` | `zlm_stream_alive_seconds` | 单位后缀规范 |
| `zlm_stream_create_stamp` | `zlm_stream_create_time_seconds` | 单位后缀规范 |
| `zlm_network_threads_total` | `zlm_network_threads` | `_total` 保留给 counter |
| `zlm_work_threads_total` | `zlm_work_threads` | 同上 |

### 删除（PromQL 可等价还原）

| 删除 | 等价查询 |
|---|---|
| `zlm_network_threads_load_total` | `sum(zlm_network_thread_load_percent)` |
| `zlm_network_threads_delay_total` | `sum(zlm_network_thread_delay_milliseconds)` |
| `zlm_work_threads_load_total` | `sum(zlm_work_thread_load_percent)` |
| `zlm_work_threads_delay_total` | `sum(zlm_work_thread_delay_milliseconds)` |

### 新增

- per-thread：`zlm_network_thread_load_percent{index}`、
  `zlm_network_thread_delay_milliseconds{index}`（work 同族）。
- `zlm_stream_bytes_total`（源字段 `totalBytes`，counter）。
- `zlm_stream_recording{vhost,app,stream,type="mp4"|"hls"}`。
- track 指标族：`zlm_stream_track_{ready,frames_total,fps,width,height,
  gop_size,gop_interval_milliseconds,key_frames_total,sample_rate,channels,sample_bit}`，
  label `{vhost,app,stream,codec_type,codec_name,track_index}`。
  **track 是源流属性、与 schema 无关**，按 `vhost/app/stream` 去重后只输出一次，
  避免基数被 schema 数量放大。
- `zlm_session_count{typeid,type}` 聚合指标（默认开启）。
- proxy 族：`zlm_stream_proxy_{info,status,live_seconds,repull_total,
  bytes_per_second,bytes_total,total}` 与 `zlm_stream_pusher_*`（`republish_total`）。
- exporter 自身：`zlm_exporter_scrape_duration_seconds{collector}`、
  `zlm_exporter_collector_success{collector}`、
  `zlm_exporter_scrape_errors_total{collector}`（首次真正注册）、
  `zlm_exporter_build_info`。

### 修复但不改名

- `zlm_stream_total_reader_count`：标签顺序由 `app,stream,vhost` 纠正为 `vhost,app,stream`。
- `zlm_rtp_server_info`：`port` 改为数字解析，新增 `vhost`/`app`/`ssrc`/`tcp_mode` label。
- `zlm_session_info`：新增 `type` label（tcp/udp）。

### 明确不做

`getServerConfig` 不做成指标。该接口响应中包含 `api.secret`，
将其做成 label 会把 API 密钥直接泄露进 Prometheus。

## Flag 变更

| 动作 | Flag | 说明 |
|---|---|---|
| 废弃 | `--web.ssl-verify` | 语义反转且命名有歧义 |
| 新增 | `--zlm.tls-insecure-skip-verify` | 默认 `false`（校验证书） |
| 新增 | `--zlm.expose-api-status` | 默认 `false` |
| 新增 | `--zlm.expose-session-info` | 默认 `false` |
| 新增 | `--zlm.expose-stream-tracks` | 默认 `true` |
| 新增 | `--zlm.disable-collectors` | 逗号分隔，如 `proxy,api` |
| 修复 | `--web.timeout` | 真正作用于 HTTP client 与 scrape context |

**升级风险**：TLS 行为反转后，使用自签证书的部署会开始失败，
需显式添加 `--zlm.tls-insecure-skip-verify`。迁移文档需在顶部醒目标注。

## 测试策略

- 移除 gin（仅测试使用，拖入约 25 个间接依赖），改用
  `httptest.NewServer` + `http.ServeMux`。
- 断言由手工逐条改为 `testutil.CollectAndCompare` 比对 exposition 文本，
  并用 `testutil.CollectAndLint` 守住命名规范。
- testdata 依据 ZLM master 真实响应重新生成；同时保留一份旧版 fixture
  作为向下兼容回归测试，确保 6.x/7.x 老实例仍可采集。

## 实施顺序

先整理、再修复、再新增（tidy-first），每步测试保持绿、可独立 review。

| 步 | 内容 | 行为变更 |
|---|---|---|
| 1 | 工程基线：Go 1.25、依赖升级、移除 gin、golangci、CI、Dockerfile | 无 |
| 2 | 重构到 `internal/{zlmapi,collector}`，测试改写 | 无 |
| 3 | 7 个 bug 修复（先写失败测试） | 修正 |
| 4 | 适配最新 ZLM：类型修复、新字段、新 collector | 新增 |
| 5 | 指标改名/删除、高基数开关、exporter 自身指标 | 破坏性 |
| 6 | README / README_CN / Grafana dashboard / MIGRATION.md，打 v1.0.0 | 文档 |

## 实施偏差

实施过程中引入 promlint 门禁（`testutil.CollectAndLint`）后，它给出的
不合规清单比本设计原先列出的五项改名更长。为避免「修一半」——只清理
线程指标的 `_total` 后缀却保留流指标的——按同一规则一次改到位：

额外改名：`zlm_stream_total` → `zlm_streams`、`zlm_session_total` →
`zlm_sessions{typeid,type}`、`zlm_rtp_server_total` → `zlm_rtp_servers`、
`zlm_stream_proxy_total` → `zlm_stream_proxies`、`zlm_stream_pusher_total`
→ `zlm_stream_pushers`、`zlm_stream_reader_count` → `zlm_stream_readers`、
`zlm_stream_total_reader_count` → `zlm_stream_total_readers`、
`zlm_stream_proxy_total_reader_count` → `zlm_stream_proxy_total_readers`、
track 的 `_milliseconds` 指标改用 seconds 基本单位、`zlm_version_info`
的标签由 camelCase 改为 snake_case。

另外 Go 版本下限由计划中的 1.24 变为 1.25.0：prometheus 全家桶
（client_golang v1.24.1、common v0.70.1、exporter-toolkit v0.19.0）
的 go.mod 已要求 1.25，无法回退。

完整迁移表见 [MIGRATION.md](../../MIGRATION.md)。
