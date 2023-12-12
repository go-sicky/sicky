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
 * @file trace.go
 * @package sicky
 * @author Dr.NP <np@herewe.tech>
 * @since 12/09/2023
 */

package sicky

import (
	"context"
	"os"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

//var DefaultTraceProvider *sdktrace.TracerProvider

func NewTraceProvider(cfg *ConfigGlobal, opts ...Option) *sdktrace.TracerProvider {
	var (
		err      error
		exporter sdktrace.SpanExporter
		res      *resource.Resource
		options  = NewOptions()
	)

	for _, opt := range opts {
		opt(options)
	}

	if cfg.Sicky.Trace.Type == "stdout" {
		var opts []stdouttrace.Option
		if cfg.Sicky.Trace.Exporter.Stdout.PrettyPrint {
			opts = append(opts, stdouttrace.WithPrettyPrint())
		}

		if !cfg.Sicky.Trace.Exporter.Stdout.Timestamps {
			opts = append(opts, stdouttrace.WithoutTimestamps())
		}

		exporter, err = stdouttrace.New(opts...)
	} else {
		var opts []otlptracegrpc.Option
		if cfg.Sicky.Trace.Exporter.GRPC.Compress {
			opts = append(opts, otlptracegrpc.WithCompressor("gzip"))
		}

		if cfg.Sicky.Trace.Exporter.GRPC.Timeout != 0 {
			opts = append(opts, otlptracegrpc.WithTimeout(
				time.Second*time.Duration(cfg.Sicky.Trace.Exporter.GRPC.Timeout),
			))
		}

		if !cfg.Sicky.Trace.Exporter.GRPC.TLS {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}

		opts = append(opts, otlptracegrpc.WithEndpoint(cfg.Sicky.Trace.Exporter.GRPC.Endpoint))
		exporter, err = otlptracegrpc.New(
			context.Background(),
			opts...,
		)
	}

	if err != nil {
		return nil
	}

	cn, _ := os.Hostname()
	res, err = resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.Sicky.Service.Name),
			semconv.ServiceVersion(cfg.Sicky.Service.Version),
			semconv.ServiceInstanceID(options.id),
			semconv.ContainerName(cn),
		),
	)
	if err != nil {
		return nil
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Override
	//DefaultTraceProvider = tp

	return tp
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
