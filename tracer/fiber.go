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
 * @file fiber.go
 * @package tracer
 * @author Dr.NP <np@herewe.tech>
 * @since 12/08/2023
 */

package tracer

import (
	"context"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type FiberMiddlewareConfig struct {
	Next             func(c *fiber.Ctx) bool
	Tracer           trace.Tracer
	SpanIDContextKey string
}

var FiberMiddlewareConfigDefault = &FiberMiddlewareConfig{
	Next:             nil,
	Tracer:           nil,
	SpanIDContextKey: "spanid",
}

func fiberMiddlewareConfigDefault(config ...*FiberMiddlewareConfig) *FiberMiddlewareConfig {
	if len(config) < 1 {
		return FiberMiddlewareConfigDefault
	}

	cfg := config[0]
	if cfg.Next == nil {
		cfg.Next = FiberMiddlewareConfigDefault.Next
	}

	if cfg.Tracer == nil {
		cfg.Tracer = FiberMiddlewareConfigDefault.Tracer
	}

	if cfg.SpanIDContextKey == "" {
		cfg.SpanIDContextKey = FiberMiddlewareConfigDefault.SpanIDContextKey
	}

	return cfg
}

func NewFiberMiddleware(config ...*FiberMiddlewareConfig) fiber.Handler {
	cfg := fiberMiddlewareConfigDefault(config...)
	pg := b3.New()

	return func(c *fiber.Ctx) error {
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		if cfg.Tracer == nil {
			return c.Next()
		}

		savedCtx, cancel := context.WithCancel(c.UserContext())
		reqHeader := make(http.Header)
		c.Request().Header.VisitAll(func(k, v []byte) {
			reqHeader.Add(string(k), string(v))
		})

		newCtx := pg.Extract(savedCtx, propagation.HeaderCarrier(reqHeader))
		spanedCtx, span := cfg.Tracer.Start(newCtx, utils.CopyString(c.Path()))
		defer func() {
			cancel()
			span.End()
		}()

		self := span.SpanContext()
		spanID := self.SpanID().String()
		c.Locals(cfg.SpanIDContextKey, spanID)
		c.SetUserContext(spanedCtx)

		err := c.Next()
		if err != nil {
			span.RecordError(err)
			_ = c.App().Config().ErrorHandler(c, err)
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
