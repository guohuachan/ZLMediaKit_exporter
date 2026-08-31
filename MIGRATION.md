# Migration to v1.0.0

v1.0.0 adapts the exporter to current ZLMediaKit and brings the metric names in
line with the Prometheus conventions. Both changes are breaking. This document
lists everything that moved and how to rewrite queries and dashboards.

## Read this first: TLS verification now happens

Before v1.0.0 the `--web.ssl-verify` flag was inverted: setting it to `true`
(the default) installed a transport with `InsecureSkipVerify: true`. The default
configuration therefore accepted **any** certificate.

The flag is gone. Certificates are verified by default:

```
--web.ssl-verify              # removed
--zlm.tls-insecure-skip-verify   # new, default false
```

`ZLM_EXPORTER_SSL_VERIFY` is replaced by
`ZLM_EXPORTER_TLS_INSECURE_SKIP_VERIFY`.

If ZLMediaKit is behind a self-signed certificate, a scrape that used to work
will now fail with `x509: certificate signed by unknown authority`. Add
`--zlm.tls-insecure-skip-verify` to keep the old behaviour, or install the CA.

The old flag was removed rather than silently ignored, so an unchanged command
line fails loudly instead of quietly changing behaviour.

## Metrics that are no longer exposed by default

| Metric | How to get it back | Why |
|---|---|---|
| `zlm_api_status` | `--zlm.expose-api-status` | One constant `1` per ZLMediaKit endpoint, around seventy series carrying no signal. |
| `zlm_session_info` | `--zlm.expose-session-info` | One time series per connection, with connection id and ports as labels. `zlm_sessions` carries the aggregate. |

`--zlm.disable-collectors` takes a comma-separated list of collector names to
skip entirely: `version`, `api`, `network_threads`, `work_threads`,
`statistics`, `session`, `stream`, `stream_proxy`, `stream_pusher`, `rtp`.

## Renamed metrics

Gauges no longer carry the `_total` or `_count` suffixes that Prometheus
reserves for counters and for histogram/summary components, and durations use
seconds as the base unit.

| Before | After |
|---|---|
| `zlm_stream_bitrate` | `zlm_stream_bytes_per_second` |
| `zlm_stream_alive_second` | `zlm_stream_alive_seconds` |
| `zlm_stream_create_stamp` | `zlm_stream_create_time_seconds` |
| `zlm_stream_reader_count` | `zlm_stream_readers` |
| `zlm_stream_total_reader_count` | `zlm_stream_total_readers` |
| `zlm_stream_total` | `zlm_streams` |
| `zlm_session_total` | `sum(zlm_sessions)` |
| `zlm_rtp_server_total` | `zlm_rtp_servers` |
| `zlm_network_threads_total` | `zlm_network_threads` |
| `zlm_work_threads_total` | `zlm_work_threads` |
| `zlm_scrape_errors_total{endpoint}` | `zlm_exporter_scrape_errors_total{collector}` |

`zlm_stream_bitrate` was never a bitrate: the value is ZLMediaKit's `bytesSpeed`
field, in **bytes** per second. If a dashboard multiplied it by 8, it was right
by accident; the name now says what the value is.

`zlm_version_info` label names changed from camelCase to snake_case:
`branchName` → `branch_name`, `buildTime` → `build_time`,
`commitHash` → `commit_hash`.

## Removed metrics

Thread load and delay are now reported per thread. The pool size tracks the CPU
count, so the cardinality stays bounded and the aggregates are one `sum()` away.

| Removed | Equivalent |
|---|---|
| `zlm_network_threads_load_total` | `sum(zlm_network_thread_load_percent)` |
| `zlm_network_threads_delay_total` | `sum(zlm_network_thread_delay_seconds) * 1000` |
| `zlm_work_threads_load_total` | `sum(zlm_work_thread_load_percent)` |
| `zlm_work_threads_delay_total` | `sum(zlm_work_thread_delay_seconds) * 1000` |

The delay is now in seconds; the old metric was in milliseconds. Average load
across the pool, which is usually what a dashboard wants, is
`avg(zlm_network_thread_load_percent)`.

## Changed labels

| Metric | Change |
|---|---|
| `zlm_stream_total_readers` | The label **values** were previously scrambled: the descriptor declared `(vhost, app, stream)` while the collector passed `(app, stream, vhost)`. Any query or recording rule that worked around the old order must be corrected. |
| `zlm_rtp_server_info` | `(port, stream_id)` → `(vhost, app, stream_id, port, ssrc, tcp_mode)`. |
| `zlm_session_info` | New `type` label (`tcp` or `udp`). |

## New metrics

- `zlm_stream_bytes_total` — total bytes transferred, per stream and schema.
- `zlm_stream_recording{type="mp4"|"hls"}` — recording state, per stream.
- `zlm_stream_track_*` — per-track detail: `ready`, `frames_total`,
  `duration_seconds`, and for video `fps`, `width`, `height`, `gop_size`,
  `gop_interval_seconds`, `key_frames_total`, for audio `sample_rate`,
  `channels`, `sample_bit`. Reported once per source stream, not per schema.
  Disable with `--zlm.expose-stream-tracks=false`.
- `zlm_stream_proxy_*` and `zlm_stream_proxies` — pull proxies
  (`listStreamProxy`).
- `zlm_stream_pusher_*` and `zlm_stream_pushers` — push proxies
  (`listStreamPusherProxy`).
- `zlm_sessions{typeid,type}` — session count by class and transport.
- `zlm_exporter_scrape_duration_seconds{collector}`,
  `zlm_exporter_collector_success{collector}`,
  `zlm_exporter_scrape_errors_total{collector}` — per-collector scrape health.
  The error counter was previously created but never registered, so scrape
  failures were invisible; it is now exposed with a zero baseline per collector.
- `zlm_exporter_build_info` — version, revision and branch of the exporter.

## Compatibility with older ZLMediaKit

v1.0.0 targets current ZLMediaKit but keeps decoding older responses:
`listRtpServer`'s `port` is accepted both as a number (current) and as a quoted
string (older builds), and the fields added by newer builds — `totalBytes`,
`tracks`, the session `type` — are simply absent from the output when the server
does not report them. A regression test scrapes a legacy fixture set to keep
this working.
