# Sicky - A Lightweight Business Framework for Go

![Go Version](https://img.shields.io/badge/go-%3E%3D1.20-blue.svg)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A minimalist framework for building scalable microservices and distributed systems in Go.

## Features

- 🚀 Multiple protocol support (HTTP/gRPC/TCP/UDP/WebSocket)
- 🔌 Pluggable service registry (Consul/mDNS)
- 📊 Built-in metrics and tracing (Prometheus/OpenTelemetry)
- ⚙️ Configuration management with Viper
- 🔄 Cron job scheduling
- 🐳 Runtime integration (Docker/Nomad)

## Quick Start

### Installation
```bash
go get github.com/go-sicky/sicky
```

### Basic Usage
```go
package main

import (
	"svc/handler"

	rgConsul "github.com/go-sicky/sicky/registry/consul"
	"github.com/go-sicky/sicky/runtime"
	"github.com/go-sicky/sicky/server"
	srvGRPC "github.com/go-sicky/sicky/server/grpc"
	srvHTTP "github.com/go-sicky/sicky/server/http"
	"github.com/go-sicky/sicky/service"
	"github.com/go-sicky/sicky/service/sicky"
)

type Config struct {
	Server struct {
		GRPC *srvGRPC.Config `json:"grpc" yaml:"grpc" mapstructure:"grpc"`
		HTTP *srvHTTP.Config `json:"http" yaml:"http" mapstructure:"http"`
	} `json:"server" yaml:"server" mapstructure:"server"`
	Registry struct {
		Consul *rgConsul.Config `json:"consul" yaml:"consul" mapstructure:"consul"`
	} `json:"registry" yaml:"registry" mapstructure:"registry"`
	Runtime *runtime.Config `json:"runtime" yaml:"runtime" mapstructure:"runtime"`
	Sicky   *sicky.Config   `json:"sicky" yaml:"sicky" mapstructure:"sicky"`
}

const (
	AppName = "svc.sicky"
	Version = "latest"
)

func main() {
	// Initialize runtime
	runtime.Init(AppName)
	runtime.LoadConfig(&Config{})
	runtime.Start(config.Runtime)

	// Create servers
	httpServer := srvHTTP.New(&server.Options{
		Name: AppName + "@http",
	}, config.Server.HTTP)
	httpServer.Handle(handler.NewHTTPGeneral())

	grpcServer := srvGRPC.New(&server.Options{
		Name: AppName + "@grpc",
	}, config.Server.GRPC)
	grpcServer.Handle(handler.NewGRPCGeneral())

	// Configure registry
	consulRegistry := rgConsul.New(nil, config.Registry.Consul)

	// Create service
	service := sicky.New(&service.Options{
		Name: AppName,
	}, config.Sicky)
	service.Servers(httpServer, grpcServer)
	service.Registries(consulRegistry)

	// Start service
	service.Run()
}
```

## Configuration

```yaml
server:
  http:
    addr: ":8080"
  grpc:
    addr: ":9090"
registry:
  consul:
    address: "localhost:8500"
runtime:
  shutdown_timeout: 30s
```

## Dependencies

- [GoFiber](https://gofiber.io/) - Web framework
- [Viper](https://github.com/spf13/viper) - Configuration management
- [OpenTelemetry](https://opentelemetry.io/) - Distributed tracing
- [Bun](https://bun.uptrace.dev/) - SQL ORM
- [Swag](https://github.com/swaggo/swag) - API documentation
- [gRPC-Go](https://grpc.io/docs/languages/go/) - RPC framework
- [Prometheus](https://prometheus.io/) - Metrics monitoring

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
