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
 * @file http.go
 * @package http
 * @author Dr.NP <np@herewe.tech>
 * @since 09/15/2024
 */

package http

import (
	"context"

	"github.com/go-sicky/sicky/tracer"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
)

type HTTPTracer struct {
	config   *Config
	ctx      context.Context
	options  *tracer.Options
	exporter *otlptrace.Exporter
}

func New(opts *tracer.Options, cfg *Config) *HTTPTracer {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	tc := &HTTPTracer{
		config:  cfg,
		ctx:     context.Background(),
		options: opts,
	}

	var oo []otlptracehttp.Option

	e, err := otlptracehttp.New(tc.ctx, oo...)
	if err != nil {
		return nil
	}

	tc.exporter = e
	tracer.Instance(opts.ID, tc)

	return tc
}

func (tc *HTTPTracer) Context() context.Context {
	return tc.ctx
}

func (tc *HTTPTracer) Options() *tracer.Options {
	return tc.options
}

func (tc *HTTPTracer) String() string {
	return "grpc"
}

func (tc *HTTPTracer) ID() uuid.UUID {
	return tc.options.ID
}

func (tc *HTTPTracer) Name() string {
	return tc.options.Name
}

func (tc *HTTPTracer) Start() error {
	return nil
}

func (tc *HTTPTracer) Stop() error {
	return nil
}

func (tc *HTTPTracer) Trace() error {
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
