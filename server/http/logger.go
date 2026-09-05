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
 * @package http
 * @author Dr.NP <np@herewe.tech>
 * @since 08/31/2026
 */

package http

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/uptrace/bunrouter"

	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/metrics"
)

// serverPID is cached: a syscall per request is pure overhead.
var serverPID = os.Getpid()

// AccessLoggerMiddlewareConfig is a http component.
type AccessLoggerMiddlewareConfig struct {
	AccessLoggerConfig *AccessLoggerConfig
	Next               func(c context.Context) bool
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
func NewAccessLoggerMiddleware(config ...AccessLoggerMiddlewareConfig) bunrouter.MiddlewareFunc {
	cfg := accessLoggerMiddlewareConfigDefault(config...)
	if cfg.Logger == nil {
		cfg.Logger = logger.Logger
	}

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

			// Metric
			start := time.Now()
			rv := r.Context().Value(cfg.AccessLoggerConfig.RequestIDContextKey)
			requestID, _ := rv.(string)
			tv := r.Context().Value(cfg.AccessLoggerConfig.TraceIDContextKey)
			traceID, _ := tv.(string)
			sv := r.Context().Value(cfg.AccessLoggerConfig.SpanIDContextKey)
			spanID, _ := sv.(string)
			pv := r.Context().Value(cfg.AccessLoggerConfig.ParentSpanIDContextKey)
			parentSpanID, _ := pv.(string)
			av := r.Context().Value(cfg.AccessLoggerConfig.SampledContextKey)
			sampled, _ := av.(string)

			err := next(w, r)

			end := time.Now()
			// Prefer the status captured by the status middleware; fall
			// back to the legacy "Status" header for handlers that answer
			// through a path bypassing the middleware.
			status := StatusFromContext(r.Context())
			if status == 0 {
				status, _ = strconv.Atoi(w.Header().Get("Status"))
			}

			if status == 0 {
				status = http.StatusOK
			}

			metrics.ObserveServerRequest("http", r.Method, r.Route(), strconv.Itoa(status), end.Sub(start))

			// Fixed-order slice: one alloc, stable field order for log
			// indexing (a map here costs an extra alloc plus random order).
			args := []any{
				"pid", serverPID,
				"status", status,
				"latency", end.Sub(start),
				"route", r.Route(),
				"method", r.Method,
				"Host", r.Host,
				"path", r.URL.Path,
				"ip", r.RemoteAddr,
				"user-agent", r.UserAgent(),
				"referer", r.Referer(),
				"request-id", requestID,
				"trace-id", traceID,
				"span-id", spanID,
				"parent-span-id", parentSpanID,
				"sampled", sampled,
			}

			l := cfg.AccessLoggerConfig.AccessLevel
			msg := "http.request"
			if err != nil {
				// Error
				if status >= http.StatusInternalServerError {
					l = cfg.AccessLoggerConfig.ServerErrorLevel
				} else if status >= http.StatusBadRequest {
					l = cfg.AccessLoggerConfig.ClientErrorLevel
				}

				msg = err.Error()
			}

			cfg.Logger.LogContext(r.Context(), logger.LogLevel(l), msg, args...)

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
