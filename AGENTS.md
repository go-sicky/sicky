# Go-Sicky Framework: Agent Development Guide

Welcome, Agent. This document provides a comprehensive overview of the **Go-Sicky** framework to assist you in understanding its architecture, conventions, and development patterns.

## 1. Project Overview
**Go-Sicky** is a highly modular and extensible Go framework designed for building robust microservices and networked applications. It provides high-level abstractions for common infrastructure, communication protocols, and service management, allowing developers to focus on business logic.

- **Primary Goal**: To provide a unified, pluggable architecture for microservices.
- **Key Features**:
  - Support for multiple protocols: HTTP (Fiber), gRPC, TCP, UDP, WebSocket.
  - Pluggable Infrastructure: SQL (Bun), NoSQL (Mongo), Cache (Redis, Ristretto), KV Store (Badger), Search (Elasticsearch), Analytics (Clickhouse), Messaging (NATS/MQTT infra), Storage (S3).
  - Messaging & Brokers: NATS, JetStream, NSQ.
  - Service Discovery: Consul, Redis. (mDNS is planned but not yet implemented.)
  - Background Jobs: Cron and Ticker-based job scheduling.
  - Concurrent Task Runner: Goroutine-pool-based task dispatch.
  - Observability: Prometheus metrics, OpenTelemetry tracing, built-in structured logger.

## 2. Core Architecture
The framework is organized into several key layers:

### 2.1 Orchestration (`sicky`)
The root `sicky` package is the central nervous system. It handles:
- **Initialization**: Command-line flag parsing (`pflag`) and configuration loading (`viper`).
- **Configuration**: Support for local files (JSON, etc.) and remote providers (e.g., Consul). Config files searched at `/etc`, `/etc/<app_name>`, `$HOME/.<app_name>`, and the current directory.
- **Lifecycle Management**: Orchestrating the start and stop of all registered services, servers, brokers, registries, tracers, and infrastructure.
- **Manager**: A built-in HTTP server within the `sicky` package that exposes health checks, metrics, version info, and service pool status.

### 2.2 Infrastructure (`infra`)
Manages connections to external systems. Each infrastructure component (e.g., `redis`, `mongo`) has a corresponding `Init*` function and a package-level global variable for singleton access. Unlike other subsystems, `infra` does not define a shared Go interface — each component is a standalone module with its own concrete type.

### 2.3 Abstractions & Interfaces
Go-Sicky heavily relies on Go interfaces to maintain decoupling:
- **`service.Service`**: Represents a logical unit of business functionality. Manages subordinate servers, brokers, jobs, registries, and tracers.
- **`server.Server`**: Handles incoming requests on a specific protocol (Fiber HTTP, gRPC, TCP, UDP, WebSocket, net/http).
- **`broker.Broker`**: Manages asynchronous message publishing and subscription.
- **`registry.Registry`**: Handles service registration and discovery (Consul, Redis, local).
- **`tracer.Tracer`**: Manages OpenTelemetry spans (OTLP/gRPC, OTLP/HTTP, Stdout, Uptrace).
- **`job.Job`**: Represents a background job (Cron, Ticker).
- **`runner.Runner`**: Consumes and dispatches concurrent tasks (static goroutine pool).
- **`client.Client`**: Outbound service client for various protocols (HTTP, gRPC, TCP, UDP, WebSocket).

### 2.4 Registration Pattern
Each abstraction layer uses a self-registration pattern via global state:
- The `New()` constructor of each implementation calls the package-level `Set(self)` function, adding itself to a global map (e.g. `broker.Set(brk)`, `tracer.Set(trc)`).
- The first registered instance becomes the "default" singleton, accessible via `package.Default()`.
- The orchestrator (`sicky.go`) creates instances via `New()` based on configuration presence, which triggers self-registration.
- **Note**: In `sicky.go` the orchestrator occasionally uses concrete types (e.g. `*rgConsul.Consul`) for implementation-specific methods like `Watch()`. This is an acknowledged exception to the "always use abstractions" rule, necessary when specific implementations expose methods not present in the interface.

## 3. Development Conventions

### 3.1 Component Pattern
Most components follow this structure:
- `interface.go`: Defines the core interface.
- `options.go`: Defines the `Options` struct — runtime parameters (name, ID, logger, context, lifecycle hooks). Uses `Ensure()` to provide defaults.
- `config.go`: Defines the `Config` struct — Viper-serializable configuration (JSON/YAML/env). Uses `Ensure()` to provide defaults.
- `Ensure()`: A method on both `Options` and `Config` that fills in sensible default values for zero-valued or nil fields. Always returns the receiver (or a new instance if nil).

**Distinction**: `Options` carries runtime state and references (context, logger, hooks). `Config` carries serializable settings (addresses, timeouts, flags). Both must have `Ensure()`.

### 3.2 Configuration
- **Viper** is used for all configuration.
- Environment variables are automatically mapped using the `SICKY_` prefix (e.g., `SICKY_APP_NAME`), with dot-to-underscore replacement.
- Config files are searched at `/etc`, `/etc/<app_name>`, `$HOME/.<app_name>`, and the current directory.
- `core.go` excludes `context.TODO()` — use `context.Background()` for static initialization or pass a derived context for operations with timeouts.
- Viper's `AutomaticEnv()` performs type coercion for environment variables. Non-numeric values for `int` fields silently become zero. A `validateConfig()` step in `sicky.go` runs after config loading to warn on suspicious zero values for timing fields.

### 3.3 Lifecycle Hooks

The framework provides hooks at two levels:

**Sicky-level** (register via `sicky.BeforeStart()`, etc.):
- `BeforeStart`, `AfterStart` — executed before/after all services start.
- `BeforeStop`, `AfterStop` — executed before/after the full shutdown sequence (service stop, broker disconnect, registry stop, tracer stop, manager stop, infra shutdown).
- Wrapper signature: `SickyWrapper func(context.Context) error`.
- Hook failures log an error but do NOT abort the process (they use `ErrorContext`, not `Fatal`).

**Server-level** (set via `server.Options.BeforeStart()` etc.):
- `BeforeStart`, `AfterStart` — executed within each server's `Start()` method.
- `BeforeStop`, `AfterStop` — executed within each server's `Stop()` method.
- Wrapper signature: `ServerWrapper func() error`.
- Registration via `server.Options` setter methods: `opts.BeforeStart(w ...ServerWrapper)`. Invocation via `opts.RunBeforeStart()` etc. inside each server implementation.

### 3.4 Error Handling
- Errors during service start/stop are aggregated using `errors.Join`. The orchestrator collects all errors and continues through the full shutdown sequence (via `goto shutdown` + `runErr` variable) even when individual services fail.
- Constructors (e.g., `New()`) for registry/tracer/broker implementations may return `nil` on initialization failure. Callers in `sicky.go` must check for nil before calling methods on the returned value.
- `logger.Fatal()` calls `os.Exit(-1)` and should NOT be used during server construction or anywhere that should support graceful error recovery. Use `logger.Error()` and return the error instead.
- All sentinel errors are defined as exported package-level variables (e.g., `broker/nats.ErrBrokerNotConnected`) to enable `errors.Is()` checks by callers.
- `broker.Message.Format()` and `Scan()` return errors for marshalling/unmarshalling failures — callers must check them.

### 3.5 Infra Conventions
- Each infra type must have a **nil guard** before overwriting the global variable: `if Xxx == nil { Xxx = client }`. This prevents connection leaks on repeated `Init*()` calls.
- Infra `Init*()` functions are expected to emit Info logs on success and Error logs on failure, using `logger.Logger`.
- Infra Config structs should implement `Ensure()` to provide defaults (currently a known gap — see TODO list).
- The infra shutdown sequence in `sicky.go` follows a specific order: Ristretto → Badger → Elastic → Nats → Redis → Bun → Clickhouse → S3 → Mongo → MQTT.

### 3.6 Metrics Conventions
- Prometheus counters are defined in `metrics/metrics.go` and exported as package-level variables.
- Each counter is registered in the `init()` function via `metrics.Register()`.
- The Manager's `/metrics` endpoint serves all registered collectors via a single `prometheus.Registry` created once during `NewManager()`.
- Counter `.Inc()` calls are placed at the following locations:
  - **Server counters**: In the access-log interceptor/middleware just before invoking the underlying handler (for gRPC/Fiber), or before the `OnData` handler loop (for TCP/UDP/WebSocket).
  - **Client counters**: As the first statement in the `Call()` method of each client implementation.

## 4. Key Packages & Directories
- `/broker`: Messaging abstraction + NATS, JetStream, NSQ implementations.
- `/client`: Client abstraction + gRPC, HTTP, TCP, UDP, WebSocket implementations.
- `/infra`: Infrastructure drivers (Redis, Bun/SQL, Badger, Ristretto, Mongo, Clickhouse, MQTT, NATS, S3, Elasticsearch). No interface — each is a standalone module.
- `/job`: Job abstraction + Cron, Ticker implementations.
- `/logger`: Built-in structured logger (`GeneralLogger`), gRPC logger adapter, Fiber middleware.
- `/metrics`: Prometheus counter definitions and registry helpers.
- `/registry`: Registry abstraction + Consul, Redis, local (in-memory) implementations.
- `/runner`: Runner abstraction + static goroutine-pool implementation.
- `/runtime`: **Deprecated**. Previously the main orchestrator; logic has moved to `sicky`. Subdirectories `docker/` and `nomad/` are empty placeholders.
- `/server`: Server abstraction + Fiber, gRPC, net/http, TCP, UDP, WebSocket implementations.
- `/service`: Service abstraction + `Standard` (background microservice), `Interactive` (CLI), `Mcp` implementations.
- `/tracer`: Tracer abstraction + OTLP/gRPC, OTLP/HTTP, Stdout, Uptrace implementations.
- `/utils`: Helper functions for networking (IP resolution, `Net2fd`), HTTP response envelopes, metadata, and debugging.

## 5. Technology Stack
- **Go Version**: 1.26.0
- **Main Dependencies**:
  - `github.com/gofiber/fiber/v2`: Web framework (Fiber + WebSocket servers).
  - `google.golang.org/grpc`: RPC framework (gRPC server + client).
  - `github.com/spf13/viper`: Configuration management (local files + remote providers).
  - `go.opentelemetry.io/otel`: OpenTelemetry tracing (SDK, exporters, B3 propagator).
  - `github.com/prometheus/client_golang`: Prometheus metrics.
  - `github.com/uptrace/bun`: SQL ORM (PostgreSQL, MySQL, SQLite, MSSQL, DaMeng).
  - `github.com/redis/go-redis/v9`: Redis client (infra + registry).
  - `github.com/nats-io/nats.go`: NATS Core + JetStream.
  - `github.com/nsqio/go-nsq`: NSQ client.
  - `github.com/hashicorp/consul/api`: Consul service discovery.
  - `github.com/dgraph-io/ristretto/v2`: High-performance in-memory cache.
  - `github.com/dgraph-io/badger/v4`: Embedded key-value store.
  - `github.com/uptrace/go-clickhouse`: Clickhouse client.
  - `github.com/elastic/go-elasticsearch/v9`: Elasticsearch client.
  - `github.com/eclipse/paho.mqtt.golang`: MQTT client.
  - `github.com/aws/aws-sdk-go-v2`: AWS S3 client.
  - `github.com/go-co-op/gocron/v2`: Cron scheduler.
  - `github.com/google/uuid`: UUID generation.
  - `github.com/vmihailenco/msgpack/v5`: MessagePack serialization.

## 6. AI Agent Instructions
When working on this codebase:
1.  **Respect Interfaces**: Use the abstractions defined in `broker`, `server`, `registry`, etc., rather than implementation-specific types, except in the orchestrator (`sicky.go`) where concrete type access is necessary for implementation-specific methods (e.g., `Watch()`, `GracefulStop()`).
2.  **Surgical Edits**: Use targeted replacements to maintain the existing coding style and license headers.
3.  **Ensure Defaults**: When adding new configurations, always implement the `Ensure()` method on both `Options` and `Config` structs to provide defaults.
4.  **Nil-Guard Globals**: When adding infra init functions, wrap the global variable assignment with `if Xxx == nil { Xxx = client }`.
5.  **Wire Metrics**: When adding new server or client protocols, add a Prometheus counter in `metrics/metrics.go` and wire its `.Inc()` call at the appropriate insertion point.
6.  **Follow the Manager**: Health checks and metrics should be exposed via the Manager HTTP server within the `sicky` package.
7.  **No `context.TODO()`**: Use `context.Background()` for static initialization, or propagate a proper context for operations that need cancellation or deadlines.
8.  **Goroutine Cleanup**: All server/manager goroutines must use `defer wg.Done()` to prevent `wg.Wait()` from hanging in `Stop()`. Drop the `error` return type from `go func()` signatures — errors should be logged internally.

## 7. Known Gaps and TODOs
- [ ] `infra/` Config types are missing `Ensure()` methods (10 types).
- [ ] Remote config loading requires the `_ "github.com/spf13/viper/remote"` import in `sicky.go` (fixed).
- [ ] gRPC server only configures unary tracing interceptors; streaming interceptors are missing.
- [ ] `runtime/` package is deprecated and contains only empty subdirectories — should be cleaned up.
- [ ] `service/` package has commented-out legacy `Run()` function — should be removed.
- [ ] `registry/mdns/` is entirely commented out — should be removed or implemented with the `zeroconf` dependency.
- [ ] Zero test coverage across the entire project.

---
*Last updated: 2026-07-06 — Auto-generated, then manually reviewed and aligned with current codebase.*
