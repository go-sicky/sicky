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
 * @file interactive.go
 * @package interactive
 * @author Dr.NP <np@herewe.tech>
 * @since 08/13/2024
 */

package interactive

import (
	"context"
	"fmt"
	"strings"
	"syscall"

	"github.com/fatih/color"
	"github.com/go-sicky/sicky/broker"
	"github.com/go-sicky/sicky/job"
	"github.com/go-sicky/sicky/registry"
	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/service"
	"github.com/go-sicky/sicky/tracer"
)

type Interactive struct {
	config  *Config
	ctx     context.Context
	options *service.Options

	servers    []server.Server
	brokers    []broker.Broker
	jobs       []job.Job
	registries []registry.Registry
	tracers    []tracer.Tracer
	handlers   []Handler
}

func New(opts *service.Options, cfg *Config) *Interactive {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	svc := &Interactive{
		config:  cfg,
		ctx:     context.Background(),
		options: opts,
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

func (s *Interactive) Context() context.Context {
	return s.ctx
}

func (s *Interactive) Options() *service.Options {
	return s.options
}

func (s *Interactive) String() string {
	return "interactive"
}

func (s *Interactive) Start() []error {
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

	if s.config.StartupInfo != "" {
		fmt.Println(s.config.StartupInfo)
	}

	// Wrapper
	if !s.config.DisableWrappers {
		for _, fn := range s.options.AfterStart() {
			if err = fn(s); err != nil {
				errs = append(errs, err)
			}
		}
	}

	go func() {
		for {
			exit := s.interact()
			if exit {
				break
			}
		}

		fmt.Println()
		syscall.Kill(syscall.Getpid(), syscall.SIGQUIT)
	}()

	return errs
}

func (s *Interactive) Stop() []error {
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

func (s *Interactive) Servers(srvs ...server.Server) []server.Server {
	if len(srvs) > 0 {
		s.servers = append(s.servers, srvs...)
	}

	return s.servers
}

func (s *Interactive) Brokers(brks ...broker.Broker) []broker.Broker {
	if len(brks) > 0 {
		s.brokers = append(s.brokers, brks...)
	}

	return s.brokers
}

func (s *Interactive) Jobs(jobs ...job.Job) []job.Job {
	if !s.config.DisableJobs && len(jobs) > 0 {
		s.jobs = append(s.jobs, jobs...)
	}

	return s.jobs
}

func (s *Interactive) Registries(rgs ...registry.Registry) []registry.Registry {
	if !s.config.DisableServerRegister && len(rgs) > 0 {
		s.registries = append(s.registries, rgs...)
	}

	return s.registries
}

func (s *Interactive) Tracers(trs ...tracer.Tracer) []tracer.Tracer {
	if s.config.DisableTracing && len(trs) > 0 {
		s.tracers = append(s.tracers, trs...)
	}

	return s.tracers
}

func (s *Interactive) Handle(hdls ...Handler) {
	for _, hdl := range hdls {
		s.handlers = append(s.handlers, hdl)
		s.options.Logger.InfoContext(
			s.ctx,
			"Interaction handler registered",
			"service", s.String(),
			"id", s.options.ID,
			"name", s.options.Name,
			"handler", hdl.Name(),
		)
	}
}

func (s *Interactive) interact() bool {
	var (
		p   *color.Color
		cmd string
	)

	switch strings.ToLower(s.config.PromptColor) {
	case "green":
		p = color.New(color.Bold, color.FgGreen)
	case "yellow":
		p = color.New(color.Bold, color.FgYellow)
	case "cyan":
		p = color.New(color.Bold, color.FgCyan)
	case "red":
		p = color.New(color.Bold, color.FgRed)
	case "blue":
		p = color.New(color.Bold, color.FgBlue)
	default:
		p = color.New(color.Bold, color.FgWhite)
	}

	p.PrintFunc()(s.config.Prompt)
	fmt.Scanf("%s", &cmd)

	cmd = strings.TrimSpace(cmd)
	parts := strings.SplitN(cmd, " ", 2)
	if len(parts) > 0 {
		if parts[0] == s.config.StopCommand {
			// Quit
			for _, hdl := range s.handlers {
				err := hdl.OnStop()
				if err != nil {
					fmt.Println("Error : ", err.Error())
				}
			}

			return true
		} else {
			// Normal
			for _, hdl := range s.handlers {
				err := hdl.OnInteract(parts[0], cmd)
				if err != nil {
					fmt.Println("Error : ", err.Error())
				}
			}
		}
	} // Or do nothing

	fmt.Println()

	return false
}

/* {{{[Command handler] */
type Handler interface {
	Name() string
	OnInteract(cmd, full string) error
	OnStop() error
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
