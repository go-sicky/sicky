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
 * @file options.go
 * @package client
 * @author Dr.NP <np@herewe.tech>
 * @since 11/20/2023
 */

package client

import (
	"context"

	"github.com/go-sicky/sicky/logger"
	"github.com/google/uuid"
)

type ClientWrapper func() error

// Options of client
type Options struct {
	Name   string
	ID     uuid.UUID
	Logger logger.GeneralLogger

	Context context.Context

	beforeConnect []ClientWrapper
	afterConnect  []ClientWrapper
	beforeClose   []ClientWrapper
	afterClose    []ClientWrapper
}

func (o *Options) Ensure() *Options {
	if o == nil {
		o = new(Options)
	}

	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}

	if o.Name == "" {
		o.Name = "Client::" + o.ID.String()
	}

	if o.Logger == nil {
		o.Logger = logger.DefaultGeneralLogger
	}

	if o.Context == nil {
		o.Context = context.Background()
	}

	if o.beforeConnect == nil {
		o.beforeConnect = make([]ClientWrapper, 0)
	}

	if o.afterConnect == nil {
		o.afterConnect = make([]ClientWrapper, 0)
	}

	if o.beforeClose == nil {
		o.beforeClose = make([]ClientWrapper, 0)
	}

	if o.afterClose == nil {
		o.afterClose = make([]ClientWrapper, 0)
	}

	return o
}

/* {{{ [Wrappers] */
func (o *Options) BeforeConnect(wrappers ...ClientWrapper) *Options {
	if o != nil {
		o.beforeConnect = append(o.beforeConnect, wrappers...)
	}

	return o
}

func (o *Options) AfterConnect(wrappers ...ClientWrapper) *Options {
	if o != nil {
		o.afterConnect = append(o.afterConnect, wrappers...)
	}

	return o
}

func (o *Options) BeforeClose(wrappers ...ClientWrapper) *Options {
	if o != nil {
		o.beforeClose = append(o.beforeClose, wrappers...)
	}

	return o
}

func (o *Options) AfterClose(wrappers ...ClientWrapper) *Options {
	if o != nil {
		o.afterClose = append(o.afterClose, wrappers...)
	}

	return o
}

/* }}} */

// type Options struct {
// 	ctx    context.Context
// 	id     string
// 	tls    *tls.Config
// 	logger logger.GeneralLogger

// 	traceProvider *sdktrace.TracerProvider
// }

// func (o *Options) ID() string {
// 	return o.id
// }

// func (o *Options) Context() context.Context {
// 	return o.ctx
// }

// func (o *Options) TLS() *tls.Config {
// 	return o.tls
// }

// func (o *Options) Logger() logger.GeneralLogger {
// 	return o.logger
// }

// func (o *Options) TraceProvider() *sdktrace.TracerProvider {
// 	return o.traceProvider
// }

// func NewOptions() *Options {
// 	return &Options{
// 		id: uuid.New().String(),
// 	}
// }

// type Option func(*Options)

// /* {{{ [Options] */
// func ID(id string) Option {
// 	return func(opts *Options) {
// 		opts.id = id
// 	}
// }

// func Context(ctx context.Context) Option {
// 	return func(opts *Options) {
// 		opts.ctx = ctx
// 	}
// }

// func TLS(tls *tls.Config) Option {
// 	return func(opts *Options) {
// 		opts.tls = tls
// 	}
// }

// func Logger(logger logger.GeneralLogger) Option {
// 	return func(opts *Options) {
// 		opts.logger = logger
// 	}
// }

// func TraceProvider(tp *sdktrace.TracerProvider) Option {
// 	return func(opts *Options) {
// 		opts.traceProvider = tp
// 	}
// }

/* }}} */

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
