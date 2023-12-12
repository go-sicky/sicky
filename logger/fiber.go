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
 * @package logger
 * @author Dr.NP <np@herewe.tech>
 * @since 11/26/2023
 */

package logger

import (
	"os"
	"time"

	"github.com/go-sicky/sicky/runtime"
	"github.com/gofiber/fiber/v2"
)

type FiberMiddlewareConfig struct {
	Next                   func(c *fiber.Ctx) bool
	RequestIDContextKey    string
	TraceIDContextKey      string
	SpanIDContextKey       string
	ParentSpanIDContextKey string
	SampledContextKey      string

	DefaultLevel     Level
	ClientErrorLevel Level
	ServerErrorLevel Level
	Logger           GeneralLogger
}

var FiberMiddlewareConfigDefault = &FiberMiddlewareConfig{
	Next:                   nil,
	RequestIDContextKey:    "requestid",
	TraceIDContextKey:      "traceid",
	SpanIDContextKey:       "spanid",
	ParentSpanIDContextKey: "parentspanid",
	SampledContextKey:      "sampled",

	DefaultLevel:     DebugLevel,
	ClientErrorLevel: WarnLevel,
	ServerErrorLevel: ErrorLevel,
}

func fiberMiddlewareConfigDefault(config ...*FiberMiddlewareConfig) *FiberMiddlewareConfig {
	if len(config) < 1 {
		return FiberMiddlewareConfigDefault
	}

	cfg := config[0]
	if cfg.Next == nil {
		cfg.Next = FiberMiddlewareConfigDefault.Next
	}

	if cfg.RequestIDContextKey == "" {
		cfg.RequestIDContextKey = FiberMiddlewareConfigDefault.RequestIDContextKey
	}

	if cfg.TraceIDContextKey == "" {
		cfg.TraceIDContextKey = FiberMiddlewareConfigDefault.TraceIDContextKey
	}

	if cfg.SpanIDContextKey == "" {
		cfg.SpanIDContextKey = FiberMiddlewareConfigDefault.SpanIDContextKey
	}

	if cfg.ParentSpanIDContextKey == "" {
		cfg.ParentSpanIDContextKey = FiberMiddlewareConfigDefault.ParentSpanIDContextKey
	}

	if cfg.SampledContextKey == "" {
		cfg.SampledContextKey = FiberMiddlewareConfigDefault.SampledContextKey
	}

	if cfg.DefaultLevel == 0 {
		cfg.DefaultLevel = FiberMiddlewareConfigDefault.DefaultLevel
	}

	if cfg.ClientErrorLevel == 0 {
		cfg.ClientErrorLevel = FiberMiddlewareConfigDefault.ClientErrorLevel
	}

	if cfg.ServerErrorLevel == 0 {
		cfg.ServerErrorLevel = FiberMiddlewareConfigDefault.ServerErrorLevel
	}

	return cfg
}

func NewFiberMiddleware(config ...*FiberMiddlewareConfig) fiber.Handler {
	cfg := fiberMiddlewareConfigDefault(config...)
	if cfg.Logger == nil {
		cfg.Logger = Logger
	}

	return func(c *fiber.Ctx) error {
		// Metric
		runtime.NumHTTPServerAccessCounter.Inc()

		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		start := time.Now()
		rv := c.Locals(cfg.RequestIDContextKey)
		requestID, _ := rv.(string)
		tv := c.Locals(cfg.TraceIDContextKey)
		traceID, _ := tv.(string)
		sv := c.Locals(cfg.SpanIDContextKey)
		spanID, _ := sv.(string)
		pv := c.Locals(cfg.ParentSpanIDContextKey)
		parentSpanID, _ := pv.(string)
		av := c.Locals(cfg.SampledContextKey)
		sampled, _ := av.(string)

		chainErr := c.Next()
		if chainErr != nil {
			_ = c.App().Config().ErrorHandler(c, chainErr)
		}

		end := time.Now()
		status := c.Response().Header.StatusCode()

		attributes := map[string]any{
			"pid":        os.Getpid(),
			"status":     status,
			"latency":    end.Sub(start),
			"route":      c.Route().Path,
			"method":     string(c.Request().Header.Method()),
			"host":       c.Hostname(),
			"path":       c.Path(),
			"ip":         c.IP(),
			"user-agent": string(c.Request().Header.UserAgent()),
			"referer":    c.Request().Header.Referer(),

			"request-id":     requestID,
			"trace-id":       traceID,
			"span-id":        spanID,
			"parent-span-id": parentSpanID,
			"sampled":        sampled,
		}

		// Extract attributes
		var args []any
		for k, v := range attributes {
			args = append(args, k, v)
		}

		l := cfg.DefaultLevel
		msg := "http.request"
		if chainErr != nil {
			if status >= fiber.StatusInternalServerError {
				l = cfg.ServerErrorLevel
			} else if status >= fiber.StatusBadRequest {
				l = cfg.ClientErrorLevel
			}

			msg = chainErr.Error()
		}

		cfg.Logger.LogContext(c.Context(), l, msg, args...)

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
