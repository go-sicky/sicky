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
 * @package grpc
 * @author Dr.NP <np@herewe.tech>
 * @since 10/21/2024
 */

package grpc

import (
	"context"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/metrics"
)

// serverPID is cached: a syscall per RPC is pure overhead.
var serverPID = os.Getpid()

// LoggerConfig is a grpc component.
type LoggerConfig struct {
	Logger logger.GeneralLogger
}

func loggerConfigDefault(config ...LoggerConfig) LoggerConfig {
	if len(config) > 0 {
		return config[0]
	}

	return LoggerConfig{
		Logger: logger.Logger,
	}
}

// Constant error text for the access log of a panicking handler. The panic
// value is deliberately never logged here: recovery owns that record (it has
// the stack and the value), and passing it to a second sink risks leaking
// request data into a pipeline that expects only a failure string.
const (
	panicUnaryLogError  = "grpc handler panicked"
	panicStreamLogError = "grpc stream handler panicked"
)

// NewAccessLoggerInterceptor creates a new AccessLoggerInterceptor.
func NewAccessLoggerInterceptor(config ...LoggerConfig) grpc.UnaryServerInterceptor {
	cfg := loggerConfigDefault(config...)
	if cfg.Logger == nil {
		cfg.Logger = logger.Logger
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		var requestID, traceID, spanID, userAgent string

		md, ok := metadata.FromIncomingContext(ctx)
		if ok {
			// Sanitize at the read site, not at the log call. The
			// tracing interceptor only runs when a tracer is configured
			// (tracer.type defaults to "none"), so deferring this to it
			// left the access log recording raw client metadata in the
			// default configuration, bounded only by gRPC's
			// MaxHeaderListSize. The HTTP stacks sanitize at the read
			// site too; user-agent is deliberately left verbatim because
			// the propagation charset would reject every real agent.
			rids := md.Get("X-Request-ID")
			if len(rids) > 0 {
				requestID = sanitizePropagatedValue(rids[0])
			}

			tids := md.Get("X-B3-Traceid")
			if len(tids) > 0 {
				traceID = sanitizePropagatedValue(tids[0])
			}

			sids := md.Get("X-B3-Spanid")
			if len(sids) > 0 {
				spanID = sanitizePropagatedValue(sids[0])
			}

			uas := md.Get("user-agent")
			if len(uas) > 0 {
				userAgent = uas[0]
			}
		}

		start := time.Now()

		// Deferred so a panicking handler is still observed. Recovery is
		// outermost, so a panic unwinds through this interceptor before
		// recovery converts it; observed inline, the request would vanish
		// from the access log and the RED metrics entirely. The re-panic
		// keeps recovery outermost and keeps its Internal answer.
		defer func() {
			end := time.Now()

			// Fixed-order slice: one alloc, stable field order for log
			// indexing (a map here costs an extra alloc plus random order).
			args := []any{
				"pid", serverPID,
				"status", 200,
				"latency", end.Sub(start),
				"method", info.FullMethod,
				"user-agent", userAgent,
				"request-id", requestID,
				"trace-id", traceID,
				"parent-span-id", spanID,
			}

			if r := recover(); r != nil {
				metrics.ObserveServerRequest("grpc", info.FullMethod, info.FullMethod, codes.Internal.String(), end.Sub(start))

				args[3] = int(codes.Internal)
				args = append(args, "error", panicUnaryLogError)
				cfg.Logger.LogContext(ctx, logger.ErrorLevel, "grpc.request", args...)

				panic(r)
			}

			code := status.Code(err).String()
			metrics.ObserveServerRequest("grpc", info.FullMethod, info.FullMethod, code, end.Sub(start))

			ll := logger.DebugLevel
			msg := "grpc.request"
			if err != nil {
				ll = logger.ErrorLevel

				// Report the real gRPC code so error-rate dashboards see
				// failures instead of a hardcoded 200.
				args[3] = int(status.Code(err))
				args = append(args, "error", err.Error())
			}

			cfg.Logger.LogContext(ctx, ll, msg, args...)
		}()

		return handler(ctx, req)
	}
}

// NewStreamAccessLoggerInterceptor creates a new StreamAccessLoggerInterceptor.
func NewStreamAccessLoggerInterceptor(config ...LoggerConfig) grpc.StreamServerInterceptor {
	cfg := loggerConfigDefault(config...)
	if cfg.Logger == nil {
		cfg.Logger = logger.Logger
	}

	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		start := time.Now()

		// Same contract as the unary interceptor: a panicking stream handler
		// must still count and log before recovery turns it into Internal.
		defer func() {
			end := time.Now()

			args := []any{
				"pid", serverPID,
				"status", 200,
				"latency", end.Sub(start),
				"method", info.FullMethod,
				"client_stream", info.IsClientStream,
				"server_stream", info.IsServerStream,
			}

			if r := recover(); r != nil {
				metrics.ObserveServerRequest("grpc", info.FullMethod, info.FullMethod, codes.Internal.String(), end.Sub(start))

				args[3] = int(codes.Internal)
				args = append(args, "error", panicStreamLogError)
				cfg.Logger.LogContext(ss.Context(), logger.ErrorLevel, "grpc.stream", args...)

				panic(r)
			}

			code := status.Code(err).String()
			metrics.ObserveServerRequest("grpc", info.FullMethod, info.FullMethod, code, end.Sub(start))

			ll := logger.DebugLevel
			msg := "grpc.stream"
			if err != nil {
				ll = logger.ErrorLevel

				// Report the real gRPC code so error-rate dashboards see
				// failures instead of a hardcoded 200.
				args[3] = int(status.Code(err))
				args = append(args, "error", err.Error())
			}

			cfg.Logger.LogContext(ss.Context(), ll, msg, args...)
		}()

		return handler(srv, ss)
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
