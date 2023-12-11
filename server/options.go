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
 * @file options.go
 * @package server
 * @author Dr.NP <np@herewe.tech>
 * @since 11/20/2023
 */

package server

import (
	"context"
	"crypto/tls"

	"github.com/go-sicky/sicky/logger"
	"github.com/google/uuid"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Options of server
type Options struct {
	ctx      context.Context
	id       string
	tls      *tls.Config
	logger   logger.GeneralLogger
	handlers []Handler

	traceProvider *sdktrace.TracerProvider
}

func (o *Options) ID() string {
	return o.id
}

func (o *Options) Context() context.Context {
	return o.ctx
}

func (o *Options) TLS() *tls.Config {
	return o.tls
}

func (o *Options) Logger() logger.GeneralLogger {
	return o.logger
}

func (o *Options) Handlers() []Handler {
	return o.handlers
}

func (o *Options) TraceProvider() *sdktrace.TracerProvider {
	return o.traceProvider
}

func NewOptions() *Options {
	return &Options{
		id: uuid.New().String(),
	}
}

type Option func(*Options)

/* {{{ [Options] */
func ID(id string) Option {
	return func(opts *Options) {
		opts.id = id
	}
}

func Context(ctx context.Context) Option {
	return func(opts *Options) {
		opts.ctx = ctx
	}
}

func TLS(tls *tls.Config) Option {
	return func(opts *Options) {
		opts.tls = tls
	}
}

func Logger(logger logger.GeneralLogger) Option {
	return func(opts *Options) {
		opts.logger = logger
	}
}

func Handle(handler Handler) Option {
	return func(opts *Options) {
		opts.handlers = append(opts.handlers, handler)
	}
}

func TraceProvider(tp *sdktrace.TracerProvider) Option {
	return func(opts *Options) {
		opts.traceProvider = tp
	}
}

/* }}} */

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
