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
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-sicky/sicky/broker"
	"github.com/go-sicky/sicky/job"
	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/registry"
	"github.com/go-sicky/sicky/server"
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
}

var (
	Instance Service
)

func Run(svc ...Service) error {
	var (
		err  error
		errs []error
		me   Service
	)

	if len(svc) > 0 {
		me = svc[0]
	} else {
		me = Instance
	}

	if me == nil {
		logger.Fatal("No service initialized")
	}

	logger.InfoContext(
		me.Context(),
		"Startring service",
		"service", me.String(),
		"id", me.Options().ID,
		"name", me.Options().Name,
		"version", me.Options().Version,
		"branch", me.Options().Branch,
	)

	// Start service
	errs = me.Start()
	if errs != nil {
		err = errors.Join(errs...)
		logger.ErrorContext(
			me.Context(),
			"Service start failed",
			"errors", err.Error(),
		)

		// Stop and exit?
		me.Stop()

		return err
	}

	logger.InfoContext(
		me.Context(),
		"Service started",
		"service", me.String(),
		"id", me.Options().ID,
		"name", me.Options().Name,
		"version", me.Options().Version,
		"branch", me.Options().Branch,
	)

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGHUP, syscall.SIGABRT, os.Interrupt)
	select {
	case <-ch:
	case <-me.Context().Done():
	}

	logger.InfoContext(
		me.Context(),
		"Stopping service",
		"service", me.String(),
		"id", me.Options().ID,
		"name", me.Options().Name,
		"version", me.Options().Version,
		"branch", me.Options().Branch,
	)

	// Stop services
	errs = me.Stop()
	if errs != nil {
		err = errors.Join(errs...)
		logger.ErrorContext(
			me.Context(),
			"Service stop failed",
			"errors", err.Error(),
		)

		return err
	}

	logger.InfoContext(
		me.Context(),
		"Service stopped",
		"service", me.String(),
		"id", me.Options().ID,
		"name", me.Options().Name,
		"version", me.Options().Version,
		"branch", me.Options().Branch,
	)

	return nil
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
