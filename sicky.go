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
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-sicky/sicky/client"
	"github.com/go-sicky/sicky/server"
	"github.com/google/uuid"
)

type ServiceWrapper func() error

// Service definition
type Service struct {
	ctx     context.Context
	logger  *slog.Logger
	servers map[string]server.Server
	clients map[string]client.Client
	options *Options

	beforeStart []ServiceWrapper
	afterStart  []ServiceWrapper
	beforeStop  []ServiceWrapper
	afterStop   []ServiceWrapper
}

var DefaultService *Service

const (
	DefaultServiceName    = "sicky.service"
	DefaultServiceVersion = "v0.0.0"
)

// NewService creates new micro service
func NewService(cfg *ConfigService, opts ...Option) *Service {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	// Default initialize
	svc := &Service{
		ctx:     ctx,
		logger:  logger,
		servers: make(map[string]server.Server),
		clients: make(map[string]client.Client),
		options: &Options{
			Name:    cfg.Name,
			Version: cfg.Version,
		},
	}

	svc.options.Service = svc

	for _, opt := range opts {
		opt(svc.options)
	}

	// Set logger
	if svc.options.Logger != nil {
		svc.logger = svc.options.Logger
	} else {
		svc.options.Logger = logger
	}

	// Set global context
	if svc.options.Context != nil {
		svc.ctx = svc.options.Context
	} else {
		svc.options.Context = ctx
	}

	// Set ID
	svc.options.ID = uuid.New().String()

	// Override
	DefaultService = svc

	return svc
}

// Boot service
func (svc *Service) Start() error {
	for _, fn := range svc.beforeStart {
		if err := fn(); err != nil {
			return err
		}
	}

	for _, srv := range svc.servers {
		if err := srv.Start(); err != nil {
			return err
		}
	}

	for _, fn := range svc.afterStart {
		if err := fn(); err != nil {
			return err
		}
	}

	return nil
}

func (svc *Service) Stop() []error {
	var errs []error
	for _, fn := range svc.beforeStop {
		if err := fn(); err != nil {
			errs = append(errs, err)
		}
	}

	for _, srv := range svc.servers {
		if err := srv.Stop(); err != nil {
			errs = append(errs, err)
		}
	}

	for _, fn := range svc.afterStop {
		if err := fn(); err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

func (svc *Service) Run() error {
	svc.logger.InfoContext(svc.ctx, "Starting service", "service", svc.options.Name)
	if err := svc.Start(); err != nil {
		svc.logger.ErrorContext(svc.ctx, "Service start failed", "error", err.Error())

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
			svc.logger.ErrorContext(svc.ctx, "Service stop failed", "error", err.Error())
		}

		return errs[0]
	}

	svc.logger.InfoContext(svc.ctx, "Stopping service", "service", svc.options.Name)

	return nil
}

func (svc *Service) Server(name string) server.Server {
	srv, ok := svc.servers[name]
	if !ok {
		return nil
	}

	return srv
}

func (svc *Service) Client(name string) client.Client {
	clt, ok := svc.clients[name]
	if !ok {
		return nil
	}

	return clt
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
