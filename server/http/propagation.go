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
 * @file propagation.go
 * @package http
 * @author Dr.NP <np@herewe.tech>
 * @since 083/31/2026
 */

package http

import (
	"context"
	"net/http"

	"github.com/uptrace/bunrouter"
)

type PropagationConfig struct {
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

var PropagationConfigDefault = PropagationConfig{
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

func propagationConfigDefault(config ...PropagationConfig) PropagationConfig {
	if len(config) < 1 {
		return PropagationConfigDefault
	}

	cfg := config[0]
	if cfg.Next == nil {
		cfg.Next = PropagationConfigDefault.Next
	}

	if cfg.RequestIDContextKey == "" {
		cfg.RequestIDContextKey = PropagationConfigDefault.RequestIDContextKey
	}

	if cfg.TraceIDContextKey == "" {
		cfg.TraceIDContextKey = PropagationConfigDefault.TraceIDContextKey
	}

	if cfg.SpanIDContextKey == "" {
		cfg.SpanIDContextKey = PropagationConfigDefault.SpanIDContextKey
	}

	if cfg.ParentSpanIDContextKey == "" {
		cfg.ParentSpanIDContextKey = PropagationConfigDefault.ParentSpanIDContextKey
	}

	if cfg.SampledContextKey == "" {
		cfg.SampledContextKey = PropagationConfigDefault.SampledContextKey
	}

	if cfg.RequestIDHeader == "" {
		cfg.RequestIDHeader = PropagationConfigDefault.RequestIDHeader
	}

	if cfg.TraceIDHeader == "" {
		cfg.TraceIDHeader = PropagationConfigDefault.TraceIDHeader
	}

	if cfg.SpanIDHeader == "" {
		cfg.SpanIDHeader = PropagationConfigDefault.SpanIDHeader
	}

	if cfg.ParentSpanIDHeader == "" {
		cfg.ParentSpanIDHeader = PropagationConfigDefault.ParentSpanIDHeader
	}

	if cfg.SampledHeader == "" {
		cfg.SampledHeader = PropagationConfigDefault.SampledHeader
	}

	return cfg
}

func NewPropagationMiddleware(config ...PropagationConfig) bunrouter.MiddlewareFunc {
	cfg := propagationConfigDefault(config...)
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

			ctx := r.Context()
			requestID := r.Header.Get(cfg.RequestIDHeader)
			traceID := r.Header.Get(cfg.TraceIDHeader)
			spanID := r.Header.Get(cfg.SpanIDHeader)
			parentSpanID := r.Header.Get(cfg.ParentSpanIDHeader)
			sampled := r.Header.Get(cfg.SampledHeader)

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
