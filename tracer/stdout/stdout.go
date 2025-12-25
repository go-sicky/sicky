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
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

type StdoutTracer struct {
	config   *Config
	ctx      context.Context
	options  *tracer.Options
	exporter *stdouttrace.Exporter
	provider *sdktrace.TracerProvider
}

func New(originalOpts *tracer.Options, originalCfg *Config) *StdoutTracer {
	opts := originalOpts.Ensure()
	cfg := originalCfg.Ensure()

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

	exporter, err := stdouttrace.New(sto...)
	if err != nil {
		tc.options.Logger.ErrorContext(
			tc.ctx,
			"Trace exporter create failed",
			"tracer", tc.String(),
			"id", tc.options.ID,
			"name", tc.options.Name,
			"error", err.Error(),
		)

		return nil
	}

	tc.exporter = exporter

	// Resource
	cn, _ := os.Hostname()

	// Validate configuration parameters
	if cfg.SampleRate < 0 || cfg.SampleRate > 1 {
		cfg.SampleRate = 1.0 // Reset to full sampling when rate is out of range
		tc.options.Logger.WarnContext(
			tc.ctx,
			"Invalid sample rate, reset to 1.0",
			"tracer", tc.String(),
			"id", tc.options.ID,
			"name", tc.options.Name,
			"service", cfg.ServiceName,
			"version", cfg.ServiceVersion,
			"invalid_rate", cfg.SampleRate,
		)
	}

	baseResource := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(cfg.ServiceName),
		semconv.ServiceVersion(cfg.ServiceVersion),
		semconv.ServiceInstanceID(opts.ID.String()),
		semconv.ContainerName(cn),
	)

	r, err := resource.Merge(
		resource.Default(),
		baseResource,
	)
	if err != nil {
		tc.options.Logger.ErrorContext(
			tc.ctx,
			"Failed to merge tracing resources",
			"tracer", tc.String(),
			"id", tc.options.ID,
			"name", tc.options.Name,
			"service", cfg.ServiceName,
			"version", cfg.ServiceVersion,
			"error", err.Error(),
		)

		return nil // Return directly if resource creation fails
	}

	// Configure sampling strategy
	sampler := sdktrace.ParentBased(
		sdktrace.TraceIDRatioBased(cfg.SampleRate), // Get sample rate from config
	)

	// Create TracerProvider with batching configuration
	tc.provider = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter), // Improve performance with batching
		sdktrace.WithResource(r),
		sdktrace.WithSampler(sampler), // Add sampling strategy
	)

	tc.options.Logger.InfoContext(
		tc.ctx,
		"Tracer initialized successfully",
		"tracer", tc.String(),
		"service", cfg.ServiceName,
		"version", cfg.ServiceVersion,
		"sample_rate", cfg.SampleRate,
		"pretty_print", cfg.PrettyPrint,
	)
	tracer.Set(tc)

	return tc
}

func (tc *StdoutTracer) Context() context.Context {
	return tc.ctx
}

func (tc *StdoutTracer) Options() *tracer.Options {
	return tc.options
}

func (tc *StdoutTracer) String() string {
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
		"service", tc.config.ServiceName,
		"version", tc.config.ServiceVersion,
	)

	return nil
}

func (tc *StdoutTracer) Stop() error {
	if tc.provider != nil {
		// Add shutdown logic to gracefully terminate the tracer
		if err := tc.provider.Shutdown(tc.ctx); err != nil {
			// Add additional cleanup for exporter
			if tc.exporter != nil {
				if shutdownErr := tc.exporter.Shutdown(tc.ctx); shutdownErr != nil {
					tc.options.Logger.WarnContext(
						tc.ctx,
						"Failed to shutdown tracer exporter",
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
				"Tracer provider shutdown failed",
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
		"Tracer stopped successfully",
		"tracer", tc.String(),
		"id", tc.options.ID,
		"name", tc.options.Name,
		"service", tc.config.ServiceName,
		"version", tc.config.ServiceVersion,
	)

	return nil
}

func (tc *StdoutTracer) StdoutExporter() *stdouttrace.Exporter {
	return tc.exporter
}

func (tc *StdoutTracer) Provider() *sdktrace.TracerProvider {
	return tc.provider
}

func (tc *StdoutTracer) Tracer(name string) trace.Tracer {
	if tc.provider == nil {
		tc.options.Logger.WarnContext(
			tc.ctx,
			"Requested tracer from nil provider",
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
		"Requested tracer",
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
