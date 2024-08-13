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

	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/service"
)

type Sicky struct {
	config  *Config
	ctx     context.Context
	options *service.Options

	servers []server.Server
}

func New(opts *service.Options, cfg *Config) *Sicky {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	svc := &Sicky{
		config:  cfg,
		ctx:     context.Background(),
		options: opts,

		servers: make([]server.Server, 0),
	}

	service.Instance = svc

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
	for _, fn := range s.options.BeforeStart() {
		if err = fn(s); err != nil {
			errs = append(errs, err)
		}
	}

	// Start servers
	for _, srv := range s.servers {
		if err = srv.Start(); err != nil {
			errs = append(errs, err)
		}
	}

	// Wrapper
	for _, fn := range s.options.AfterStart() {
		if err = fn(s); err != nil {
			errs = append(errs, err)
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
	for _, fn := range s.options.BeforeStop() {
		if err = fn(s); err != nil {
			errs = append(errs, err)
		}
	}

	// Stop servers
	for _, srv := range s.servers {
		if err = srv.Stop(); err != nil {
			errs = append(errs, err)
		}
	}

	// Wrapper
	for _, fn := range s.options.AfterStop() {
		if err = fn(s); err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

/* {{{ [Sicky] */
func (s *Sicky) Servers(srvs ...server.Server) []server.Server {
	s.servers = append(s.servers, srvs...)

	return s.servers
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
