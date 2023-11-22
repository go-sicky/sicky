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
	"log/slog"

	"github.com/go-sicky/sicky/client"
	"github.com/go-sicky/sicky/server"
)

type Options struct {
	Context context.Context
	Name    string
	ID      string
	Version string
	Logger  *slog.Logger

	Service *Service
}

type Option func(*Options)

/* {{{ [Options] */
func Name(n string) Option {
	return func(opts *Options) {
		opts.Name = n
	}
}

func ID(id string) Option {
	return func(opts *Options) {
		opts.ID = id
	}
}

func Version(v string) Option {
	return func(opts *Options) {
		opts.Version = v
	}
}

func Logger(l *slog.Logger) Option {
	return func(opts *Options) {
		opts.Logger = l
	}
}

func Server(srv server.Server) Option {
	return func(opts *Options) {
		// Append server
		opts.Service.servers[srv.Options().Name] = srv
	}
}

func Client(clt client.Client) Option {
	return func(opts *Options) {
		// Append client
		opts.Service.clients[clt.Options().Name] = clt
	}
}

func BeforeStart(fn ServiceWrapper) Option {
	return func(opts *Options) {
		opts.Service.beforeStart = append(opts.Service.beforeStart, fn)
	}
}

func AfterStart(fn ServiceWrapper) Option {
	return func(opts *Options) {
		opts.Service.afterStart = append(opts.Service.afterStart, fn)
	}
}

func BeforeStop(fn ServiceWrapper) Option {
	return func(opts *Options) {
		opts.Service.beforeStop = append(opts.Service.beforeStop, fn)
	}
}

func AfterStop(fn ServiceWrapper) Option {
	return func(opts *Options) {
		opts.Service.afterStop = append(opts.Service.afterStop, fn)
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
