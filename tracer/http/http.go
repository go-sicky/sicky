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
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/go-sicky/sicky/tracer"
	"github.com/go-sicky/sicky/tracer/internal"
)

// HTTPTracer is a http component.
type HTTPTracer struct {
	config   *Config
	ctx      context.Context
	options  *tracer.Options
	exporter *otlptrace.Exporter
	provider *sdktrace.TracerProvider
}

// New creates a new instance (nil on invalid config).
func New(opts *tracer.Options, cfg *Config) *HTTPTracer {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	tc := &HTTPTracer{
		config:  cfg,
		ctx:     opts.Context,
		options: opts,
	}

	var oo []otlptracehttp.Option

	if cfg.Endpoint != "" {
		oo = append(oo, otlptracehttp.WithEndpoint(cfg.Endpoint))
	}

	if cfg.Compress {
		oo = append(oo, otlptracehttp.WithCompression(otlptracehttp.GzipCompression))
	}

	if cfg.Timeout > 0 {
		oo = append(oo, otlptracehttp.WithTimeout(time.Duration(cfg.Timeout)*time.Second))
	}

	if len(cfg.Headers) > 0 {
		oo = append(oo, otlptracehttp.WithHeaders(cfg.Headers))
	}

	if cfg.Insecure {
		oo = append(oo, otlptracehttp.WithInsecure())
	}

	// Exporter
	e, err := otlptracehttp.New(tc.ctx, oo...)
	if err != nil {
		tc.options.Logger.ErrorContext(
			tc.ctx,
			"Trace exporter create failed",
			"tracer", tc.String(),
			"id", tc.options.ID,
			"name", tc.options.Name,
			"endpoint", cfg.Endpoint,
			"service", cfg.ServiceName,
			"version", cfg.ServiceVersion,
			"sample_rate", cfg.SampleRate,
			"error", err.Error(),
		)

		return nil
	}

	tc.exporter = e

	if clamped := internal.ClampSampleRate(cfg.SampleRate); clamped != cfg.SampleRate {
		cfg.SampleRate = clamped
		tc.options.Logger.WarnContext(
			tc.ctx,
			"Invalid sample rate, reset to 1.0",
			"tracer", tc.String(),
			"id", tc.options.ID,
			"name", tc.options.Name,
			"endpoint", cfg.Endpoint,
			"service", cfg.ServiceName,
			"version", cfg.ServiceVersion,
			"sample_rate", cfg.SampleRate,
		)
	}

	// Provider (shared standard-OTLP construction).
	provider, err := internal.NewOTLPProvider(
		cfg.ServiceName, cfg.ServiceVersion, opts.ID.String(), cfg.SampleRate, e,
	)
	if err != nil {
		tc.options.Logger.ErrorContext(
			tc.ctx,
			"Failed to merge tracing resources",
			"tracer", tc.String(),
			"id", tc.options.ID,
			"name", tc.options.Name,
			"endpoint", cfg.Endpoint,
			"service", cfg.ServiceName,
			"version", cfg.ServiceVersion,
			"sample_rate", cfg.SampleRate,
			"error", err.Error(),
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
		"endpoint", cfg.Endpoint,
		"service", cfg.ServiceName,
		"version", cfg.ServiceVersion,
		"sample_rate", cfg.SampleRate,
	)
	tracer.Set(tc)

	return tc
}

// Context returns the component context.
func (tc *HTTPTracer) Context() context.Context {
	return tc.ctx
}

// Options returns the runtime options.
func (tc *HTTPTracer) Options() *tracer.Options {
	return tc.options
}

// String returns a human-readable name.
func (tc *HTTPTracer) String() string {
	return "http"
}

// ID returns the unique instance ID.
func (tc *HTTPTracer) ID() uuid.UUID {
	return tc.options.ID
}

// Name returns the component name.
func (tc *HTTPTracer) Name() string {
	return tc.options.Name
}

// Start starts the component.
func (tc *HTTPTracer) Start() error {
	tc.options.Logger.InfoContext(
		tc.ctx,
		"Tracer started",
		"tracer", tc.String(),
		"id", tc.options.ID,
		"name", tc.options.Name,
		"endpoint", tc.config.Endpoint,
		"service", tc.config.ServiceName,
		"version", tc.config.ServiceVersion,
		"sample_rate", tc.config.SampleRate,
	)

	return nil
}

// Stop stops the component and releases resources.
func (tc *HTTPTracer) Stop() error {
	if tc.provider != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tc.provider.Shutdown(ctx); err != nil {
			if tc.exporter != nil {
				if shutdownErr := tc.exporter.Shutdown(ctx); shutdownErr != nil {
					tc.options.Logger.WarnContext(
						tc.ctx,
						"Failed to shutdown tracer exporter",
						"tracer", tc.String(),
						"id", tc.options.ID,
						"name", tc.options.Name,
						"endpoint", tc.config.Endpoint,
						"service", tc.config.ServiceName,
						"version", tc.config.ServiceVersion,
						"sample_rate", tc.config.SampleRate,
						"error", shutdownErr.Error(),
					)
				}
			}

			tc.options.Logger.ErrorContext(
				tc.ctx,
				"Tracer provider shutdown failed",
				"tracer", tc.String(),
				"id", tc.options.ID,
				"name", tc.options.Name,
				"endpoint", tc.config.Endpoint,
				"service", tc.config.ServiceName,
				"version", tc.config.ServiceVersion,
				"sample_rate", tc.config.SampleRate,
				"error", err.Error(),
			)

			return err
		}
	}

	tc.options.Logger.InfoContext(
		tc.ctx,
		"Tracer stopped",
		"tracer", tc.String(),
		"id", tc.options.ID,
		"name", tc.options.Name,
		"endpoint", tc.config.Endpoint,
		"service", tc.config.ServiceName,
		"version", tc.config.ServiceVersion,
		"sample_rate", tc.config.SampleRate,
	)

	return nil
}

// Exporter returns the exporter.
func (tc *HTTPTracer) Exporter() *otlptrace.Exporter {
	return tc.exporter
}

// Provider returns the provider.
func (tc *HTTPTracer) Provider() *sdktrace.TracerProvider {
	return tc.provider
}

// Tracer returns the tracer.
func (tc *HTTPTracer) Tracer(name string) trace.Tracer {
	if tc.provider == nil {
		tc.options.Logger.WarnContext(
			tc.ctx,
			"Requested tracer from nil provider",
			"tracer", tc.String(),
			"id", tc.options.ID,
			"name", tc.options.Name,
			"endpoint", tc.config.Endpoint,
			"service", tc.config.ServiceName,
			"version", tc.config.ServiceVersion,
			"sample_rate", tc.config.SampleRate,
		)

		return noop.NewTracerProvider().Tracer(name)
	}

	tc.options.Logger.DebugContext(
		tc.ctx,
		"Requested tracer",
		"tracer", tc.String(),
		"id", tc.options.ID,
		"name", tc.options.Name,
		"endpoint", tc.config.Endpoint,
		"service", tc.config.ServiceName,
		"version", tc.config.ServiceVersion,
		"sample_rate", tc.config.SampleRate,
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
