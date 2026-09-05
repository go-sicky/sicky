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
 * @file recovery.go
 * @package grpc
 * @author Dr.NP <np@herewe.tech>
 * @since 09/04/2026
 */

package grpc

import (
	"context"
	"runtime/debug"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/go-sicky/sicky/logger"
)

// NewRecoveryInterceptor recovers panicking unary handlers and converts
// them to Internal errors. It must sit outermost in the interceptor chain
// so panics from tracing/logging/handler code are all contained: without
// it a single bad request crashes the whole Serve loop.
//
// The stack trace and panic value are logged server-side only; the client
// receives a generic Internal status so request data, credentials or SQL
// embedded in the panic can never leak off the box.
func NewRecoveryInterceptor(config ...LoggerConfig) grpc.UnaryServerInterceptor {
	cfg := loggerConfigDefault(config...)

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				logPanic(cfg, ctx, info.FullMethod, "unary", r)
				resp = nil
				err = status.Error(codes.Internal, "grpc handler panicked")
			}
		}()

		return handler(ctx, req)
	}
}

// NewStreamRecoveryInterceptor is the streaming counterpart of
// NewRecoveryInterceptor: a panicking stream handler must not kill the
// server either.
func NewStreamRecoveryInterceptor(config ...LoggerConfig) grpc.StreamServerInterceptor {
	cfg := loggerConfigDefault(config...)

	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if r := recover(); r != nil {
				logPanic(cfg, ss.Context(), info.FullMethod, "stream", r)
				err = status.Error(codes.Internal, "grpc stream handler panicked")
			}
		}()

		return handler(srv, ss)
	}
}

func logPanic(cfg LoggerConfig, ctx context.Context, method, kind string, r any) {
	if cfg.Logger == nil {
		cfg.Logger = logger.Logger
	}

	cfg.Logger.ErrorContext(
		ctx,
		"grpc handler panicked",
		"method", method,
		"kind", kind,
		"panic", r,
		"stack", string(debug.Stack()),
	)
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
