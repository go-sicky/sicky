/*
 * The MIT License (MIT)
 *
 * Copyright (c) 2024 HereweTech Co.LTD
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
 * @file runtime.go
 * @package runtime
 * @author Dr.NP <np@herewe.tech>
 * @since 11/20/2023
 */

package runtime

import (
	"strings"
	"time"

	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/registry"
	"github.com/go-sicky/sicky/tracer/grpc"
	"github.com/go-sicky/sicky/tracer/http"
	"github.com/go-sicky/sicky/tracer/stdout"
	"github.com/spf13/pflag"
)

var (
	configLoc           = "config"
	configType          = "json"
	metricsExporterAddr = ":9870"
	metricsExporterPath = "/metrics"

	AppName = "sicky"
)

func Init(name string) {
	pflag.StringVarP(&configLoc, "config", "C", configLoc, "Config definition, local filename or remote K/V store with format : REMOTE://ADDR/PATH (For example: consul://localhost:8500/app/config).")
	pflag.StringVar(&configType, "config-type", configType, "Configuration data format.")
	pflag.StringVar(&metricsExporterAddr, "metrics-addr", metricsExporterAddr, "Address of prometheus exporter.")
	pflag.StringVar(&metricsExporterPath, "metrics-path", metricsExporterPath, "Path of prometheus exporter.")
	pflag.Parse()

	if name != "" {
		AppName = name
	}

	// Load config
	//LoadConfig()

	// Start prometheus exporter
	//StartMetrics()
}

func Start(cfg *Config) {
	cfg = cfg.Ensure()

	lvl := logger.LogLevel(cfg.LogLevel)
	logger.Logger.Level(lvl)

	if cfg.RegistryPoolPurgeInterval > 0 {
		// Start pool looper
		go func() {
			ticker := time.NewTicker(time.Second * time.Duration(cfg.RegistryPoolPurgeInterval))
			defer ticker.Stop()

			for range ticker.C {
				registry.PurgeInstances()
			}
		}()
	}

	switch strings.ToLower(cfg.Tracer.Type) {
	case "grpc":
		grpc.New(nil, &grpc.Config{
			Endpoint: cfg.Tracer.Endpoint,
			Compress: cfg.Tracer.Compress,
			Timeout:  cfg.Tracer.Timeout,
		})
	case "http":
		http.New(nil, &http.Config{
			Endpoint: cfg.Tracer.Endpoint,
		})
	case "stdout":
		stdout.New(nil, &stdout.Config{
			PrettyPrint: cfg.Tracer.PrettyPrint,
			Timestamps:  cfg.Tracer.Timestamps,
		})
	}
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
