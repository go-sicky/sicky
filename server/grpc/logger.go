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

	"github.com/go-sicky/sicky/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

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

func NewAccessLoggerInterceptor(config ...LoggerConfig) grpc.UnaryServerInterceptor {
	cfg := loggerConfigDefault(config...)
	if cfg.Logger == nil {
		cfg.Logger = logger.Logger
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		var (
			requestID, traceID, spanID, userAgent string
		)

		md, ok := metadata.FromIncomingContext(ctx)
		if ok {
			rids := md.Get("X-Request-ID")
			if len(rids) > 0 {
				requestID = rids[0]
			}

			tids := md.Get("X-B3-Traceid")
			if len(tids) > 0 {
				traceID = tids[0]
			}

			sids := md.Get("X-B3-Spanid")
			if len(sids) > 0 {
				spanID = sids[0]
			}

			uas := md.Get("user-agent")
			if len(uas) > 0 {
				userAgent = uas[0]
			}
		}

		start := time.Now()
		resp, err := handler(ctx, req)
		end := time.Now()

		attributes := map[string]any{
			"pid":            os.Getpid(),
			"status":         200,
			"latency":        end.Sub(start),
			"method":         info.FullMethod,
			"user-agent":     userAgent,
			"request-id":     requestID,
			"trace-id":       traceID,
			"parent-span-id": spanID,
		}

		var args []any
		for k, v := range attributes {
			args = append(args, k, v)
		}

		ll := logger.DebugLevel
		msg := "grpc.request"
		if err != nil {
			ll = logger.ErrorLevel

			args = append(args, "error", err.Error())
		}

		cfg.Logger.LogContext(ctx, ll, msg, args...)

		return resp, err
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
