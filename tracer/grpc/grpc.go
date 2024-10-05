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
 * @file grpc.go
 * @package grpc
 * @author Dr.NP <np@herewe.tech>
 * @since 09/15/2024
 */

package grpc

import (
	"context"

	"github.com/go-sicky/sicky/tracer"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
)

type GRPCTracer struct {
	config   *Config
	ctx      context.Context
	options  *tracer.Options
	exporter *otlptrace.Exporter
}

func New(opts *tracer.Options, cfg *Config) *GRPCTracer {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	tc := &GRPCTracer{
		config:  cfg,
		ctx:     context.Background(),
		options: opts,
	}

	var oo []otlptracegrpc.Option

	e, err := otlptracegrpc.New(tc.ctx, oo...)
	if err != nil {
		return nil
	}

	tc.exporter = e
	tracer.Instance(opts.ID, tc)

	return tc
}

func (tc *GRPCTracer) Context() context.Context {
	return tc.ctx
}

func (tc *GRPCTracer) Options() *tracer.Options {
	return tc.options
}

func (tc *GRPCTracer) String() string {
	return "grpc"
}

func (tc *GRPCTracer) ID() uuid.UUID {
	return tc.options.ID
}

func (tc *GRPCTracer) Name() string {
	return tc.options.Name
}

func (tc *GRPCTracer) Start() error {
	return nil
}

func (tc *GRPCTracer) Stop() error {
	return nil
}

func (tc *GRPCTracer) Trace() error {
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
