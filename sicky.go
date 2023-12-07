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
 * @file sicky.go
 * @package sicky
 * @author Dr.NP <np@herewe.tech>
 * @since 11/20/2023
 */

package sicky

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-sicky/sicky/client"
	"github.com/go-sicky/sicky/driver"
	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/server"
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"github.com/uptrace/bun"
)

type ServiceWrapper func() error

// Service definition
type Service struct {
	config  *ConfigGlobal
	options *Options
	ctx     context.Context
	drivers struct {
		bun   *bun.DB
		nats  *nats.Conn
		redis *redis.Client
	}

	metricsRegistry *prometheus.Registry
	metricsServer   *http.Server
}

var DefaultService *Service

// NewService creates new micro service
func NewService(cfg *ConfigGlobal, opts ...Option) *Service {
	var err error
	ctx := context.Background()
	// Default initialize
	svc := &Service{
		config:  cfg,
		ctx:     ctx,
		options: NewOptions(),
	}

	svc.options.service = svc
	for _, opt := range opts {
		opt(svc.options)
	}

	// Set logger
	if svc.options.logger == nil {
		svc.options.logger = logger.Logger
	}

	// Set global context
	if svc.options.ctx != nil {
		svc.ctx = svc.options.ctx
	} else {
		svc.options.ctx = ctx
	}

	// Load drivers
	if svc.config.Sicky.Drivers.Nats != nil {
		svc.drivers.nats, err = driver.InitNats(svc.config.Sicky.Drivers.Nats)
		if err != nil {
			svc.Logger().Fatalf("Initialize nats failed : %s", err)
		}
	}

	if svc.config.Sicky.Drivers.Redis != nil {
		svc.drivers.redis, err = driver.InitRedis(svc.config.Sicky.Drivers.Redis)
		if err != nil {
			svc.Logger().Fatalf("Initialize redis failed : %s", err)
		}
	}

	if svc.config.Sicky.Drivers.Bun != nil {
		svc.drivers.bun, err = driver.InitBun(svc.config.Sicky.Drivers.Bun)
		if err != nil {
			svc.Logger().Fatalf("Initialize database failed : %s", err)
		}
	}

	// Prometheus metrics
	svc.metricsRegistry = prometheus.NewRegistry()
	svc.metricsServer = &http.Server{Addr: svc.config.Sicky.Metrics.Exporter.Addr}
	http.Handle(
		svc.config.Sicky.Metrics.Exporter.Path,
		promhttp.HandlerFor(
			svc.metricsRegistry,
			promhttp.HandlerOpts{
				Registry: svc.metricsRegistry,
			},
		),
	)

	// Override
	DefaultService = svc

	return svc
}

// Boot service
func (svc *Service) Start() error {
	for _, fn := range svc.options.beforeStart {
		if err := fn(); err != nil {
			return err
		}
	}

	for _, srv := range svc.options.servers {
		if err := srv.Start(); err != nil {
			return err
		}
	}

	for _, fn := range svc.options.afterStart {
		if err := fn(); err != nil {
			return err
		}
	}

	go func() {
		svc.Logger().DebugContext(svc.ctx, "Starting prometheus exporter", "addr", svc.config.Sicky.Metrics.Exporter.Addr)
		err := svc.metricsServer.ListenAndServe()
		if err != nil {
			svc.Logger().ErrorContext(svc.ctx, "Listen prometheus exporter failed", "error", err)
		}
	}()

	return nil
}

func (svc *Service) Stop() []error {
	var errs []error
	for _, fn := range svc.options.beforeStop {
		if err := fn(); err != nil {
			errs = append(errs, err)
		}
	}

	for _, srv := range svc.options.servers {
		if err := srv.Stop(); err != nil {
			errs = append(errs, err)
		}
	}

	for _, fn := range svc.options.afterStop {
		if err := fn(); err != nil {
			errs = append(errs, err)
		}
	}

	svc.Logger().DebugContext(svc.ctx, "Stopping prometheus exporter")
	err := svc.metricsServer.Shutdown(svc.ctx)
	if err != nil {
		errs = append(errs, err)
	}

	return errs
}

func (svc *Service) Run() error {
	svc.Logger().InfoContext(
		svc.ctx,
		"Starting service",
		"id", svc.options.id,
		"service", svc.config.Sicky.Service.Name,
		"version", svc.config.Sicky.Service.Version,
	)
	if err := svc.Start(); err != nil {
		svc.Logger().ErrorContext(svc.ctx, "Service start failed", "error", err.Error())

		return err
	}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGHUP)
	select {
	case <-ch:
	case <-svc.ctx.Done():
	}

	if errs := svc.Stop(); errs != nil {
		for _, err := range errs {
			svc.Logger().ErrorContext(svc.ctx, "Service stop failed", "error", err.Error())
		}

		return errs[0]
	}

	svc.Logger().InfoContext(
		svc.ctx,
		"Stopping service",
		"id", svc.options.id,
		"service", svc.config.Sicky.Service.Name,
		"version", svc.config.Sicky.Service.Version,
	)

	return nil
}

/* {{{ [Values] */

func (svc *Service) Name() string {
	return svc.config.Sicky.Service.Name
}

func (svc *Service) ID() string {
	return svc.options.id
}

func (svc *Service) Version() string {
	return svc.config.Sicky.Service.Version
}

func (svc *Service) Logger() logger.GeneralLogger {
	return svc.options.logger
}

func (svc *Service) Server(name string) server.Server {
	srv, ok := svc.options.servers[name]
	if !ok {
		return nil
	}

	return srv
}

func (svc *Service) Client(name string) client.Client {
	clt, ok := svc.options.clients[name]
	if !ok {
		return nil
	}

	return clt
}

func (svc *Service) Nats() *nats.Conn {
	return svc.drivers.nats
}

func (svc *Service) Redis() *redis.Client {
	return svc.drivers.redis
}

func (svc *Service) Bun() *bun.DB {
	return svc.drivers.bun
}

/* }}} */

func (svc *Service) RegisterMetrics(s prometheus.Collector) {
	svc.metricsRegistry.MustRegister(s)
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
