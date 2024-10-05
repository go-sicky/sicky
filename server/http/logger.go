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
 * @file http.go
 * @package http
 * @author Dr.NP <np@herewe.tech>
 * @since 09/06/2024
 */

package http

import (
	"os"
	"time"

	"github.com/go-sicky/sicky/logger"
	"github.com/gofiber/fiber/v2"
)

type AccessLoggerMiddlewareConfig struct {
	AccessLoggerConfig *AccessLoggerConfig
	Next               func(c *fiber.Ctx) bool
	Logger             logger.GeneralLogger
}

func accessLoggerMiddlewareConfigDefault(config ...*AccessLoggerMiddlewareConfig) *AccessLoggerMiddlewareConfig {
	if len(config) < 1 {
		return &AccessLoggerMiddlewareConfig{
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

func NewAccessLoggerMiddleware(config ...*AccessLoggerMiddlewareConfig) fiber.Handler {
	cfg := accessLoggerMiddlewareConfigDefault(config...)
	if cfg.Logger == nil {
		cfg.Logger = logger.Logger
	}

	return func(c *fiber.Ctx) error {
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		start := time.Now()
		rv := c.Locals(cfg.AccessLoggerConfig.RequestIDContextKey)
		requestID, _ := rv.(string)
		tv := c.Locals(cfg.AccessLoggerConfig.TraceIDContextKey)
		traceID, _ := tv.(string)
		sv := c.Locals(cfg.AccessLoggerConfig.SpanIDContextKey)
		spanID, _ := sv.(string)
		pv := c.Locals(cfg.AccessLoggerConfig.ParentSpanIDContextKey)
		parentSpanID, _ := pv.(string)
		av := c.Locals(cfg.AccessLoggerConfig.SampledContextKey)
		sampled, _ := av.(string)
		chainErr := c.Next()
		if chainErr != nil {
			_ = c.App().Config().ErrorHandler(c, chainErr)
		}

		end := time.Now()
		status := c.Response().Header.StatusCode()
		attributes := map[string]any{
			"pid":            os.Getpid(),
			"status":         status,
			"latency":        end.Sub(start),
			"route":          c.Route().Path,
			"method":         string(c.Request().Header.Method()),
			"host":           c.Hostname(),
			"path":           c.Path(),
			"ip":             c.IP(),
			"user-agent":     string(c.Request().Header.UserAgent()),
			"referer":        c.Request().Header.Referer(),
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

		l := cfg.AccessLoggerConfig.AccessLevel
		msg := "http.request"
		if chainErr != nil {
			if status >= fiber.StatusInternalServerError {
				l = cfg.AccessLoggerConfig.ServerErrorLevel
			} else if status >= fiber.StatusBadRequest {
				l = cfg.AccessLoggerConfig.ClientErrorLevel
			}

			msg = chainErr.Error()
		}

		cfg.Logger.LogContext(c.Context(), logger.LogLevel(l), msg, args...)

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
