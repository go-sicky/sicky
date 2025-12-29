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
 * @file standard.go
 * @package standard
 * @author Dr.NP <np@herewe.tech>
 * @since 08/01/2024
 */

package standard

import (
	"context"

	"github.com/go-sicky/sicky/broker"
	"github.com/go-sicky/sicky/job"
	"github.com/go-sicky/sicky/registry"
	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/service"
	"github.com/go-sicky/sicky/tracer"
)

type Standard struct {
	config  *Config
	ctx     context.Context
	options *service.Options

	servers    []server.Server
	brokers    []broker.Broker
	jobs       []job.Job
	registries []registry.Registry
	tracers    []tracer.Tracer
}

func New(opts *service.Options, cfg *Config) *Standard {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	svc := &Standard{
		config:  cfg,
		ctx:     opts.Context,
		options: opts,

		servers:    make([]server.Server, 0),
		brokers:    make([]broker.Broker, 0),
		jobs:       make([]job.Job, 0),
		registries: make([]registry.Registry, 0),
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

	service.Set(svc)

	return svc
}

func (s *Standard) Context() context.Context {
	return s.ctx
}

func (s *Standard) Options() *service.Options {
	return s.options
}

func (s *Standard) String() string {
	return "standard"
}

func (s *Standard) Start() []error {
	var (
		err  error
		errs []error
	)

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

	// Registry
	// if !s.config.DisableServerRegister {
	// 	for _, rg := range s.registries {
	// 		rg.Watch()
	// 		for _, srv := range s.servers {
	// 			if srv.Running() {
	// 				if err = rg.Register(srv); err != nil {
	// 					errs = append(errs, err)
	// 				}
	// 			}
	// 		}
	// 	}
	// }

	return errs
}

func (s *Standard) Stop() []error {
	var (
		err  error
		errs []error
	)

	// Deregister
	// if !s.config.DisableServerRegister {
	// 	for _, rg := range s.registries {
	// 		for _, srv := range s.servers {
	// 			if srv.Running() {
	// 				if err = rg.Deregister(srv); err != nil {
	// 					errs = append(errs, err)
	// 				}
	// 			}
	// 		}

	// 		rg.Context().Done()
	// 	}
	// }

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

	return errs
}

/* {{{ [Standard] */
func (s *Standard) Servers(srvs ...server.Server) []server.Server {
	if len(srvs) > 0 {
		s.servers = append(s.servers, srvs...)
	}

	return s.servers
}

func (s *Standard) Brokers(brks ...broker.Broker) []broker.Broker {
	if len(brks) > 0 {
		s.brokers = append(s.brokers, brks...)
	}

	return s.brokers
}

func (s *Standard) Jobs(jobs ...job.Job) []job.Job {
	if !s.config.DisableJobs && len(jobs) > 0 {
		s.jobs = append(s.jobs, jobs...)
	}

	return s.jobs
}

func (s *Standard) Registries(rgs ...registry.Registry) []registry.Registry {
	if !s.config.DisableServerRegister && len(rgs) > 0 {
		s.registries = append(s.registries, rgs...)
	}

	return s.registries
}

func (s *Standard) Tracers(trs ...tracer.Tracer) []tracer.Tracer {
	if !s.config.DisableTracing && len(trs) > 0 {
		s.tracers = append(s.tracers, trs...)
	}

	return s.tracers
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
