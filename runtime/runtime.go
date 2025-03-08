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
	"sync/atomic"
	"time"

	"github.com/go-sicky/sicky/driver"
	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/registry"
	"github.com/go-sicky/sicky/tracer/grpc"
	"github.com/go-sicky/sicky/tracer/http"
	"github.com/go-sicky/sicky/tracer/stdout"
	"github.com/spf13/pflag"
)

type FlagSwitchCallback func() error

type FlagSwitch struct {
	Flag     string
	On       bool
	Usage    string
	Callback FlagSwitchCallback
}

type TickerHander func(time.Time, uint64) error

var (
	configLoc           = "config"
	configType          = "json"
	metricsExporterAddr = ":9870"
	metricsExporterPath = "/metrics"

	switchesVars = make(map[string]*FlagSwitch)

	AppName = "sicky"
	silence = false

	BaseTicker         *time.Ticker
	BaseTickerCounter  atomic.Uint64
	BaseTickerHandlers = make([]TickerHander, 0)

	RuntimeDone = make(chan struct{})
)

func Init(name string, switches ...*FlagSwitch) {
	pflag.StringVarP(&configLoc, "config", "C", configLoc, "Config definition, local filename or remote K/V store with format : REMOTE://ADDR/PATH (For example: consul://localhost:8500/app/config).")
	pflag.StringVar(&configType, "config-type", configType, "Configuration data format.")
	pflag.StringVar(&metricsExporterAddr, "metrics-addr", metricsExporterAddr, "Address of prometheus exporter.")
	pflag.StringVar(&metricsExporterPath, "metrics-path", metricsExporterPath, "Path of prometheus exporter.")
	if len(switches) > 0 {
		for _, sw := range switches {
			//sw.On = false
			switchesVars[sw.Flag] = sw
			pflag.BoolVar(&sw.On, sw.Flag, sw.On, sw.Usage)
		}
	}

	pflag.Parse()

	if name != "" {
		AppName = name
	}
}

func Silence() {
	silence = true
}

func Start(cfg *Config) {
	cfg = cfg.Ensure()

	// Logger level
	if silence {
		logger.Logger.Level(logger.SilenceLevel)
	} else {
		lvl := logger.LogLevel(cfg.LogLevel)
		logger.Logger.Level(lvl)
	}

	// Metrics
	if cfg.Metrics != nil {
		// <TODO>
	}

	// Driver
	if cfg.Driver.DB != nil {
		_, err := driver.InitDB(cfg.Driver.DB)
		if err != nil {
			logger.Logger.Fatal(
				"Initialize database failed",
				"error", err.Error(),
			)
		}
	}

	if cfg.Driver.Redis != nil {
		_, err := driver.InitRedis(cfg.Driver.Redis)
		if err != nil {
			logger.Logger.Fatal(
				"Initialize redis failed",
				"error", err.Error(),
			)
		}
	}

	if cfg.Driver.Nats != nil {
		_, err := driver.InitNats(cfg.Driver.Nats)
		if err != nil {
			logger.Logger.Fatal(
				"Initialize nats failed",
				"error", err.Error(),
			)
		}
	}

	if cfg.Driver.Cache != nil {
		_, err := driver.InitCache(cfg.Driver.Cache)
		if err != nil {
			logger.Logger.Fatal(
				"Initialize cache failed",
				"error", err.Error(),
			)
		}
	}

	if cfg.Driver.KV != nil {
		_, err := driver.InitKV(cfg.Driver.KV)
		if err != nil {
			logger.Logger.Fatal(
				"Initialize kv failed",
				"error", err.Error(),
			)
		}
	}

	// Tracer
	switch strings.ToLower(cfg.Tracer.Type) {
	case "grpc":
		grpc.New(nil, &grpc.Config{
			Endpoint:   cfg.Tracer.Endpoint,
			Compress:   cfg.Tracer.Compress,
			Timeout:    cfg.Tracer.Timeout,
			SampleRate: cfg.Tracer.SampleRate,
		})
	case "http":
		http.New(nil, &http.Config{
			Endpoint:   cfg.Tracer.Endpoint,
			SampleRate: cfg.Tracer.SampleRate,
		})
	case "stdout":
		stdout.New(nil, &stdout.Config{
			PrettyPrint: cfg.Tracer.PrettyPrint,
			Timestamps:  cfg.Tracer.Timestamps,
			SampleRate:  cfg.Tracer.SampleRate,
		})
	}

	// Command flags
	for flag, sw := range switchesVars {
		if sw.Flag == flag && sw.On && sw.Callback != nil {
			err := sw.Callback()
			if err != nil {
				logger.Logger.Fatal(
					"Call flag command failed",
					"flag", flag,
					"error", err.Error(),
				)
			}
		}
	}

	// Ticker
	BaseTicker = time.NewTicker(1 * time.Second)
	go func() {
		for {
			select {
			case <-RuntimeDone:
				BaseTicker.Stop()
				if driver.KV != nil {
					driver.KV.Close()
				}

				if driver.Cache != nil {
					driver.Cache.Close()
				}

				if driver.Nats != nil {
					driver.Nats.Close()
				}

				if driver.Redis != nil {
					driver.Redis.Close()
				}

				if driver.DB != nil {
					driver.DB.Close()
				}

				return
			case t := <-BaseTicker.C:
				for _, hdl := range BaseTickerHandlers {
					err := hdl(t, BaseTickerCounter.Load())
					if err != nil {
						logger.Logger.Error(
							"Ticker handler failed",
							"error", err.Error(),
						)
					}
				}

				// Increase counter
				BaseTickerCounter.Add(1)
			}
		}
	}()

	// Registry purger
	if cfg.RegistryPoolPurgeInterval > 0 {
		HandleTicker(func(t time.Time, c uint64) error {
			if c&uint64(cfg.RegistryPoolPurgeInterval) == 0 {
				registry.PurgeInstances()
			}

			return nil
		})
	}
}

func HandleTicker(handler TickerHander) {
	BaseTickerHandlers = append(BaseTickerHandlers, handler)
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
