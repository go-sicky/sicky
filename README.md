# Go-Sicky

A modular, extensible Go framework for building robust microservices and networked applications.

[![Go Version](https://img.shields.io/badge/Go-1.26.0-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Go-Sicky provides a unified, pluggable architecture that abstracts away infrastructure, communication protocols, and service management, letting you focus on business logic.

---

## Features

### Multi-Protocol Servers
| Protocol | Implementation | Network | Default Address | Notes |
|---|---|---|---|---|
| HTTP (Fiber) | Fiber (gofiber/v2) | `tcp` | `:9990` | |
| gRPC | google.golang.org/grpc | `tcp` | `:0` (random) | OS-assigned ephemeral port; actual port registered via `listener.Addr()` |
| TCP | Raw socket | `tcp` | `:9981` | |
| UDP | Raw socket | `udp` | `:9980` | Same numeric port as net/http but different network — can coexist |
| WebSocket | Fiber + gorilla/websocket | `tcp` | `:9991` | Path `/conn` by default |
| net/http | Standard library (bunrouter) | `tcp` | `:9980` | Same numeric port as UDP but different network — can coexist |

### Pluggable Infrastructure
| Component | Library | Purpose |
|---|---|---|
| Redis | go-redis/v9 | Cache / KV store |
| Bun | uptrace/bun | SQL ORM (Pg, MySQL, SQLite, MSSQL, DaMeng) |
| Ristretto | dgraph-io/ristretto/v2 | In-memory cache |
| Badger | dgraph-io/badger/v4 | Embedded KV store |
| Elasticsearch | elastic/go-elasticsearch/v9 | Full-text search |
| Clickhouse | uptrace/go-clickhouse | Analytics database |
| MongoDB | go.mongodb.org/mongo-driver/v2 | Document database |
| MQTT | eclipse/paho.mqtt.golang | IoT messaging |
| NATS | nats-io/nats.go | Messaging |
| S3 | aws-sdk-go-v2 | Object storage |

### Messaging & Brokers
- **NATS Core** — At-least-once pub/sub
- **NATS JetStream** — Persistent streams with configurable retention
- **NSQ** — Distributed, at-least-once messaging

### Service Discovery
- **Consul** — HashiCorp Consul Agent API
- **Redis** — Redis Hash + Pub/Sub notifications
- **Local** — Filesystem-based (JSON files + fsnotify)
- ~~mDNS~~ — Deprecated (`registry/mdns/` commented out, kept for reference; no longer supported)

### Observability
- **OpenTelemetry tracing** — OTLP/gRPC, OTLP/HTTP, Stdout exporters; B3 propagation
- **Uptrace** — Managed tracing via Uptrace SaaS
- **Prometheus metrics** — 11 counters (6 server + 5 client) + 3 collectors (`build_info`, `go`, `process`) via Manager `/metrics`
- **Structured logging** — slog-based logger with Fiber/gRPC adapters
- **Manager endpoints** — 8 built-in endpoints: `/metrics` (public, Prometheus scraping), `/health` + `/ready` (10 infra + registered business checkers; only `unhealthy` degrades; backend error text redacted), `/live` (static 200, liveness only), `/version`, `/info`, `/config` (gated by `expose_config`, secrets redacted, Bearer-or-loopback), `/services` (Bearer-or-loopback). Set `manager.auth_token` to require `Authorization: Bearer` on `/config` and `/services`; without a token those two are loopback-only. Optional `manager.tls_cert_pem`/`tls_key_pem` serve the manager over HTTPS.

### Background Jobs & Concurrency
- **Cron** — gocron/v2-based cron job scheduler
- **Ticker** — Interval-based task runner
- **Static Runner** — Goroutine-pool-based concurrent task dispatcher

### MCP (Model Context Protocol)
- Built-in MCP server implementation (JSON-RPC 2.0)
- Supports Tools, Resources, Prompts
- Pluggable transport: stdio or HTTP (SSE at `/mcp` + POST at `/mcp/message`)

### CLI
- `sicky new` — Scaffold new projects (Standard / MCP / Interactive)
- `sicky generate` — Generate handlers, tools, resources, documentation
- `sicky serve` — Run as MCP server
- `sicky version` — Print version info

---

## Installation

Requires Go 1.26.0 or later.

```bash
go install github.com/go-sicky/sicky/cmd/sicky@latest
```

Or add as a dependency:

```bash
go get github.com/go-sicky/sicky
```

---

## Quick Start

### 1. Create a configuration file

`config.json`:

```json
{
  "log_level": "info",
  "manager": {
    "enable": true,
    "address": ":8888"
  }
}
```

### 2. Write your service

`main.go`:

```go
package main

import (
    "errors"
    "log"

    "github.com/go-sicky/sicky"
    "github.com/go-sicky/sicky/server"
    "github.com/go-sicky/sicky/service"
    svcStandard "github.com/go-sicky/sicky/service/standard"
    srvFiber "github.com/go-sicky/sicky/server/fiber"
    "github.com/gofiber/fiber/v2"
)

func main() {
    if err := sicky.Init(&sicky.Options{
        AppName: "myapp",
        Version: "1.0.0",
    }); err != nil {
        if errors.Is(err, sicky.ErrVersionShown) {
            return // --version: version already printed
        }
        log.Fatalf("init failed: %v", err)
    }

    // Create service — New() auto-registers via service.Set()
    svc := svcStandard.New(&service.Options{Name: "myapp"}, nil)

    // Create HTTP server
    srv := srvFiber.New(&server.Options{Name: "http-server"}, &srvFiber.Config{
        Address: ":8080",
    })

    // Register routes
    srv.App().Get("/", func(c *fiber.Ctx) error {
        return c.JSON(fiber.Map{"message": "Hello from Go-Sicky!"})
    })

    // Attach server to service
    svc.Servers(srv)

    // Load config (Viper → struct) and run — Run() blocks until signal
    cfg := &sicky.Config{}
    if err := sicky.ConfigUnmarshal(cfg); err != nil {
        log.Fatalf("config unmarshal failed: %v", err)
    }
    if err := sicky.Run(cfg); err != nil {
        log.Fatalf("run failed: %v", err)
    }
}
```

### 3. Run

```bash
go run main.go
```

Access the health endpoint at `http://localhost:8888/health` and your API at `http://localhost:8080/`.

---

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                    sicky (Orchestrator)                       │
│  Init(): flags(--config/-C, --config-type, --version)          │
│        → Viper load (local /etc, /etc/<app>, $HOME/.<app>, . │
│          or remote consul://) + env SICKY_ + Must* checks     │
│        → returns error (never os.Exit; ErrVersionShown for -V)│
│  Run(): beforeStart → Infra(10) → Tracer(4) → Registry(3)     │
│        → Broker(3) → Flag callbacks → Manager(6 endpoints)    │
│        → Services.Start + Registry.Register → afterStart       │
│        → wait (TERM/INT/QUIT/ABRT shutdown, HUP reloads,      │
│          or Context cancel) → beforeStop → Services.Stop      │
│        → Broker.Disconnect → Registry.Stop → Tracer.Stop       │
│        → Manager.Stop → Infra.Close → afterStop               │
├──────────────────────────────────────────────────────────────┤
│                  service.Service                              │
│  ┌──────────┐  ┌─────────────┐  ┌─────┐                       │
│  │ Standard │  │ Interactive │  │ MCP │ (+ protocol)          │
│  └──────────┘  └─────────────┘  └─────┘                       │
├──────────────────────────────────────────────────────────────┤
│   server.Server  ←→  broker.Broker  ←→  client.Client        │
│   job.Job        ←→  runner.Runner                            │
├──────────────────────────────────────────────────────────────┤
│   registry.Registry    tracer.Tracer (OTLP/gRPC|HTTP, Stdout, │
│                                      Uptrace + B3)            │
├──────────────────────────────────────────────────────────────┤
│                       infra (10)                              │
│  Redis Bun Ristretto Badger Elastic Clickhouse                │
│  Mongo MQTT NATS S3                                           │
└──────────────────────────────────────────────────────────────┘
```

> Full lifecycle order: `sicky.go:253-870`. See `AGENTS.md §2.1` for detailed phase breakdown.

### Self-Registration Pattern

Every component registers itself into a global pool on construction:

```go
svc := svcStandard.New(...)   // → service.Set(svc)
srv := srvFiber.New(...)      // → server.Set(srv)
brk := brkNats.New(...)       // → broker.Set(brk)
```

The orchestrator iterates all registered instances during `Run()`.

---

## Configuration

Go-Sicky uses [Viper](https://github.com/spf13/viper) for configuration, supporting local files and remote providers.

### Config File Locations

Config files are searched in order (`sicky.go:140-143`):
1. `/etc/<config>.json`
2. `/etc/<app_name>/<config>.json`
3. `$HOME/.<app_name>/<config>.json`
4. `./<config>.json`

`<config>` defaults to `config` (`--config/-C` flag, `sicky.go:99`) and extension defaults to `json` (`--config-type` flag, `sicky.go:100`).

### Remote Config

Prefix the config path with a scheme for remote providers:

```bash
-c consul://localhost:8500/myapp/config
```

### Environment Variables

All config keys are available as environment variables with the `SICKY_` prefix (dots replaced with underscores):

```bash
SICKY_LOG_LEVEL=debug
SICKY_MANAGER_ADDRESS=:9999
SICKY_INFRA_REDIS_ADDR=localhost:6379
```

### Core Configuration

```json
{
  "log_level": "info",

  "manager": {
    "enable": true,
    "address": ":8888",
    "advertise_address": "",
    "enable_swagger": false,
    "expose_config": false,
    "auth_token": "",
    "tls_cert_pem": "",   // optional HTTPS (PEM); both cert+key required, half-config fails Start
    "tls_key_pem": "",
    "read_timeout": 10,
    "write_timeout": 10,
    "idle_timeout": 60,
    "shutdown_timeout": 5,
    "metrics_path": "/metrics",
    "health_path": "/health",
    "live_path": "/live",
    "ready_path": "/ready",
    "version_path": "/version",
    "info_path": "/info",
    "swagger_path": "/swagger.json",
    "config_path": "/config",
    "service_pool_path": "/services"
  },

  "infra": {
    // A present section means "enable it": missing required fields abort
    // startup (e.g. empty dsn/url/addr/broker, unknown bun driver).
    "redis":     { "addr": "localhost:6379", "username": "", "password": "", "db": 0, "enable_tls": false, "dial_timeout_sec": 5, "read_timeout_sec": 3, "write_timeout_sec": 3, "pool_size": 0, "min_idle_conns": 0 },
    "bun":       { "driver": "pg", "dsn": "postgres://...", "debug": false, "verbose": false, "slow_duration": 0, "max_open_conns": 0, "max_idle_conns": 0, "conn_max_lifetime_sec": 0, "conn_max_idle_time_sec": 0 },
    "badger":    { "path": "/tmp/badger" },
    "ristretto": { "num_counters": 10000000, "max_cost": 100000000, "buffer_items": 64 },
    "nats":      { "url": "nats://localhost:4222", "token": "", "username": "", "password": "", "creds_file": "", "nkey_file": "", "enable_tls": false, "root_ca_file": "", "timeout_sec": 5, "reconnect_wait_sec": 2, "max_reconnects": 60 }, // -1 = retry forever
    "mqtt":      { "broker": "tcp://localhost:1883", "client_id": "", "username": "", "password": "", "enable_tls": false, "ca_file": "", "keep_alive_sec": 30, "connect_timeout_sec": 5 },
    "elastic":   { "addresses": ["http://localhost:9200"], "username": "", "password": "", "cloud_id": "", "api_key": "", "service_token": "", "ca_cert_file": "", "timeout_sec": 5 }, // timeout_sec = startup check only; per-request via ctx
    "clickhouse":{ "dsn": "clickhouse://..." },
    "mongo":     { "uri": "mongodb://localhost:27017", "db": "mydb", "max_pool_size": 0, "connect_timeout_sec": 0 }, // 0 = driver defaults
    "s3":        { "region": "us-east-1", "endpoint": "", "bucket": "", "use_path_style": false, "timeout": 5, "request_timeout_sec": 0 } // region required; bucket optional; endpoint for MinIO/LocalStack; 0 = SDK default
  },

  "tracer": {
    "type": "grpc",          // none | grpc | http | stdout | uptrace
    "endpoint": "localhost:4317",
    "sample_rate": 1.0,
    "compress": false,
    "timeout": 30,
    "dsn": "",                // uptrace only
    "pretty_print": false,    // stdout only
    "timestamps": false       // stdout only
  },

  "registry": {
    "pool_purge_interval": 60,
    "consul": { "endpoint": "http://localhost:8500" },
    "redis":  { "addr": "localhost:6379", "password": "", "db": 0 },
    "local":  { "registry_file_path": "/tmp/sicky/registry", "cleanup_on_start": false } // path must be absolute; cleanup only removes <uuid>.json
  },

  "broker": {
    "nats":      { "url": "nats://localhost:4222" },
    "nsq":       { "endpoint": "127.0.0.1:4150", "channel": "sicky" },
    "jetstream": { "url": "nats://localhost:4222", "stream": { "name": "sicky", "subjects": ["*"], "max_consumers": 256 } }
  }
}
```

---

## API Examples

### Creating a Service

```go
import (
    svcStandard "github.com/go-sicky/sicky/service/standard"
    "github.com/go-sicky/sicky/service"
)

svc := svcStandard.New(&service.Options{
    Name:    "user-service",
    Version: "1.0.0",
}, nil) // second arg *service.Config may be nil
```

### Fiber HTTP Server

```go
import (
    srvFiber "github.com/go-sicky/sicky/server/fiber"
    "github.com/go-sicky/sicky/server"
    "github.com/gofiber/fiber/v2"
)

srv := srvFiber.New(&server.Options{Name: "api"}, &srvFiber.Config{
    Address:        ":8080",
    EnableSwagger:  true,
    SwaggerPageTitle: "My API",
})

srv.App().Get("/users", listUsers)
srv.App().Post("/users", createUser)
```

> CORS is deny-by-default on both HTTP stacks: with empty `cors.allowed_origins` no `Access-Control-Allow-Origin` headers are emitted. Combining `"*"` with `allow_credentials: true` is rejected (`CORSConfig.Validate()`) and fails closed to deny-all. Example:
>
> ```go
> srv := srvFiber.New(&server.Options{Name: "api"}, &srvFiber.Config{
>     Address: ":8080",
>     CORS: &srvFiber.CORSConfig{
>         AllowedOrigins:   []string{"https://app.example.com"},
>         AllowCredentials: true,
>     },
> })
> ```

### gRPC Server

```go
import (
    srvGrpc "github.com/go-sicky/sicky/server/grpc"
    "github.com/go-sicky/sicky/server"
)

srv := srvGrpc.New(&server.Options{Name: "grpc"}, &srvGrpc.Config{
    Address: ":9090",
})

// Register your protobuf service
pb.RegisterUserServiceServer(srv.App(), &userServer{})
```

### gRPC Client (TLS + Service Discovery)

```go
import (
    cltGrpc "github.com/go-sicky/sicky/client/grpc"
    "github.com/go-sicky/sicky/client"
)

// Direct-dial mode
clt := cltGrpc.New(&client.Options{Name: "user-client"}, &cltGrpc.Config{
    Addr: "127.0.0.1:9090",
})

// Service-discovery mode: endpoints resolve from the registry pool
// (Instance.Servers[type==grpc]) and follow live pool updates
disc := cltGrpc.New(&client.Options{Name: "user-client"}, &cltGrpc.Config{
    Service:  "user-service",
    Balancer: "round_robin",
})

// mTLS: both fields required — a half-configured pair fails fast
// (nil client, ErrIncompleteTLSConfig), never silent plaintext
secure := cltGrpc.New(&client.Options{Name: "user-client"}, &cltGrpc.Config{
    Addr:       "10.0.0.5:9090",
    TLSCertPEM: certPEM,
    TLSKeyPEM:  keyPEM,
})
defer secure.Disconnect()
```

### TCP/UDP Server

```go
import (
    srvTCP "github.com/go-sicky/sicky/server/tcp"
)

type MyHandler struct{}

func (h *MyHandler) OnConnect(sess *srvTCP.Session) error {
    logger.Info("Client connected")
    return nil
}

func (h *MyHandler) OnData(sess *srvTCP.Session, data []byte) error {
    sess.Send([]byte("Echo: " + string(data)))
    return nil
}

// ... implement OnClose, OnError

srv := srvTCP.New(&server.Options{Name: "tcp"}, &srvTCP.Config{
    Address:    ":9981",
    BufferSize: 4096,
    // MaxMessageBytes: 1 << 20, // optional per-connection receive cap (0 = unlimited)
})
srv.Handle(&MyHandler{})
```

### WebSocket Server

```go
import (
    srvWS "github.com/go-sicky/sicky/server/websocket"
)

srv := srvWS.New(&server.Options{Name: "ws"}, &srvWS.Config{
    Address: ":9991",
    Path:    "/ws",
})
srv.Handle(&MyWSHandler{})
```

### Messaging (NATS)

```go
import (
    brkNats "github.com/go-sicky/sicky/broker/nats"
    "github.com/go-sicky/sicky/broker"
)

// Publish
brk := brkNats.New(&broker.Options{Name: "nats"}, &brkNats.Config{
    URL: "nats://localhost:4222",
})
brk.Connect()

msg := &broker.Message{Topic: "orders.created"}
msg.Format(myOrder)
brk.Publish("orders.created", msg)

// Subscribe
brk.Subscribe("orders.created", func(m *broker.Message) error {
    var order Order
    m.Scan(&order)
    // process order...
    return nil
})
```

### Background Jobs

```go
import (
    jobCron "github.com/go-sicky/sicky/job/cron"
    "github.com/go-sicky/sicky/job"
)

j := jobCron.New(&job.Options{Name: "cleanup"}, &jobCron.Config{})
j.Add(&jobCron.Task{
    Expression: "0 0 * * *",
    Timeout:    5 * time.Minute, // optional watchdog: expiry reports failure (leaked run can't be killed)
    Handler: func() error {
        // daily cleanup
        return nil
    },
})
j.Start()
```

### Task Runner (back-pressure)

`Static.Task()` blocks when the queue is full — never call it from inside a Handler (deadlock). Use `TryTask` for bounded enqueue:

```go
if err := r.TryTask(&runner.Task{Data: work}, 100*time.Millisecond); errors.Is(err, runner.ErrPoolFull) {
    // shed load: retry later or drop
}
_ = r.Len() // queued depth for observability
```

### Infrastructure

```go
import "github.com/go-sicky/sicky/infra"

// Redis
infra.Redis.Set(ctx, "key", "value", 0)
val, _ := infra.Redis.Get(ctx, "key").Result()

// Bun (SQL)
infra.Bun.NewSelect().Model(&users).Scan(ctx)

// Ristretto (cache)
infra.Ristretto.Set("token:123", userData, 1)
val, _ := infra.Ristretto.Get("token:123")

// Badger (KV store)
infra.Badger.Update(func(txn *badger.Txn) error {
    return txn.Set([]byte("key"), []byte("value"))
})

// Race-free reads via getters (recommended in handlers):
// infra.GetRedis(), infra.GetBun(), infra.GetMongoDB("mydb")
```

### Lifecycle Hooks

```go
sicky.BeforeStart(func(ctx context.Context) error {
    logger.Info("About to start services")
    return nil
})

sicky.AfterStop(func(ctx context.Context) error {
    logger.Info("All services stopped, cleaning up")
    return nil
})

// SIGHUP reload (process keeps serving; errors are logged, never fatal)
sicky.OnReload(func(ctx context.Context) error {
    logger.Info("Reloading on SIGHUP")
    return nil
})

// Business health check, merged into /health and /ready
sicky.RegisterHealthChecker("order-db", func(ctx context.Context) error {
    return orderStore.Ping(ctx)
})
```

### Errors (codes + generic envelope)

```go
import "github.com/go-sicky/sicky/utils"

_ = utils.RegisterErrorCode(40010, "order already paid", utils.StatusConflict)
err := utils.NewCodedError(40010, "", dbErr) // Unwrap-compatible
code := utils.CodeOf(err)                    // walks the chain, default 5000

ok := utils.OkT(order)                       // success envelope
fail := utils.FailT[any](utils.CodeNotFound, "").
    WithRequestID(reqID)                     // message/status from registry
```

---

## CLI

```bash
# Scaffold a new standard microservice (project name is required positional arg)
sicky new myapp --type standard --module github.com/myorg/myapp
# Optional: --output/-o <dir> (default "."), --no-grpc

# Scaffold an MCP server
sicky new my-mcp --type mcp --module github.com/myorg/my-mcp

# Generate handler/tool/resource (name is positional, no --name flag)
sicky generate handler User
sicky generate tool Search
sicky generate resource Article
sicky generate doc

# Run as MCP server (transport: stdio or http)
sicky serve --transport stdio --name my-mcp
sicky serve --transport http --listen :3000

# Print version / help
sicky version
sicky help        # also: sicky serve -h, sicky new -h
```

---

## Package Reference

| Package | Description |
|---|---|
| `sicky` | Core orchestrator — `Init()` (returns error; `ErrVersionShown`/`ErrAlreadyInitialized` sentinels), `Run()` (returns joined error, graceful shutdown), `Viper()`, `ConfigUnmarshal()`, lifecycle hooks (`Before/AfterStart/Stop`, `OnReload` on SIGHUP), `FlagSwitch`, Manager |
| `server` | Server interface + Fiber, gRPC, net/http (bunrouter), TCP, UDP, WebSocket implementations |
| `broker` | Broker interface + NATS, JetStream, NSQ implementations |
| `client` | Client interface + gRPC, HTTP, TCP, UDP, WebSocket implementations (gRPC supports TLS 1.2+ mTLS fail-fast and registry-based service discovery) |
| `service` | Service interface + Standard (background), Interactive (CLI), MCP (+ `mcp/protocol`) |
| `registry` | Registry interface + Consul, Redis, Local (file JSON) — `mdns` deprecated (commented, kept) |
| `tracer` | Tracer interface + OTLP/gRPC, OTLP/HTTP, Stdout, Uptrace (+ `tracer/fiber.go` B3 helper) |
| `infra` | Infrastructure drivers (Redis, Bun, Ristretto, Badger, Elasticsearch, Clickhouse, MongoDB, MQTT, NATS, S3) — 10 files, no interface |
| `job` | Job interface + Cron (gocron), Ticker implementations |
| `runner` | Runner interface + Static goroutine pool implementation |
| `logger` | Structured logger (slog-based), Fiber/gRPC adapters |
| `metrics` | Prometheus 11 counters (6 server + 5 client) + 3 collectors (`build_info`, `go`, `process`) |
| `utils` | Helpers — `metadata`, `net` (IP/`Net2fd`), `http` envelopes, `debug`, `misc` |
| `internal` | Internal request `Context` (ID, AppName, broker/registry/tracer/logger carriers) |
| `cli` | CLI entry point — `sicky new/generate/serve/version/help` (`cli/sicky.go`) |
| `cmd` | Binary entry point (`cmd/sicky/main.go`) |

---

## License

MIT © 2024 HereweTech Co.LTD
