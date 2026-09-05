# Metrics Catalog (`sicky_*`)

All metrics are registered in `metrics/metrics.go:init()` and exposed via the
Manager `/metrics` endpoint (public, Prometheus scraping). The legacy
unlabelled `num_*` counters were removed — this is a breaking change, see the
mapping table below.

Helpers in `metrics/metrics.go` keep call sites to one line:
`ObserveServerRequest`, `ObserveClientRequest`, `ObserveBrokerPublish`,
`ObserveBrokerHandler`, `ObserveJobRun`, `ObserveRegistryOp`,
`ObserveInfraOp`, `CountInfraInit`, `SetInfraUp`, `ResultOf`.

Histogram buckets: RPC/HTTP `{.005,.01,.025,.05,.1,.25,.5,1,2.5,5,10}`;
infra slow paths `{.01,.05,.1,.25,.5,1,2.5,5,15,30}`.

## Server (6 protocols)

| Metric | Labels |
|---|---|
| `sicky_server_requests_total` | `server,method,route,code` |
| `sicky_server_request_duration_seconds` | `server,method,route` |
| `sicky_server_rejected_total` | `server,reason` (`session_cap,message_cap,rate_limit,origin,connect`) |
| `sicky_server_connections` | `server` (gauge) |
| `sicky_server_panics_total` | `server,method` |
| `sicky_server_io_bytes_total` | `server,direction` |

gRPC code is the real `status.Code` (not hardcoded 200). HTTP/Fiber route is
the route template. `Call()`-less protocols (TCP/UDP/WS) use
`method=data|datagram|text|binary`.

## Client (5 protocols)

| Metric | Labels |
|---|---|
| `sicky_client_requests_total` | `client,method,host,code` |
| `sicky_client_request_duration_seconds` | `client,method,host` |
| `sicky_client_errors_total` | `client,method,reason` |
| `sicky_client_stream_open_total` | `client,method,result` |

`Call()` placeholders only bump `{method="noop"}`. Real counts: gRPC
`Invoke` + stream opens, HTTP `Do`. TCP/UDP/WS clients have no transport yet.

## Broker (nats / jetstream / nsq)

| Metric | Labels |
|---|---|
| `sicky_broker_publish_total` / `_duration_seconds` | `broker,topic[,result]` |
| `sicky_broker_handler_total` / `_duration_seconds` | `broker,topic,result=ok/error/panic/acked/nacked/requeued` |
| `sicky_broker_subscribe_total` | `broker,topic,result=ok/error/dup` |
| `sicky_broker_connected` | `broker` (gauge 1/0) |

`result` is full-fidelity on `topic` by decision — watch `/metrics` line
count after adding high-cardinality topics.

## Job (cron / ticker)

| Metric | Labels |
|---|---|
| `sicky_job_runs_total` / `sicky_job_run_duration_seconds` | `job,task,result=ok/error/timeout/panic` |
| `sicky_job_ticks_total` | `job,result=fired/skipped` |
| `sicky_job_registered_total` | `job,result` |
| `sicky_job_running` | `job` (gauge) |

Timeout expiry reports failure; the leaked run cannot be killed (documented).
Non-timeout cron handlers are wrapped too (panic re-panics after recording).

## Runner (static pool)

| Metric | Labels |
|---|---|
| `sicky_runner_submit_total` | `runner,result=enqueued/dropped_not_started/dropped_stopping` |
| `sicky_runner_try_submit_total` | `runner,result=enqueued/full/full_timeout/dropped_*` |
| `sicky_runner_task_runs_total` / `_duration_seconds` | `runner,result=ok/error/panic` |
| `sicky_runner_queue_depth` / `_inflight` / `_workers` | `runner` (gauges) |

## Registry (consul / redis / local)

| Metric | Labels |
|---|---|
| `sicky_registry_ops_total` / `_op_duration_seconds` | `backend,op=register/deregister/load/check/watch,result` |
| `sicky_registry_instances` | `backend` (gauge, set on Load) |
| `sicky_registry_watch_events_total` | `backend,type=reload/error` |

`check` has no histogram (high-frequency probe); `missing` is a separate
result from `error`.

## Infra (10 components)

| Metric | Labels |
|---|---|
| `sicky_infra_ops_total` / `_op_duration_seconds` | `infra,op,result` |
| `sicky_infra_up` | `infra` (gauge 1/0; `Clear*` drops to 0) |
| `sicky_infra_init_total` | `infra,result` |

Auto-observed operations (no call-site changes): Redis every command
(`ProcessHook`, op = command name), Bun every query (`QueryHook`, op =
`Operation()`), Mongo every command (`CommandMonitor`, server-reported
latency). Elastic records `ping`, S3 records `head_bucket` (both fire on
`/health` probes). Badger/Ristretto/MQTT/NATS record init/up only — business
operations through them should call `metrics.ObserveInfraOp` (helpers are
exported for this). `uri/url/dsn` must NEVER enter labels.

## Manager / orchestrator

| Metric | Labels |
|---|---|
| `sicky_manager_health_check_duration_seconds` | `component` (10 infra + business checkers) |
| `sicky_config_reloads_total` | `result` (SIGHUP wrappers) |
| `sicky_service_starts_total` | `service,result` |

Plus the standard `build_info`, `go_*`, `process_*` collectors.

## Legacy mapping (removed)

`num_grpc/http/fiber/tcp/udp/websocket_server_access` →
`sicky_server_requests_total{server=...}` (+ duration);
`num_grpc/http/tcp/udp/websocket_client_call` →
`sicky_client_requests_total{client=...}` (+ duration).
