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
 * @file sicky.go
 * @package sicky
 * @author Dr.NP <np@herewe.tech>
 * @since 12/20/2025
 */

package sicky

import (
	"context"
	"errors"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/go-sicky/sicky/infra"
	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/service"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type FlagSwitchCallback func() error

type FlagSwitch struct {
	Flag     string
	On       bool
	Usage    string
	Callback FlagSwitchCallback
}

type TickerHander func(time.Time, uint64) error

type SickyWrapper func() error

var (
	appName = "sicky"

	configLoc  = "config"
	configType = "json"
	silence    = false

	switchesVars = make(map[string]*FlagSwitch)

	beforeStartWrappers []SickyWrapper
	afterStartWrappers  []SickyWrapper
	beforeStopWrappers  []SickyWrapper
	afterStopWrappers   []SickyWrapper

	BaseTicker         *time.Ticker
	BaseTickerCounter  atomic.Uint64
	BaseTickerHandlers = make([]TickerHander, 0)
)

func HandleTicker(handler TickerHander) {
	BaseTickerHandlers = append(BaseTickerHandlers, handler)
}

func Init(name string, raw any, switches ...*FlagSwitch) {
	pflag.StringVarP(&configLoc, "config", "C", configLoc, "Config definition, local filename or remote K/V store with format : REMOTE://ADDR/PATH (For example: consul://localhost:8500/app/config).")
	pflag.StringVar(&configType, "config-type", configType, "Configuration data format.")
	if len(switches) > 0 {
		for _, sw := range switches {
			// sw.On = false
			switchesVars[sw.Flag] = sw
			pflag.BoolVar(&sw.On, sw.Flag, sw.On, sw.Usage)
		}
	}

	appName = name

	pflag.Parse()

	// Load config
	cfg := viper.New()
	cfg.SetConfigType(configType)

	// Try config source
	u, err := url.Parse(configLoc)
	if err == nil && u != nil && u.Scheme != "" && u.Path != "" {
		// Remote config source
		remote := strings.ToLower(u.Scheme)
		err = cfg.AddRemoteProvider(remote, u.Host, u.Path)
		if err != nil {
			logger.Logger.Fatal("Add remote config source failed", "error", err.Error())
		}

		err = cfg.ReadRemoteConfig()
	} else {
		// Local file
		cfg.SetConfigName(configLoc)
		cfg.AddConfigPath("/etc")
		cfg.AddConfigPath("/etc/" + appName)
		cfg.AddConfigPath("$HOME/." + appName)
		cfg.AddConfigPath(".")

		err = cfg.ReadInConfig()
	}

	if err != nil {
		logger.Logger.Fatal("Read config failed", "error", err.Error())
	}

	logger.Logger.Info("Config read", "location", configLoc)

	// Read config from environment variables
	cfg.SetEnvPrefix(strings.ToUpper(appName))
	cfg.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	cfg.AutomaticEnv()

	// Marshal
	if raw != nil {
		cfg.Unmarshal(raw)
	}
}

func Silence() {
	silence = true
	logger.Logger.Level(logger.SilenceLevel)
}

func Run(ctx context.Context, cfg *Config) error {
	var (
		err     error
		errs    []error
		manager *Manager
	)

	if ctx == nil {
		ctx = context.Background()
	}

	cfg = cfg.Ensure()

	// Wrappers
	for _, fn := range beforeStartWrappers {
		err = fn()
		if err != nil {
			logger.Logger.Fatal(
				"Before start wrapper failed",
				"error", err.Error(),
			)
		}
	}

	// Infra
	if cfg.Infra == nil {
		cfg.Infra = &InfraConfig{}
	}

	if cfg.Infra.Badger != nil {
		_, err = infra.InitBadger(cfg.Infra.Badger)
		if err != nil {
			logger.Logger.Fatal(
				"Initialize badger failed",
				"error", err.Error(),
			)
		}
	}

	if cfg.Infra.Bun != nil {
		_, err = infra.InitBun(cfg.Infra.Bun)
		if err != nil {
			logger.Logger.Fatal(
				"Initialize bun failed",
				"error", err.Error(),
			)
		}
	}

	if cfg.Infra.Clickhouse != nil {
		_, err = infra.InitClickhouse(cfg.Infra.Clickhouse)
		if err != nil {
			logger.Logger.Fatal(
				"Initialize clickhouse failed",
				"error", err.Error(),
			)
		}
	}

	if cfg.Infra.Elastic != nil {
		_, err = infra.InitElastic(cfg.Infra.Elastic)
		if err != nil {
			logger.Logger.Fatal(
				"Initialize elastic failed",
				"error", err.Error(),
			)
		}
	}

	if cfg.Infra.MQTT != nil {
		_, err = infra.InitMQTT(cfg.Infra.MQTT)
		if err != nil {
			logger.Logger.Fatal(
				"Initialize mqtt failed",
				"error", err.Error(),
			)
		}
	}

	if cfg.Infra.Mongo != nil {
		_, err = infra.InitMongo(cfg.Infra.Mongo)
		if err != nil {
			logger.Logger.Fatal(
				"Initialize mongo failed",
				"error", err.Error(),
			)
		}
	}

	if cfg.Infra.Nats != nil {
		_, err = infra.InitNats(cfg.Infra.Nats)
		if err != nil {
			logger.Logger.Fatal(
				"Initialize nats failed",
				"error", err.Error(),
			)
		}
	}

	if cfg.Infra.Redis != nil {
		_, err = infra.InitRedis(cfg.Infra.Redis)
		if err != nil {
			logger.Logger.Fatal(
				"Initialize redis failed",
				"error", err.Error(),
			)
		}
	}

	if cfg.Infra.Ristretto != nil {
		_, err = infra.InitRistretto(cfg.Infra.Ristretto)
		if err != nil {
			logger.Logger.Fatal(
				"Initialize ristretto failed",
				"error", err.Error(),
			)
		}
	}

	if cfg.Infra.S3 != nil {
		_, err = infra.InitS3(cfg.Infra.S3)
		if err != nil {
			logger.Logger.Fatal(
				"Initialize s3 failed",
				"error", err.Error(),
			)
		}
	}

	// Tracer

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
		for t := range BaseTicker.C {
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
	}()

	// Services
	for id, svc := range service.Services() {
		logger.InfoContext(
			ctx,
			"Starting service",
			"service", svc.String(),
			"id", id,
			"name", svc.Options().Name,
			"version", svc.Options().Version,
			"branch", svc.Options().Branch,
		)

		// Start service
		errs = svc.Start()
		if errs != nil {
			err = errors.Join(errs...)
			logger.ErrorContext(
				ctx,
				"Service start failed",
				"service", svc.String(),
				"id", id,
				"name", svc.Options().Name,
				"version", svc.Options().Version,
				"branch", svc.Options().Branch,
				"errors", err.Error(),
			)

			svc.Stop()

			return err
		}

		logger.InfoContext(
			ctx,
			"Service started",
			"service", svc.String(),
			"id", id,
			"name", svc.Options().Name,
			"version", svc.Options().Version,
			"branch", svc.Options().Branch,
		)
	}

	if cfg.Manager.Enable {
		manager = NewManager(cfg.Manager)
		err = manager.Start()
		if err != nil {
			logger.ErrorContext(
				ctx,
				"Manager start failed",
				"error", err.Error(),
			)
		}
	}

	// Wrappers
	for _, fn := range afterStartWrappers {
		err = fn()
		if err != nil {
			logger.Logger.Fatal(
				"After start wrapper failed",
				"error", err.Error(),
			)
		}
	}

	// Wait for signal
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGHUP, syscall.SIGABRT)
	select {
	case <-ch:
	case <-ctx.Done():
	}

	// Wrappers
	for _, fn := range beforeStopWrappers {
		err = fn()
		if err != nil {
			logger.Logger.Fatal(
				"Before stop wrapper failed",
				"error", err.Error(),
			)
		}
	}

	// Stop manager
	if manager != nil {
		manager.Stop()
	}

	for id, svc := range service.Services() {
		logger.InfoContext(
			ctx,
			"Stopping service",
			"service", svc.String(),
			"id", id,
			"name", svc.Options().Name,
			"version", svc.Options().Version,
			"branch", svc.Options().Branch,
		)

		// Stop service
		errs = svc.Stop()
		if errs != nil {
			err = errors.Join(errs...)
			logger.ErrorContext(
				ctx,
				"Service stop failed",
				"service", svc.String(),
				"id", id,
				"name", svc.Options().Name,
				"Version", svc.Options().Version,
				"Branch", svc.Options().Branch,
				"errors", err.Error(),
			)

			return err
		}

		logger.InfoContext(
			ctx,
			"Service stopped",
			"service", svc.String(),
			"id", id,
			"name", svc.Options().Name,
			"version", svc.Options().Version,
			"branch", svc.Options().Branch,
		)
	}

	BaseTicker.Stop()

	if infra.Ristretto != nil {
		infra.Ristretto.Close()
	}

	if infra.Badger != nil {
		infra.Badger.Close()
	}

	if infra.Elastic != nil {
		infra.Elastic.Close(ctx)
	}

	if infra.Nats != nil {
		infra.Nats.Close()
	}

	if infra.Redis != nil {
		infra.Redis.Close()
	}

	if infra.Bun != nil {
		infra.Bun.Close()
	}

	if infra.Clickhouse != nil {
		infra.Clickhouse.Close()
	}

	if infra.Mongo != nil {
		infra.Mongo.Disconnect(context.TODO())
	}

	if infra.MQTT != nil {
		infra.MQTT.Disconnect(0)
	}

	// Wrappers
	for _, fn := range afterStopWrappers {
		err = fn()
		if err != nil {
			logger.Logger.Fatal(
				"After stop wrapper failed",
				"error", err.Error(),
			)
		}
	}

	return nil
}

func BeforeStart(wrappers ...SickyWrapper) []SickyWrapper {
	beforeStartWrappers = append(beforeStartWrappers, wrappers...)

	return beforeStartWrappers
}

func AfterStart(wrappers ...SickyWrapper) []SickyWrapper {
	afterStartWrappers = append(afterStartWrappers, wrappers...)

	return afterStartWrappers
}

func BeforeStop(wrappers ...SickyWrapper) []SickyWrapper {
	beforeStopWrappers = append(beforeStopWrappers, wrappers...)

	return beforeStopWrappers
}

func AfterStop(wrappers ...SickyWrapper) []SickyWrapper {
	afterStopWrappers = append(afterStopWrappers, wrappers...)

	return afterStopWrappers
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
