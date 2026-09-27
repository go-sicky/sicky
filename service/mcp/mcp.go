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
 * @file mcp.go
 * @package sicky
 * @author Dr.NP <np@herewe.tech>
 * @since 12/15/2024
 */

package mcp

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

// MCP is an MCP service.
type MCP struct {
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
	handlers   []Handler
	mcpServer  *MCPServer
}

// New creates a new instance (nil on invalid config).
func New(opts *service.Options, cfg *Config) *MCP {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	svc := &MCP{
		config:  cfg,
		ctx:     opts.Context,
		options: opts,
	}

	serverCaps := ServerCapabilities{
		Tools:     &ToolsCapability{},
		Resources: &ResourcesCapability{},
		Prompts:   &PromptsCapability{},
	}

	svc.mcpServer = NewMCPServer(
		ImplementationInfo{
			Name:    opts.Name,
			Version: opts.Version,
		},
		serverCaps,
	)

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
func (s *MCP) Context() context.Context {
	return s.ctx
}

// Options returns the runtime options.
func (s *MCP) Options() *service.Options {
	return s.options
}

// String returns a human-readable name.
func (s *MCP) String() string {
	return "mcp"
}

// subordinateSnapshot is what Start/Stop walk: a copy taken under the
// lock, so a concurrent registration cannot resize a slice mid-iteration.
func (s *MCP) subordinateSnapshot() (servers []server.Server, brokers []broker.Broker, jobs []job.Job, registries []registry.Registry, tracers []tracer.Tracer) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return slices.Clone(s.servers), slices.Clone(s.brokers), slices.Clone(s.jobs), slices.Clone(s.registries), slices.Clone(s.tracers)
}

// Start starts the component.
func (s *MCP) Start() []error {
	var (
		err  error
		errs []error
	)

	// Walk a snapshot: registrations may run concurrently and resizing a
	// slice while it is walked is a data race.
	servers, brokers, jobs, registries, tracers := s.subordinateSnapshot()

	for _, srv := range servers {
		if err = srv.Start(); err != nil {
			errs = append(errs, err)
		}
	}

	for _, brk := range brokers {
		if err = brk.Connect(); err != nil {
			errs = append(errs, err)
		}
	}

	for _, j := range jobs {
		if err = j.Start(); err != nil {
			errs = append(errs, err)
		}
	}

	for _, rg := range registries {
		if err = rg.Watch(); err != nil {
			errs = append(errs, err)
		}
	}

	for _, tr := range tracers {
		if err = tr.Start(); err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

// Stop stops the component and releases resources.
func (s *MCP) Stop() []error {
	var (
		err  error
		errs []error
	)

	// Walk a snapshot: registrations may run concurrently and resizing a
	// slice while it is walked is a data race.
	servers, brokers, jobs, registries, tracers := s.subordinateSnapshot()

	for _, j := range jobs {
		if err = j.Stop(); err != nil {
			errs = append(errs, err)
		}
	}

	for _, rg := range registries {
		if err = rg.Stop(); err != nil {
			errs = append(errs, err)
		}
	}

	for _, tr := range tracers {
		if err = tr.Stop(); err != nil {
			errs = append(errs, err)
		}
	}

	for _, brk := range brokers {
		if err = brk.Disconnect(); err != nil {
			errs = append(errs, err)
		}
	}

	for _, srv := range servers {
		if err = srv.Stop(); err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

// Servers returns the managed servers.
func (s *MCP) Servers(srvs ...server.Server) []server.Server {
	s.mu.Lock()
	if len(srvs) > 0 {
		s.servers = append(s.servers, srvs...)
	}

	out := slices.Clone(s.servers)
	s.mu.Unlock()

	return out
}

// Brokers returns a copy of the broker registry.
func (s *MCP) Brokers(brks ...broker.Broker) []broker.Broker {
	s.mu.Lock()
	if len(brks) > 0 {
		s.brokers = append(s.brokers, brks...)
	}

	out := slices.Clone(s.brokers)
	s.mu.Unlock()

	return out
}

// Jobs returns a copy of the job registry.
func (s *MCP) Jobs(jobs ...job.Job) []job.Job {
	s.mu.Lock()
	if !s.config.DisableJobs && len(jobs) > 0 {
		s.jobs = append(s.jobs, jobs...)
	}

	out := slices.Clone(s.jobs)
	s.mu.Unlock()

	return out
}

// Registries returns the managed registries.
func (s *MCP) Registries(rgs ...registry.Registry) []registry.Registry {
	s.mu.Lock()
	if !s.config.DisableServerRegister && len(rgs) > 0 {
		s.registries = append(s.registries, rgs...)
	}

	out := slices.Clone(s.registries)
	s.mu.Unlock()

	return out
}

// Tracers returns the managed tracers.
func (s *MCP) Tracers(trs ...tracer.Tracer) []tracer.Tracer {
	s.mu.Lock()
	if !s.config.DisableTracing && len(trs) > 0 {
		s.tracers = append(s.tracers, trs...)
	}

	out := slices.Clone(s.tracers)
	s.mu.Unlock()

	return out
}

// Handle registers handlers.
func (s *MCP) Handle(hdls ...Handler) {
	s.mu.Lock()
	s.handlers = append(s.handlers, hdls...)
	s.mu.Unlock()

	s.mcpServer.Handle(hdls...)

	s.options.Logger.InfoContext(
		s.ctx,
		"mcp handler registered",
		"service", s.String(),
		"count", len(hdls),
	)
}

// MCPServer returns the underlying MCP server.
func (s *MCP) MCPServer() *MCPServer {
	return s.mcpServer
}

// Handlers returns the registered handlers.
func (s *MCP) Handlers() []Handler {
	s.mu.Lock()
	defer s.mu.Unlock()

	return slices.Clone(s.handlers)
}

// Deprecated: use MCP.
type Mcp = MCP

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
