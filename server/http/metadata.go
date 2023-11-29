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
 * @file metadata.go
 * @package http
 * @author Dr.NP <np@herewe.tech>
 * @since 11/29/2023
 */

package http

import (
	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc/metadata"
)

type MetadataConfig struct {
	Next                func(c *fiber.Ctx) bool
	RequestIDContextKey string
}

var MetadataConfigDefault = MetadataConfig{
	Next:                nil,
	RequestIDContextKey: "requestid",
}

func metadataConfigDefault(config ...MetadataConfig) MetadataConfig {
	if len(config) < 1 {
		return MetadataConfigDefault
	}

	cfg := config[0]

	if cfg.RequestIDContextKey == "" {
		cfg.RequestIDContextKey = MetadataConfigDefault.RequestIDContextKey
	}

	return cfg
}

func NewMetadataMiddleware(config ...MetadataConfig) fiber.Handler {
	cfg := metadataConfigDefault(config...)

	return func(c *fiber.Ctx) error {
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		requestIDv := c.Locals(cfg.RequestIDContextKey)
		requestID, _ := requestIDv.(string)

		md := metadata.New(
			map[string]string{
				cfg.RequestIDContextKey: requestID,
			},
		)

		c.SetUserContext(metadata.NewOutgoingContext(c.Context(), md))

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
