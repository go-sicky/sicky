/*
 * The MIT License (MIT)
 *
 * Copyright (c) 2021 HereweTech Co.LTD
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy of
 * this software and associated documentation files (the "Software"), to deal in
 * the Software without restriction, including without limitation the rights to
 * use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
 * the Software, and to permit persons to whom the Software is furnished to do so,
 * subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
 * FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
 * COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
 * IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
 * CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 */

/**
 * @file config.go
 * @package sicky
 * @author Dr.NP <np@herewe.tech>
 * @since 11/21/2023
 */

package sicky

import (
	"strings"

	cgrpc "github.com/go-sicky/sicky/client/grpc"
	"github.com/go-sicky/sicky/driver"
	sgrpc "github.com/go-sicky/sicky/server/grpc"
	shttp "github.com/go-sicky/sicky/server/http"
	swebsocket "github.com/go-sicky/sicky/server/websocket"
	"github.com/spf13/viper"
)

const (
	DefaultServiceName         = "sicky.service"
	DefaultServiceVersion      = "latest"
	DefaultMetricsExporterPath = "/metrics"
	DefaultMetricsExporterAddr = ":9999"
	DefaultLogLevel            = "info"
)

type ConfigService struct {
	Name    string `json:"name" yaml:"name" mapstructure:"name"`
	Version string `json:"version" yaml:"version" mapstructure:"version"`
}

type ConfigGlobal struct {
	Sicky struct {
		Service ConfigService `json:"service" yaml:"service" mapstructure:"service"`
		Servers struct {
			HTTP      map[string]*shttp.Config      `json:"http" yaml:"http" mapstructure:"http"`
			GRPC      map[string]*sgrpc.Config      `json:"grpc" yaml:"grpc" mapstructure:"grpc"`
			Websocket map[string]*swebsocket.Config `json:"websocket" yaml:"websocket" mapstructure:"websocket"`
		} `json:"servers" yaml:"servers" mapstructure:"servers"`
		Clients struct {
			GRPC map[string]*cgrpc.Config `json:"grpc" yaml:"grpc" mapstructure:"grpc"`
		} `json:"clients" yaml:"clients" mapstructure:"clients"`
		Drivers struct {
			Bun   *driver.BunConfig   `json:"bun" yaml:"bun" mapstructure:"bun"`
			Nats  *driver.NatsConfig  `json:"nats" yaml:"nats" mapstructure:"nats"`
			Redis *driver.RedisConfig `json:"redis" yaml:"redis" mapstructure:"redis"`
		} `json:"drivers" yaml:"drivers" mapstructure:"drivers"`
		Metrics struct {
			Exporter struct {
				Addr string `json:"addr" yaml:"addr" mapstructure:"addr"`
				Path string `json:"path" yaml:"path" mapstructure:"path"`
			} `json:"exporter" yaml:"exporter" mapstructure:"exporter"`
		} `json:"metrics" yaml:"metrics" mapstructure:"metrics"`
		LogLevel string `json:"log_level" yaml:"log_level" mapstructure:"log_level"`
	} `json:"sicky" yaml:"sicky" mapstructure:"sicky"`
	App interface{} `json:"app" yaml:"app" mapstructure:"app"`
}

func (cg *ConfigGlobal) HTTPServer(name string) *shttp.Config {
	cfg := cg.Sicky.Servers.HTTP[name]
	if cfg == nil {
		cfg = shttp.DefaultConfig(name)
	}

	return cfg
}

func (cg *ConfigGlobal) GRPCServer(name string) *sgrpc.Config {
	cfg := cg.Sicky.Servers.GRPC[name]
	if cfg == nil {
		cfg = sgrpc.DefaultConfig(name)
	}

	return cfg
}

func (cg *ConfigGlobal) WebsocketServer(name string) *swebsocket.Config {
	cfg := cg.Sicky.Servers.Websocket[name]
	if cfg == nil {
		cfg = swebsocket.DefaultConfig(name)
	}

	return cfg
}

func (cg *ConfigGlobal) GRPCClient(name string) *cgrpc.Config {
	cfg := cg.Sicky.Clients.GRPC[name]
	if cfg == nil {
		cfg = cgrpc.DefaultConfig(name)
	}

	return cfg
}

// var defaultConfig = map[string]interface{}{}
func DefaultConfig(name, version string) *ConfigGlobal {
	cfg := new(ConfigGlobal)
	cfg.Sicky.Service.Name = name
	cfg.Sicky.Service.Version = version
	cfg.Sicky.LogLevel = DefaultLogLevel
	cfg.Sicky.Metrics.Exporter.Addr = DefaultMetricsExporterAddr
	cfg.Sicky.Metrics.Exporter.Path = DefaultMetricsExporterPath

	return cfg
}

func LoadConfig(name string) (*ConfigGlobal, error) {
	cfg := viper.New()
	cfg.SetConfigName(name)
	cfg.SetConfigType("json")
	cfg.AddConfigPath("/etc/" + name)
	cfg.AddConfigPath("$HOME/." + name)
	cfg.AddConfigPath(".")
	err := cfg.ReadInConfig()
	if err != nil {
		return nil, err
	}

	cfg.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	cfg.AutomaticEnv()

	g := DefaultConfig(DefaultServiceName, DefaultServiceVersion)
	err = cfg.Unmarshal(g)

	return g, err
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
