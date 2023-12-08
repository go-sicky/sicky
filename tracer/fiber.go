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
	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel/trace"
)

type FiberMiddlewareConfig struct {
	Next func(c *fiber.Ctx) bool

	Tracer trace.Tracer
}

var FiberMiddlewareConfigDefault = &FiberMiddlewareConfig{
	Next: nil,
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

	return cfg
}

func NewFiberMiddleware(config ...*FiberMiddlewareConfig) fiber.Handler {
	cfg := fiberMiddlewareConfigDefault(config...)

	return func(c *fiber.Ctx) error {
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		if cfg.Tracer == nil {
			return c.Next()
		}

		_, span := cfg.Tracer.Start(c.Context(), c.Path())
		c.Next()
		span.End()

		return nil
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
