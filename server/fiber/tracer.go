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
 * @package fiber
 * @author Dr.NP <np@herewe.tech>
 * @since 12/29/2024
 */

package fiber

import (
	"context"
	"encoding/hex"
	"net/http"

	"github.com/gofiber/fiber/v2"
	futils "github.com/gofiber/fiber/v2/utils"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/go-sicky/sicky/tracer"
	"github.com/go-sicky/sicky/utils"
)

// TracerConfig is a fiber component.
type TracerConfig struct {
	Next              func(c *fiber.Ctx) bool
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

// TracerConfigDefault is a shared fiber value.
var TracerConfigDefault = TracerConfig{
	Next:              nil,
	Tracer:            nil,
	SpanIDContextKey:  DefaultSpanIDContextKey,
	TraceIDContextKey: DefaultTraceIDContextKey,
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

// NewTracerMiddleware creates a new TracerMiddleware.
func NewTracerMiddleware(config ...TracerConfig) fiber.Handler {
	cfg := tracerConfigDefault(config...)

	return func(c *fiber.Ctx) error {
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		for _, p := range cfg.SkipPaths {
			if c.Path() == p {
				return c.Next()
			}
		}

		if cfg.Tracer == nil {
			c.Locals(cfg.SpanIDContextKey, hex.EncodeToString(utils.RandomHex(8)))

			return c.Next()
		}

		savedCtx, cancel := context.WithCancel(c.UserContext())
		// Extract only the propagation headers instead of copying the
		// whole header set: full copies cost a string alloc per header
		// on every request. W3C (traceparent/tracestate/baggage) + B3
		// dual-extract via the shared propagator.
		reqHeader := make(http.Header, 9)
		for _, k := range []string{"traceparent", "tracestate", "baggage", "B3", "X-Request-Id", DefaultB3TraceIDHeader, DefaultB3SpanIDHeader, DefaultB3ParentSpanIDHeader, DefaultB3SampledHeader} {
			if v := c.Get(k); v != "" {
				reqHeader.Set(k, v)
			}
		}

		newCtx := tracer.Extract(savedCtx, propagation.HeaderCarrier(reqHeader))
		// Prefer the route template; fall back to a low-cardinality
		// method label (never the raw path: /users/123 would explode
		// the tracing backend index).
		spanName := "HTTP " + c.Method()
		if route := c.Route().Path; route != "" {
			spanName = futils.CopyString(route)
		}

		spanedCtx, span := cfg.Tracer.Start(newCtx, spanName)
		defer func() {
			// End the span before canceling its parent context so
			// the export is never cut off mid-flight.
			span.End()
			cancel()
		}()

		self := span.SpanContext()
		spanID := self.SpanID().String()
		traceID := self.TraceID().String()

		c.Locals(cfg.SpanIDContextKey, spanID)
		c.Locals(cfg.TraceIDContextKey, traceID)
		c.SetUserContext(spanedCtx)
		err := c.Next()
		if err != nil {
			span.RecordError(err)
			// The fiber core invokes ErrorHandler exactly once for a
			// chain error; calling it here as well would run the handler
			// 2-3 times per failing request.
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
