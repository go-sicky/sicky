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
 * @since 08/01/2024
 */

package sicky

import (
	"context"

	"github.com/go-sicky/sicky/broker"
	"github.com/go-sicky/sicky/job"
	"github.com/go-sicky/sicky/registry"
	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/service"
	"github.com/go-sicky/sicky/tracer"
)

type Sicky struct {
	config  *Config
	ctx     context.Context
	options *service.Options

	servers    []server.Server
	brokers    []broker.Broker
	tracers    []tracer.Tracer
	jobs       []job.Job
	registries []registry.Registry
}

func New(opts *service.Options, cfg *Config) *Sicky {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	svc := &Sicky{
		config:  cfg,
		ctx:     context.Background(),
		options: opts,

		servers:    make([]server.Server, 0),
		brokers:    make([]broker.Broker, 0),
		jobs:       make([]job.Job, 0),
		registries: make([]registry.Registry, 0),
	}

	if service.Instance == nil {
		service.Instance = svc
	}

	svc.options.Logger.InfoContext(
		svc.ctx,
		"Service created",
		"service", svc.String(),
		"id", svc.options.ID,
		"name", svc.options.Name,
		"version", svc.options.Version,
		"branch", svc.options.Branch,
	)

	return svc
}

func (s *Sicky) Context() context.Context {
	return s.ctx
}

func (s *Sicky) Options() *service.Options {
	return s.options
}

func (s *Sicky) String() string {
	return "sicky"
}

func (s *Sicky) Start() []error {
	var (
		err  error
		errs []error
	)

	// Wrapper
	if !s.config.DisableWrappers {
		for _, fn := range s.options.BeforeStart() {
			if err = fn(s); err != nil {
				errs = append(errs, err)
			}
		}
	}

	// Start servers
	for _, srv := range s.servers {
		if err = srv.Start(); err != nil {
			errs = append(errs, err)
		}
	}

	// Connect brokers
	for _, brk := range s.brokers {
		if err = brk.Connect(); err != nil {
			errs = append(errs, err)
		}
	}

	// Tracers
	if !s.config.DisableTrace {
		for _, trc := range s.tracers {
			if err = trc.Start(); err != nil {
				errs = append(errs, err)
			}
		}
	}

	// Registry
	if !s.config.DisableServerRegister {
		for _, rg := range s.registries {
			rg.Watch()
			for _, srv := range s.servers {
				if err = rg.Register(srv); err != nil {
					errs = append(errs, err)
				}
			}
		}
	}

	// Wrapper
	if !s.config.DisableWrappers {
		for _, fn := range s.options.AfterStart() {
			if err = fn(s); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errs
}

func (s *Sicky) Stop() []error {
	var (
		err  error
		errs []error
	)

	// Wrapper
	if !s.config.DisableWrappers {
		for _, fn := range s.options.BeforeStop() {
			if err = fn(s); err != nil {
				errs = append(errs, err)
			}
		}
	}

	// Deregister
	if !s.config.DisableServerRegister {
		for _, rg := range s.registries {
			for _, srv := range s.servers {
				if err = rg.Deregister(srv); err != nil {
					errs = append(errs, err)
				}
			}

			rg.Context().Done()
		}
	}

	// Tracers
	if !s.config.DisableTrace {
		for _, trc := range s.tracers {
			if err = trc.Stop(); err != nil {
				errs = append(errs, err)
			}
		}
	}

	// Disconnect brokers
	for _, brk := range s.brokers {
		if err = brk.Disconnect(); err != nil {
			errs = append(errs, err)
		}
	}

	// Stop servers
	for _, srv := range s.servers {
		if err = srv.Stop(); err != nil {
			errs = append(errs, err)
		}

		srv.Context().Done()
	}

	// Wrapper
	if !s.config.DisableWrappers {
		for _, fn := range s.options.AfterStop() {
			if err = fn(s); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errs
}

/* {{{ [Sicky] */
func (s *Sicky) Servers(srvs ...server.Server) []server.Server {
	if len(srvs) > 0 {
		s.servers = append(s.servers, srvs...)
	}

	return s.servers
}

func (s *Sicky) Brokers(brks ...broker.Broker) []broker.Broker {
	if len(brks) > 0 {
		s.brokers = append(s.brokers, brks...)
	}

	return s.brokers
}

func (s *Sicky) Tracers(trcs ...tracer.Tracer) []tracer.Tracer {
	if !s.config.DisableTrace && len(trcs) > 0 {
		s.tracers = append(s.tracers, trcs...)
	}

	return s.tracers
}

func (s *Sicky) Jobs(jobs ...job.Job) []job.Job {
	if !s.config.DisableJobs && len(jobs) > 0 {
		s.jobs = append(s.jobs, jobs...)
	}

	return s.jobs
}

func (s *Sicky) Registries(rgs ...registry.Registry) []registry.Registry {
	if !s.config.DisableServerRegister && len(rgs) > 0 {
		s.registries = append(s.registries, rgs...)
	}

	return s.registries
}

/* }}} */

/*/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
