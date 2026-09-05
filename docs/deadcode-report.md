# go-sicky Deadcode Baseline Report

> Generated: 2026-09-05. Policy: report + gate, no deletion.
> `P0` = confirmed dead but deliberately kept. `P1` = defensive/unreachable-via-orchestrator.
> `P2` = reserved placeholders / compat shims. CLI orphans listed here were
> **wired back in** (see §4) and are no longer dead.

## P0 — confirmed dead code, deliberately kept (~270 lines / ~11KB)

| Location | Size | Reason to keep |
|---|---|---|
| `registry/mdns/mdns.go:33-239`, `registry/mdns/config.go:33-49` | 224 lines | Deprecated by decision; comments kept, no `zeroconf` dep will be added |
| `registry/consul/watcher.go:63-127` `HybridHandler` | 64 lines | Superseded by `Load()+PurgePool` at `:129`; safe-delete candidate if policy changes |
| `client/http/http.go:80-103`, `client/websocket/websocket.go:78-99` | 24+22 lines | Legacy functional-options comments; safe-delete candidate |
| `server/grpc/tracer.go:123-125` | 3 lines | Superseded by `metadata.Join` at `:128`; safe-delete candidate |
| `server/grpc/metadata.go:40-52` `NewMetadataInterceptor` | 13 lines | Placeholder for future baggage propagation; 0 callers by design, now marked `Deprecated: do not mount` |
| `tracer/fiber.go:79-122`, `logger/fiber.go:106-191` `NewFiberMiddleware` x2 | ~330 lines live-but-deprecated | Never mount logger variant alongside `server/fiber` built-in chain (double-counts `sicky_server_requests_total{server="fiber"}`); tracer variant is counter-free and safe to stack |
| `registry/pool.go NewPool/SetPool` | 2 funcs | Now marked `Deprecated`: `NewPool` creates a pool disconnected from global state, `SetPool` swaps the `Notify` channel under watchers; use `InitPool`/`PurgePool` instead. Instance methods (`RegisterService/GetService/...`) stay — they serve snapshot readers |
| `server/http/cors.go:86-95` `CORSMiddleware` | 10 lines | Deny-all alias; use `NewCORSMiddleware` with explicit whitelist |

## P1 — defensive / unreachable-via-orchestrator (kept as defense-in-depth)

- `infra/*: if cfg == nil { return nil, nil }` (10 files: `badger.go:55`, `bun.go:97`, `clickhouse.go:72`, `elastic.go:89`, `mongo.go:71`, `mqtt.go:91`, `nats.go:115`, `redis.go:75`, `ristretto.go:62`, `s3.go:110`) — `sicky.go:455-622` already gates on non-nil; only direct calls hit these.
- `infra/bun.go:112-175 default: ErrBunUnsupportedDriver` — unreachable after `Validate()` rejects unknown drivers; kept as defense.
- `sicky.go:697 default: Warn Unknown` tracer selector + `config.go:262 case ""` — unreachable after `Ensure()+Validate()`; only direct `New()` with unvalidated config hits them.
- `server/*` constructors called `opts.Logger.Fatal` without `return` — unreachable with the default logger, but with a mock logger execution continued with nil addr. Fixed 2026-09-05: all 12 sites across the 6 constructors now `return nil` after `Fatal` (callers must nil-check, same convention as registry/tracer/broker constructors).
- `Err*` sentinels joined into `Run()` but never `errors.Is`-checked in prod: `sicky.go:112,115,118,121,127,133`, `broker/* ErrBrokerNotConnected`, `server/udp ErrObtainUDPAddress/ErrServerNotRunning`, `config.go:251 ErrTracerNoEndpoint` (0 returns, OTLP uses exporter default), `service/mcp/protocol ErrInternalError/ErrServerNotInitialized`. Fixed 2026-09-05: fiber `ErrShutdownTimeout` is now joined when `ShutdownWithTimeout` surfaces `context.DeadlineExceeded` (verified against fasthttp `ShutdownWithContext`, which returns `ctx.Err()` on timeout), mirroring the gRPC server.
- Config read-but-dropped by design: `Tracer.SampleRate` ignored for `uptrace` (server-side sampling), `PrettyPrint/Timestamps` stdout-only, `fiber` has no `ReadHeaderTimeout` field.
- Empty `Config struct{}` placeholders (7): `server`, `client`, `tracer`, `job`, `broker`, `runner`, `runner/static` — all have nil-safe `Ensure()`; only `broker.Config` is referenced (`sicky/config.go:317`).
- Zero-caller exported registry helpers (public API compat, test-only `Clear()`): `broker/server/client/job/runner Get/Default/List`, `tracer Get/Tracers/Provider/Propagator`, `registry Get/Registries/GetService/RegisterService/GetInstance/UnregisterInstance` (+ now-`Deprecated` `NewPool/SetPool`), `service.Get`, `utils PrintContextInternals/JSONAny/XMLAny/E + ObtainIPs/Net2fd`, `metrics Unregister/UnregisterAll/Get`, `infra GetClickhouse/GetNats/ClearClickhouse` aliases, `infra GetMongoDB` (test-only), `broker.DefaultConfig`, `sicky.go TickerHander` alias.
  - Deliberately NOT marked `Deprecated` (2026-09-05 review): the `Set/Get/Default/List/Clear` sets are coherent public surface consumed by downstream business code (in-tree zero callers is expected — the orchestrator holds concrete refs); likewise `utils/*` generic helpers and `metrics` management API exist for external users. Only the two footguns (`NewPool`/`SetPool`, which break the stable-`NotifyChan` contract) were deprecated. The mis-cased `infra` aliases and typo aliases were already `Deprecated`.

## P2 — reserved / compat shims (kept)

- `cli/generate_extra.go:24 firstPositional` — reserved extension point, `//lint:ignore U1000`.
- `Deprecated:` aliases: `infra GetClickhouse/GetNats/ClearClickhouse`, `infra/clickhouse.go ErrClickhouseDSNEmpty/ClickhouseConfig/InitClickhouse`, `infra/nats.go Nats*`, `sicky.go TickerHander/ErrNatsBrokerNil/ErrNsqBrokerNil/ErrJetstreamBrokerNil`, `broker/message.go MsgJson/MsgMsgpack`, `broker/jetstream MaxConsummers`, `Jetstream/JetStream`, `Nsq/NSQ`, `Mcp/MCP` type aliases, `server/http TLSKeyPem tls_key_pem_deprecated`, `server/grpc DisableReflection`, `tracer/uptrace SampleRate`.
- Scaffold `not implemented` gotmpl stubs (`tool.go`, `resource.go`, `handler.go`) — fail loudly by design.
- `manager.go:633-646` health switch has no `default` and fast-paths only 6 infra names — confirmed by-design 2026-09-05, no change: `badger/ristretto/mqtt/nats` use `local` callbacks (`manager.go:605-620`), never the `ping` path the switch guards. No asymmetry.

## Gate whitelist (mirrors `.golangci.yml`)

`staticcheck U1000` exclusions cover exactly: `registry/mdns/*`, `server/grpc/metadata.go`, `logger/fiber.go + tracer/fiber.go NewFiberMiddleware`, `server/http/cors.go CORSMiddleware`, `cli/generate_extra.go firstPositional`, empty-`Config.Ensure`, `Deprecated` aliases, defense-in-depth `default` branches. Any **new** unused symbol outside this list fails CI.

## §4 — CLI orphans wired back (2026-09-05, no longer dead)

- `cli/sicky.go Run()` now dispatches: `serve/s`, `mcp`, `run/r`, `doctor`, `info/i`, `config/c`, `proto/p`, `new/n`, `generate/g`, `version/v`, `help`. `helpRun` documents all of them.
- `cli/mcp.go serveMCP()` is the single implementation; `serveRun` delegates (dedup; also gains the `pflag.ErrHelp → 0` handling `serve` previously lacked).
- `cli/generate.go generateRun` now dispatches all `generate_extra.go` stubs: `server, client, service, broker, job, middleware, proto, config, docker, k8s` (plus existing `handler, tool, resource, doc`). `generateProtoFile` is shared with `sicky proto new`.
- `cli/run.go` fixed: custom `--config` now also forwards non-default `--config-type` (previously silently dropped, forcing json parsing).
- Build fix included: `cli/new.go newContext` gained `Servers/Registry/Tracer/DockerRegistry` compat fields + `normalizeNewContext`; new `cli/generate_flags.go` provides the missing `parseGenerateFlags/writeGuard` helpers (`generate_extra.go` + `config_cmd.go` referenced them but they were never committed — `go build ./...` failed before this change).
