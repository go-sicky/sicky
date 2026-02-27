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
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	brkJetstream "github.com/go-sicky/sicky/broker/jetstream"
	brkNats "github.com/go-sicky/sicky/broker/nats"
	brkNsq "github.com/go-sicky/sicky/broker/nsq"
	"github.com/go-sicky/sicky/infra"
	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/registry"
	rgConsul "github.com/go-sicky/sicky/registry/consul"
	rgRedis "github.com/go-sicky/sicky/registry/redis"
	"github.com/go-sicky/sicky/service"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type (
	FlagSwitchCallback func() error
	FlagSwitch         struct {
		Flag     string
		On       bool
		Usage    string
		Callback FlagSwitchCallback
	}
)

type (
	TickerHander func(time.Time, uint64) error
	SickyWrapper func(context.Context) error
)

var (
	options *Options

	configLoc  = "config"
	configType = "json"
	configIns  = viper.New()
	verSw      = false

	switchesVars = make(map[string]*FlagSwitch)
	MustInfra    = make(map[string]bool)
	MustBroker   = false
	MustRegistry = false

	beforeStartWrappers []SickyWrapper
	afterStartWrappers  []SickyWrapper
	beforeStopWrappers  []SickyWrapper
	afterStopWrappers   []SickyWrapper
)

func Init(opts *Options, switches ...*FlagSwitch) {
	pflag.StringVarP(&configLoc, "config", "C", configLoc, "Config definition, local filename or remote K/V store with format : REMOTE://ADDR/PATH (For example: consul://localhost:8500/app/config).")
	pflag.StringVar(&configType, "config-type", configType, "Configuration data format.")
	pflag.BoolVarP(&verSw, "version", "V", false, "Show version.")
	if len(switches) > 0 {
		for _, sw := range switches {
			// sw.On = false
			switchesVars[sw.Flag] = sw
			pflag.BoolVar(&sw.On, sw.Flag, sw.On, sw.Usage)
		}
	}

	pflag.Parse()
	options = opts.Ensure()
	if options.Silence {
		logger.Logger.Level(logger.SilenceLevel)
	}

	// fmt.Println(verSw)
	if verSw {
		fmt.Println("  " + options.AppName + " -- Version : " + options.Version + " (" + options.Branch + ") Build : " + options.Commit + " (" + options.BuildTime + ")")

		os.Exit(0)
	}

	// Load config
	configIns.SetConfigType(configType)

	// Try config source
	u, err := url.Parse(configLoc)
	if err == nil && u != nil && u.Scheme != "" && u.Path != "" {
		// Remote config source
		remote := strings.ToLower(u.Scheme)
		err = configIns.AddRemoteProvider(remote, u.Host, u.Path)
		if err != nil {
			logger.Logger.Fatal("Add remote config source failed", "error", err.Error())
		}

		err = configIns.ReadRemoteConfig()
	} else {
		// Local file
		configIns.SetConfigName(configLoc)
		configIns.AddConfigPath("/etc")
		configIns.AddConfigPath("/etc/" + options.AppName)
		configIns.AddConfigPath("$HOME/." + options.AppName)
		configIns.AddConfigPath(".")

		err = configIns.ReadInConfig()
	}

	if err != nil {
		logger.Logger.Fatal("Read config failed", "error", err.Error())
	}

	logger.Logger.Info("Config read", "location", configLoc)

	// Read config from environment variables
	configIns.SetEnvPrefix(strings.ToUpper(options.EnvPrefix))
	configIns.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	configIns.AutomaticEnv()

	// MustInfra
	for _, infra := range options.MustInfra {
		infra = strings.TrimSpace(infra)
		switch infra {
		case "badger", "bun", "clickhouse", "elastic", "mqtt", "mongo", "nats", "redis", "ristretto", "s3":
			MustInfra[infra] = true
		}
	}

	// MustBroker
	MustBroker = options.MustBroker

	// MustRegistry
	MustRegistry = options.MustRegistry
}

func Viper() *viper.Viper {
	return configIns
}

func ConfigUnmarshal(raw any) {
	if raw != nil {
		configIns.Unmarshal(raw)
	}
}

func serviceToRegistryInstance(svc service.Service) *registry.Instance {
	ins := &registry.Instance{
		ID:          svc.Options().ID,
		ServiceMame: svc.Options().Name,
		Type:        svc.String(),
		Servers:     make(map[string]*registry.Server),
		Topics:      make(map[string]*registry.Topic),
	}

	if manager != nil {
		ins.ManagerAddress = manager.Addr()
		ins.ManagerPort = manager.Port()
	}

	// Servers
	for _, srv := range svc.Servers() {
		ins.Servers[srv.Name()] = &registry.Server{
			ID:               srv.ID(),
			InstanceID:       ins.ID,
			Type:             srv.String(),
			Name:             srv.Name(),
			AdvertiseAddress: srv.Addr().String(),
			Port:             srv.Port(),
		}
	}

	return ins
}

func Run(cfg *Config) error {
	var (
		err  error
		errs []error
	)

	if options == nil {
		options = &Options{}
		options = options.Ensure()
	}

	if options.Context == nil {
		options.Context = context.Background()
	}

	cfg = cfg.Ensure()

	// Wrappers
	for _, fn := range beforeStartWrappers {
		err = fn(options.Context)
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

		if MustInfra["badger"] {
			MustInfra["badger"] = false
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

		if MustInfra["bun"] {
			MustInfra["bun"] = false
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

		if MustInfra["clickhouse"] {
			MustInfra["clickhouse"] = false
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

		if MustInfra["elastic"] {
			MustInfra["elastic"] = false
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

		if MustInfra["mqtt"] {
			MustInfra["mqtt"] = false
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

		if MustInfra["mongo"] {
			MustInfra["mongo"] = false
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

		if MustInfra["nats"] {
			MustInfra["nats"] = false
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

		if MustInfra["redis"] {
			MustInfra["redis"] = false
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

		if MustInfra["ristretto"] {
			MustInfra["ristretto"] = false
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

		if MustInfra["s3"] {
			MustInfra["s3"] = false
		}
	}

	// Check infra
	for infra, must := range MustInfra {
		if must {
			logger.Logger.Fatal(
				"Must infrastructure is not initialized",
				"infra", infra,
			)
		}
	}

	// Tracer

	// Registries
	var (
		rgConsulIns *rgConsul.Consul
		rgRedisIns  *rgRedis.Redis
	)
	if cfg.Registry.Consul != nil {
		rgConsulIns = rgConsul.New(nil, cfg.Registry.Consul)
		rgConsulIns.Watch()
		MustRegistry = false
	}

	if cfg.Registry.Redis != nil {
		rgRedisIns = rgRedis.New(nil, cfg.Registry.Redis)
		MustRegistry = false
	}

	if MustRegistry {
		logger.Logger.Fatal(
			"Registry is not initialized",
		)
	}

	// Brokers
	var (
		brkNatsIns      *brkNats.Nats
		brkNsqIns       *brkNsq.Nsq
		brkJetstreamIns *brkJetstream.Jetstream
	)
	if cfg.Broker.Nats != nil {
		brkNatsIns = brkNats.New(nil, cfg.Broker.Nats)
		err = brkNatsIns.Connect()
		if err != nil {
			logger.Logger.Fatal(
				"Nats broker connect failed",
				"error", err.Error(),
			)
		}

		MustBroker = false
	}

	if cfg.Broker.Nsq != nil {
		brkNsqIns = brkNsq.New(nil, cfg.Broker.Nsq)
		err = brkNsqIns.Connect()
		if err != nil {
			logger.Logger.Fatal(
				"Nsq broker connect failed",
				"error", err.Error(),
			)
		}

		MustBroker = false
	}

	if cfg.Broker.Jetstream != nil {
		brkJetstreamIns = brkJetstream.New(nil, cfg.Broker.Jetstream)
		err = brkJetstreamIns.Connect()
		if err != nil {
			logger.Logger.Fatal(
				"Jetstream broker connect failed",
				"error", err.Error(),
			)
		}

		MustBroker = false
	}

	if MustBroker {
		logger.Logger.Fatal(
			"Broker is not initialized",
		)
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

	// Start manager
	if cfg.Manager != nil && cfg.Manager.Enable {
		manager = NewManager(cfg.Manager)
		err = manager.Start()
		if err != nil {
			logger.ErrorContext(
				options.Context,
				"Manager start failed",
				"error", err.Error(),
			)
		}
	}

	// Services
	for id, svc := range service.Services() {
		logger.InfoContext(
			options.Context,
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
				options.Context,
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
			options.Context,
			"Service started",
			"service", svc.String(),
			"id", id,
			"name", svc.Options().Name,
			"version", svc.Options().Version,
			"branch", svc.Options().Branch,
		)

		// Registry instance
		ins := serviceToRegistryInstance(svc)
		err = registry.Register(ins)
		if err != nil {
			logger.ErrorContext(
				options.Context,
				"Registry instance failed",
				"service", svc.String(),
				"id", id,
				"name", svc.Options().Name,
				"version", svc.Options().Version,
				"branch", svc.Options().Branch,
				"registry", registry.Default().String(),
				"error", err.Error(),
			)
		}
	}

	// Wrappers
	for _, fn := range afterStartWrappers {
		err = fn(options.Context)
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
	case <-options.Context.Done():
	}

	// Wrappers
	for _, fn := range beforeStopWrappers {
		err = fn(options.Context)
		if err != nil {
			logger.Logger.Fatal(
				"Before stop wrapper failed",
				"error", err.Error(),
			)
		}
	}

	for id, svc := range service.Services() {
		// Deregistry instance
		err = registry.Deregister(id)
		if err != nil {
			logger.ErrorContext(
				options.Context,
				"Deregistry instance failed",
				"service", svc.String(),
				"id", id,
				"name", svc.Options().Name,
				"version", svc.Options().Version,
				"branch", svc.Options().Branch,
				"registry", registry.Default().String(),
				"error", err.Error(),
			)
		}

		logger.InfoContext(
			options.Context,
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
				options.Context,
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
			options.Context,
			"Service stopped",
			"service", svc.String(),
			"id", id,
			"name", svc.Options().Name,
			"version", svc.Options().Version,
			"branch", svc.Options().Branch,
		)
	}

	// Brokers
	if brkNatsIns != nil {
		brkNatsIns.Disconnect()
	}

	if brkNsqIns != nil {
		brkNsqIns.Disconnect()
	}

	if brkJetstreamIns != nil {
		brkJetstreamIns.Disconnect()
	}

	// Registries
	if rgConsulIns != nil {
		// Do noting
	}

	if rgRedisIns != nil {
		// Do nothing
	}

	// Tracer

	// Stop manager
	if manager != nil {
		manager.Stop()
	}

	if infra.Ristretto != nil {
		infra.Ristretto.Close()
	}

	if infra.Badger != nil {
		infra.Badger.Close()
	}

	if infra.Elastic != nil {
		infra.Elastic.Close(options.Context)
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
		err = fn(options.Context)
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
