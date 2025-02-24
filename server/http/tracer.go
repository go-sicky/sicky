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
 * @since 12/29/2024
 */

package http

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-sicky/sicky/utils"
	"github.com/gofiber/fiber/v2"
	futils "github.com/gofiber/fiber/v2/utils"
	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type TracerConfig struct {
	Next              func(c *fiber.Ctx) bool
	Tracer            trace.Tracer
	SpanIDContextKey  string
	TraceIDContextKey string
}

var TracerConfigDefault = TracerConfig{
	Next:              nil,
	Tracer:            nil,
	SpanIDContextKey:  "spanid",
	TraceIDContextKey: "traceid",
}

func tracerConfigDefault(config ...TracerConfig) TracerConfig {
	if len(config) < 1 {
		return TracerConfigDefault
	}

	cfg := config[0]
	if cfg.Next == nil {
		cfg.Next = TracerConfigDefault.Next
	}

	if cfg.SpanIDContextKey == "" {
		cfg.SpanIDContextKey = TracerConfigDefault.SpanIDContextKey
	}

	if cfg.TraceIDContextKey == "" {
		cfg.TraceIDContextKey = TracerConfigDefault.TraceIDContextKey
	}

	return cfg
}

func NewTracerMiddleware(config ...TracerConfig) fiber.Handler {
	cfg := tracerConfigDefault(config...)
	pg := b3.New()

	return func(c *fiber.Ctx) error {
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		if cfg.Tracer == nil {
			c.Locals(cfg.SpanIDContextKey, fmt.Sprintf("%x", utils.RandomHex(8)))

			return c.Next()
		}

		savedCtx, cancel := context.WithCancel(c.UserContext())
		// Dump HTTP request header from fiber
		reqHeader := make(http.Header)
		c.Request().Header.VisitAll(func(k, v []byte) {
			reqHeader.Add(string(k), string(v))
		})

		newCtx := pg.Extract(savedCtx, propagation.HeaderCarrier(reqHeader))
		spanedCtx, span := cfg.Tracer.Start(newCtx, futils.CopyString(c.Path()))
		defer func() {
			cancel()
			span.End()
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
			c.App().Config().ErrorHandler(c, err)
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
