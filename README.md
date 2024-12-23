# Sicky - A lightweight business framework written in GO

---

## Just simple and tiny

Just like this

'''go
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

type ConfigDef struct {
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

var (
	config ConfigDef
)

const (
	AppName = "svc.sicky"
	Version = "latest"
)

func main() {
	// Runtime
	runtime.Init(AppName)
	runtime.LoadConfig(&config)
	runtime.Start(config.Runtime)

	// HTTP server
	httpSrv := srvHTTP.New(&server.Options{Name: AppName + "@http"}, config.Server.HTTP)
	httpSrv.Handle(handler.NewHTTPGeneral())

	// GRPC server
	grpcSrv := srvGRPC.New(&server.Options{Name: AppName + "@grpc"}, config.Server.GRPC)
	grpcSrv.Handle(handler.NewGRPCGeneral())

	// Registry
	rgConsul := rgConsul.New(nil, config.Registry.Consul)

	// Service
	svc := sicky.New(&service.Options{Name: AppName}, config.Sicky)
	svc.Servers(httpSrv, grpcSrv)
	svc.Registries(rgConsul)

	service.Run()
}
'''

Rush!!
