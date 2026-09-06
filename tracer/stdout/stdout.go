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
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/go-sicky/sicky/tracer"
	"github.com/go-sicky/sicky/tracer/internal"
)

// StdoutTracer is a stdout component.
type StdoutTracer struct {
	config   *Config
	ctx      context.Context
	options  *tracer.Options
	exporter *stdouttrace.Exporter
	provider *sdktrace.TracerProvider
}

// New creates a new instance (nil on invalid config).
func New(originalOpts *tracer.Options, originalCfg *Config) *StdoutTracer {
	opts := originalOpts.Ensure()
	cfg := originalCfg.Ensure()

	tc := &StdoutTracer{
		config:  cfg,
		ctx:     opts.Context,
		options: opts,
	}

	var sto []stdouttrace.Option
	if cfg.PrettyPrint {
		sto = append(sto, stdouttrace.WithPrettyPrint())
	}

	if !cfg.Timestamps {
		sto = append(sto, stdouttrace.WithoutTimestamps())
	}

	exporter, err := stdouttrace.New(sto...)
	if err != nil {
		tc.options.Logger.ErrorContext(
			tc.ctx,
			"trace exporter create failed",
			"tracer", tc.String(),
			"id", tc.options.ID,
			"name", tc.options.Name,
			"error", err.Error(),
		)

		return nil
	}

	tc.exporter = exporter

	if clamped := internal.ClampSampleRate(cfg.SampleRate); clamped != cfg.SampleRate {
		cfg.SampleRate = clamped
		tc.options.Logger.WarnContext(
			tc.ctx,
			"invalid sample rate, reset to 1.0",
			"tracer", tc.String(),
			"id", tc.options.ID,
			"name", tc.options.Name,
			"service", cfg.ServiceName,
			"version", cfg.ServiceVersion,
			"invalid_rate", cfg.SampleRate,
		)
	}

	// Create TracerProvider with batching configuration (shared helper).
	provider, err := internal.NewOTLPProvider(
		cfg.ServiceName, cfg.ServiceVersion, opts.ID.String(), cfg.SampleRate, exporter,
	)
	if err != nil {
		tc.options.Logger.ErrorContext(
			tc.ctx,
			"failed to merge tracing resources",
			"tracer", tc.String(),
			"id", tc.options.ID,
			"name", tc.options.Name,
			"service", cfg.ServiceName,
			"version", cfg.ServiceVersion,
			"error", err.Error(),
		)

		return nil // Return directly if resource creation fails
	}

	tc.provider = provider

	tc.options.Logger.InfoContext(
		tc.ctx,
		"tracer initialized successfully",
		"tracer", tc.String(),
		"service", cfg.ServiceName,
		"version", cfg.ServiceVersion,
		"sample_rate", cfg.SampleRate,
		"pretty_print", cfg.PrettyPrint,
	)
	tracer.Set(tc)

	return tc
}

// Context returns the component context.
func (tc *StdoutTracer) Context() context.Context {
	return tc.ctx
}

// Options returns the runtime options.
func (tc *StdoutTracer) Options() *tracer.Options {
	return tc.options
}

// String returns a human-readable name.
func (tc *StdoutTracer) String() string {
	return "stdout"
}

// ID returns the unique instance ID.
func (tc *StdoutTracer) ID() uuid.UUID {
	return tc.options.ID
}

// Name returns the component name.
func (tc *StdoutTracer) Name() string {
	return tc.options.Name
}

// Start starts the component.
func (tc *StdoutTracer) Start() error {
	tc.options.Logger.InfoContext(
		tc.ctx,
		"tracer started",
		"tracer", tc.String(),
		"id", tc.options.ID,
		"name", tc.options.Name,
		"service", tc.config.ServiceName,
		"version", tc.config.ServiceVersion,
	)

	return nil
}

// Stop stops the component and releases resources.
func (tc *StdoutTracer) Stop() error {
	if tc.provider != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		// Add shutdown logic to gracefully terminate the tracer
		if err := tc.provider.Shutdown(ctx); err != nil {
			// Add additional cleanup for exporter
			if tc.exporter != nil {
				if shutdownErr := tc.exporter.Shutdown(ctx); shutdownErr != nil {
					tc.options.Logger.WarnContext(
						tc.ctx,
						"failed to shutdown tracer exporter",
						"tracer", tc.String(),
						"id", tc.options.ID,
						"name", tc.options.Name,
						"service", tc.config.ServiceName,
						"version", tc.config.ServiceVersion,
						"error", shutdownErr.Error(),
					)
				}
			}

			tc.options.Logger.ErrorContext(
				tc.ctx,
				"tracer provider shutdown failed",
				"tracer", tc.String(),
				"id", tc.options.ID,
				"name", tc.options.Name,
				"service", tc.config.ServiceName,
				"version", tc.config.ServiceVersion,
				"error", err.Error(),
			)

			return err
		}
	}

	tc.options.Logger.InfoContext(
		tc.ctx,
		"tracer stopped successfully",
		"tracer", tc.String(),
		"id", tc.options.ID,
		"name", tc.options.Name,
		"service", tc.config.ServiceName,
		"version", tc.config.ServiceVersion,
	)

	return nil
}

// StdoutExporter is part of the public API.
func (tc *StdoutTracer) StdoutExporter() *stdouttrace.Exporter {
	return tc.exporter
}

// Provider returns the provider.
func (tc *StdoutTracer) Provider() *sdktrace.TracerProvider {
	return tc.provider
}

// Tracer returns the tracer.
func (tc *StdoutTracer) Tracer(name string) trace.Tracer {
	if tc.provider == nil {
		tc.options.Logger.WarnContext(
			tc.ctx,
			"requested tracer from nil provider",
			"tracer", tc.String(),
			"id", tc.options.ID,
			"name", tc.options.Name,
			"service", tc.config.ServiceName,
			"version", tc.config.ServiceVersion,
		)

		return noop.NewTracerProvider().Tracer(name)
	}

	tc.options.Logger.DebugContext(
		tc.ctx,
		"requested tracer",
		"tracer", tc.String(),
		"id", tc.options.ID,
		"name", tc.options.Name,
		"service", tc.config.ServiceName,
		"version", tc.config.ServiceVersion,
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
