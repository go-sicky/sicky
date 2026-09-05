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
 * @file trace.go
 * @package grpc
 * @author Dr.NP <np@herewe.tech>
 * @since 03/03/2025
 */

package grpc

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	sickytracer "github.com/go-sicky/sicky/tracer"
)

// injectSpanContext merges the span context into the outgoing gRPC
// metadata (W3C traceparent/tracestate/baggage + B3, per the shared
// propagator) and ensures an X-Request-ID exists for correlation.
func injectSpanContext(ctx context.Context) context.Context {
	carrier := propagation.MapCarrier{}
	sickytracer.Inject(ctx, carrier)

	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.MD{}
	} else {
		md = md.Copy()
	}

	for k, v := range carrier {
		if v == "" {
			continue
		}

		md.Set(strings.ToLower(k), v)
	}

	if vals := md.Get("x-request-id"); len(vals) == 0 || vals[0] == "" {
		md.Set("x-request-id", uuid.New().String())
	}

	return metadata.NewOutgoingContext(ctx, md)
}

// NewClientTracingInterceptor creates a new ClientTracingInterceptor.
func NewClientTracingInterceptor(tracer trace.Tracer) grpc.UnaryClientInterceptor {
	if tracer != nil {
		return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			spanCtx, span := tracer.Start(ctx, method)
			defer span.End()

			err := invoker(injectSpanContext(spanCtx), method, req, reply, cc, opts...)
			if err != nil {
				span.RecordError(err)
			}

			return err
		}
	} else {
		return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			return invoker(ctx, method, req, reply, cc, opts...)
		}
	}
}

// tracingClientStream ends the span when the stream's context finishes.
type tracingClientStream struct {
	grpc.ClientStream
	span trace.Span
}

// NewClientStreamTracingInterceptor creates a new ClientStreamTracingInterceptor.
func NewClientStreamTracingInterceptor(tracer trace.Tracer) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		if tracer == nil {
			return streamer(ctx, desc, cc, method, opts...)
		}

		streamCtx, span := tracer.Start(ctx, method)
		streamCtx = injectSpanContext(streamCtx)
		cs, err := streamer(streamCtx, desc, cc, method, opts...)
		if err != nil {
			span.RecordError(err)
			span.End()

			return nil, err
		}

		wrapped := &tracingClientStream{ClientStream: cs, span: span}
		go func() {
			<-streamCtx.Done()
			span.End()
		}()

		return wrapped, nil
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
