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
 * @package fiber
 * @author Dr.NP <np@herewe.tech>
 * @since 11/29/2023
 */

package fiber

import (
	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc/metadata"
)

type MetadataConfig struct {
	Next                   func(c *fiber.Ctx) bool
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

func NewMetadataMiddleware(config ...MetadataConfig) fiber.Handler {
	cfg := metadataConfigDefault(config...)

	return func(c *fiber.Ctx) error {
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

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

		md := metadata.Pairs(
			cfg.RequestIDHeader, requestID,
			cfg.TraceIDHeader, traceID,
			cfg.SpanIDHeader, spanID,
			cfg.ParentSpanIDHeader, parentSpanID,
			cfg.SampledHeader, sampled,
		)

		newCtx := metadata.NewOutgoingContext(c.UserContext(), md)
		c.SetUserContext(newCtx)

		return c.Next()
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
