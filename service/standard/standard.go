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
	"slices"
	"sync"

	"github.com/go-sicky/sicky/broker"
	"github.com/go-sicky/sicky/job"
	"github.com/go-sicky/sicky/registry"
	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/service"
	"github.com/go-sicky/sicky/tracer"
)

// Standard is a standard component.
type Standard struct {
	config  *Config
	ctx     context.Context
	options *service.Options

	// mu guards every slice below: the getters return copies so a
	// registration from another goroutine never races a Start() walk.
	mu sync.Mutex

	servers    []server.Server
	brokers    []broker.Broker
	jobs       []job.Job
	registries []registry.Registry
	tracers    []tracer.Tracer
}

// New creates a new instance (nil on invalid config).
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
		tracers:    make([]tracer.Tracer, 0),
	}

	svc.options.Logger.InfoContext(
		svc.ctx,
		"service created",
		"service", svc.String(),
		"id", svc.options.ID,
		"name", svc.options.Name,
		"version", svc.options.Version,
		"branch", svc.options.Branch,
	)

	service.Set(svc)

	return svc
}

// Context returns the component context.
func (s *Standard) Context() context.Context {
	return s.ctx
}

// Options returns the runtime options.
func (s *Standard) Options() *service.Options {
	return s.options
}

// String returns a human-readable name.
func (s *Standard) String() string {
	return "standard"
}

// subordinateSnapshot is what Start/Stop walk: a copy taken under the
// lock, so a concurrent registration cannot resize a slice mid-iteration.
func (s *Standard) subordinateSnapshot() (servers []server.Server, brokers []broker.Broker, jobs []job.Job, registries []registry.Registry, tracers []tracer.Tracer) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return slices.Clone(s.servers), slices.Clone(s.brokers), slices.Clone(s.jobs), slices.Clone(s.registries), slices.Clone(s.tracers)
}

// Start starts the component.
func (s *Standard) Start() []error {
	var (
		err  error
		errs []error
	)

	// Walk a snapshot: registrations may run concurrently and resizing a
	// slice while it is walked is a data race.
	servers, brokers, jobs, registries, tracers := s.subordinateSnapshot()

	// Start servers
	for _, srv := range servers {
		if err = srv.Start(); err != nil {
			errs = append(errs, err)
		}
	}

	// Connect brokers
	for _, brk := range brokers {
		if err = brk.Connect(); err != nil {
			errs = append(errs, err)
		}
	}

	// Start jobs
	for _, j := range jobs {
		if err = j.Start(); err != nil {
			errs = append(errs, err)
		}
	}

	// Start registries
	for _, rg := range registries {
		if err = rg.Watch(); err != nil {
			errs = append(errs, err)
		}
	}

	// Start tracers
	for _, tr := range tracers {
		if err = tr.Start(); err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

// Stop stops the component and releases resources.
func (s *Standard) Stop() []error {
	var (
		err  error
		errs []error
	)

	// Walk a snapshot: registrations may run concurrently and resizing a
	// slice while it is walked is a data race.
	servers, brokers, jobs, registries, tracers := s.subordinateSnapshot()

	// Stop jobs
	for _, j := range jobs {
		if err = j.Stop(); err != nil {
			errs = append(errs, err)
		}
	}

	// Stop registries
	for _, rg := range registries {
		if err = rg.Stop(); err != nil {
			errs = append(errs, err)
		}
	}

	// Stop tracers
	for _, tr := range tracers {
		if err = tr.Stop(); err != nil {
			errs = append(errs, err)
		}
	}

	// Disconnect brokers
	for _, brk := range brokers {
		if err = brk.Disconnect(); err != nil {
			errs = append(errs, err)
		}
	}

	// Stop servers
	for _, srv := range servers {
		if err = srv.Stop(); err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

/* {{{ [Standard]. */
func (s *Standard) Servers(srvs ...server.Server) []server.Server {
	s.mu.Lock()
	if len(srvs) > 0 {
		s.servers = append(s.servers, srvs...)
	}

	out := slices.Clone(s.servers)
	s.mu.Unlock()

	return out
}

// Brokers returns a copy of the broker registry.
func (s *Standard) Brokers(brks ...broker.Broker) []broker.Broker {
	s.mu.Lock()
	if len(brks) > 0 {
		s.brokers = append(s.brokers, brks...)
	}

	out := slices.Clone(s.brokers)
	s.mu.Unlock()

	return out
}

// Jobs returns a copy of the job registry.
func (s *Standard) Jobs(jobs ...job.Job) []job.Job {
	s.mu.Lock()
	if !s.config.DisableJobs && len(jobs) > 0 {
		s.jobs = append(s.jobs, jobs...)
	}

	out := slices.Clone(s.jobs)
	s.mu.Unlock()

	return out
}

// Registries returns the managed registries.
func (s *Standard) Registries(rgs ...registry.Registry) []registry.Registry {
	s.mu.Lock()
	if !s.config.DisableServerRegister && len(rgs) > 0 {
		s.registries = append(s.registries, rgs...)
	}

	out := slices.Clone(s.registries)
	s.mu.Unlock()

	return out
}

// Tracers returns the managed tracers.
func (s *Standard) Tracers(trs ...tracer.Tracer) []tracer.Tracer {
	s.mu.Lock()
	if !s.config.DisableTracing && len(trs) > 0 {
		s.tracers = append(s.tracers, trs...)
	}

	out := slices.Clone(s.tracers)
	s.mu.Unlock()

	return out
}

/* }}} */

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
