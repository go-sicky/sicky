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
 * @package http
 * @author Dr.NP <np@herewe.tech>
 * @since 09/01/2026
 */

package http

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-sicky/sicky/tracer"
	"github.com/go-sicky/sicky/utils"
	"github.com/uptrace/bunrouter"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type TracerConfig struct {
	Next              func(context.Context) bool
	Tracer            trace.Tracer
	SpanIDContextKey  string
	TraceIDContextKey string
	// SkipPaths bypasses span creation (probe traffic would otherwise
	// burn trace-backend budget). Nil selects the default probe list;
	// set an explicit empty slice to trace everything.
	SkipPaths []string
}

// DefaultTracerSkipPaths covers the conventional probe endpoints.
var DefaultTracerSkipPaths = []string{"/health", "/metrics", "/docs"}

var TracerConfigDefault = TracerConfig{
	Next:              nil,
	Tracer:            nil,
	SpanIDContextKey:  "spanid",
	TraceIDContextKey: "traceid",
	SkipPaths:         DefaultTracerSkipPaths,
}

func tracerConfigDefault(config ...TracerConfig) TracerConfig {
	if len(config) < 1 {
		return TracerConfigDefault
	}

	cfg := config[0]
	if cfg.Next == nil {
		cfg.Next = TracerConfigDefault.Next
	}

	if cfg.SkipPaths == nil {
		cfg.SkipPaths = append([]string(nil), TracerConfigDefault.SkipPaths...)
	}

	if cfg.SpanIDContextKey == "" {
		cfg.SpanIDContextKey = TracerConfigDefault.SpanIDContextKey
	}

	if cfg.TraceIDContextKey == "" {
		cfg.TraceIDContextKey = TracerConfigDefault.TraceIDContextKey
	}

	return cfg
}

func NewTracerMiddleware(config ...TracerConfig) bunrouter.MiddlewareFunc {
	cfg := tracerConfigDefault(config...)
	if cfg.Next == nil {
		cfg.Next = func(c context.Context) bool {
			return true
		}
	}

	return func(next bunrouter.HandlerFunc) bunrouter.HandlerFunc {
		return func(w http.ResponseWriter, r bunrouter.Request) error {
			if !cfg.Next(r.Context()) {
				return next(w, r)
			}

			for _, p := range cfg.SkipPaths {
				if r.URL.Path == p {
					return next(w, r)
				}
			}

			if cfg.Tracer == nil {
				ctx := context.WithValue(r.Context(), cfg.SpanIDContextKey, fmt.Sprintf("%x", utils.RandomHex(8)))
				r = r.WithContext(ctx)

				return next(w, r)
			}

			savedCtx, cancel := context.WithCancel(r.Context())
			// W3C + B3 dual-extract via the shared propagator.
			newCtx := tracer.Extract(savedCtx, propagation.HeaderCarrier(r.Header))

			// Prefer the route template; fall back to a low-cardinality
			// method label (never the raw path: /users/:id would explode
			// the tracing backend index).
			spanName := "HTTP " + r.Method
			if route := r.Route(); route != "" {
				spanName = route
			}
			spanedCtx, span := cfg.Tracer.Start(newCtx, spanName)
			defer func() {
				// End the span before cancelling its parent context so
				// the export is never cut off mid-flight.
				span.End()
				cancel()
			}()

			self := span.SpanContext()
			spanID := self.SpanID().String()
			traceID := self.TraceID().String()

			ctx := context.WithValue(spanedCtx, cfg.SpanIDContextKey, spanID)
			ctx = context.WithValue(ctx, cfg.TraceIDContextKey, traceID)
			r = r.WithContext(ctx)

			err := next(w, r)
			if err != nil {
				span.RecordError(err)
			}

			return err
		}
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
