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
 * @file otlp.go
 * @package internal
 * @author Dr.NP <np@herewe.tech>
 * @since 09/04/2026
 */

// Package internal holds the shared standard-OTLP provider construction
// used by the grpc/http/stdout tracer tracks. The Uptrace track is
// deliberately independent (uptrace-go SDK owns its provider) and must
// not use this helper.
package internal

import (
	"fmt"
	"os"

	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

// ClampSampleRate normalizes out-of-range rates to full sampling.
func ClampSampleRate(rate float64) float64 {
	if rate < 0.0 || rate > 1.0 {
		return 1.0
	}

	return rate
}

// NewSampler builds the sampling strategy for an OTLP tracer.
//
// With trustRemoteSampled false (the default) a client-supplied
// traceparent must not decide how much this service exports: sampling by
// request header is a cost DoS - the configured sample_rate would be
// bypassed by sending `traceparent: ...-01`. The remote parent's sampled
// flag is therefore replaced by this service's own ratio, while an
// upstream drop is still respected so spans never appear without their
// parents.
//
// trustRemoteSampled true keeps the classic ParentBased behavior for
// trusted meshes that coordinate sampling through the header.
func NewSampler(sampleRate float64, trustRemoteSampled bool) sdktrace.Sampler {
	ratio := sdktrace.TraceIDRatioBased(ClampSampleRate(sampleRate))
	if trustRemoteSampled {
		return sdktrace.ParentBased(ratio)
	}

	return sdktrace.ParentBased(ratio,
		sdktrace.WithRemoteParentSampled(ratio),
		sdktrace.WithRemoteParentNotSampled(sdktrace.NeverSample()),
	)
}

// NewOTLPProvider builds a batching TracerProvider with the standard
// sicky resource attributes (service name/version/instance, container)
// and the sampler described by NewSampler.
func NewOTLPProvider(serviceName, serviceVersion, instanceID string, sampleRate float64, trustRemoteSampled bool, exporter sdktrace.SpanExporter) (*sdktrace.TracerProvider, error) {
	cn, _ := os.Hostname()

	r, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
			semconv.ServiceInstanceID(instanceID),
			semconv.ContainerName(cn),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("merge tracing resources: %w", err)
	}

	sampler := NewSampler(sampleRate, trustRemoteSampled)

	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(r),
		sdktrace.WithSampler(sampler),
	), nil
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
