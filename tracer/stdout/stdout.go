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
 * @file stdout.go
 * @package stdout
 * @author Dr.NP <np@herewe.tech>
 * @since 09/14/2024
 */

package stdout

import (
	"context"
	"os"

	"github.com/go-sicky/sicky/tracer"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

type StdoutTracer struct {
	config   *Config
	ctx      context.Context
	options  *tracer.Options
	exporter *stdouttrace.Exporter
	provider *trace.TracerProvider
}

func New(opts *tracer.Options, cfg *Config) *StdoutTracer {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	tc := &StdoutTracer{
		config:  cfg,
		ctx:     context.Background(),
		options: opts,
	}

	var sto []stdouttrace.Option
	if cfg.PrettyPrint {
		sto = append(sto, stdouttrace.WithPrettyPrint())
	}

	if !cfg.Timestamps {
		sto = append(sto, stdouttrace.WithoutTimestamps())
	}

	st, err := stdouttrace.New(sto...)
	if err != nil {
		tc.options.Logger.ErrorContext(
			tc.ctx,
			"Trace exporter create failed",
			"exporter", tc.String(),
			"id", tc.options.ID,
			"name", tc.options.Name,
		)

		return nil
	}

	tc.exporter = st

	// Resource
	cn, _ := os.Hostname()
	r, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			//semconv.ServiceName(cfg.Sicky.Service.Name),
			//semconv.ServiceVersion(cfg.Sicky.Service.Version),
			//semconv.ServiceInstanceID(options.id),
			semconv.ContainerName(cn),
		),
	)

	if err != nil {
		tc.options.Logger.ErrorContext(
			tc.ctx,
			"Merge tracer provider resource failed",
			"tracer", tc.String(),
			"id", tc.options.ID,
			"name", tc.options.Name,
			"error", err.Error(),
		)
	}

	// Provider
	tc.provider = trace.NewTracerProvider(
		trace.WithBatcher(st),
		trace.WithResource(r),
	)

	tc.options.Logger.InfoContext(
		tc.ctx,
		"Tracer created",
		"tracer", tc.String(),
		"id", tc.options.ID,
		"name", tc.options.Name,
	)
	tracer.Instance(opts.ID, tc)

	return tc
}

func (exp *StdoutTracer) Context() context.Context {
	return exp.ctx
}

func (exp *StdoutTracer) Options() *tracer.Options {
	return exp.options
}

func (exp *StdoutTracer) String() string {
	return "stdout"
}

func (tc *StdoutTracer) ID() uuid.UUID {
	return tc.options.ID
}

func (tc *StdoutTracer) Name() string {
	return tc.options.Name
}

func (tc *StdoutTracer) Start() error {
	tc.options.Logger.InfoContext(
		tc.ctx,
		"Tracer started",
		"tracer", tc.String(),
		"id", tc.options.ID,
		"name", tc.options.Name,
	)

	return nil
}

func (tc *StdoutTracer) Stop() error {
	tc.options.Logger.InfoContext(
		tc.ctx,
		"Tracer stopped",
		"tracer", tc.String(),
		"id", tc.options.ID,
		"name", tc.options.Name,
	)

	return nil
}

func (tc *StdoutTracer) Trace() error {
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
