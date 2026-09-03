# Go-Sicky Framework: Agent Development Guide

Welcome, Agent. This document provides a comprehensive overview of the **Go-Sicky** framework to assist you in understanding its architecture, conventions, and development patterns.

## 1. Project Overview
**Go-Sicky** is a highly modular and extensible Go framework designed for building robust microservices and networked applications. It provides high-level abstractions for common infrastructure, communication protocols, and service management, allowing developers to focus on business logic.

- **Primary Goal**: To provide a unified, pluggable architecture for microservices.
- **Key Features**:
  - Support for multiple protocols: HTTP dual-stack — Fiber (`server/fiber`) and net/http via `uptrace/bunrouter` (`server/http`) — plus gRPC, TCP, UDP, WebSocket (6 server implementations total).
  - Pluggable Infrastructure: SQL (Bun — PostgreSQL, MySQL, SQLite, MSSQL, DaMeng via `oracledialect`), NoSQL (Mongo), Cache (Redis, Ristretto), KV Store (Badger), Search (Elasticsearch), Analytics (Clickhouse), Messaging (NATS/MQTT infra), Storage (S3). Flat layout `infra/*.go` (10 files, no sub-packages, no shared interface).
  - Messaging & Brokers: NATS, JetStream, NSQ.
  - Service Discovery: Consul, Redis, Local (file-based JSON at `registry/local` — not in-memory). mDNS exists as `registry/mdns/` but is 100% commented out (planned, requires `zeroconf`).
  - Background Jobs: Cron and Ticker-based job scheduling.
  - Concurrent Task Runner: Goroutine-pool-based task dispatch (`runner/static`).
  - Observability: Prometheus metrics (11 counters + 3 collectors), OpenTelemetry tracing (OTLP/gRPC, OTLP/HTTP, Stdout, Uptrace + B3 propagator), built-in structured logger.
  - CLI Scaffolding: `sicky new/generate/serve/version` via `cli/` + `cmd/sicky` (MCP server stdio/http, project scaffolding for standard/mcp/interactive).

## 2. Core Architecture
The framework is organized into several key layers:

### 2.1 Orchestration (`sicky` — root package)
The root `sicky` package (`sicky.go`, `options.go`, `config.go`, `manager.go` all at repository root, `package sicky`) is the central nervous system. It handles:
- **Initialization**: Command-line flag parsing (`pflag`) and configuration loading (`viper`). Flags: `--config/-C` default `config`, `--config-type` default `json`, `--version/-V` prints `AppName Version (Branch) Build Commit BuildTime` and returns `ErrVersionShown` (the caller exits 0; the library never calls `os.Exit`). Additional `FlagSwitch` bool flags via `Init(opts, switches...)`, which now returns `error` (`ErrAlreadyInitialized` on second call — flag registration is not idempotent, guarded by mutex).
- **Configuration**: Support for local files (JSON etc.) and remote providers (e.g., Consul via `url.Parse(configLoc)` + `AddRemoteProvider`). Config files searched at `/etc`, `/etc/<app_name>`, `$HOME/.<app_name>` (via `os.UserHomeDir`, no longer a literal `$HOME` string), and the current directory. Env: `SICKY_` prefix + dot-to-underscore.
- **Lifecycle Management**: Orchestrating the start and stop of all registered services, servers, brokers, registries, tracers, and infrastructure. Actual `Run()` order: `beforeStart → infra → tracer → registry (InitPool first, then pool purge ticker with done-channel) → broker → flag callbacks → manager → services.Start + registry.Register → afterStart → signal wait (TERM/INT/QUIT/ABRT shutdown, HUP triggers OnReload callbacks, or Context cancel) → beforeStop → services.Stop+Deregister (failed service skipped, no double-stop) → broker.Disconnect → registry.Stop → tracer.Stop → manager.Stop → infra shutdown (errors collected via errors.Join) → afterStop`. All startup failures return a joined `error` and run the full shutdown sequence — no `os.Exit` inside the library.
- **Manager**: A built-in HTTP server (`manager.go NewManager`, nil-config safe) that exposes 6 endpoints: `/metrics` (public, for Prometheus scraping), `/health` (only `unhealthy` degrades overall status; `not_configured` is reported but healthy; checks run concurrently under a 2s ctx, Bun via `PingContext`), `/version`, `/info`, `/config` (403 if `ExposeConfig==false`, secrets redacted when exposed), `/services` (pool snapshot). `/config` + `/services` require `Authorization: Bearer <auth_token>` when `ManagerConfig.AuthToken` is set, else loopback-only. `http.Server` carries `Read/Write/IdleTimeout` (defaults 10/10/60s); default bind stays `:8888` (external) by project decision.

### 2.2 Infrastructure (`infra`)
Manages connections to external systems. Flat package `infra` with 10 files (`badger.go`, `bun.go`, `clickhouse.go`, `elastic.go`, `mongo.go`, `mqtt.go`, `nats.go`, `redis.go`, `ristretto.go`, `s3.go`), each with `*Config` struct + package-level global singleton + `Init*(*Config)` function. Unlike other subsystems, `infra` does not define a shared Go interface — each component is a standalone module with its own concrete type. Globals: `Badger *badger.DB`, `Bun *bun.DB`, `Clickhouse *ch.DB`, `Elastic *elasticsearch.Client`, `Mongo *mongo.Client`, `MQTT mqtt.Client`, `Nats *nats.Conn`, `Redis *redis.Client`, `Ristretto *ristretto.Cache[string,any]`, `S3 *s3.Client`.

### 2.3 Abstractions & Interfaces
Go-Sicky heavily relies on Go interfaces to maintain decoupling:
- **`service.Service`**: Represents a logical unit of business functionality. Manages subordinate servers, brokers, jobs, registries, and tracers. Implementations: `Standard` (background microservice), `Interactive` (CLI), `Mcp` (+ `mcp/protocol`).
- **`server.Server`**: Handles incoming requests on a specific protocol (Fiber HTTP, net/http via bunrouter, gRPC, TCP, UDP, WebSocket — 6 implementations).
- **`broker.Broker`**: Manages asynchronous message publishing and subscription (NATS, JetStream, NSQ).
- **`registry.Registry`**: Handles service registration and discovery (Consul, Redis, Local file-based JSON).
- **`tracer.Tracer`**: Manages OpenTelemetry spans (OTLP/gRPC, OTLP/HTTP, Stdout, Uptrace). Helper `tracer/fiber.go` for Fiber propagation.
- **`job.Job`**: Represents a background job (Cron via `gocron/v2`, Ticker).
- **`runner.Runner`**: Consumes and dispatches concurrent tasks (static goroutine pool at `runner/static`).
- **`client.Client`**: Outbound service client for various protocols (HTTP, gRPC, TCP, UDP, WebSocket).

### 2.4 Registration Pattern
Each abstraction layer uses a self-registration pattern via global state:
- The `New()` constructor of each implementation calls the package-level `Set(self)` function, adding itself to a global map (e.g. `broker.Set(brk)` at `broker/nats/nats.go:78`, `tracer.Set(trc)`, `server.Set(srv)` at `server/grpc/grpc.go:171`).
- The first registered instance becomes the "default" singleton, accessible via `package.Default()` (for `broker`, `registry`, `tracer`, `service`; `server` is an exception — see below).
- The orchestrator (`sicky.go`) creates instances via `New()` based on configuration presence, which triggers self-registration.
- **Exceptions**:
  - `server/server.go:75` has `Set/Get` but **no `Default()`** (unlike `broker/broker.go:71` etc.). The global map is now `sync.RWMutex`-guarded like the others.
  - In `sicky.go` the orchestrator uses concrete types (e.g. `*rgConsul.Consul`, `*rgRedis.Redis`, `*brkNats.Nats`) for implementation-specific methods like `Watch()`. This is an acknowledged exception to the "always use abstractions" rule, necessary when specific implementations expose methods not present in the interface.

## 3. Development Conventions

### 3.1 Component Pattern
Most components follow this structure (interface lives in `{package}.go`, not `interface.go`):
- `{package}.go` (e.g. `broker/broker.go:40`, `server/server.go:42`, `registry/registry.go:40`): Defines the core interface (`Broker`, `Server`, `Registry`, etc.).
- `options.go`: Defines the `Options` struct — runtime parameters (name, ID, logger, context, lifecycle hooks). Uses `Ensure()` to provide defaults.
- `config.go`: Defines the `Config` struct — Viper-serializable configuration (JSON/YAML/env). Uses `Ensure()` to provide defaults.
- `Ensure()`: A method on both `Options` and `Config` that fills in sensible default values for zero-valued or nil fields. Always returns the receiver (or a new instance if nil).

**Distinction**: `Options` carries runtime state and references (context, logger, hooks). `Config` carries serializable settings (addresses, timeouts, flags). Both must have `Ensure()`.

**Current gap**: Top-level `Config` structs for `server` (`server/config.go:33`), `client` (`client/config.go:33`), `tracer` (`tracer/config.go:33`), `job` (`job/config.go:33`) are empty (`type Config struct{}`) and lack `Ensure()` — only their sub-implementations (e.g. `server/grpc/config.go:97`, `tracer/grpc/config.go:56`) have `Ensure()`. `infra` is done: all 10 `*Config` types have `Ensure()` + `Validate()` (§3.5).

**Options duplicate**: `options.go:64` and `options.go:84` both check `if o.Context == nil { o.Context = context.Background() }` — harmless duplication.

### 3.2 Configuration
- **Viper** is used for all configuration.
- Environment variables are automatically mapped using the `SICKY_` prefix (e.g., `SICKY_APP_NAME` from `options.go:39 DefaultEnvPrefix`), with dot-to-underscore replacement (`sicky.go:156-158` `SetEnvPrefix`+`SetEnvKeyReplacer`+`AutomaticEnv()`).
- Config files are searched at `/etc`, `/etc/<app_name>`, `$HOME/.<app_name>`, and the current directory (`sicky.go:139-143`, `configLoc` default `config` at `sicky.go:82`, `configType` default `json` at `sicky.go:83`).
- No `context.TODO()` exists in the codebase (`grep` 0 hits) — use `context.Background()` for static initialization (`options.go:65`, `sicky.go:239`, `manager.go:78`, `broker/options.go:66` etc.) or pass a derived context for operations with timeouts.
- Viper's `AutomaticEnv()` performs type coercion for environment variables. Non-numeric values for `int` fields silently become zero. `validateConfig()` in `sicky.go` runs after `Ensure()` and **clamps** invalid values back to defaults with a Warn (never aborts): `Tracer.Timeout<0 → 0`, `SampleRate` outside `[0,1] → 1.0`, `Manager.Shutdown/Read/Write/IdleTimeout<=0 → defaults`, unknown `LogLevel` warns (behaves as info). Infra timing fields (e.g. `Ristretto.NumCounters`) are still not validated and will silently zero-out with a bad env var.

### 3.3 Lifecycle Hooks

The framework provides hooks at two levels:

**Sicky-level** (register via `sicky.BeforeStart()`, etc.):
- `BeforeStart`, `AfterStart` — executed before/after all services start.
- `BeforeStop`, `AfterStop` — executed before/after the full shutdown sequence (service stop, broker disconnect, registry stop, tracer stop, manager stop, infra shutdown).
- `OnReload` — executed on `SIGHUP` in a dedicated goroutine while the process keeps serving (register via `sicky.OnReload()`). `SIGHUP` no longer triggers shutdown.
- Wrapper signature: `SickyWrapper func(context.Context) error`.
- Hook failures log an error but do NOT abort the process (they use `ErrorContext`, not `Fatal`).

**Server-level** (set via `server.Options.BeforeStart()` etc. at `server/options.go:98`):
- `BeforeStart`, `AfterStart` — executed within each server's `Start()` method.
- `BeforeStop`, `AfterStop` — executed within each server's `Stop()` method.
- Wrapper signature: `ServerWrapper func() error` (`server/options.go:41` — no `context.Context` unlike `SickyWrapper`).
- Registration via `server.Options` setter methods: `opts.BeforeStart(w ...ServerWrapper)`. Invocation via `opts.RunBeforeStart()` etc. inside each server implementation (`server/grpc/grpc.go:211,317,331,344` etc.). Runners log failures via `ErrorContext` (`server/options.go:136`).

### 3.4 Error Handling
- Errors during service start/stop are aggregated using `errors.Join`. The orchestrator collects all errors and continues through the full shutdown sequence (via `goto shutdown` + `runErr` variable) even when individual services fail.
- Constructors (e.g., `New()`) for registry/tracer/broker implementations may return `nil` on initialization failure (`tracer/grpc/grpc.go:89,140`, `registry/consul/consul.go:76` etc.). Callers in `sicky.go` must check for nil before calling methods on the returned value. Tracer `nil` is logged as Warn (tracing silently disabled otherwise).
- `logger.Fatal()` calls `os.Exit(-1)` (`logger/logger.go:194`) and must NOT be used in the orchestrator or request paths — `sicky.go` no longer calls it (all startup failures return errors and run shutdown). Remaining `Fatal` calls in `server/*` constructors are tolerated for startup but avoid in request paths.
- All sentinel errors are defined as exported package-level variables (e.g., `broker/nats.ErrBrokerNotConnected`, `sicky.ErrVersionShown`, `sicky.ErrAlreadyInitialized`) to enable `errors.Is()` checks by callers.
- `broker.Message.Format()` and `Scan()` return errors for marshalling/unmarshalling failures — callers must check them.

### 3.5 Infra Conventions
- Each infra singleton is guarded by the package `mu sync.RWMutex` (`infra/infra.go`). Writers are `Init*` (first-wins) and `Clear*` (shutdown, resets to nil so a later `Run()` rebuilds fresh). **Readers must use the `Get*()` helpers** — reading the exported globals directly races with Init/Clear. When adding infra init functions, keep the first-wins duplicate handling (close the just-opened duplicate, return the existing singleton) instead of leaking it.
- Each infra type must have a **nil guard** before overwriting the global variable: `if Xxx == nil { Xxx = client }`. This prevents connection leaks on repeated `Init*()` calls. All 10 comply (`infra/badger.go:64`, `infra/bun.go:137`, `infra/clickhouse.go:73`, `infra/elastic.go:62`, `infra/mongo.go:86`, `infra/mqtt.go:65`, `infra/nats.go:56`, `infra/redis.go:74`, `infra/ristretto.go:74`, `infra/s3.go:57`).
- Infra `Init*()` functions are expected to emit Info logs on success and Error logs on failure, using `logger.Logger`. All 10 comply; credentials (DSN/URI userinfo, passwords, tokens, API keys) must never appear in log fields — use `redactDSN()` for DSN/URI and log only endpoint/username for the rest.
- Infra Config structs should implement `Ensure()` to provide defaults (currently a known gap — 10 types lack it, plus 4 top-level package configs — see §3.1 and TODO list).
- The infra shutdown sequence in `sicky.go` follows a specific order: Ristretto → Badger → Elastic → Nats → Redis → Bun → Clickhouse → S3 (log-only, `s3.Client` has no `Close()`) → Mongo (`Disconnect` with 5s timeout ctx) → MQTT (`Disconnect(250)` grace). Close/Disconnect errors are collected via `errors.Join` into the `Run()` return value, never aborting the sequence. Init order differs (Badger→Bun→Clickhouse→Elastic→MQTT→Mongo→Nats→Redis→Ristretto→S3) — not required to mirror shutdown.

### 3.6 Metrics Conventions
- Prometheus counters are defined in `metrics/metrics.go:45-114` (11 counters: 6 server +5 client) and 3 collectors (`build_info`, `go_collector`, `process_collector` via `collectors.NewBuildInfoCollector` etc. at `metrics/metrics.go:164-166`), all exported as package-level variables.
- Each counter/collector is registered in the `init()` function via `metrics.Register()` (`metrics/metrics.go:148`).
- The Manager's `/metrics` endpoint serves all registered collectors via a single `prometheus.Registry` created once during `NewManager()` (`manager.go:83-85` `prometheus.NewRegistry()` + `MustRegister(slices.Collect(maps.Values(metrics.GetAll())))`, served via `promhttp.HandlerFor` at `manager.go:196`).
- Counter `.Inc()` calls are placed at the following locations:
  - **Server counters**: In the access-log interceptor/middleware just before invoking the underlying handler (for gRPC `server/grpc/logger.go:93` and Fiber `server/fiber/logger.go:80`/`server/http/logger.go:91`), or before the `OnData` handler loop (for TCP `server/tcp/tcp.go:313`, UDP `server/udp/udp.go:244`, WebSocket `server/websocket/websocket.go:446`).
  - **Client counters**: As the first statement in the `Call()` method of each client implementation (`client/http/http.go:130`, `client/tcp/tcp.go:140`, `client/udp/udp.go:144`, `client/websocket/websocket.go:129`). **Exception**: gRPC client `client/grpc/grpc.go:225 Call()` is a stub; real increment is in `Invoke():253`; `NewStream()` has no counter.
- **Known metric bugs** (see TODOs): gRPC unary interceptors are assembled with a single `ChainUnaryInterceptor(tracing, logging)` / `WithChainUnaryInterceptor` call (merged — on grpc v1.83.2 repeated `Chain*` calls append rather than overwrite, so nothing was lost, but a single call is version-proof); streaming RPCs now have `ChainStreamInterceptor`/`WithChainStreamInterceptor` (tracing + logging + counters). `logger.NewFiberMiddleware` (`logger/fiber.go:113`) is deprecated legacy — never mount it alongside `server/fiber`'s built-in chain or `num_fiber_server_access` double-counts; `tracer/fiber.go` middleware has no counter and is safe to stack.

### 3.7 Manager Conventions
- `NewManager(cfg, appName, appVersion)` is nil-config safe (calls `Ensure()`) and creates `prometheus.Registry`, capturing `cfgVar` for `/config` exposure.
- 6 handlers: `MetricsPath` (`/metrics`, public), `HealthPath` (`/health` with `collectComponentHealth` for 10 infra types → `healthy/not_configured/unhealthy`; only `unhealthy` degrades overall status; checks run concurrently under a 2s ctx), `VersionPath`, `InfoPath`, `ConfigPath` (403 if `ExposeConfig==false`, secrets redacted when exposed), `ServicePoolPath` (`registry.GetPool()` snapshot). `/config` + `/services` require `Authorization: Bearer <auth_token>` when `ManagerConfig.AuthToken` is set, else loopback-only. `http.Server` carries `Read/Write/IdleTimeout` (defaults 10/10/60s); default bind stays `:8888` (external) by project decision.
- `Start()` does `wg.Add(1)` + `go func() { defer wg.Done(); srv.ListenAndServe() }`; `Stop()` does `Shutdown` with `ShutdownTimeout` then `wg.Wait()`. All Manager and server goroutines use `defer wg.Done()` (fixed in `server/http`, `server/tcp`, `server/udp`); `go func()` signatures carry no `error` return — errors are logged internally.

## 4. Key Packages & Directories
- `/broker`: Messaging abstraction (`broker/broker.go`) + NATS, JetStream, NSQ implementations.
- `/client`: Client abstraction + gRPC, HTTP, TCP, UDP, WebSocket implementations (top-level `client.Config` empty, sub-clients have `Ensure()`).
- `/cli`: CLI scaffolding — `sicky serve/new/generate/version/help` (`cli/sicky.go:46 Run()`), transports stdio/http.
- `/cmd/sicky`: Entrypoint (`cmd/sicky/main.go`).
- `/infra`: Infrastructure drivers — 10 files at `infra/*.go` (Redis, Bun/SQL, Badger, Ristretto, Mongo, Clickhouse, MQTT, NATS, S3, Elasticsearch). No interface — each is a standalone module. No `infra/interface.go`.
- `/internal`: Internal helpers (`internal/context.go`).
- `/job`: Job abstraction + Cron (`job/cron`), Ticker implementations.
- `/logger`: Built-in structured logger (`GeneralLogger` at `logger/general.go`), gRPC logger adapter (`logger/grpc.go`), Fiber middleware (`logger/fiber.go`).
- `/metrics`: Prometheus counter definitions and registry helpers (`metrics/metrics.go`, `metrics/config.go` legacy commented `StartMetrics` at `metrics/metrics.go:169`).
- `/registry`: Registry abstraction + Consul, Redis, Local (file-based JSON at `registry/local/local.go`) implementations. `registry/mdns/` exists but is 100% commented out.
- `/runner`: Runner abstraction + static goroutine-pool implementation (`runner/static`).
- `/runtime`: **Deprecated**. Previously the main orchestrator; `runtime/runtime.go` and `runtime/config.go` are almost entirely commented out (only `_ "viper/remote"` side-effect import remains at `runtime/config.go:34`). Subdirectories `docker/` and `nomad/` are empty placeholders (`.gitkeep` only).
- `/server`: Server abstraction (`server/server.go`, `server/options.go`, `server/config.go`, `server/session.go`) + 6 implementations: Fiber, gRPC, net/http (bunrouter), TCP, UDP, WebSocket.
- `/service`: Service abstraction + `Standard` (background microservice), `Interactive` (CLI), `Mcp` (+ `mcp/protocol`) implementations. Contains commented legacy `Run()` at `service/service.go:104-208`.
- `/tracer`: Tracer abstraction + OTLP/gRPC, OTLP/HTTP, Stdout, Uptrace implementations, plus Fiber helper (`tracer/fiber.go`).
- `/utils`: Helper functions for networking (IP resolution, `Net2fd` at `utils/net.go`), HTTP response envelopes (`utils/http.go`), metadata (`utils/metadata.go`), and debugging (`utils/debug.go`), with tests (`utils/*_test.go`).

## 5. Technology Stack
- **Go Version**: 1.26.0 (`go.mod:3`)
- **Main Dependencies**:
  - `github.com/gofiber/fiber/v2 v2.52.15`: Web framework (Fiber + WebSocket servers) + `gofiber/contrib/websocket v1.3.4` + `gofiber/swagger v1.1.1`.
  - `google.golang.org/grpc v1.83.2`: RPC framework (gRPC server + client) + `protobuf v1.36.12`.
  - `github.com/spf13/viper v1.21.0` + `viper/remote v1.21.0`: Configuration management (local files + remote providers via `_ "viper/remote"` at `sicky.go:61`).
  - `go.opentelemetry.io/otel v1.46.0`: OpenTelemetry tracing (SDK `v1.46.0`, exporters `otlptrace/grpc|http`, `stdouttrace`, B3 propagator `contrib/propagators/b3`).
  - `github.com/prometheus/client_golang v1.24.1`: Prometheus metrics.
  - `github.com/uptrace/bun v1.2.18` + dialects `pgdialect/mysqldialect/sqlitedialect/mssqldialect/oracledialect` + `pgdriver` + `bundebug` + `bunrouter v1.0.23`: SQL ORM (PostgreSQL, MySQL, SQLite, MSSQL, DaMeng via `oracledialect` fallback in `infra/bun.go:103`).
  - DB drivers (blank imports in `infra/bun.go:37`): `go-sql-driver/mysql v1.10.1`, `denisenkom/go-mssqldb v0.12.3`, `ncruces/go-sqlite3 v0.35.4`, plus `godoes/gorm-dameng v0.7.2`.
  - `github.com/redis/go-redis/v9 v9.22.0`: Redis client (infra + registry).
  - `github.com/nats-io/nats.go v1.53.1`: NATS Core + JetStream.
  - `github.com/nsqio/go-nsq v1.1.0`: NSQ client.
  - `github.com/hashicorp/consul/api v1.34.4`: Consul service discovery.
  - `github.com/dgraph-io/ristretto/v2 v2.4.2`: High-performance in-memory cache.
  - `github.com/dgraph-io/badger/v4 v4.9.6`: Embedded key-value store.
  - `github.com/uptrace/go-clickhouse v0.3.1`: Clickhouse client.
  - `github.com/elastic/go-elasticsearch/v9 v9.5.2`: Elasticsearch client.
  - `github.com/eclipse/paho.mqtt.golang v1.5.1`: MQTT client.
  - `github.com/aws/aws-sdk-go-v2/config v1.33.2` + `service/s3 v1.110.0`: AWS S3 client.
  - `github.com/go-co-op/gocron/v2 v2.22.0` + `robfig/cron/v3`: Cron scheduler.
  - `github.com/google/uuid v1.6.0`: UUID generation.
  - `github.com/vmihailenco/msgpack/v5 v5.4.1`: MessagePack serialization.
  - `github.com/uptrace/uptrace-go v1.43.0`: Uptrace tracer.
  - Other notable: `spf13/pflag v1.0.10`, `spf13/cast`, `fsnotify`, `color`, `clickhouse` etc. (see `go.mod:7-56`).

## 6. AI Agent Instructions
When working on this codebase:
1.  **Respect Interfaces**: Use the abstractions defined in `broker`, `server`, `registry`, etc., rather than implementation-specific types, except in the orchestrator (`sicky.go`) where concrete type access is necessary for implementation-specific methods (e.g., `Watch()`, `GracefulStop()`).
2.  **Surgical Edits**: Use targeted replacements to maintain the existing coding style and license headers.
3.  **Ensure Defaults**: When adding new configurations, always implement the `Ensure()` method on both `Options` and `Config` structs to provide defaults. Note top-level `server/client/tracer/job Config` currently lack `Ensure()` — add them when touching those packages. Infra adds `Validate()` next to `Ensure()`: presence-means-enabled abort on missing required fields; timing fields use zero-fills-default / negative-aborts.
4.  **Nil-Guard Globals**: When adding infra init functions, wrap the global variable assignment with `if Xxx == nil { Xxx = client }` (`infra/*.go:56-76`).
5.  **Wire Metrics**: When adding new server or client protocols, add a Prometheus counter in `metrics/metrics.go` and wire its `.Inc()` call at the appropriate insertion point (server: interceptor/middleware before handler or before `OnData` loop; client: first statement of `Call()`/`Invoke`). Never mount `logger.NewFiberMiddleware` (deprecated) alongside `server/fiber`'s built-in chain.
6.  **Follow the Manager**: Health checks and metrics should be exposed via the Manager HTTP server (`manager.go`) — do not start separate metric servers (legacy `metrics.StartMetrics` at `metrics/metrics.go:169` is commented out).
7.  **No `context.TODO()`**: Use `context.Background()` for static initialization, or propagate a proper context for operations that need cancellation or deadlines. The codebase has 0 `TODO` contexts.
8.  **Goroutine Cleanup**: All server/manager goroutines must use `defer wg.Done()` to prevent `wg.Wait()` from hanging in `Stop()`. Drop the `error` return type from `go func()` signatures — errors should be logged internally.
9.  **gRPC Interceptors**: Merge tracing + logging into a single `grpc.ChainUnaryInterceptor(tracing, logging)` / `grpc.WithChainUnaryInterceptor` call; do not call `ChainUnaryInterceptor` twice. Streaming RPCs must have `ChainStreamInterceptor`/`WithChainStreamInterceptor` (tracing + logging + counters).
10. **Config Validation**: Extend `validateConfig()` (`sicky.go`) when adding timing/size fields — Viper env zeroing is silent for `int` fields.

## 7. Known Gaps and TODOs
- [x] `infra/` Config types are missing `Ensure()` methods (10 types: `BadgerConfig`, `BunConfig`, `ClickhouseConfig`, `ElasticConfig`, `MongoConfig`, `MQTTConfig`, `NatsConfig`, `RedisConfig`, `RistrettoConfig`, `S3Config`).
- [ ] Top-level `Config` structs for `server`, `client`, `tracer`, `job` are empty and lack `Ensure()` (4 types at `server/config.go:33`, `client/config.go:33`, `tracer/config.go:33`, `job/config.go:33`).
- [x] Remote config loading requires the `_ "github.com/spf13/viper/remote"` import in `sicky.go:61` (fixed; note `runtime/config.go:34` also still imports it as side-effect).
- [x] gRPC interceptors merged into single `ChainUnaryInterceptor(tracing, logging)` / `WithChainUnaryInterceptor` calls; streaming RPCs now covered by `ChainStreamInterceptor`/`WithChainStreamInterceptor` (tracing + logging + counters). `server/grpc/metadata.go:40 NewMetadataInterceptor` stays unwired (no-op placeholder, kept deliberately).
- [ ] Fiber double-count: `logger/fiber.go` `NewFiberMiddleware` is deprecated legacy (kept, not maintained) — if mounted alongside `server/fiber`'s built-in chain, `num_fiber_server_access` double-counts. Never mount both.
- [ ] `runtime/` package is deprecated and contains only empty subdirectories (`docker/.gitkeep`, `nomad/.gitkeep`) and commented `runtime.go`/`config.go` — should be cleaned up.
- [ ] `service/` package has commented-out legacy `Run()` function at `service/service.go:104-208` — should be removed.
- [ ] `registry/mdns/` is entirely commented out (`mdns.go:33-248`, `config.go` all `//`) — should be removed or implemented with the `zeroconf` dependency.
- [ ] Infra `Init*` logging partially non-compliant: `Elastic`, `MQTT`, `Nats`, `S3` omit `Error` log on `NewClient/Connect` failure; `Bun` partially.
- [x] Test coverage was low — `server/` now has tests: `tcp/session_test.go` (SetKey, MaxSessions), `udp/session_test.go` (string-key index, rate limiter), `grpc/recovery_test.go` + `interceptors_test.go`, `http/middleware_test.go` (CORS, status, body-limit, sanitize, Ensure), `fiber/config_test.go` (Ensure, sanitize). Other packages still uncovered.

### Fixed in 2026-09-04 hardening pass (P0/P1)
- [x] Manager hardening: `AuthToken` Bearer guard on `/config`+`/services` (loopback-only without token), `/metrics` public, `Read/Write/IdleTimeout`, secrets redaction on `/config`, concurrent health checks (only `unhealthy` degrades), nil-safe `NewManager`, `RLock` in `Addr()/Port()`. Default bind stays `:8888` (external) by decision.
- [x] Orchestrator: `Init` returns `error` (`ErrVersionShown`, `ErrAlreadyInitialized`, no `os.Exit`); all `Run` startup failures join errors and run full shutdown; cancellable `Context`; `SIGHUP` → `OnReload()` instead of shutdown; force-shutdown timeout follows `Manager.ShutdownTimeout`; no double `Stop()` on failed service; shutdown errors collected.
- [x] Registry pool: fixed `GetInstance` RLock/Unlock mismatch and `RegisterService` RLock-on-write; `GetPool()` returns a deep-copy snapshot; `PurgePool` merges in place (stable pointer + `Notify`); pool purge ticker has a done-channel and `InitPool()` runs before ticker start.
- [x] `metrics.Get/GetAll` are now lock-guarded and `GetAll` returns a copy.
- [x] `MustInfra` wipe bug fixed (merge instead of reset); `validateConfig()` clamps invalid values with Warn; `$HOME` config path resolved via `os.UserHomeDir`; `logger.LogLevel("silence")` maps to `SilenceLevel`; infra shutdown uses Mongo 5s ctx + MQTT `Disconnect(250)`; `Manager.Stop` never blocks on a cancelless context.

### Fixed in 2026-09-04 hardening pass (P2)
- [x] `server/*` lifecycle: `server.go` map lock-guarded; `http/tcp/udp` use `defer wg.Done()` with no `error` return on `go func()`; `tcp/udp wg.Add` moved after successful `Listen`; `Running/Addr/IP/Port/Advertise*` read under `RLock`; `tcp/udp Stop` continues past `Close` errors; `http/websocket` TLS config failure is fail-fast; TCP tracks live connections (`conns` map + `wg`) and closes them on `Stop`.
- [x] gRPC interceptors merged (unary) + stream interceptors added (server + client, tracing + logging + counters); fixed client tracing nil-branch swallowing RPCs. `AGENTS.md` "Chain overwrite" claim corrected (grpc v1.83.2 appends).
- [x] `logger.NewFiberMiddleware` marked deprecated (code untouched); double-count documented as never-mount-both.
- [x] `Ensure()` added to all 10 `infra/*Config` types (Ristretto fills README defaults) and 4 empty top-level `Config` types; `sicky.Config.Ensure()` wires infra sub-configs.

### Fixed in 2026-09-04 follow-up (server audit Phases 0–2)
- [x] Config bindings: fixed `mapstructures`/`maptructure` typos (grpc `max_concurrent_streams`, grpc/fiber `access_logger`), unified `enable_stack_trace` yaml tags, added missing `mapstructure` tags on all `http.Config` fields, removed duplicate `TraceIDContextKey` checks (grpc/fiber/http).
- [x] TCP: transient `Accept` errors no longer kill the loop (backoff + continue); read errors classified (timeout → continue, else `OnError` + break, no CPU spin); `On*` callbacks panic-isolated; `Send` serialized + write deadlines; handlers atomic snapshot; pool fully lock-guarded with lock-free `Purge`/`Foreach`; idle reaper with done-channel; `MaxSessions` enforced; `SetKey` added; `Metadata()` returns a `Clone()`.
- [x] UDP: transient read errors no longer kill the packet loop; `n>0` filter; `addrs` re-keyed `map[string]` (pointer keys dropped every packet before); `Session.Send` uses `WriteToUDP`, `Server.Send` really sends (`ErrServerNotRunning`/`ErrNilSessionAddr`); `MaxSessions` + `MaxPacketsPerSecond` fixed-window limiter; read/write deadlines; same pool/handler hardening as TCP.
- [x] gRPC: metadata empty-slice panic guard; `Join(nmd, md)` so real span values shadow client-supplied ones; stream interceptor backfills trace metadata; unary+stream recovery interceptors outermost; half-TLS fail-fast (`ErrIncompleteTLSConfig`); `NextProtos: ["h2"]`; plaintext Warn; `DisableReflection` gate (zero value = on, compatible); keepalive + `ConnectionTimeout`-as-`MaxConnectionAge` wiring.
- [x] HTTP: recovery + whitelist `CORSConfig` (deny-by-default, `Vary:Origin`, explicit 204 preflight) + `BodyLimit` (`MaxBytesReader`) middlewares; `http.Server` timeouts + `MaxHeaderBytes` + keepalive wired from config; chain reordered to Metadata-before-Tracer (tracer writes last); status-capture middleware feeds the access logger (4xx/5xx classification works); route-template span names with `HTTP <method>` fallback; B3/request-id sanitize + regenerate.
- [x] Fiber: `cors.New()` replaced by `CORSConfig` (deny-by-default); timeouts + `BodyLimit/Concurrency/BufferSize` defaults wired; half-TLS fail-fast; swagger `All→Get`, validator default `""`; route-template span names; propagation sanitize; header-copy trimmed to propagation headers only.
- [x] `Stop()` on all 5 servers: check-and-flag + unlock-during-drain (no more `Lock`-held `Wait`), `stopping` guard serializes Stop/Start, real errors returned (joined; `sicky.go` already joins them), `ShutdownTimeout` (http/fiber/grpc, default 10s; grpc falls back to force-`Stop()`); TCP splits accept vs connection WaitGroups (two-phase stop, Add/Wait race-free).
- [x] Observability cost: `serverPID` cached, ordered log args (no per-request map), `span.End()` before parent `cancel()`, tracer `SkipPaths` (`/health`, `/metrics`, `/docs` by default, explicit empty = trace all).
- [x] Scaffold examples: `standard/mcp config.json.gotmpl` use real `"address"` key (was `"addr"`/`"listen"`, silently dropped) + new-field examples. Duration-unit note: http/fiber use `time.Duration` (viper parses `"10s"`), tcp/udp/manager use int seconds — documented, not normalized (would break configs).

### Fixed in 2026-09-04 follow-up (infra P0/P1)
- [x] Abort-on-presence: all 10 `*Config` types have `Validate()` + sentinel errors; non-nil empty config aborts startup via `Init*` (bun unknown driver no longer falls back to pg); `sicky.go validateConfig()` warns early.
- [x] Singleton safety: `infra.go` `mu` + `Get*/Clear*`; `Init*` first-wins closes duplicates; shutdown clears globals; manager health uses getters.
- [x] Leak/timeout unification: bun/redis ping with 5s ctx, mqtt `WaitTimeout`, nats `Timeout`, S3 load with timeout; failure paths close partial handles; secrets redacted (`redactDSN`).
- [x] S3 no longer an empty struct: `region/endpoint/access_key/secret_key/session_token/bucket/use_path_style/timeout` (region required, bucket optional, endpoint for MinIO).

### Fixed in 2026-09-04 follow-up (infra P2)
- [x] Bun: pool knobs (`max_open_conns/max_idle_conns/conn_max_lifetime_sec/conn_max_idle_time_sec`, negative aborts); `SlowDuration` frozen as milliseconds (bundebug v1.2.x has no threshold API); `Verbose` split out of `Debug` (**behavior change**: Debug no longer implies verbose query logging).
- [x] Redis: `username/enable_tls/tls_skip_verify/dial+read+write_timeout_sec/pool_size/min_idle_conns` (TLS 1.2+); cluster/sentinel out of scope.
- [x] Elastic: `cloud_id/api_key/service_token/certificate_fingerprint/ca_cert_file/timeout_sec`; endpoint = addresses-or-cloudid (`ErrElasticNoEndpoint`); startup `Info()` fail-fast; manager health is a real `Info()` ping.
- [x] S3: `PingS3` (`HeadBucket` when bucket set); manager health uses it.
- [x] MQTT: `username/password/enable_tls/ca_file/keep_alive_sec/connect_timeout_sec/clean_session`; NATS: `token/username/password/creds_file/nkey_file/enable_tls/root_ca_file/timeout_sec/reconnect_wait_sec/max_reconnects` (file paths `os.Stat`-checked).
- [x] Mongo: `GetMongoDB(name...)` (explicit > `cfg.DB` > URI path); manager `Ping` uses `readpref.Primary()`.
- [x] `Ensure()` convention for timing fields: zero fills default, negative stays for `Validate()` to abort.

### Fixed in 2026-09-04 follow-up (tracer dual-track + W3C/B3)
- [x] Dual-track: standard OTLP (`grpc`/`http`/`stdout`, explicit exporter+provider) vs Uptrace (`uptrace-go` SDK, owns its provider). `sicky.Config.Tracer` is the selector: `type + service_name/version` (empty falls back to `AppName/Version`) `+ endpoint/dsn/compress/timeout/insecure/headers/sample_rate`, with `Validate()` (`ErrTracerUnknownType`, `ErrTracerNoDSN`). Uptrace `SampleRate` is deprecated/ignored (server-side sampling); DSN never logged in clear (`tracer.RedactDSN`).
- [x] Shared OTLP construction in `tracer/internal` (resource + `ParentBased(TraceIDRatio)` + Batcher) used by grpc/http/stdout. **Latent bug fixed**: `semconv/v1.26.0` vs SDK `resource.Default()` schema `1.43.0` made `resource.Merge` always fail (all OTLP `New()` returned nil); bumped to `semconv/v1.43.0`.
- [x] Propagation unified in `tracer/propagation.go`: `Propagator()` = W3C TraceContext + W3C Baggage + B3 (single+multi), `Extract/Inject/Sanitize` helpers, `InstallPropagator()` called once by `sicky.go` after tracer init. Server interceptors (fiber/http/grpc) extract via it and re-inject W3C+B3 downstream; sanitize helpers in all three servers delegate to `tracer.Sanitize` (`tracestate` values pass through OTEL only, never sanitized/stored).
- [x] Client fix: gRPC unary interceptor dropped the span ctx (`_, span := Start` + `invoker(ctx)`) and never injected — now `invoker(injectSpanContext(spanCtx))` with `traceparent/tracestate/baggage` + B3 + `X-Request-ID` generation. Stream client injects too. `client/http` gained `StartSpan/InjectHTTP/Do` (span `<method> <host>` + `http.DefaultClient`).
- [x] Out of scope (documented blind spots): TCP/UDP/WebSocket servers and clients have no tracing (no standard carrier); `tracer/fiber.go NewFiberMiddleware` is deprecated (use `server/fiber.NewTracerMiddleware`); `tracestate` values are passed through but not stored/sanitized.
- [x] Tests: `tracer/propagation_test.go` (sanitize/redact/W3C+B3 round-trip), `tracer/internal/otlp_test.go` (clamp + provider), `tracer_config_test.go` (Ensure/Validate), client unary inject test, server unary W3C continuity test. Scaffold `standard/mcp config.json.gotmpl` ship a `tracer` stanza (`type:none` default; set `grpc`+`endpoint` or `uptrace`+`dsn`).
- [x] All tracer `Stop()` paths use a 5s timeout ctx instead of the ambient ctx.

---
*Last updated: 2026-09-04 — P0/P1/P2 hardening applied. Commented legacy blocks (`runtime/`, `service Run`, `mdns`, `metrics.StartMetrics`) deliberately kept.*
