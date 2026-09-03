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
 * @file metadata.go
 * @package http
 * @author Dr.NP <np@herewe.tech>
 * @since 08/31/2026
 */

package http

import (
	"context"
	"net/http"

	"github.com/uptrace/bunrouter"
)

type MetadataConfig struct {
	Next                   func(c context.Context) bool
	RequestIDContextKey    string
	TraceIDContextKey      string
	SpanIDContextKey       string
	ParentSpanIDContextKey string
	SampledContextKey      string
	RequestIDHeader        string
	TraceIDHeader          string
	SpanIDHeader           string
	ParentSpanIDHeader     string
	SampledHeader          string
}

var MetadataConfigDefault = MetadataConfig{
	Next:                   nil,
	RequestIDContextKey:    "requestid",
	TraceIDContextKey:      "traceid",
	SpanIDContextKey:       "spanid",
	ParentSpanIDContextKey: "parentspanid",
	SampledContextKey:      "sampled",
	RequestIDHeader:        "X-Request-ID",
	TraceIDHeader:          "X-B3-Traceid",
	SpanIDHeader:           "X-B3-Spanid",
	ParentSpanIDHeader:     "X-B3-Parentspanid",
	SampledHeader:          "X-B3-Sampled",
}

func metadataConfigDefault(config ...MetadataConfig) MetadataConfig {
	if len(config) < 1 {
		return MetadataConfigDefault
	}

	cfg := config[0]
	if cfg.Next == nil {
		cfg.Next = MetadataConfigDefault.Next
	}

	if cfg.RequestIDContextKey == "" {
		cfg.RequestIDContextKey = MetadataConfigDefault.RequestIDContextKey
	}

	if cfg.TraceIDContextKey == "" {
		cfg.TraceIDContextKey = MetadataConfigDefault.TraceIDContextKey
	}

	if cfg.SpanIDContextKey == "" {
		cfg.SpanIDContextKey = MetadataConfigDefault.SpanIDContextKey
	}

	if cfg.ParentSpanIDContextKey == "" {
		cfg.ParentSpanIDContextKey = MetadataConfigDefault.ParentSpanIDContextKey
	}

	if cfg.SampledContextKey == "" {
		cfg.SampledContextKey = MetadataConfigDefault.SampledContextKey
	}

	if cfg.RequestIDHeader == "" {
		cfg.RequestIDHeader = MetadataConfigDefault.RequestIDHeader
	}

	if cfg.TraceIDHeader == "" {
		cfg.TraceIDHeader = MetadataConfigDefault.TraceIDHeader
	}

	if cfg.SpanIDHeader == "" {
		cfg.SpanIDHeader = MetadataConfigDefault.SpanIDHeader
	}

	if cfg.ParentSpanIDHeader == "" {
		cfg.ParentSpanIDHeader = MetadataConfigDefault.ParentSpanIDHeader
	}

	if cfg.SampledHeader == "" {
		cfg.SampledHeader = MetadataConfigDefault.SampledHeader
	}

	return cfg
}

func NewMetadataMiddleware(config ...MetadataConfig) bunrouter.MiddlewareFunc {
	cfg := metadataConfigDefault(config...)

	return func(next bunrouter.HandlerFunc) bunrouter.HandlerFunc {
		return func(w http.ResponseWriter, r bunrouter.Request) error {
			if cfg.Next == nil {
				cfg.Next = func(c context.Context) bool {
					return true
				}
			}

			ctx := r.Context()

			requestID := sanitizePropagatedValue(r.Header.Get(cfg.RequestIDHeader))
			traceID := sanitizePropagatedValue(r.Header.Get(cfg.TraceIDHeader))
			spanID := sanitizePropagatedValue(r.Header.Get(cfg.SpanIDHeader))
			parentSpanID := sanitizePropagatedValue(r.Header.Get(cfg.ParentSpanIDHeader))
			sampled := sanitizePropagatedValue(r.Header.Get(cfg.SampledHeader))

			ctx = context.WithValue(ctx, cfg.RequestIDContextKey, requestID)
			ctx = context.WithValue(ctx, cfg.TraceIDContextKey, traceID)
			ctx = context.WithValue(ctx, cfg.SpanIDContextKey, spanID)
			ctx = context.WithValue(ctx, cfg.ParentSpanIDContextKey, parentSpanID)
			ctx = context.WithValue(ctx, cfg.SampledContextKey, sampled)

			r = r.WithContext(ctx)

			return next(w, r)
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
