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
 * @package sicky
 * @author Dr.NP <np@herewe.tech>
 * @since 11/21/2023
 */

package sicky

import (
	"context"

	"github.com/go-sicky/sicky/client"
	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/server"
	"github.com/google/uuid"
)

type Options struct {
	service *Service

	ctx         context.Context
	id          string
	logger      logger.GeneralLogger
	servers     map[string]server.Server
	clients     map[string]client.Client
	beforeStart []ServiceWrapper
	afterStart  []ServiceWrapper
	beforeStop  []ServiceWrapper
	afterStop   []ServiceWrapper
}

func NewOptions() *Options {
	return &Options{
		servers: make(map[string]server.Server),
		clients: make(map[string]client.Client),
		id:      uuid.New().String(),
	}
}

type Option func(*Options)

/* {{{ [Options] */
func ID(id string) Option {
	return func(opts *Options) {
		opts.id = id
	}
}

func Logger(logger logger.GeneralLogger) Option {
	return func(opts *Options) {
		opts.logger = logger
	}
}

func Server(srv server.Server) Option {
	return func(opts *Options) {
		// Append server
		if srv != nil {
			opts.servers[srv.Name()] = srv
		}
	}
}

func Client(clt client.Client) Option {
	return func(opts *Options) {
		// Append client
		if clt != nil {
			opts.clients[clt.Name()] = clt
		}
	}
}

func BeforeStart(fn ServiceWrapper) Option {
	return func(opts *Options) {
		opts.beforeStart = append(opts.beforeStart, fn)
	}
}

func AfterStart(fn ServiceWrapper) Option {
	return func(opts *Options) {
		opts.afterStart = append(opts.afterStart, fn)
	}
}

func BeforeStop(fn ServiceWrapper) Option {
	return func(opts *Options) {
		opts.beforeStop = append(opts.beforeStop, fn)
	}
}

func AfterStop(fn ServiceWrapper) Option {
	return func(opts *Options) {
		opts.afterStop = append(opts.afterStop, fn)
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
