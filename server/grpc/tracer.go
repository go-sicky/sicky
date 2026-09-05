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
 * @file tracer.go
 * @package grpc
 * @author Dr.NP <np@herewe.tech>
 * @since 12/29/2024
 */

package grpc

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/go-sicky/sicky/tracer"
)

// TracerConfig is a grpc component.
type TracerConfig struct {
	Tracer            trace.Tracer
	SpanIDContextKey  string
	TraceIDContextKey string
}

// TracerConfigDefault is a shared grpc value.
var TracerConfigDefault = TracerConfig{
	Tracer:            nil,
	SpanIDContextKey:  "spanid",
	TraceIDContextKey: "traceid",
}

func tracerConfigDefault(config ...TracerConfig) TracerConfig {
	if len(config) < 1 {
		return TracerConfigDefault
	}

	cfg := config[0]
	if cfg.Tracer == nil {
		cfg.Tracer = TracerConfigDefault.Tracer
	}

	if cfg.SpanIDContextKey == "" {
		cfg.SpanIDContextKey = TracerConfigDefault.SpanIDContextKey
	}

	if cfg.TraceIDContextKey == "" {
		cfg.TraceIDContextKey = TracerConfigDefault.TraceIDContextKey
	}

	return cfg
}

// NewTracingInterceptor creates a new TracingInterceptor.
func NewTracingInterceptor(config ...TracerConfig) grpc.UnaryServerInterceptor {
	cfg := tracerConfigDefault(config...)

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if cfg.Tracer == nil {
			return handler(ctx, req)
		}

		reqHeader := make(http.Header)
		md, ok := metadata.FromIncomingContext(ctx)
		if ok {
			for k, v := range md {
				if len(v) == 0 {
					continue
				}

				reqHeader.Set(k, v[0])
			}
		}

		newCtx := tracer.Extract(ctx, propagation.HeaderCarrier(reqHeader))
		spanedCtx, span := cfg.Tracer.Start(newCtx, info.FullMethod)
		defer span.End()

		self := span.SpanContext()
		spanID := self.SpanID().String()
		traceID := self.TraceID().String()
		// Inject the new span context (W3C + B3) so downstream
		// handlers/clients see traceparent as well as X-B3-*.
		w3c := propagation.MapCarrier{}
		tracer.Inject(spanedCtx, w3c)
		nmd := metadata.Pairs(
			"X-B3-Traceid", traceID,
			"X-B3-Spanid", spanID,
			"X-B3-Parentspanid", sanitizePropagatedValue(reqHeader.Get("X-B3-Spanid")),
			"X-B3-Sampled", sanitizePropagatedValue(reqHeader.Get("X-B3-Sampled")),
			"X-Request-ID", sanitizePropagatedValue(reqHeader.Get("X-Request-ID")),
			"traceparent", w3c.Get("traceparent"),
			"tracestate", w3c.Get("tracestate"),
			"baggage", w3c.Get("baggage"),
		)
		// savedCtx := metadata.NewOutgoingContext(spanedCtx, nmd)
		// savedCtx = context.WithValue(savedCtx, cfg.SpanIDContextKey, spanID)
		// savedCtx = context.WithValue(savedCtx, cfg.TraceIDContextKey, traceID)
		// New span values go first: downstream readers take index 0,
		// so client-supplied (spoofable) values must not shadow them.
		joined := metadata.Join(nmd, md)
		savedCtx := metadata.NewIncomingContext(spanedCtx, joined)
		resp, err := handler(savedCtx, req)
		if err != nil {
			span.RecordError(err)
		}

		return resp, err
	}
}

// wrappedServerStream carries the span context into the stream handler.
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

// Context returns the component context.
func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}

// NewStreamTracingInterceptor creates a new StreamTracingInterceptor.
func NewStreamTracingInterceptor(config ...TracerConfig) grpc.StreamServerInterceptor {
	cfg := tracerConfigDefault(config...)

	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()
		if cfg.Tracer == nil {
			return handler(srv, ss)
		}

		reqHeader := make(http.Header)
		md, ok := metadata.FromIncomingContext(ctx)
		if ok {
			for k, v := range md {
				if len(v) == 0 {
					continue
				}

				reqHeader.Set(k, v[0])
			}
		}

		newCtx := tracer.Extract(ctx, propagation.HeaderCarrier(reqHeader))
		spanedCtx, span := cfg.Tracer.Start(newCtx, info.FullMethod)
		defer span.End()

		self := span.SpanContext()
		w3c := propagation.MapCarrier{}
		tracer.Inject(spanedCtx, w3c)
		nmd := metadata.Pairs(
			"X-B3-Traceid", self.TraceID().String(),
			"X-B3-Spanid", self.SpanID().String(),
			"X-B3-Parentspanid", sanitizePropagatedValue(reqHeader.Get("X-B3-Spanid")),
			"X-B3-Sampled", sanitizePropagatedValue(reqHeader.Get("X-B3-Sampled")),
			"X-Request-ID", sanitizePropagatedValue(reqHeader.Get("X-Request-ID")),
			"traceparent", w3c.Get("traceparent"),
			"tracestate", w3c.Get("tracestate"),
			"baggage", w3c.Get("baggage"),
		)
		// Mirror the unary interceptor: the span context plus the new
		// span values (first, so they shadow client-supplied ones).
		spanedCtx = metadata.NewIncomingContext(spanedCtx, metadata.Join(nmd, md))

		err := handler(srv, &wrappedServerStream{ServerStream: ss, ctx: spanedCtx})
		if err != nil {
			span.RecordError(err)
		}

		return err
	}
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
