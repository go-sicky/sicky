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
 * @file uptrace.go
 * @package uptrace
 * @author Dr.NP <np@herewe.tech>
 * @since 12/30/2025
 */

package uptrace

import (
	"context"

	"github.com/go-sicky/sicky/tracer"
	"github.com/google/uuid"
	"github.com/uptrace/uptrace-go/uptrace"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

type UptraceTracer struct {
	config   *Config
	ctx      context.Context
	options  *tracer.Options
	provider *sdktrace.TracerProvider
}

func New(opts *tracer.Options, cfg *Config) *UptraceTracer {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	tc := &UptraceTracer{
		config:  cfg,
		ctx:     opts.Context,
		options: opts,
	}

	if cfg.DSN == "" {
		tc.options.Logger.ErrorContext(
			tc.ctx,
			"Uptrace DSN is required",
			"tracer", tc.String(),
			"id", tc.options.ID,
			"name", tc.options.Name,
		)

		return nil
	}

	// Validate configuration parameters
	if cfg.SampleRate < 0 || cfg.SampleRate > 1 {
		cfg.SampleRate = 1.0
		tc.options.Logger.WarnContext(
			tc.ctx,
			"Invalid sample rate, reset to 1.0",
			"tracer", tc.String(),
			"id", tc.options.ID,
			"name", tc.options.Name,
			"sample_rate", cfg.SampleRate,
		)
	}

	// Configure Uptrace
	uptrace.ConfigureOpentelemetry(
		uptrace.WithDSN(cfg.DSN),
		uptrace.WithServiceName(opts.Name),
		uptrace.WithServiceVersion("latest"),
	)

	// Get the TracerProvider configured by Uptrace
	provider, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider)
	if !ok {
		tc.options.Logger.ErrorContext(
			tc.ctx,
			"Failed to get TracerProvider from Uptrace",
			"tracer", tc.String(),
			"id", tc.options.ID,
			"name", tc.options.Name,
		)

		return nil
	}

	tc.provider = provider

	tc.options.Logger.InfoContext(
		tc.ctx,
		"Tracer created",
		"tracer", tc.String(),
		"id", tc.options.ID,
		"name", tc.options.Name,
		"dsn", cfg.DSN,
		"sample_rate", cfg.SampleRate,
	)
	tracer.Set(tc)

	return tc
}

func (tc *UptraceTracer) Context() context.Context {
	return tc.ctx
}

func (tc *UptraceTracer) Options() *tracer.Options {
	return tc.options
}

func (tc *UptraceTracer) String() string {
	return "uptrace"
}

func (tc *UptraceTracer) ID() uuid.UUID {
	return tc.options.ID
}

func (tc *UptraceTracer) Name() string {
	return tc.options.Name
}

func (tc *UptraceTracer) Start() error {
	tc.options.Logger.InfoContext(
		tc.ctx,
		"Tracer started",
		"tracer", tc.String(),
		"id", tc.options.ID,
		"name", tc.options.Name,
		"dsn", tc.config.DSN,
		"sample_rate", tc.config.SampleRate,
	)

	return nil
}

func (tc *UptraceTracer) Stop() error {
	if tc.provider != nil {
		if err := tc.provider.Shutdown(tc.ctx); err != nil {
			tc.options.Logger.ErrorContext(
				tc.ctx,
				"Tracer provider shutdown failed",
				"tracer", tc.String(),
				"id", tc.options.ID,
				"name", tc.options.Name,
				"dsn", tc.config.DSN,
				"error", err.Error(),
			)

			return err
		}
	}

	// Shutdown Uptrace
	uptrace.Shutdown(tc.ctx)

	tc.options.Logger.InfoContext(
		tc.ctx,
		"Tracer stopped",
		"tracer", tc.String(),
		"id", tc.options.ID,
		"name", tc.options.Name,
		"dsn", tc.config.DSN,
	)

	return nil
}

func (tc *UptraceTracer) Provider() *sdktrace.TracerProvider {
	return tc.provider
}

func (tc *UptraceTracer) Tracer(name string) trace.Tracer {
	if tc.provider == nil {
		tc.options.Logger.WarnContext(
			tc.ctx,
			"Requested tracer from nil provider",
			"tracer", tc.String(),
			"id", tc.options.ID,
			"name", tc.options.Name,
			"dsn", tc.config.DSN,
		)

		return noop.NewTracerProvider().Tracer(name)
	}

	tc.options.Logger.DebugContext(
		tc.ctx,
		"Requested tracer",
		"tracer", tc.String(),
		"id", tc.options.ID,
		"name", tc.options.Name,
		"dsn", tc.config.DSN,
		"request", name,
	)

	return tc.provider.Tracer(name)
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
