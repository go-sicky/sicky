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
 * @package service
 * @author Dr.NP <np@herewe.tech>
 * @since 08/01/2024
 */

package service

import (
	"github.com/go-sicky/sicky/client"
	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/server"
	"github.com/google/uuid"
)

const (
	DefaultServiceVersion = "latest"
	DefaultServiceBranch  = "main"
)

type ServiceWrapper func() error

type Options struct {
	Name    string
	Version string
	Branch  string
	ID      uuid.UUID
	Logger  logger.GeneralLogger
	Servers map[uuid.UUID]server.Server
	Clients map[uuid.UUID]client.Client

	beforeStart []ServiceWrapper
	afterStart  []ServiceWrapper
	beforeStop  []ServiceWrapper
	afterStop   []ServiceWrapper
}

func (o *Options) Ensure() *Options {
	if o == nil {
		o = new(Options)
	}

	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}

	if o.Name == "" {
		o.Name = "service::" + o.ID.String()
	}

	if o.Version == "" {
		o.Version = DefaultServiceVersion
	}

	if o.Branch == "" {
		o.Branch = DefaultServiceBranch
	}

	if o.Logger == nil {
		o.Logger = logger.DefaultGeneralLogger
	}

	if o.beforeStart == nil {
		o.beforeStart = make([]ServiceWrapper, 0)
	}

	if o.afterStart == nil {
		o.afterStart = make([]ServiceWrapper, 0)
	}

	if o.beforeStart == nil {
		o.beforeStop = make([]ServiceWrapper, 0)
	}

	if o.afterStop == nil {
		o.afterStop = make([]ServiceWrapper, 0)
	}

	return o
}

/* {{{ [Wrappers] */
func (o *Options) BeforeStart(wrappers ...ServiceWrapper) *Options {
	if o != nil {
		o.beforeStart = append(o.beforeStart, wrappers...)
	}

	return o
}

func (o *Options) AfterStart(wrappers ...ServiceWrapper) *Options {
	if o != nil {
		o.afterStart = append(o.afterStart, wrappers...)
	}

	return o
}

func (o *Options) BeforeStop(wrappers ...ServiceWrapper) *Options {
	if o != nil {
		o.beforeStop = append(o.beforeStop, wrappers...)
	}

	return o
}

func (o *Options) AfterStop(wrappers ...ServiceWrapper) *Options {
	if o != nil {
		o.afterStop = append(o.afterStop, wrappers...)
	}

	return o
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
