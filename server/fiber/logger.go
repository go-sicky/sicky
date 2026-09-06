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
 * @file logger.go
 * @package fiber
 * @author Dr.NP <np@herewe.tech>
 * @since 09/06/2024
 */

package fiber

import (
	"os"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/metrics"
)

// serverPID is cached: a syscall per request is pure overhead.
var serverPID = os.Getpid()

// AccessLoggerMiddlewareConfig is a fiber component.
type AccessLoggerMiddlewareConfig struct {
	AccessLoggerConfig *AccessLoggerConfig
	Next               func(c fiber.Ctx) bool
	Logger             logger.GeneralLogger
}

func accessLoggerMiddlewareConfigDefault(config ...AccessLoggerMiddlewareConfig) AccessLoggerMiddlewareConfig {
	if len(config) < 1 {
		return AccessLoggerMiddlewareConfig{
			AccessLoggerConfig: DefaultAccessLogger,
			Next:               nil,
			Logger:             logger.Logger,
		}
	}

	cfg := config[0]
	if cfg.Logger == nil {
		cfg.Logger = logger.Logger
	}

	if cfg.AccessLoggerConfig == nil {
		cfg.AccessLoggerConfig = DefaultAccessLogger
	}

	return cfg
}

// NewAccessLoggerMiddleware creates a new AccessLoggerMiddleware.
func NewAccessLoggerMiddleware(config ...AccessLoggerMiddlewareConfig) fiber.Handler {
	cfg := accessLoggerMiddlewareConfigDefault(config...)
	if cfg.Logger == nil {
		cfg.Logger = logger.Logger
	}

	return func(c fiber.Ctx) error {
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		start := time.Now()
		requestID := fiber.Locals[string](c, cfg.AccessLoggerConfig.RequestIDContextKey)
		traceID := fiber.Locals[string](c, cfg.AccessLoggerConfig.TraceIDContextKey)
		spanID := fiber.Locals[string](c, cfg.AccessLoggerConfig.SpanIDContextKey)
		parentSpanID := fiber.Locals[string](c, cfg.AccessLoggerConfig.ParentSpanIDContextKey)
		sampled := fiber.Locals[string](c, cfg.AccessLoggerConfig.SampledContextKey)
		chainErr := c.Next()
		// The fiber core invokes ErrorHandler exactly once for a chain
		// error; calling it here as well would run the handler 2-3 times
		// per failing request (logger + tracer + core).
		_ = chainErr

		end := time.Now()
		status := c.Response().Header.StatusCode()
		metrics.ObserveServerRequest("fiber", string(c.Request().Header.Method()), c.Route().Path, strconv.Itoa(status), end.Sub(start))
		// Fixed-order slice: one alloc, stable field order for log
		// indexing (a map here costs an extra alloc plus random order).
		args := []any{
			"pid", serverPID,
			"status", status,
			"latency", end.Sub(start),
			"route", c.Route().Path,
			"method", string(c.Request().Header.Method()),
			"host", c.Hostname(),
			"path", c.Path(),
			"ip", c.IP(),
			"user-agent", string(c.Request().Header.UserAgent()),
			"referer", c.Request().Header.Referer(),
			"request-id", requestID,
			"trace-id", traceID,
			"span-id", spanID,
			"parent-span-id", parentSpanID,
			"sampled", sampled,
		}

		l := cfg.AccessLoggerConfig.AccessLevel
		msg := "fiber.request"
		if chainErr != nil {
			if status >= fiber.StatusInternalServerError {
				l = cfg.AccessLoggerConfig.ServerErrorLevel
			} else if status >= fiber.StatusBadRequest {
				l = cfg.AccessLoggerConfig.ClientErrorLevel
			}

			msg = chainErr.Error()
		}

		cfg.Logger.LogContext(c.Context(), logger.LogLevel(l), msg, args...)

		return chainErr
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
