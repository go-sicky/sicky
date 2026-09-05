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
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	_ "github.com/spf13/viper/remote"

	"github.com/google/uuid"

	brkJetstream "github.com/go-sicky/sicky/broker/jetstream"
	brkNats "github.com/go-sicky/sicky/broker/nats"
	brkNsq "github.com/go-sicky/sicky/broker/nsq"
	"github.com/go-sicky/sicky/infra"
	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/metrics"
	"github.com/go-sicky/sicky/registry"
	rgConsul "github.com/go-sicky/sicky/registry/consul"
	rgLocal "github.com/go-sicky/sicky/registry/local"
	rgRedis "github.com/go-sicky/sicky/registry/redis"
	"github.com/go-sicky/sicky/service"
	"github.com/go-sicky/sicky/tracer"
	tracerGrpc "github.com/go-sicky/sicky/tracer/grpc"
	tracerHTTP "github.com/go-sicky/sicky/tracer/http"
	tracerStdout "github.com/go-sicky/sicky/tracer/stdout"
	tracerUptrace "github.com/go-sicky/sicky/tracer/uptrace"
	"github.com/go-sicky/sicky/utils"
)

type (
	// FlagSwitchCallback is a sicky component.
	FlagSwitchCallback func() error
	// FlagSwitch is a sicky component.
	FlagSwitch struct {
		Flag     string
		On       bool
		Usage    string
		Callback FlagSwitchCallback
	}
)

type (
	// TickerHandler is a sicky component.
	TickerHandler func(time.Time, uint64) error
	// Deprecated: misspelled, use TickerHandler.
	// TickerHander is a sicky component.
	TickerHander = TickerHandler
	// SickyWrapper is a sicky component.
	SickyWrapper func(context.Context) error
)

var (
	options *Options

	configLoc  = "config"
	configType = "json"
	configIns  = viper.New()
	verSw      = false

	// ErrVersionShown signals that --version was requested. Init handled
	// it by printing the version; the caller should exit 0. The library
	// itself never calls os.Exit.
	// ErrVersionShown is a shared sicky value.
	ErrVersionShown = errors.New("version shown")

	// ErrAlreadyInitialized is returned when Init is called twice.
	// Flag registration on the global pflag set is not idempotent.
	// ErrAlreadyInitialized is a shared sicky value.
	ErrAlreadyInitialized = errors.New("sicky already initialized")

	// ErrMustInfraNotInitialized signals a required infrastructure
	// dependency never became ready. Join with the infra name
	// (fmt.Errorf("%w: %s", ErrMustInfraNotInitialized, name)) so
	// callers can match with errors.Is.
	ErrMustInfraNotInitialized = errors.New("sicky: must infrastructure is not initialized")

	// ErrRegistryNotInitialized signals no registry backend was configured.
	ErrRegistryNotInitialized = errors.New("sicky: registry is not initialized")

	// ErrBrokerNotInitialized signals no broker backend was configured.
	ErrBrokerNotInitialized = errors.New("sicky: broker is not initialized")

	// ErrNATSBrokerNil signals the NATS broker constructor returned nil.
	ErrNATSBrokerNil = errors.New("sicky: nats broker init returned nil")

	// Deprecated: use ErrNATSBrokerNil.
	ErrNatsBrokerNil = ErrNATSBrokerNil

	// ErrNSQBrokerNil signals the NSQ broker constructor returned nil.
	ErrNSQBrokerNil = errors.New("sicky: nsq broker init returned nil")

	// Deprecated: use ErrNSQBrokerNil.
	ErrNsqBrokerNil = ErrNSQBrokerNil

	// ErrJetStreamBrokerNil signals the JetStream broker constructor returned nil.
	ErrJetStreamBrokerNil = errors.New("sicky: jetstream broker init returned nil")

	// Deprecated: use ErrJetStreamBrokerNil.
	ErrJetstreamBrokerNil = ErrJetStreamBrokerNil

	initMu      sync.Mutex
	initialized bool
	runMu       sync.Mutex
	wrapperMu   sync.RWMutex

	switchesVars = make(map[string]*FlagSwitch)
	// MustInfra is a shared sicky value.
	MustInfra = make(map[string]bool)
	// MustBroker is a shared sicky value.
	MustBroker = false
	// MustRegistry is a shared sicky value.
	MustRegistry = false

	beforeStartWrappers []SickyWrapper
	afterStartWrappers  []SickyWrapper
	beforeStopWrappers  []SickyWrapper
	afterStopWrappers   []SickyWrapper
	reloadWrappers      []SickyWrapper
)

// Init is part of the public API.
func Init(opts *Options, switches ...*FlagSwitch) error {
	initMu.Lock()
	defer initMu.Unlock()

	if initialized {
		return ErrAlreadyInitialized
	}

	pflag.StringVarP(&configLoc, "config", "C", configLoc, "Config definition, local filename or remote K/V store with format : REMOTE://ADDR/PATH (For example: consul://localhost:8500/app/config).")
	pflag.StringVar(&configType, "config-type", configType, "Configuration data format.")
	pflag.BoolVarP(&verSw, "version", "V", false, "Show version.")
	if len(switches) > 0 {
		for _, sw := range switches {
			if sw == nil || sw.Flag == "" {
				continue
			}

			if _, exists := switchesVars[sw.Flag]; exists {
				continue
			}

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

	if verSw {
		fmt.Println("  " + options.AppName + " -- Version : " + options.Version + " (" + options.Branch + ") Build : " + options.Commit + " (" + options.BuildTime + ")")

		return ErrVersionShown
	}

	if !options.DisableConfig {
		// Load config
		configIns.SetConfigType(configType)

		// Try config source
		u, err := url.Parse(configLoc)
		if err == nil && u != nil && u.Scheme != "" && u.Path != "" {
			// Remote config source
			remote := strings.ToLower(u.Scheme)
			err = configIns.AddRemoteProvider(remote, u.Host, u.Path)
			if err != nil {
				return fmt.Errorf("add remote config source: %w", err)
			}

			err = configIns.ReadRemoteConfig()
		} else {
			// Local file
			configIns.SetConfigName(configLoc)
			configIns.AddConfigPath("/etc")
			configIns.AddConfigPath("/etc/" + options.AppName)
			if home, herr := os.UserHomeDir(); herr == nil && home != "" {
				configIns.AddConfigPath(home + "/." + options.AppName)
			}

			configIns.AddConfigPath(".")

			err = configIns.ReadInConfig()
		}

		if err != nil {
			return fmt.Errorf("read config: %w", err)
		}

		// Never log embedded credentials (e.g. consul://user:pass@host/path).
		location := configLoc
		if u, uerr := url.Parse(configLoc); uerr == nil && u != nil {
			location = u.Redacted()
		}

		logger.Logger.Info("Config read", "location", location)
		if used := configIns.ConfigFileUsed(); used != "" {
			if cwd, cerr := os.Getwd(); cerr == nil {
				if sameDir, serr := filepath.Abs(filepath.Dir(used)); serr == nil {
					if cwdAbs, cerr2 := filepath.Abs(cwd); cerr2 == nil && sameDir == cwdAbs {
						logger.Logger.Warn("Config loaded from working directory; ensure the CWD is trusted", "file", used)
					}
				}
			}
		}
	}

	// Read config from environment variables
	configIns.SetEnvPrefix(strings.ToUpper(options.EnvPrefix))
	configIns.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	configIns.AutomaticEnv()

	// MustInfra
	for _, infra := range options.MustInfra {
		key := strings.ToLower(strings.TrimSpace(infra))
		switch key {
		case componentBadger, componentBun, componentClickhouse, componentElastic, componentMQTT, componentMongo, componentNATS, componentRedis, componentRistretto, componentS3:
			MustInfra[key] = true
		default:
			if key != "" {
				logger.Logger.Warn("Unknown MustInfra entry ignored", "infra", infra)
			}
		}
	}

	// MustBroker
	MustBroker = options.MustBroker

	// MustRegistry
	MustRegistry = options.MustRegistry

	initialized = true

	return nil
}

// Viper is part of the public API.
func Viper() *viper.Viper {
	return configIns
}

// ConfigUnmarshal is part of the public API.
// ErrConfigNilTarget is returned when ConfigUnmarshal is asked to
// unmarshal into a nil target, so callers can distinguish "nothing
// provided" from "unmarshaled".
var ErrConfigNilTarget = errors.New("config unmarshal target is nil")

func ConfigUnmarshal(raw any) error {
	if raw == nil {
		return ErrConfigNilTarget
	}

	return configIns.Unmarshal(raw)
}

func registryName() string {
	rg := registry.Default()
	if rg == nil {
		return DefaultTracerType
	}

	return rg.String()
}

func serviceToRegistryInstance(svc service.Service) *registry.Instance {
	ins := &registry.Instance{
		ID:          svc.Options().ID,
		ServiceName: svc.Options().Name,
		Type:        svc.String(),
		Servers:     make(map[string]*registry.Server),
		Topics:      make(map[string]*registry.Topic),
		Metadata:    utils.NewMetadata(),
	}

	if managerApp != nil {
		ins.ManagerAddress = managerApp.Addr()
		ins.ManagerPort = managerApp.Port()
	}

	// Servers (skip servers with unresolvable advertise address).
	for _, srv := range svc.Servers() {
		addr := srv.Addr()
		if addr == nil {
			logger.Logger.WarnContext(
				options.Context,
				"Skip server with nil address in registry instance",
				"server", srv.Name(),
			)

			continue
		}

		ins.Servers[srv.Name()] = &registry.Server{
			ID:               srv.ID(),
			InstanceID:       ins.ID,
			Type:             srv.String(),
			Name:             srv.Name(),
			AdvertiseAddress: addr.String(),
			Port:             srv.Port(),
		}
	}

	if svc.Options().Metadata != nil {
		ins.Metadata = svc.Options().Metadata.Clone()
		if ins.Metadata == nil {
			ins.Metadata = utils.NewMetadata()
		}
	}

	ins.Metadata.Set("AppName", options.AppName)
	ins.Metadata.Set("version", options.Version)
	ins.Metadata.Set("Commit", options.Commit)
	ins.Metadata.Set("BuildTime", options.BuildTime)
	ins.Metadata.Set("Branch", options.Branch)

	return ins
}

// Run runs the component.
func Run(cfg *Config) error {
	runMu.Lock()
	defer runMu.Unlock()

	var (
		err    error
		errs   []error
		runErr error

		rgConsulIns  *rgConsul.Consul
		rgRedisIns   *rgRedis.Redis
		rgLocalIns   *rgLocal.Local
		rgTicker     *time.Ticker
		rgTickerDone chan struct{}
		rgTickerWg   sync.WaitGroup

		brkNatsIns      *brkNats.Nats
		brkNsqIns       *brkNsq.NSQ
		brkJetstreamIns *brkJetstream.JetStream

		// failedSvcs tracks services whose Start failed so the shutdown
		// path does not Stop them twice (already stopped inline).
		failedSvcs = make(map[service.Service]struct{})
		// startedIDs tracks services whose Start succeeded: the shutdown
		// path only Stops/Deregisters those, never never-started ones.
		startedIDs = make(map[uuid.UUID]struct{})

		beforeStart []SickyWrapper
		afterStart  []SickyWrapper
	)

	if options == nil {
		options = options.Ensure()
	}

	parentCtx := options.Context
	if parentCtx == nil {
		parentCtx = context.Background()
	}

	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()
	defer func() {
		// Restore parent so a second Run does not inherit a canceled ctx.
		options.Context = parentCtx
	}()
	options.Context = ctx

	cfg = cfg.Ensure()

	if !options.Silence {
		// Log level
		logger.Logger.Level(logger.LogLevel(cfg.LogLevel))
	}

	// Merge MustInfra from options without wiping entries set by Init().
	// Init() already populates the global map; Run() may be called without
	// Init() (options carry MustInfra list), so merge here instead of reset.
	if MustInfra == nil {
		MustInfra = make(map[string]bool)
	}

	for _, infra := range options.MustInfra {
		key := strings.ToLower(strings.TrimSpace(infra))
		switch key {
		case componentBadger, componentBun, componentClickhouse, componentElastic, componentMQTT, componentMongo, componentNATS, componentRedis, componentRistretto, componentS3:
			MustInfra[key] = true
		default:
			if key != "" {
				logger.Logger.Warn("Unknown MustInfra entry ignored", "infra", infra)
			}
		}
	}

	validateConfig(cfg)

	// Wrappers
	wrapperMu.RLock()
	beforeStart = append([]SickyWrapper(nil), beforeStartWrappers...)
	wrapperMu.RUnlock()
	for _, fn := range beforeStart {
		if fn == nil {
			continue
		}

		err = fn(options.Context)
		if err != nil {
			logger.Logger.ErrorContext(
				options.Context,
				"Before start wrapper failed",
				"error", err.Error(),
			)
			runErr = errors.Join(runErr, fmt.Errorf("before start wrapper: %w", err))
		}
	}

	// Infra
	if cfg.Infra == nil {
		cfg.Infra = &InfraConfig{}
	}

	if cfg.Infra.Badger != nil {
		_, err = infra.InitBadger(cfg.Infra.Badger)
		if err != nil {
			logger.Logger.ErrorContext(
				options.Context,
				"Initialize badger failed",
				"error", err.Error(),
			)
			runErr = errors.Join(runErr, fmt.Errorf("initialize badger: %w", err))
			goto shutdown
		}

		if MustInfra[componentBadger] {
			MustInfra[componentBadger] = false
		}
	}

	if cfg.Infra.Bun != nil {
		_, err = infra.InitBun(cfg.Infra.Bun)
		if err != nil {
			logger.Logger.ErrorContext(
				options.Context,
				"Initialize bun failed",
				"error", err.Error(),
			)
			runErr = errors.Join(runErr, fmt.Errorf("initialize bun: %w", err))
			goto shutdown
		}

		if MustInfra[componentBun] {
			MustInfra[componentBun] = false
		}
	}

	if cfg.Infra.Clickhouse != nil {
		_, err = infra.InitClickHouse(cfg.Infra.Clickhouse)
		if err != nil {
			logger.Logger.ErrorContext(
				options.Context,
				"Initialize clickhouse failed",
				"error", err.Error(),
			)
			runErr = errors.Join(runErr, fmt.Errorf("initialize clickhouse: %w", err))
			goto shutdown
		}

		if MustInfra[componentClickhouse] {
			MustInfra[componentClickhouse] = false
		}
	}

	if cfg.Infra.Elastic != nil {
		_, err = infra.InitElastic(cfg.Infra.Elastic)
		if err != nil {
			logger.Logger.ErrorContext(
				options.Context,
				"Initialize elastic failed",
				"error", err.Error(),
			)
			runErr = errors.Join(runErr, fmt.Errorf("initialize elastic: %w", err))
			goto shutdown
		}

		if MustInfra[componentElastic] {
			MustInfra[componentElastic] = false
		}
	}

	if cfg.Infra.MQTT != nil {
		_, err = infra.InitMQTT(cfg.Infra.MQTT)
		if err != nil {
			logger.Logger.ErrorContext(
				options.Context,
				"Initialize mqtt failed",
				"error", err.Error(),
			)
			runErr = errors.Join(runErr, fmt.Errorf("initialize mqtt: %w", err))
			goto shutdown
		}

		if MustInfra[componentMQTT] {
			MustInfra[componentMQTT] = false
		}
	}

	if cfg.Infra.Mongo != nil {
		_, err = infra.InitMongo(cfg.Infra.Mongo)
		if err != nil {
			logger.Logger.ErrorContext(
				options.Context,
				"Initialize mongo failed",
				"error", err.Error(),
			)
			runErr = errors.Join(runErr, fmt.Errorf("initialize mongo: %w", err))
			goto shutdown
		}

		if MustInfra[componentMongo] {
			MustInfra[componentMongo] = false
		}
	}

	if cfg.Infra.Nats != nil {
		_, err = infra.InitNATS(cfg.Infra.Nats)
		if err != nil {
			logger.Logger.ErrorContext(
				options.Context,
				"Initialize nats failed",
				"error", err.Error(),
			)
			runErr = errors.Join(runErr, fmt.Errorf("initialize nats: %w", err))
			goto shutdown
		}

		if MustInfra[componentNATS] {
			MustInfra[componentNATS] = false
		}
	}

	if cfg.Infra.Redis != nil {
		_, err = infra.InitRedis(cfg.Infra.Redis)
		if err != nil {
			logger.Logger.ErrorContext(
				options.Context,
				"Initialize redis failed",
				"error", err.Error(),
			)
			runErr = errors.Join(runErr, fmt.Errorf("initialize redis: %w", err))
			goto shutdown
		}

		if MustInfra[componentRedis] {
			MustInfra[componentRedis] = false
		}
	}

	if cfg.Infra.Ristretto != nil {
		_, err = infra.InitRistretto(cfg.Infra.Ristretto)
		if err != nil {
			logger.Logger.ErrorContext(
				options.Context,
				"Initialize ristretto failed",
				"error", err.Error(),
			)
			runErr = errors.Join(runErr, fmt.Errorf("initialize ristretto: %w", err))
			goto shutdown
		}

		if MustInfra[componentRistretto] {
			MustInfra[componentRistretto] = false
		}
	}

	if cfg.Infra.S3 != nil {
		_, err = infra.InitS3(cfg.Infra.S3)
		if err != nil {
			logger.Logger.ErrorContext(
				options.Context,
				"Initialize s3 failed",
				"error", err.Error(),
			)
			runErr = errors.Join(runErr, fmt.Errorf("initialize s3: %w", err))
			goto shutdown
		}

		if MustInfra[componentS3] {
			MustInfra[componentS3] = false
		}
	}

	// Check infra
	for name, must := range MustInfra {
		if must {
			logger.Logger.ErrorContext(
				options.Context,
				"Must infrastructure is not initialized",
				"infra", name,
			)
			runErr = errors.Join(runErr, fmt.Errorf("%w: %s", ErrMustInfraNotInitialized, name))
			goto shutdown
		}
	}

	// Tracer (dual-track: standard OTLP grpc/http/stdout vs Uptrace SDK).
	// ServiceName/Version fall back to AppName/Version when unset.
	// Scoped in a block so function-level `goto shutdown` calls above
	// never jump over its local declarations.
	{
		tracerSvc := cfg.Tracer.ServiceName
		if tracerSvc == "" {
			tracerSvc = options.AppName
		}

		tracerVer := cfg.Tracer.ServiceVersion
		if tracerVer == "" {
			tracerVer = options.Version
		}

		if err := cfg.Tracer.Validate(); err != nil {
			logger.Logger.Warn("Invalid tracer config, continuing without tracing", "error", err.Error(), "type", cfg.Tracer.Type)
		} else if cfg.Tracer.Type != DefaultTracerType {
			var tcOk bool
			switch cfg.Tracer.Type {
			case tracerTypeGRPC:
				tc := tracerGrpc.New(nil, &tracerGrpc.Config{
					ServiceName:    tracerSvc,
					ServiceVersion: tracerVer,
					Endpoint:       cfg.Tracer.Endpoint,
					Compress:       cfg.Tracer.Compress,
					Timeout:        cfg.Tracer.Timeout,
					Insecure:       cfg.Tracer.Insecure,
					Headers:        cfg.Tracer.Headers,
					SampleRate:     cfg.Tracer.SampleRate,
				})
				tcOk = tc != nil
			case tracerTypeHTTP:
				tc := tracerHTTP.New(nil, &tracerHTTP.Config{
					ServiceName:    tracerSvc,
					ServiceVersion: tracerVer,
					Endpoint:       cfg.Tracer.Endpoint,
					Compress:       cfg.Tracer.Compress,
					Timeout:        cfg.Tracer.Timeout,
					Insecure:       cfg.Tracer.Insecure,
					Headers:        cfg.Tracer.Headers,
					SampleRate:     cfg.Tracer.SampleRate,
				})
				tcOk = tc != nil
			case tracerTypeStdout:
				tc := tracerStdout.New(nil, &tracerStdout.Config{
					ServiceName:    tracerSvc,
					ServiceVersion: tracerVer,
					PrettyPrint:    cfg.Tracer.PrettyPrint,
					Timestamps:     cfg.Tracer.Timestamps,
					SampleRate:     cfg.Tracer.SampleRate,
				})
				tcOk = tc != nil
			case tracerTypeUptrace:
				tc := tracerUptrace.New(nil, &tracerUptrace.Config{
					DSN:            cfg.Tracer.DSN,
					ServiceName:    tracerSvc,
					ServiceVersion: tracerVer,
				})
				tcOk = tc != nil
			default:
				logger.Logger.Warn("Unknown tracer type", "type", cfg.Tracer.Type)
			}

			if tcOk {
				// sicky owns the global propagator: W3C + B3 dual emit.
				tracer.InstallPropagator()
				logger.Logger.Info("Tracer initialized", "type", cfg.Tracer.Type)
			} else if cfg.Tracer.Type == tracerTypeGRPC || cfg.Tracer.Type == tracerTypeHTTP || cfg.Tracer.Type == tracerTypeStdout || cfg.Tracer.Type == tracerTypeUptrace {
				logger.Logger.Warn("Tracer initialization failed, continuing without tracing", "type", cfg.Tracer.Type)
			}
		}
	}

	// Registries
	if cfg.Registry.Consul != nil {
		rgConsulIns = rgConsul.New(nil, cfg.Registry.Consul)
		if rgConsulIns != nil {
			if werr := rgConsulIns.Watch(); werr != nil {
				logger.Logger.WarnContext(options.Context, "Consul watch failed", "error", werr.Error())
			}

			MustRegistry = false
		} else {
			logger.Logger.WarnContext(options.Context, "Consul registry init returned nil")
		}
	}

	if cfg.Registry.Redis != nil {
		rgRedisIns = rgRedis.New(nil, cfg.Registry.Redis)
		if rgRedisIns != nil {
			MustRegistry = false
		} else {
			logger.Logger.WarnContext(options.Context, "Redis registry init returned nil")
		}
	}

	if cfg.Registry.Local != nil {
		rgLocalIns = rgLocal.New(nil, cfg.Registry.Local)
		if rgLocalIns != nil {
			MustRegistry = false
		} else {
			logger.Logger.WarnContext(options.Context, "Local registry init returned nil")
		}
	}

	if MustRegistry {
		logger.Logger.ErrorContext(
			options.Context,
			"Registry is not initialized",
		)
		runErr = errors.Join(runErr, ErrRegistryNotInitialized)
		goto shutdown
	}

	registry.InitPool()
	if err := registry.Watch(); err != nil {
		runErr = errors.Join(runErr, err)
		goto shutdown
	}

	if cfg.Registry.PoolPurgeInterval > 0 &&
		(rgRedisIns != nil || rgConsulIns != nil || rgLocalIns != nil) {
		rgTicker = time.NewTicker(time.Duration(cfg.Registry.PoolPurgeInterval) * time.Second)
		rgTickerDone = make(chan struct{})
		rgTickerWg.Go(func() {
			for {
				select {
				case <-rgTickerDone:
					return
				case <-rgTicker.C:
					ins, err := registry.Load()
					if err != nil {
						logger.ErrorContext(
							options.Context,
							"Registry pool purge failed",
							"error", err.Error(),
						)
					} else {
						registry.PurgePool(ins)
						logger.InfoContext(
							options.Context,
							"Registry pool purged",
						)
					}
				}
			}
		})
	}

	// Brokers
	if cfg.Broker.Nats != nil {
		brkNatsIns = brkNats.New(nil, cfg.Broker.Nats)
		if brkNatsIns == nil {
			runErr = errors.Join(runErr, ErrNATSBrokerNil)
			goto shutdown
		}

		err = brkNatsIns.Connect()
		if err != nil {
			logger.Logger.ErrorContext(
				options.Context,
				"Nats broker connect failed",
				"error", err.Error(),
			)
			runErr = errors.Join(runErr, fmt.Errorf("nats broker connect: %w", err))
			goto shutdown
		}

		MustBroker = false
	}

	if cfg.Broker.Nsq != nil {
		brkNsqIns = brkNsq.New(nil, cfg.Broker.Nsq)
		if brkNsqIns == nil {
			runErr = errors.Join(runErr, ErrNSQBrokerNil)
			goto shutdown
		}

		err = brkNsqIns.Connect()
		if err != nil {
			logger.Logger.ErrorContext(
				options.Context,
				"Nsq broker connect failed",
				"error", err.Error(),
			)
			runErr = errors.Join(runErr, fmt.Errorf("nsq broker connect: %w", err))
			goto shutdown
		}

		MustBroker = false
	}

	if cfg.Broker.Jetstream != nil {
		brkJetstreamIns = brkJetstream.New(nil, cfg.Broker.Jetstream)
		if brkJetstreamIns == nil {
			runErr = errors.Join(runErr, ErrJetStreamBrokerNil)
			goto shutdown
		}

		err = brkJetstreamIns.Connect()
		if err != nil {
			logger.Logger.ErrorContext(
				options.Context,
				"Jetstream broker connect failed",
				"error", err.Error(),
			)
			runErr = errors.Join(runErr, fmt.Errorf("jetstream broker connect: %w", err))
			goto shutdown
		}

		MustBroker = false
	}

	if MustBroker {
		logger.Logger.ErrorContext(
			options.Context,
			"Broker is not initialized",
		)
		runErr = errors.Join(runErr, ErrBrokerNotInitialized)
		goto shutdown
	}

	// Command flags
	for flag, sw := range switchesVars {
		if sw.Flag == flag && sw.On && sw.Callback != nil {
			err := sw.Callback()
			if err != nil {
				logger.Logger.ErrorContext(
					options.Context,
					"Call flag command failed",
					"flag", flag,
					"error", err.Error(),
				)
				runErr = errors.Join(runErr, fmt.Errorf("flag command %s: %w", flag, err))
				goto shutdown
			}
		}
	}

	// Start manager
	if cfg.Manager != nil && cfg.Manager.Enable && !options.DisableManager {
		managerApp = NewManager(cfg.Manager, options.AppName, options.Version)
		managerApp.cfgVar = cfg
		err = managerApp.Start()
		if err != nil {
			logger.ErrorContext(
				options.Context,
				"Manager start failed",
				"error", err.Error(),
			)
			runErr = errors.Join(runErr, fmt.Errorf("manager start: %w", err))
			goto shutdown
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
		if len(errs) > 0 {
			metrics.ServiceStartsTotal.WithLabelValues(svc.String(), "error").Inc()
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

			if stopErrs := svc.Stop(); len(stopErrs) > 0 {
				err = errors.Join(err, errors.Join(stopErrs...))
			}

			// Mark so the shutdown path does not Stop it again.
			failedSvcs[svc] = struct{}{}
			runErr = errors.Join(runErr, err)

			goto shutdown
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
		metrics.ServiceStartsTotal.WithLabelValues(svc.String(), "ok")

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
				"registry", registryName(),
				"error", err.Error(),
			)

			// A service nobody can discover is worse than a stopped one:
			// deregister best-effort and stop it so startup is not half-broken.
			if derr := registry.Deregister(id); derr != nil {
				runErr = errors.Join(runErr, fmt.Errorf("deregister service %s: %w", id, derr))
			}

			if stopErrs := svc.Stop(); len(stopErrs) > 0 {
				runErr = errors.Join(runErr, errors.Join(stopErrs...))
			}

			failedSvcs[svc] = struct{}{}
			runErr = errors.Join(runErr, fmt.Errorf("registry register %s: %w", id, err))

			continue
		}

		startedIDs[id] = struct{}{}
	}

	// Wrappers
	wrapperMu.RLock()
	afterStart = append([]SickyWrapper(nil), afterStartWrappers...)
	wrapperMu.RUnlock()
	for _, fn := range afterStart {
		if fn == nil {
			continue
		}

		err = fn(options.Context)
		if err != nil {
			logger.Logger.ErrorContext(
				options.Context,
				"After start wrapper failed",
				"error", err.Error(),
			)
			runErr = errors.Join(runErr, fmt.Errorf("after start wrapper: %w", err))
		}
	}

	// Wait for signal
shutdown:
	if runErr == nil {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGABRT)
		defer signal.Stop(ch)

		hupCh := make(chan os.Signal, 1)
		signal.Notify(hupCh, syscall.SIGHUP)
		defer signal.Stop(hupCh)

		reloadDone := make(chan struct{})
		var reloadCloseOnce sync.Once
		reloadDoneClose := func() {
			reloadCloseOnce.Do(func() { close(reloadDone) })
		}

		defer reloadDoneClose()
		var reloadWg sync.WaitGroup
		reloadWg.Go(func() {
			for {
				select {
				case <-reloadDone:
					return
				case <-hupCh:
					wrapperMu.RLock()
					reload := append([]SickyWrapper(nil), reloadWrappers...)
					wrapperMu.RUnlock()
					for _, fn := range reload {
						if fn == nil {
							continue
						}

						if rerr := fn(options.Context); rerr != nil {
							metrics.ConfigReloadsTotal.WithLabelValues("error").Inc()
							logger.Logger.ErrorContext(
								options.Context,
								"Reload wrapper failed",
								"error", rerr.Error(),
							)
						} else {
							metrics.ConfigReloadsTotal.WithLabelValues("ok").Inc()
						}
					}
				}
			}
		})

		select {
		case <-ch:
		case <-options.Context.Done():
		}

		// Join the reload goroutine so it cannot outlive Run() while
		// wrappers still touch options.Context.
		reloadDoneClose()
		reloadWg.Wait()

		forceTimeout := 30 * time.Second
		if cfg.Manager != nil && cfg.Manager.ShutdownTimeout > 0 {
			if d := time.Duration(cfg.Manager.ShutdownTimeout) * time.Second; d > forceTimeout {
				forceTimeout = d
			}
		}

		// No os.Exit in library: second signal only cancels, force timeout
		// only logs. Caller decides process exit from returned error.
		forceDone := make(chan struct{})
		defer close(forceDone)
		go func() {
			select {
			case <-ch:
				logger.Logger.Error("Second signal received, canceling shutdown")
				cancel()
			case <-time.After(forceTimeout):
				logger.Logger.Error("Shutdown timed out, canceling", "timeout", forceTimeout.String())
				cancel()
			case <-forceDone:
			}
		}()
	}

	// Stop the purge ticker and join its goroutine before tearing down
	// registries (Load() must not race registry.Stop()).
	if rgTickerDone != nil {
		close(rgTickerDone)
	}

	if rgTicker != nil {
		rgTicker.Stop()
	}

	rgTickerWg.Wait()

	// Wrappers
	wrapperMu.RLock()
	beforeStop := append([]SickyWrapper(nil), beforeStopWrappers...)
	wrapperMu.RUnlock()
	for _, fn := range beforeStop {
		if fn == nil {
			continue
		}

		err = fn(options.Context)
		if err != nil {
			logger.Logger.ErrorContext(
				options.Context,
				"Before stop wrapper failed",
				"error", err.Error(),
			)
			runErr = errors.Join(runErr, fmt.Errorf("before stop wrapper: %w", err))
		}
	}

	for id, svc := range service.Services() {
		if _, failed := failedSvcs[svc]; failed {
			// Already stopped inline at Start failure; do not stop twice.
			continue
		}

		if _, started := startedIDs[id]; !started {
			// Never started (e.g. shutdown from an infra/broker failure):
			// skip entirely instead of stopping a service that was not up.
			continue
		}

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
				"registry", registryName(),
				"error", err.Error(),
			)
			runErr = errors.Join(runErr, fmt.Errorf("deregister service %s: %w", id, err))
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
		if len(errs) > 0 {
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

			runErr = errors.Join(runErr, err)
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
		if derr := brkNatsIns.Disconnect(); derr != nil {
			logger.Logger.ErrorContext(options.Context, "Nats broker disconnect failed", "error", derr.Error())
			runErr = errors.Join(runErr, fmt.Errorf("nats broker disconnect: %w", derr))
		}
	}

	if brkNsqIns != nil {
		if derr := brkNsqIns.Disconnect(); derr != nil {
			logger.Logger.ErrorContext(options.Context, "Nsq broker disconnect failed", "error", derr.Error())
			runErr = errors.Join(runErr, fmt.Errorf("nsq broker disconnect: %w", derr))
		}
	}

	if brkJetstreamIns != nil {
		if derr := brkJetstreamIns.Disconnect(); derr != nil {
			logger.Logger.ErrorContext(options.Context, "Jetstream broker disconnect failed", "error", derr.Error())
			runErr = errors.Join(runErr, fmt.Errorf("jetstream broker disconnect: %w", derr))
		}
	}

	// Registries
	if rerr := registry.Stop(); rerr != nil {
		logger.Logger.ErrorContext(options.Context, "Registry stop failed", "error", rerr.Error())
		runErr = errors.Join(runErr, fmt.Errorf("registry stop: %w", rerr))
	}

	// Registry instances are stopped via registry.Stop() above;
	// per-implementation handles need no extra teardown.

	// Tracer
	if tracer.Default() != nil {
		if err := tracer.Default().Stop(); err != nil {
			logger.Logger.Error("Tracer stop failed", "error", err.Error())
			runErr = errors.Join(runErr, fmt.Errorf("tracer stop: %w", err))
		}
	}

	// Stop manager
	if managerApp != nil {
		if merr := managerApp.Stop(); merr != nil {
			logger.Logger.ErrorContext(options.Context, "Manager stop failed", "error", merr.Error())
			runErr = errors.Join(runErr, fmt.Errorf("manager stop: %w", merr))
		}
	}

	// Stop infra (errors are collected, never abort the sequence).
	// Singletons are cleared after Close so a later Run() builds fresh
	// connections instead of reusing closed ones.
	if r := infra.GetRistretto(); r != nil {
		r.Close()
		infra.ClearRistretto()
	}

	if b := infra.GetBadger(); b != nil {
		if cerr := b.Close(); cerr != nil {
			logger.Logger.ErrorContext(options.Context, "Badger close failed", "error", cerr.Error())
			runErr = errors.Join(runErr, fmt.Errorf("badger close: %w", cerr))
		}

		infra.ClearBadger()
	}

	if e := infra.GetElastic(); e != nil {
		ecloseCtx, ecloseCancel := context.WithTimeout(context.Background(), 5*time.Second)
		cerr := e.Close(ecloseCtx)
		ecloseCancel()
		if cerr != nil {
			logger.Logger.ErrorContext(options.Context, "Elastic close failed", "error", cerr.Error())
			runErr = errors.Join(runErr, fmt.Errorf("elastic close: %w", cerr))
		}

		infra.ClearElastic()
	}

	if n := infra.GetNATS(); n != nil {
		n.Close()
		infra.ClearNats()
	}

	if rdb := infra.GetRedis(); rdb != nil {
		if cerr := rdb.Close(); cerr != nil {
			logger.Logger.ErrorContext(options.Context, "Redis close failed", "error", cerr.Error())
			runErr = errors.Join(runErr, fmt.Errorf("redis close: %w", cerr))
		}

		infra.ClearRedis()
	}

	if db := infra.GetBun(); db != nil {
		if cerr := db.Close(); cerr != nil {
			logger.Logger.ErrorContext(options.Context, "Bun close failed", "error", cerr.Error())
			runErr = errors.Join(runErr, fmt.Errorf("bun close: %w", cerr))
		}

		infra.ClearBun()
	}

	if chdb := infra.GetClickHouse(); chdb != nil {
		if cerr := chdb.Close(); cerr != nil {
			logger.Logger.ErrorContext(options.Context, "Clickhouse close failed", "error", cerr.Error())
			runErr = errors.Join(runErr, fmt.Errorf("clickhouse close: %w", cerr))
		}

		infra.ClearClickHouse()
	}

	if s3c := infra.GetS3(); s3c != nil {
		logger.Logger.InfoContext(
			options.Context,
			"S3 client shutdown (connection managed by AWS SDK)",
		)
		infra.ClearS3()
	}

	if m := infra.GetMongo(); m != nil {
		mctx, mcancel := context.WithTimeout(context.Background(), 5*time.Second)
		if derr := m.Disconnect(mctx); derr != nil {
			logger.Logger.ErrorContext(options.Context, "Mongo disconnect failed", "error", derr.Error())
			runErr = errors.Join(runErr, fmt.Errorf("mongo disconnect: %w", derr))
		}

		mcancel()
		infra.ClearMongo()
	}

	if mc := infra.GetMQTT(); mc != nil {
		// Give in-flight messages a grace period instead of dropping them.
		mc.Disconnect(250)
		infra.ClearMQTT()
	}

	// Wrappers
	wrapperMu.RLock()
	afterStop := append([]SickyWrapper(nil), afterStopWrappers...)
	wrapperMu.RUnlock()
	for _, fn := range afterStop {
		if fn == nil {
			continue
		}

		err = fn(options.Context)
		if err != nil {
			logger.Logger.ErrorContext(
				options.Context,
				"After stop wrapper failed",
				"error", err.Error(),
			)
			runErr = errors.Join(runErr, fmt.Errorf("after stop wrapper: %w", err))
		}
	}

	return runErr
}

func validateConfig(cfg *Config) {
	if cfg == nil {
		return
	}

	// Clamp invalid values back to defaults (Viper env coercion zeroes
	// int fields silently) and warn. Never aborts startup.
	if cfg.Tracer != nil && cfg.Tracer.Type != DefaultTracerType {
		if cfg.Tracer.Timeout < 0 {
			logger.Logger.Warn("Tracer timeout is invalid (possibly zeroed by environment variable), clamped to 0 (exporter default)",
				"old", cfg.Tracer.Timeout)
			cfg.Tracer.Timeout = 0
		}

		if cfg.Tracer.SampleRate < 0 || cfg.Tracer.SampleRate > 1 {
			logger.Logger.Warn("Tracer sample rate out of range [0,1], clamped to 1.0",
				"old", cfg.Tracer.SampleRate)
			cfg.Tracer.SampleRate = 1.0
		}
	}

	if cfg.Manager != nil && cfg.Manager.Enable {
		if cfg.Manager.ShutdownTimeout <= 0 {
			logger.Logger.Warn("Manager shutdown timeout is invalid (possibly zeroed by environment variable), clamped to default",
				"old", cfg.Manager.ShutdownTimeout, "new", DefaultShutdownTimeout)
			cfg.Manager.ShutdownTimeout = DefaultShutdownTimeout
		}

		if cfg.Manager.ReadTimeout <= 0 {
			logger.Logger.Warn("Manager read timeout is invalid, clamped to default",
				"old", cfg.Manager.ReadTimeout, "new", DefaultManagerReadTimeout)
			cfg.Manager.ReadTimeout = DefaultManagerReadTimeout
		}

		if cfg.Manager.WriteTimeout <= 0 {
			logger.Logger.Warn("Manager write timeout is invalid, clamped to default",
				"old", cfg.Manager.WriteTimeout, "new", DefaultManagerWriteTimeout)
			cfg.Manager.WriteTimeout = DefaultManagerWriteTimeout
		}

		if cfg.Manager.IdleTimeout <= 0 {
			logger.Logger.Warn("Manager idle timeout is invalid, clamped to default",
				"old", cfg.Manager.IdleTimeout, "new", DefaultManagerIdleTimeout)
			cfg.Manager.IdleTimeout = DefaultManagerIdleTimeout
		}
	}

	// Infra presence-without-config precheck. A non-nil sub-config means
	// "enable this infra"; missing required fields abort startup in Run()
	// via Init*. This only warns early so misconfigurations surface in
	// logs before the shutdown sequence runs.
	if cfg.Infra != nil {
		infraCfgs := map[string]func() error{
			componentBadger:     func() error { return cfg.Infra.Badger.Ensure().Validate() },
			componentBun:        func() error { return cfg.Infra.Bun.Ensure().Validate() },
			componentClickhouse: func() error { return cfg.Infra.Clickhouse.Ensure().Validate() },
			componentElastic:    func() error { return cfg.Infra.Elastic.Ensure().Validate() },
			componentMongo:      func() error { return cfg.Infra.Mongo.Ensure().Validate() },
			componentMQTT:       func() error { return cfg.Infra.MQTT.Ensure().Validate() },
			componentNATS:       func() error { return cfg.Infra.Nats.Ensure().Validate() },
			componentRedis:      func() error { return cfg.Infra.Redis.Ensure().Validate() },
			componentRistretto:  func() error { return cfg.Infra.Ristretto.Ensure().Validate() },
			componentS3:         func() error { return cfg.Infra.S3.Ensure().Validate() },
		}

		enabled := map[string]bool{
			componentBadger:     cfg.Infra.Badger != nil,
			componentBun:        cfg.Infra.Bun != nil,
			componentClickhouse: cfg.Infra.Clickhouse != nil,
			componentElastic:    cfg.Infra.Elastic != nil,
			componentMongo:      cfg.Infra.Mongo != nil,
			componentMQTT:       cfg.Infra.MQTT != nil,
			componentNATS:       cfg.Infra.Nats != nil,
			componentRedis:      cfg.Infra.Redis != nil,
			componentRistretto:  cfg.Infra.Ristretto != nil,
			componentS3:         cfg.Infra.S3 != nil,
		}

		for name, check := range infraCfgs {
			if !enabled[name] {
				continue
			}

			if err := check(); err != nil {
				logger.Logger.Warn("Infra config invalid, startup will abort",
					"infra", name,
					"error", err.Error(),
				)
			}
		}
	}

	switch cfg.LogLevel {
	case "trace", "debug", "info", "notice", "warn", "error", "fatal", "silence", "":
		// Known levels ("silence" is honored via Options.Silence, and an
		// empty level is defaulted to info by Ensure).
	default:
		logger.Logger.Warn("Unknown log level, it will behave as info",
			"log_level", cfg.LogLevel)
	}
}

// BeforeStart is part of the public API.
func BeforeStart(wrappers ...SickyWrapper) []SickyWrapper {
	wrapperMu.Lock()
	defer wrapperMu.Unlock()
	beforeStartWrappers = append(beforeStartWrappers, wrappers...)

	return append([]SickyWrapper(nil), beforeStartWrappers...)
}

// AfterStart is part of the public API.
func AfterStart(wrappers ...SickyWrapper) []SickyWrapper {
	wrapperMu.Lock()
	defer wrapperMu.Unlock()
	afterStartWrappers = append(afterStartWrappers, wrappers...)

	return append([]SickyWrapper(nil), afterStartWrappers...)
}

// BeforeStop is part of the public API.
func BeforeStop(wrappers ...SickyWrapper) []SickyWrapper {
	wrapperMu.Lock()
	defer wrapperMu.Unlock()
	beforeStopWrappers = append(beforeStopWrappers, wrappers...)

	return append([]SickyWrapper(nil), beforeStopWrappers...)
}

// OnReload registers callbacks invoked on SIGHUP. Reload handlers run in a
// dedicated goroutine while the process keeps serving; failures are logged
// and never abort the process. Config watching itself (e.g. viper.WatchConfig)
// is the caller's responsibility.
func OnReload(wrappers ...SickyWrapper) []SickyWrapper {
	wrapperMu.Lock()
	defer wrapperMu.Unlock()
	reloadWrappers = append(reloadWrappers, wrappers...)

	return append([]SickyWrapper(nil), reloadWrappers...)
}

// AfterStop is part of the public API.
func AfterStop(wrappers ...SickyWrapper) []SickyWrapper {
	wrapperMu.Lock()
	defer wrapperMu.Unlock()
	afterStopWrappers = append(afterStopWrappers, wrappers...)

	return append([]SickyWrapper(nil), afterStopWrappers...)
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
