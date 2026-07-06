# Go-Sicky

A modular, extensible Go framework for building robust microservices and networked applications.

[![Go Version](https://img.shields.io/badge/Go-1.26.0-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Go-Sicky provides a unified, pluggable architecture that abstracts away infrastructure, communication protocols, and service management, letting you focus on business logic.

---

## Features

### Multi-Protocol Servers
| Protocol | Implementation | Default Port |
|---|---|---|
| HTTP | Fiber (gofiber/v2) | `:9990` |
| gRPC | google.golang.org/grpc | `:0` (random) |
| TCP | Raw socket | `:9981` |
| UDP | Raw socket | `:9980` |
| WebSocket | Fiber + gorilla/websocket | `:9991` |
| net/http | Standard library | `:9980` |

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

### Observability
- **OpenTelemetry tracing** — OTLP/gRPC, OTLP/HTTP, Stdout exporters; B3 propagation
- **Uptrace** — Managed tracing via Uptrace SaaS
- **Prometheus metrics** — Built-in counters for all server/client access
- **Structured logging** — slog-based logger with Fiber/gRPC adapters
- **Health checks** / **Version** / **Info** endpoints via built-in Manager

### Background Jobs & Concurrency
- **Cron** — gocron/v2-based cron job scheduler
- **Ticker** — Interval-based task runner
- **Static Runner** — Goroutine-pool-based concurrent task dispatcher

### MCP (Model Context Protocol)
- Built-in MCP server implementation (JSON-RPC 2.0)
- Supports Tools, Resources, Prompts
- Pluggable transport: stdio or HTTP

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
    "github.com/go-sicky/sicky"
    "github.com/go-sicky/sicky/logger"
    "github.com/go-sicky/sicky/server"
    svcStandard "github.com/go-sicky/sicky/service/standard"
    srvFiber "github.com/go-sicky/sicky/server/fiber"
    "github.com/gofiber/fiber/v2"
)

type AppService struct {
    *svcStandard.Standard
}

func main() {
    sicky.Init(&sicky.Options{
        AppName: "myapp",
        Version: "1.0.0",
    })
    defer sicky.Run(sicky.Viper())

    // Create service
    svc := svcStandard.New(&service.Options{Name: "myapp"})

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
}

func init() {
    sicky.RegisterService(&AppService{})
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
┌─────────────────────────────────────────────────────┐
│                     sicky (Orchestrator)             │
│  Init() → Config → Infra → Tracer → Registry → Run  │
├─────────────────────────────────────────────────────┤
│                 service.Service                      │
│  ┌──────────┐  ┌─────────────┐  ┌─────┐            │
│  │ Standard │  │ Interactive │  │ MCP │            │
│  └──────────┘  └─────────────┘  └─────┘            │
├─────────────────────────────────────────────────────┤
│   server.Server  ←→  broker.Broker  ←→  client.Client │
│   job.Job        ←→  runner.Runner                  │
├─────────────────────────────────────────────────────┤
│   registry.Registry    tracer.Tracer                │
├─────────────────────────────────────────────────────┤
│                      infra                          │
│  Redis Bun Ristretto Badger Elastic Clickhouse      │
│  Mongo MQTT NATS S3                                  │
└─────────────────────────────────────────────────────┘
```

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

Config files are searched in order:
1. `/etc/<config>.json`
2. `/etc/<app_name>/<config>.json`
3. `$HOME/.<app_name>/<config>.json`
4. `./<config>.json`

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
    "shutdown_timeout": 5,
    "metrics_path": "/metrics",
    "health_path": "/health",
    "version_path": "/version",
    "info_path": "/info",
    "config_path": "/config",
    "service_pool_path": "/services"
  },

  "infra": {
    "redis":     { "addr": "localhost:6379", "password": "", "db": 0 },
    "bun":       { "driver": "pg", "dsn": "postgres://...", "debug": false },
    "badger":    { "path": "/tmp/badger" },
    "ristretto": { "num_counters": 10000000, "max_cost": 100000000, "buffer_items": 64 },
    "nats":      { "url": "nats://localhost:4222" },
    "mqtt":      { "broker": "tcp://localhost:1883", "client_id": "" },
    "elastic":   { "addresses": ["http://localhost:9200"], "username": "", "password": "" },
    "clickhouse":{ "dsn": "clickhouse://..." },
    "mongo":     { "uri": "mongodb://localhost:27017", "db": "mydb" },
    "s3":        {}
  },

  "tracer": {
    "type": "grpc",
    "endpoint": "localhost:4317",
    "sample_rate": 1.0,
    "compress": false,
    "timeout": 30
  },

  "registry": {
    "pool_purge_interval": 60,
    "consul": { "endpoint": "http://localhost:8500" },
    "redis":  { "addr": "localhost:6379", "password": "", "db": 0 },
    "local":  { "registry_file_path": "/tmp/sicky/registry", "cleanup_on_start": false }
  },

  "broker": {
    "pool_purge_interval": 60,
    "nats":      { "url": "nats://localhost:4222" },
    "nsq":       { "endpoint": "127.0.0.1:4150", "channel": "sicky" },
    "jetstream": { "url": "nats://localhost:4222", "stream": { "name": "sicky", "subjects": ["*"], "max_consummers": 256 } }
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
})
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
    Handler: func() error {
        // daily cleanup
        return nil
    },
})
j.Start()
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
```

---

## CLI

```bash
# Scaffold a new standard microservice
sicky new --type standard --module github.com/myorg/myapp

# Scaffold an MCP server
sicky new --type mcp --module github.com/myorg/my-mcp

# Generate handler code
sicky generate handler --name User

# Generate MCP tool
sicky generate tool --name Search

# Run as MCP server
sicky serve --transport stdio --name my-mcp

# Print version
sicky version
```

---

## Package Reference

| Package | Description |
|---|---|
| `sicky` | Core orchestrator — `Init()`, `Run()`, lifecycle hooks, config loading |
| `server` | Server interface + Fiber, gRPC, net/http, TCP, UDP, WebSocket implementations |
| `broker` | Broker interface + NATS, JetStream, NSQ implementations |
| `client` | Client interface + gRPC, HTTP, TCP, UDP, WebSocket implementations |
| `service` | Service interface + Standard (background), Interactive (CLI), MCP implementations |
| `registry` | Registry interface + Consul, Redis, Local implementations |
| `tracer` | Tracer interface + OTLP/gRPC, OTLP/HTTP, Stdout, Uptrace implementations |
| `infra` | Infrastructure drivers (Redis, Bun, Ristretto, Badger, Elasticsearch, Clickhouse, MongoDB, MQTT, NATS, S3) |
| `job` | Job interface + Cron (gocron), Ticker implementations |
| `runner` | Runner interface + Static goroutine pool implementation |
| `logger` | Structured logger (slog-based), Fiber/gRPC adapters |
| `metrics` | Prometheus counters for server access and client calls |
| `utils` | Helpers — metadata, networking, HTTP envelopes, debugging |
| `cli` | CLI entry point — scaffolding, code generation, MCP serve |
| `cmd` | Binary entry point (`cmd/sicky/main.go`) |

---

## License

MIT © 2024 HereweTech Co.LTD
