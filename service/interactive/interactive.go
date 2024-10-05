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
	return nil
}

func (s *Interactive) Stop() []error {
	return nil
}

func (s *Interactive) Servers(srvs ...server.Server) []server.Server {
	return nil
}

func (s *Interactive) Brokers(brks ...broker.Broker) []broker.Broker {
	return nil
}

func (s *Interactive) Tracers(trcs ...tracer.Tracer) []tracer.Tracer {
	return nil
}

func (s *Interactive) Jobs(jobs ...job.Job) []job.Job {
	return nil
}

func (s *Interactive) Registries(rgs ...registry.Registry) []registry.Registry {
	return nil
}

/*/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
