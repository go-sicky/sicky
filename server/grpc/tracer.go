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

	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type TracerConfig struct {
	Tracer            trace.Tracer
	SpanIDContextKey  string
	TraceIDContextKey string
}

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

func NewTracingInterceptor(config ...TracerConfig) grpc.UnaryServerInterceptor {
	cfg := tracerConfigDefault(config...)
	pg := b3.New()

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if cfg.Tracer == nil {
			//ctx := context.WithValue(ctx, cfg.SpanIDContextKey, fmt.Sprintf("%x", utils.RandomHex(8)))

			return handler(ctx, req)
		}

		reqHeader := make(http.Header)
		md, ok := metadata.FromIncomingContext(ctx)
		if ok {
			for k, v := range md {
				reqHeader.Set(k, v[0])
			}
		}

		newCtx := pg.Extract(ctx, propagation.HeaderCarrier(reqHeader))
		spanedCtx, span := cfg.Tracer.Start(newCtx, info.FullMethod)
		defer span.End()

		self := span.SpanContext()
		spanID := self.SpanID().String()
		traceID := self.TraceID().String()
		nmd := metadata.Pairs(
			"X-B3-Traceid", traceID,
			"X-B3-Spanid", spanID,
			"X-B3-Parentspanid", reqHeader.Get("X-B3-Spanid"),
			"X-B3-Sampled", reqHeader.Get("X-B3-Sampled"),
			"X-Request-ID", reqHeader.Get("X-Request-ID"),
		)
		//savedCtx := metadata.NewOutgoingContext(spanedCtx, nmd)
		//savedCtx = context.WithValue(savedCtx, cfg.SpanIDContextKey, spanID)
		//savedCtx = context.WithValue(savedCtx, cfg.TraceIDContextKey, traceID)
		joined := metadata.Join(md, nmd)
		savedCtx := metadata.NewIncomingContext(spanedCtx, joined)
		resp, err := handler(savedCtx, req)
		if err != nil {
			span.RecordError(err)
		}

		return resp, err
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
