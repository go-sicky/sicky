# Go-Sicky（中文文档）

> 英文原版：[README.md](./README.md) · Agent 开发指南：[AGENTS.md](./AGENTS.md)

一个模块化、可扩展的 Go 微服务与网络应用业务框架。

[![Go Version](https://img.shields.io/badge/Go-1.26.0-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Go-Sicky 提供统一、可插拔的架构，把基础设施、通信协议、服务治理这些脏活抽象掉，让你只写业务逻辑。

> 本文档按实际代码编写。如发现文档与代码不一致，以代码为准。仓库中存在但尚未接入分发的
> 功能会明确标注「⚠️ 不可达 / 保留未实现」，不会静默省略。

---

## 目录

- [功能特性](#功能特性)
- [安装](#安装)
- [快速开始](#快速开始)
- [架构与生命周期](#架构与生命周期)
- [配置](#配置)
- [API 示例](#api-示例)
- [CLI](#cli)
- [包索引](#包索引)
- [可观测性与运维](#可观测性与运维)
- [已知保留项与不可达功能](#已知保留项与不可达功能)
- [License](#license)

---

## 功能特性

### 多协议 Server（6 个实现）

| 协议 | 实现 | 网络 | 默认监听 | 说明 |
|---|---|---|---|---|
| HTTP（Fiber） | gofiber/v2 | `tcp` | `:9990` | |
| net/http | 标准库 + `uptrace/bunrouter` | `tcp` | `:9980` | 与 UDP 同端口号、不同 network，可共存 |
| gRPC | google.golang.org/grpc | `tcp` | `:0`（随机） | 系统分配临时端口，实际端口以 `listener.Addr()` 为准并注册 |
| TCP | 原生 socket | `tcp` | `:9981` | 支持 `MaxMessageBytes` 按连接累计接收上限（0=不限） |
| UDP | 原生 socket | `udp` | `:9980` | 与 net/http 同端口号、不同 network，可共存；支持 `MaxPacketsPerSecond` 限流 |
| WebSocket | Fiber + `gofiber/contrib/websocket` | `tcp` | `:9991` | 默认路径 `/conn`（注：`gorilla/websocket` 只是间接依赖，直接依赖是 contrib 包） |

### 可插拔基础设施（10 个组件）

| 组件 | 库 | 用途 |
|---|---|---|
| Redis | go-redis/v9 | 缓存 / KV |
| Bun | uptrace/bun | SQL ORM（Pg、MySQL、SQLite、MSSQL、达梦经 `oracledialect` 兼容） |
| Ristretto | dgraph-io/ristretto/v2 | 进程内高性能缓存 |
| Badger | dgraph-io/badger/v4 | 嵌入式 KV |
| Elasticsearch | elastic/go-elasticsearch/v9 | 全文检索 |
| Clickhouse | uptrace/go-clickhouse | 分析型数据库 |
| MongoDB | go.mongodb.org/mongo-driver/v2 | 文档数据库 |
| MQTT | eclipse/paho.mqtt.golang | 物联网消息 |
| NATS（infra） | nats-io/nats.go | 消息连接（与 broker 层 NATS 是不同用途） |
| S3 | aws-sdk-go-v2 | 对象存储 |

规则（按实际代码）：**配置节出现即表示启用**。`Init*` 对空必填字段直接 abort 启动；
所有 `*Config` 都有 nil-safe 的 `Ensure()`（补默认值）+ `Validate()`（缺必填 abort；
timing 类字段 0 填默认、负值 abort，NATS 的 `max_reconnects: -1` 例外，表示无限重连）。
单例首次写入胜出（first-wins），重复 `Init` 会关闭刚建好的重复连接后返回已有单例，不泄漏。
业务 handler 里请用 `infra.GetRedis()` / `infra.GetBun()` / `infra.GetMongoDB("mydb")` 等 getter 读单例，
不要直读导出的全局变量（与 `Init`/`Clear` 并发读写会 data race）。

### 消息 Broker（3 个）

- **NATS Core** — 至少一次投递的 pub/sub
- **NATS JetStream** — 持久化流，可配 retention
- **NSQ** — 分布式、至少一次投递

### 服务发现（3 个可用 + 1 个已废弃）

- **Consul** — HashiCorp Consul Agent API
- **Redis** — Redis Hash + Pub/Sub 通知
- **Local** — 文件系统 JSON（`registry/local`，`registry_file_path` 必须绝对路径；`MkdirAll 0700`；
  `cleanup_on_start` 只删 `<uuid>.json`）
- ~~mDNS~~ — ⚠️ 已废弃：`registry/mdns/` 下两个文件 100% 注释，仅作纪念保留，不再支持，也不会引入 `zeroconf` 依赖

### 可观测性

- **OpenTelemetry tracing** — OTLP/gRPC、OTLP/HTTP、Stdout 三种标准 exporter；Uptrace 走独立 SDK（见下）
- **Uptrace** — `dsn` 方式接入 Uptrace SaaS（`sample_rate` 在 Uptrace 模式下被忽略，服务端采样）
- **传播协议** — W3C TraceContext + W3C Baggage + B3（single/multi）双发；`tracer.InstallPropagator()` 由 `sicky.go` 在 tracer 初始化成功后统一安装一次
- **Prometheus metrics** — `sicky_` 前缀的 RED 指标（server/client/broker/job/runner/registry/infra/manager）+ 3 个 collector（`build_info`、`go`、`process`），由 Manager `/metrics` 统一暴露，不要另起 metrics 端口
- **结构化日志** — 基于 slog 的内置 logger，带 Fiber/gRPC 适配器。⚠️ `logger.NewFiberMiddleware` 是已废弃的遗产（现已无计数），
  不要和 `server/fiber` 内置链路同时挂载，否则 `sicky_server_requests_total{server="fiber"}` 会 double-count
- **Manager 端点** — 实际注册 **8 个**（`manager.go:256-263`）：
  `/metrics`（公开，给 Prometheus 抓）、`/health` + `/ready`（同一套聚合：10 个 infra + 注册的业务 checker；
  只有 `unhealthy` 拉低整体状态，`not_configured` 上报但视为健康；infra 部分并发跑在 2s ctx 下，
  业务 checker 部分是串行的；后端错误文本不回显给调用方，未配置 token 时 NATS/MQTT 失联显示 `disconnected`、
  其余显示 `unhealthy`，详情只打服务端日志）、`/live`（静态 200，只做存活）、`/version`、`/info`、
  `/config`（`expose_config=false` 时 403 优先；开了也会脱敏；需鉴权）、`/services`（注册池快照，需鉴权）。
  鉴权：设了 `manager.auth_token` 则 `/config`、`/services` 要求 `Authorization: Bearer <token>`（token 对不上回 401）；
  没设 token 时这两个端点只接受 loopback。无 token 又监听在外网地址（默认 `:8888` 就是外网）时启动会打 Warn。
  可选 `tls_cert_pem`/`tls_key_pem` 切 HTTPS（TLS 1.2+，只配一半则 `Start` 直接失败，绝不静默回落明文）；
  为空保持明文（默认）。`ReadHeaderTimeout=5s`、`MaxHeaderBytes=1M` 为硬编码不可配。

### 后台任务与并发

- **Cron** — 基于 gocron/v2 的 cron 调度；`Task` 可配 `Timeout` 看门狗（超时记失败，但泄漏的那次跑是杀不掉的，会在文档和日志里说明）
- **Ticker** — 固定间隔任务；`Stop` 会等循环退出，支持 Start-Stop-Start
- **Static Runner** — goroutine 池任务分发。`Task()` 队满时阻塞——**绝不要在 Handler 里调它**（死锁）；
  用 `TryTask(t, timeout)` 有界入队（`timeout<=0` 非阻塞，满/未启动/已停止回 `runner.ErrPoolFull`），`Len()` 看排队深度

### MCP（Model Context Protocol）

- 内置 MCP server（JSON-RPC 2.0），支持 Tools、Resources、Prompts
- 可插拔传输：stdio 或 HTTP（SSE 在 `/mcp`，POST 在 `/mcp/message`）

### CLI

`sicky new/generate/serve/version/help`，详见 [CLI](#cli) 章节（含不可达命令标注）。

---

## 安装

要求 Go 1.26.0 及以上。

```bash
go install github.com/go-sicky/sicky/cmd/sicky@latest
```

或作为依赖引入：

```bash
go get github.com/go-sicky/sicky
```

---

## 快速开始

### 1. 写配置文件

`config.json`：

```json
{
  "log_level": "info",
  "manager": {
    "enable": true,
    "address": ":8888"
  }
}
```

> ⚠️ 注意：`sicky.Config` 里 `Manager` 字段为 `nil` 表示**禁用** Manager（breaking 语义，
> 只有 `DefaultConfig()` 才默认启用）。上面 JSON 能生效是因为 Viper 反序列化后 `Manager` 非 nil。
> 如果你在代码里手写 `cfg := &sicky.Config{}` 然后 `Run(cfg)`，Manager 是不会启动的，
> `/health`、`/metrics` 都没有——这是符合代码行为的，不是 bug。

### 2. 写业务服务

`main.go`：

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
            return // --version：版本已打印，直接退出
        }
        log.Fatalf("init failed: %v", err)
    }

    // 建服务 —— New() 自动 service.Set() 自注册
    svc := svcStandard.New(&service.Options{Name: "myapp"}, nil)

    // 建 HTTP server
    srv := srvFiber.New(&server.Options{Name: "http-server"}, &srvFiber.Config{
        Address: ":8080",
    })

    // 注册路由
    srv.App().Get("/", func(c *fiber.Ctx) error {
        return c.JSON(fiber.Map{"message": "Hello from Go-Sicky!"})
    })

    // 把 server 挂到 service 上
    svc.Servers(srv)

    // 读配置（Viper → struct）并运行 —— Run() 阻塞到信号到来
    cfg := &sicky.Config{}
    if err := sicky.ConfigUnmarshal(cfg); err != nil {
        log.Fatalf("config unmarshal failed: %v", err)
    }
    if err := sicky.Run(cfg); err != nil {
        log.Fatalf("run failed: %v", err)
    }
}
```

### 3. 运行

```bash
go run main.go
```

健康检查看 `http://localhost:8888/health`，业务接口看 `http://localhost:8080/`。

---

## 架构与生命周期

```
┌──────────────────────────────────────────────────────────────┐
│                    sicky（编排器）                             │
│  Init(): flags(--config/-C 大写, --config-type, --version/-V)  │
│        + FlagSwitch bool 扩展 flags                            │
│        → Viper 加载（本地 /etc、/etc/<app>、$HOME/.<app>、.    │
│          或远端 consul://）+ SICKY_ 环境变量 + Must* 检查       │
│        → 返回 error（绝不 os.Exit；-V 返回 ErrVersionShown）     │
│  Run(): validateConfig(钳制非法值并 Warn)                      │
│        → beforeStart → Infra(10) → Tracer(4) → Registry(3)     │
│          (InitPool → Watch → pool purge ticker)                │
│        → Broker(3) → Flag 回调 → Manager(8 个端点)              │
│        → Services.Start + Registry.Register → afterStart       │
│        → 等待（TERM/INT/QUIT/ABRT 关机，HUP 只触发 reload 不停机，│
│          或 Context 取消）→ beforeStop → Registry.Deregister    │
│        → Services.Stop → Broker.Disconnect → Registry.Stop     │
│        → Tracer.Stop → Manager.Stop → Infra.Close → afterStop  │
└──────────────────────────────────────────────────────────────┘
│                  service.Service                              │
│  ┌──────────┐  ┌─────────────┐  ┌─────┐                       │
│  │ Standard │  │ Interactive │  │ MCP │ (+ protocol)          │
│  └──────────┘  └─────────────┘  └─────┘                       │
├──────────────────────────────────────────────────────────────┤
│   server.Server  ←→  broker.Broker  ←→  client.Client        │
│   job.Job        ←→  runner.Runner                            │
├──────────────────────────────────────────────────────────────┤
│   registry.Registry    tracer.Tracer（OTLP gRPC/HTTP、Stdout、 │
│                                      Uptrace + B3）            │
├──────────────────────────────────────────────────────────────┤
│                       infra（10 组件）                         │
│  Redis Bun Ristretto Badger Elastic Clickhouse                │
│  Mongo MQTT NATS S3                                           │
└──────────────────────────────────────────────────────────────┘
```

要点（按实际代码）：

- **自注册模式**：每个实现的 `New()` 都会调包级 `Set(self)` 把自己塞进全局 map
  （如 `broker.Set(brk)`、`server.Set(srv)`）。编排器 `Run()` 时遍历已注册实例。
  `server` 包有 `Set/Get` 但**没有 `Default()`**（和 broker/registry/tracer/service 不一样）。
- **启动失败**：任何一步失败都把 error join 进返回值，并走完整 shutdown 链，库内绝不 `os.Exit`。
- **停机顺序是先摘注册再停服务**（`Deregister` → `Stop`），启动失败的 service 只 Stop 一次，不会 double-stop。
- **SIGHUP**：只触发 `OnReload` 回调（独立 goroutine，进程继续服务），不再触发关机。
- **二次信号 / 超时**：第二遍信号只 cancel；强制关机超时取 `max(30s, Manager.ShutdownTimeout)`，超时只 cancel+记日志，由调用方决定进程退出。
- **Hook 失败**只记 error 日志，不中断流程。Sicky 级 hook 签名 `func(ctx) error`；
  Server 级 hook 签名 `func() error`（无 ctx），两边不一样，注意别混。

---

## 配置

Go-Sicky 用 [Viper](https://github.com/spf13/viper) 读配置，支持本地文件和远端 provider。

### 命令行 flags（按实际代码）

- `--config/-C`（注意是大写 C）：配置名，默认 `config`
- `--config-type`：配置格式，默认 `json`（无短 flag）
- `--version/-V`：打印 `AppName Version (Branch) Build Commit BuildTime` 并返回 `ErrVersionShown`（调用方自行 exit 0）
- `Init(opts, switches...)` 还可注册额外的 bool 型 `FlagSwitch`；`Init` 调两次返回 `ErrAlreadyInitialized`
- ⚠️ 别用小写 `-c`，代码里不存在

### 配置文件查找（按实际代码：4 个目录 + 配置名）

1. `/etc/`（找 `<config>.<type>`）
2. `/etc/<app_name>/`
3. `$HOME/.<app_name>/`（经 `os.UserHomeDir()` 解析，不是字面量 `$HOME`）
4. `./`（当前目录；从 CWD 加载会打 Warn，提醒确认 CWD 可信）

### 远端配置

配置路径带 scheme 即走远端 provider：

```bash
-C consul://localhost:8500/myapp/config
```

远端地址含凭据（如 `consul://user:pass@host/path`）时，读配置成功的日志会用 `url.Redacted()` 脱敏。

### 环境变量

统一 `SICKY_` 前缀，点转下划线：

```bash
SICKY_LOG_LEVEL=debug
SICKY_MANAGER_ADDRESS=:9999
SICKY_INFRA_REDIS_ADDR=localhost:6379
```

⚠️ Viper 的 `AutomaticEnv` 对 int 字段做类型-coerce 失败时会静默置零。
`validateConfig()` 会在 `beforeStart` 之前把非法值钳回默认并打 Warn（不清零启动）：
`tracer.timeout<0→0`、`sample_rate∉[0,1]→1.0`、`manager.shutdown/read/write/idle<=0→默认值`、未知 `log_level` 警告（按 info 跑）。

### 完整配置示例（按实际字段）

```json
{
  "log_level": "info",

  "manager": {
    "enable": true,
    "address": ":8888",
    "advertise_address": "",
    "expose_config": false,
    "auth_token": "",
    "tls_cert_pem": "",
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
    "config_path": "/config",
    "service_pool_path": "/services"
  },

  "infra": {
    "redis":     { "addr": "localhost:6379", "username": "", "password": "", "db": 0, "enable_tls": false, "tls_skip_verify": false, "dial_timeout_sec": 5, "read_timeout_sec": 3, "write_timeout_sec": 3, "pool_size": 0, "min_idle_conns": 0 },
    "bun":       { "driver": "pg", "dsn": "postgres://...", "debug": false, "verbose": false, "slow_duration": 0, "max_open_conns": 0, "max_idle_conns": 0, "conn_max_lifetime_sec": 0, "conn_max_idle_time_sec": 0 },
    "badger":    { "path": "/tmp/badger" },
    "ristretto": { "num_counters": 10000000, "max_cost": 100000000, "buffer_items": 64 },
    "nats":      { "url": "nats://localhost:4222", "token": "", "username": "", "password": "", "creds_file": "", "nkey_file": "", "enable_tls": false, "root_ca_file": "", "timeout_sec": 5, "reconnect_wait_sec": 2, "max_reconnects": 60 },
    "mqtt":      { "broker": "tcp://localhost:1883", "client_id": "", "username": "", "password": "", "enable_tls": false, "ca_file": "", "keep_alive_sec": 30, "connect_timeout_sec": 5, "clean_session": true },
    "elastic":   { "addresses": ["http://localhost:9200"], "username": "", "password": "", "cloud_id": "", "api_key": "", "service_token": "", "certificate_fingerprint": "", "ca_cert_file": "", "timeout_sec": 5 },
    "clickhouse":{ "dsn": "clickhouse://..." },
    "mongo":     { "uri": "mongodb://localhost:27017", "db": "mydb", "max_pool_size": 0, "connect_timeout_sec": 0 },
    "s3":        { "region": "us-east-1", "endpoint": "", "access_key": "", "secret_key": "", "session_token": "", "bucket": "", "use_path_style": false, "timeout": 5, "request_timeout_sec": 0 }
  },

  "tracer": {
    "type": "none",
    "service_name": "",
    "service_version": "",
    "endpoint": "localhost:4317",
    "insecure": false,
    "headers": {},
    "sample_rate": 1.0,
    "compress": false,
    "timeout": 30,
    "dsn": "",
    "pretty_print": false,
    "timestamps": false
  },

  "registry": {
    "pool_purge_interval": 60,
    "consul": { "endpoint": "http://localhost:8500" },
    "redis":  { "addr": "localhost:6379", "password": "", "db": 0 },
    "local":  { "registry_file_path": "/tmp/sicky/registry", "cleanup_on_start": false }
  },

  "broker": {
    "nats":      { "url": "nats://localhost:4222" },
    "nsq":       { "endpoint": "127.0.0.1:4150", "channel": "sicky" },
    "jetstream": { "url": "nats://localhost:4222", "stream": { "name": "sicky", "subjects": ["*"], "max_consumers": 256 } }
  }
}
```

字段说明（按实际 `Validate` 行为）：

- `manager`：`enable_swagger` / `swagger_path` ⚠️ **保留未实现**——字段存在、有默认值，
  但 Manager 实际只注册 8 个端点（见上），不 serve swagger。`advertise_address` 为空时回退到 `address`。
  `auth_token` 为空时 `/config`、`/services` 仅 loopback 可访；`expose_config=false` 时 `/config` 直接 403（优先级最高）。
- `infra.nats`：`max_reconnects: -1` 表示无限重连（特批放行），其他负值 abort；
  auth 四选一（token/username+password/creds_file/nkey_file）冲突 abort；
  配了 `root_ca_file` 却没开 `enable_tls` 则 abort；文件路径都会 `os.Stat` 检查。
- `infra.mqtt`：有 `password` 没 `username` 直接 abort（`ErrMQTTPasswordOrphan`）；`clean_session` 是 `*bool`，nil 表示用 paho 默认。
- `infra.elastic`：`addresses` 与 `cloud_id` 至少其一；`timeout_sec` 只管启动检查，单次请求超时走调用方 ctx。
- `infra.s3`：`region` 必填，`bucket` 可选（没配 bucket 时健康检查只判 client 非 nil）；
  有 `access_key` 没 `secret_key` 则 abort；`endpoint` 给 MinIO/LocalStack 用。
- `infra.bun`：未知 driver 直接 abort，不会回落 pg；`debug` 不再隐含 verbose（行为变更，verbose 要单开）；
  `max_open_conns` 不限会打 Warn。达梦走 `oracledialect` 兼容。
- `infra.mongo`：库名优先级 显式参数 > `cfg.db` > URI path；健康检查用 `readpref.Primary()` ping。
- `tracer`：`type: none|grpc|http|stdout|uptrace`（默认示例里写 `none` 即关闭）；
  `service_name/version` 为空回退到 `AppName/Version`；`uptrace` 必须配 `dsn`（`sample_rate` 会被忽略）；
  标准 OTLP 三件套缺 `endpoint` 时按 exporter 默认走，不强制 abort（`ErrTracerNoEndpoint` 已定义但当前未使用）。
- `registry.local`：路径必须绝对路径，相对路径 fail-fast；`cleanup_on_start` 只清 `<uuid>.json`。
- 时间单位注意：http/fiber 用 `time.Duration`（viper 里写 `"10s"`），tcp/udp/manager 用 int 秒——历史原因未统一，改了会 break 现有配置，故保持现状。

---

## API 示例

### 建服务

```go
import (
    svcStandard "github.com/go-sicky/sicky/service/standard"
    "github.com/go-sicky/sicky/service"
)

svc := svcStandard.New(&service.Options{
    Name:    "user-service",
    Version: "1.0.0",
}, nil) // 第二个 *service.Config 可 nil
```

### Fiber HTTP Server（含 CORS 正例）

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

> CORS 两个 HTTP 栈都是默认拒绝：`cors.allowed_origins` 为空就不发 `Access-Control-Allow-Origin`。
> `"*"` + `allow_credentials: true` 非法（`Validate()` 拒绝，server 构造 fail-closed 到 deny-all）。
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
>
> ⚠️ 别用废弃的 `server/http.CORSMiddleware`（现在是 deny-all 别名，挂了等于全拒）。
> 要用 `NewCORSMiddleware` + 显式白名单。

### gRPC Server

```go
import (
    srvGrpc "github.com/go-sicky/sicky/server/grpc"
    "github.com/go-sicky/sicky/server"
)

srv := srvGrpc.New(&server.Options{Name: "grpc"}, &srvGrpc.Config{
    Address: ":9090",
})

// 注册你的 protobuf 服务
pb.RegisterUserServiceServer(srv.App(), &userServer{})
```

### gRPC Client（TLS + 服务发现）

```go
import (
    cltGrpc "github.com/go-sicky/sicky/client/grpc"
    "github.com/go-sicky/sicky/client"
)

// 直连模式
clt := cltGrpc.New(&client.Options{Name: "user-client"}, &cltGrpc.Config{
    Addr: "127.0.0.1:9090",
})

// 服务发现模式：从注册池解析（Instance.Servers[type==grpc]），跟随池更新
disc := cltGrpc.New(&client.Options{Name: "user-client"}, &cltGrpc.Config{
    Service:  "user-service",
    Balancer: "round_robin",
})

// mTLS：两个字段必须成对 —— 配一半直接失败（nil client + ErrIncompleteTLSConfig），绝不静默明文
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

// ... 实现 OnClose、OnError

srv := srvTCP.New(&server.Options{Name: "tcp"}, &srvTCP.Config{
    Address:    ":9981",
    BufferSize: 4096,
    // MaxMessageBytes: 1 << 20, // 可选：按连接累计接收上限（0=不限）
})
srv.Handle(&MyHandler{})
```

TCP accept 失败和 UDP 读失败走 capped 指数退避 + jitter，不会打爆 CPU，也不会一个瞬时错误就杀掉整个循环；
`On*` 回调都有 panic 隔离；`Send` 串行化 + 写 deadline。

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

### 消息（NATS）

```go
import (
    brkNats "github.com/go-sicky/sicky/broker/nats"
    "github.com/go-sicky/sicky/broker"
)

// 发布
brk := brkNats.New(&broker.Options{Name: "nats"}, &brkNats.Config{
    URL: "nats://localhost:4222",
})
brk.Connect()

msg := &broker.Message{Topic: "orders.created"}
msg.Format(myOrder)
brk.Publish("orders.created", msg)

// 订阅
brk.Subscribe("orders.created", func(m *broker.Message) error {
    var order Order
    m.Scan(&order)
    // 处理订单...
    return nil
})
```

注意：`Message.Format` / `Scan` 会返回编解码 error，调用方必须检查。

### 后台任务

```go
import (
    jobCron "github.com/go-sicky/sicky/job/cron"
    "github.com/go-sicky/sicky/job"
)

j := jobCron.New(&job.Options{Name: "cleanup"}, &jobCron.Config{})
j.Add(&jobCron.Task{
    Expression: "0 0 * * *",
    Timeout:    5 * time.Minute, // 可选看门狗：超时记失败（泄漏的那次跑杀不掉）
    Handler: func() error {
        // 每天清理
        return nil
    },
})
j.Start()
```

Handler 签名没有 ctx（`func() error` / `func(time.Time, uint64) error`），为兼容性保留。

### 任务 Runner（背压）

`Static.Task()` 队满阻塞——别在 Handler 里调。`TryTask` 有界入队：

```go
if err := r.TryTask(&runner.Task{Data: work}, 100*time.Millisecond); errors.Is(err, runner.ErrPoolFull) {
    //  shedding：稍后重试或丢弃
}
_ = r.Len() // 排队深度，可做观测
```

### 基础设施

```go
import "github.com/go-sicky/sicky/infra"

// Redis
infra.Redis.Set(ctx, "key", "value", 0)
val, _ := infra.Redis.Get(ctx, "key").Result()

// Bun (SQL)
infra.Bun.NewSelect().Model(&users).Scan(ctx)

// Ristretto（缓存）
infra.Ristretto.Set("token:123", userData, 1)
val, _ := infra.Ristretto.Get("token:123")

// Badger（KV）
infra.Badger.Update(func(txn *badger.Txn) error {
    return txn.Set([]byte("key"), []byte("value"))
})

// handler 里推荐用 getter（无 race）：
// infra.GetRedis()、infra.GetBun()、infra.GetMongoDB("mydb")
```

### 生命周期 Hook

```go
sicky.BeforeStart(func(ctx context.Context) error {
    logger.Info("About to start services")
    return nil
})

sicky.AfterStop(func(ctx context.Context) error {
    logger.Info("All services stopped, cleaning up")
    return nil
})

// SIGHUP 热加载（进程继续服务；失败只记日志）
sicky.OnReload(func(ctx context.Context) error {
    logger.Info("Reloading on SIGHUP")
    return nil
})

// 业务健康检查，并入 /health 和 /ready
sicky.RegisterHealthChecker("order-db", func(ctx context.Context) error {
    return orderStore.Ping(ctx)
})
```

Server 级 hook 签名是 `func() error`（无 ctx），在各 server 的 `Start()`/`Stop()` 内执行。

### 错误码 + 通用 envelope

```go
import "github.com/go-sicky/sicky/utils"

_ = utils.RegisterErrorCode(40010, "order already paid", utils.StatusConflict)
err := utils.NewCodedError(40010, "", dbErr) // 支持 Unwrap
code := utils.CodeOf(err)                    // 沿链找，默认 5000

ok := utils.OkT(order)                       // 成功 envelope
fail := utils.FailT[any](utils.CodeNotFound, "").
    WithRequestID(reqID)                     // message/status 走注册表
```

---

## CLI

实际可达命令只有 5 个（`cli/sicky.go:51 Run()`）：`serve|new|generate|version|help`
（各有单字母别名 `s/n/g/v`；无参默认进 `serve`；`-` 开头透传给 `serve`；未知命令 exit 1）。

```bash
# 脚手架：标准微服务（项目名是必填位置参数）
sicky new myapp --type standard --module github.com/myorg/myapp
# 可选：--output/-o <dir>（默认 "."）、--no-grpc

# 脚手架：MCP server
sicky new my-mcp --type mcp --module github.com/myorg/my-mcp

# 生成 handler/tool/resource（名字是位置参数，没有 --name flag）
sicky generate handler User
sicky generate tool Search
sicky generate resource Article
sicky generate doc

# 以 MCP server 运行（--name/--version 也可用）
sicky serve --transport stdio --name my-mcp
sicky serve --transport http --listen :3000

# 版本 / 帮助
sicky version
sicky help        # 另有：sicky serve -h、sicky new -h
```

`new` 缺省进交互式提问（standard/mcp/interactive + gRPC/Fiber/Proto 追问）。

> ⚠️ 不可达功能（保留标注，当前调了会报 `unknown command`）：
> `sicky config（validate|init|show）`、`sicky proto（build|new）`、`sicky doctor`、
> `sicky info`、`sicky run/run --watch`、`sicky mcp` —— `cli/*.go` 里有实现，
> 但 `Run()` 的 switch 没引用。同时 `generate` 还有 `server/client/service/broker/job/middleware/proto/config/docker/k8s`
> 共 9 个 stub 生成器同样无分发入口、不可达。文档如实保留，将来要么注册要么删除，本次不动代码。

---

## 包索引

| 包 | 说明 |
|---|---|
| `sicky` | 编排器 —— `Init()`（返回 error；`ErrVersionShown`/`ErrAlreadyInitialized` 哨兵）、`Run()`（返回 join 后的 error，优雅关机）、`Viper()`、`ConfigUnmarshal()`、生命周期 hook（`Before/AfterStart/Stop`，HUP 触发 `OnReload`）、`FlagSwitch`、Manager |
| `server` | Server 接口 + Fiber、gRPC、net/http（bunrouter）、TCP、UDP、WebSocket 实现（有 `Set/Get` 无 `Default()`） |
| `broker` | Broker 接口 + NATS、JetStream、NSQ 实现 |
| `client` | Client 接口 + gRPC、HTTP、TCP、UDP、WebSocket 实现（gRPC 支持 TLS 1.2+ mTLS fail-fast 和基于注册池的服务发现；发现模式有 NotifyChan watcher + 30s resync，`Disconnect` 停） |
| `service` | Service 接口 + Standard（后台）、Interactive（CLI）、MCP（+ `mcp/protocol`） |
| `registry` | Registry 接口 + Consul、Redis、Local（文件 JSON）—— `mdns` 已废弃（注释保留）；`pool.go` 有 `NotifyChan()`（`InitPool` 前为 nil，`PurgePool` 后保持稳定）供 gRPC client 这类实时订阅者用 |
| `tracer` | Tracer 接口 + OTLP/gRPC、OTLP/HTTP、Stdout、Uptrace（+ 已废弃的 `tracer/fiber.go` B3 helper，新代码用 `server/fiber` 的 tracer 中间件）；TCP/UDP/WebSocket 无 tracing（无标准载体，by design） |
| `infra` | 10 个驱动（Redis、Bun、Ristretto、Badger、Elasticsearch、Clickhouse、MongoDB、MQTT、NATS、S3）—— 扁平 `infra/*.go`，无接口、无 `interface.go` |
| `job` | Job 接口 + Cron（gocron）、Ticker 实现 |
| `runner` | Runner 接口 + Static goroutine 池 |
| `logger` | 结构化日志（slog 底）+ Fiber/gRPC 适配器（`NewFiberMiddleware` 已废弃，见上） |
| `metrics` | Prometheus 11 个 counter（6 server + 5 client）+ 3 个 collector（`build_info`、`go`、`process`）；gRPC `Call()` 只 bump counter 满足接口，真实 unary 计数在 `Invoke`，`NewStream` 无 counter |
| `utils` | `metadata`、`net`（IP 解析/`Net2fd`/`Advertise`）、`http` envelope（含 `Pagination`）、`debug`、`misc`、`backoff`（`NewBackoff` capped 指数+jitter + `NewLogSampler` 限频）、`errors`（错误码注册表 + `CodedError` + 泛型 `EnvelopeT[T]`） |
| `internal` | 内部请求 `Context`（ID、AppName、Version、Branch、Metadata、StartTime、broker/registry/tracer/logger 载体） |
| `cli` | CLI 分发 —— 可达 `serve/new/generate/version/help`（`cli/sicky.go:51 Run()`） |
| `cmd` | 二进制入口（`cmd/sicky/main.go`，仅 `os.Exit(cli.Run())`） |

---

## 可观测性与运维

- **指标**：新加 server/client 协议时，去 `metrics/metrics.go` 加 counter 并在正确位置 `.Inc()`：
  server 在 access-log 拦截器/中间件调 handler 前（TCP/UDP/WS 在 `OnData` 循环前），client 在 `Call()`/`Invoke` 第一句。
- **健康**：业务检查走 `RegisterHealthChecker`，自动并入 `/health` + `/ready`（同 2s ctx，错误文本脱敏）。
  不要另起 metrics/health 端口，一律走 Manager。
- **追踪**：`tracer.type` 选 `grpc|http|stdout` 走标准 OTLP（显式 exporter+provider），
  `uptrace` 走 uptrace-go SDK（自带 provider）。server 拦截器（fiber/http/grpc）经统一 propagator
  提取后向下游重注 W3C+B3；`tracestate` 只透传不存储/不脱敏。`SkipPaths` 默认跳过 `/health`、`/metrics`、`/docs`
  （显式配空 = 全量追踪）。
- **日志与密钥**：infra 成功打 Info、失败打 Error；DSN/URI userinfo、password、token、api-key、secret 一律不进日志
 （DSN 走 `redactDSN()`，其余只记 endpoint/username）。`consul://user:pass@...` 这类远端地址在日志里自动 `Redacted()`。
- **goroutine**：所有 server/manager 后台 goroutine 用 `defer wg.Done()`，`go func()` 不带 error 返回，错误内部记日志。
  TCP 分 accept 与连接两组 WaitGroup 两阶段停；fiber 用 `ShutdownWithTimeout`（裸 `fasthttp Shutdown` 在自定义 listener 上会 hang）。
- **gRPC 拦截器**：tracing + logging 合并在**单个** `ChainUnaryInterceptor(tracing, logging)` /
  `WithChainUnaryInterceptor` 里（grpc v1.83.2 下重复调 Chain 是 append 不是覆盖，但单次调用版本-proof）；
  streaming 必须配 `ChainStreamInterceptor` / `WithChainStreamInterceptor`（tracing + logging + counters）。
  `server/grpc/metadata.go` 的 `NewMetadataInterceptor` 是故意留的 no-op 占位，不用接线。

---

## 已知保留项与不可达功能

按实际代码如实保留，不删除、不隐瞒：

- `registry/mdns/`：100% 注释，纪念保留，不支持，不引 `zeroconf`。
- `server/grpc/metadata.go:40 NewMetadataInterceptor`：no-op 占位，故意不接线。
- `logger/fiber.go:113 NewFiberMiddleware`、`tracer/fiber.go`：已废弃，保留不维护；别和新链路叠挂。
- 脚手架里 `tool.go.gotmpl`（`ReadResource`/`GetPrompt`）、`resource.go.gotmpl`（`CallTool`/`GetPrompt`）
  会返回显式 `not implemented` error（和 `project/mcp/handler.go.gotmpl` 一致），不是静默 `nil, nil`。
- CLI 悬空命令与 generate stub（见 [CLI](#cli)），有定义、无注册。
- `manager.enable_swagger` / `swagger_path`：保留未实现（见上）。
- `config.ErrTracerNoEndpoint`：已定义、当前 `Validate()` 未使用（OTLP 缺 endpoint 走 exporter 默认）。
- 默认值 by-design：Manager 默认 `:8888`（外网）；TLS 只做 1.2+；`0`=禁用/不限 cracking；gRPC keepalive、NATS 耗尽等保持现状；
  `BodyLimit` 非正填默认（本版无 opt-out）；TCP/UDP 负超时/会话数/限流钳制 + 大声记日志（不断言 abort）。

---

## License

MIT © 2024 HereweTech Co.LTD
