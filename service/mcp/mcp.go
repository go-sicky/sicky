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

	"github.com/go-sicky/sicky/broker"
	"github.com/go-sicky/sicky/job"
	"github.com/go-sicky/sicky/registry"
	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/service"
	"github.com/go-sicky/sicky/tracer"
)

type Mcp struct {
	config  *Config
	ctx     context.Context
	options *service.Options

	servers    []server.Server
	brokers    []broker.Broker
	jobs       []job.Job
	registries []registry.Registry
	tracers    []tracer.Tracer
	handlers   []Handler
	mcpServer  *MCPServer
}

func New(opts *service.Options, cfg *Config) *Mcp {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	svc := &Mcp{
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

func (s *Mcp) Context() context.Context {
	return s.ctx
}

func (s *Mcp) Options() *service.Options {
	return s.options
}

func (s *Mcp) String() string {
	return "mcp"
}

func (s *Mcp) Start() []error {
	var (
		err  error
		errs []error
	)

	for _, srv := range s.servers {
		if err = srv.Start(); err != nil {
			errs = append(errs, err)
		}
	}

	for _, brk := range s.brokers {
		if err = brk.Connect(); err != nil {
			errs = append(errs, err)
		}
	}

	for _, j := range s.jobs {
		if err = j.Start(); err != nil {
			errs = append(errs, err)
		}
	}

	for _, rg := range s.registries {
		if err = rg.Watch(); err != nil {
			errs = append(errs, err)
		}
	}

	for _, tr := range s.tracers {
		if err = tr.Start(); err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

func (s *Mcp) Stop() []error {
	var (
		err  error
		errs []error
	)

	for _, j := range s.jobs {
		if err = j.Stop(); err != nil {
			errs = append(errs, err)
		}
	}

	for _, rg := range s.registries {
		if err = rg.Stop(); err != nil {
			errs = append(errs, err)
		}
	}

	for _, tr := range s.tracers {
		if err = tr.Stop(); err != nil {
			errs = append(errs, err)
		}
	}

	for _, brk := range s.brokers {
		if err = brk.Disconnect(); err != nil {
			errs = append(errs, err)
		}
	}

	for _, srv := range s.servers {
		if err = srv.Stop(); err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

func (s *Mcp) Servers(srvs ...server.Server) []server.Server {
	if len(srvs) > 0 {
		s.servers = append(s.servers, srvs...)
	}

	return s.servers
}

func (s *Mcp) Brokers(brks ...broker.Broker) []broker.Broker {
	if len(brks) > 0 {
		s.brokers = append(s.brokers, brks...)
	}

	return s.brokers
}

func (s *Mcp) Jobs(jobs ...job.Job) []job.Job {
	if !s.config.DisableJobs && len(jobs) > 0 {
		s.jobs = append(s.jobs, jobs...)
	}

	return s.jobs
}

func (s *Mcp) Registries(rgs ...registry.Registry) []registry.Registry {
	if !s.config.DisableServerRegister && len(rgs) > 0 {
		s.registries = append(s.registries, rgs...)
	}

	return s.registries
}

func (s *Mcp) Tracers(trs ...tracer.Tracer) []tracer.Tracer {
	if !s.config.DisableTracing && len(trs) > 0 {
		s.tracers = append(s.tracers, trs...)
	}

	return s.tracers
}

func (s *Mcp) Handle(hdls ...Handler) {
	s.handlers = append(s.handlers, hdls...)
	s.mcpServer.Handle(hdls...)

	s.options.Logger.InfoContext(
		s.ctx,
		"MCP handler registered",
		"service", s.String(),
		"count", len(hdls),
	)
}

func (s *Mcp) MCPServer() *MCPServer {
	return s.mcpServer
}

func (s *Mcp) Handlers() []Handler {
	return s.handlers
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
