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
 * @file service.go
 * @package service
 * @author Dr.NP <np@herewe.tech>
 * @since 08/01/2024
 */

package service

import (
	"context"
	"time"

	"github.com/go-sicky/sicky/broker"
	"github.com/go-sicky/sicky/job"
	"github.com/go-sicky/sicky/registry"
	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/tracer"
	"github.com/google/uuid"
)

type Service interface {
	// Get context
	Context() context.Context
	// Service options
	Options() *Options
	// Stringify
	String() string
	// Start service
	Start() []error
	// Stop service
	Stop() []error

	// Subdinates
	Servers(...server.Server) []server.Server
	Brokers(...broker.Broker) []broker.Broker
	Jobs(...job.Job) []job.Job
	Registries(...registry.Registry) []registry.Registry
	Tracers(...tracer.Tracer) []tracer.Tracer
}

type TickerHander func(time.Time, uint64) error

var (
	services       = make(map[uuid.UUID]Service)
	defaultService Service
	managerEnabled bool
	managerAddr    string
)

func Set(svcs ...Service) {
	for _, svc := range svcs {
		services[svc.Options().ID] = svc
		if defaultService == nil {
			defaultService = svc
		}
	}
}

func Get(id uuid.UUID) Service {
	return services[id]
}

func Default() Service {
	return defaultService
}

func EnableManager(addr string) {
	managerEnabled = true
	managerAddr = addr
}

func Services() map[uuid.UUID]Service {
	return services
}

// func Run() error {
// 	var (
// 		err     error
// 		errs    []error
// 		manager *Manager
// 	)

// 	if defaultService == nil {
// 		logger.Fatal("null service implementation")
// 	}

// 	logger.InfoContext(
// 		defaultService.Context(),
// 		"Startring service",
// 		"service", defaultService.String(),
// 		"id", defaultService.Options().ID,
// 		"name", defaultService.Options().Name,
// 		"version", defaultService.Options().Version,
// 		"branch", defaultService.Options().Branch,
// 	)

// 	// Start service
// 	errs = defaultService.Start()
// 	if errs != nil {
// 		err = errors.Join(errs...)
// 		logger.ErrorContext(
// 			defaultService.Context(),
// 			"Service start failed",
// 			"errors", err.Error(),
// 		)

// 		// Stop and exit?
// 		defaultService.Stop()

// 		return err
// 	}

// 	logger.InfoContext(
// 		defaultService.Context(),
// 		"Service started",
// 		"service", defaultService.String(),
// 		"id", defaultService.Options().ID,
// 		"name", defaultService.Options().Name,
// 		"version", defaultService.Options().Version,
// 		"branch", defaultService.Options().Branch,
// 	)

// 	if managerEnabled {
// 		manager = NewManager(managerAddr)
// 		manager.Start()
// 	}

// 	ch := make(chan os.Signal, 1)
// 	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGHUP, syscall.SIGABRT, os.Interrupt)
// 	select {
// 	case <-ch:
// 	case <-defaultService.Context().Done():
// 	}

// 	if managerEnabled && manager != nil {
// 		manager.Stop()
// 	}

// 	logger.InfoContext(
// 		defaultService.Context(),
// 		"Stopping service",
// 		"service", defaultService.String(),
// 		"id", defaultService.Options().ID,
// 		"name", defaultService.Options().Name,
// 		"version", defaultService.Options().Version,
// 		"branch", defaultService.Options().Branch,
// 	)

// 	// Stop runtime
// 	runtime.RuntimeDone <- struct{}{}

// 	// Stop services
// 	errs = defaultService.Stop()
// 	if errs != nil {
// 		err = errors.Join(errs...)
// 		logger.ErrorContext(
// 			defaultService.Context(),
// 			"Service stop failed",
// 			"errors", err.Error(),
// 		)

// 		return err
// 	}

// 	logger.InfoContext(
// 		defaultService.Context(),
// 		"Service stopped",
// 		"service", defaultService.String(),
// 		"id", defaultService.Options().ID,
// 		"name", defaultService.Options().Name,
// 		"version", defaultService.Options().Version,
// 		"branch", defaultService.Options().Branch,
// 	)

// 	return nil
// }

// func Shutdown() error {
// 	return nil
// }

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
