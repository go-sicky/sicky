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
 * @file nats.go
 * @package nats
 * @author Dr.NP <np@herewe.tech>
 * @since 12/08/2023
 */

package nats

import (
	"context"
	"sync"

	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/server"
	"go.opentelemetry.io/otel/trace"
)

type NatsServer struct {
	config  *Config
	options *server.Options
	ctx     context.Context
	runing  bool

	sync.RWMutex
	wg sync.WaitGroup

	tracer trace.Tracer
}

// New NATs server
func NewServer(cfg *Config, opts ...server.Option) server.Server {
	ctx := context.Background()
	srv := &NatsServer{
		config:  cfg,
		ctx:     ctx,
		runing:  false,
		options: server.NewOptions(),
	}

	for _, opt := range opts {
		opt(srv.options)
	}

	// Set logger
	if srv.options.Logger() == nil {
		server.Logger(logger.Logger)(srv.options)
	}

	// Set global context
	if srv.options.Context() != nil {
		srv.ctx = srv.options.Context()
	} else {
		server.Context(ctx)(srv.options)
	}

	// Set tracer
	if srv.options.TraceProvider() != nil {
		srv.tracer = srv.options.TraceProvider().Tracer(srv.Name() + "@" + srv.String())
	}

	return srv
}

func (srv *NatsServer) Options() *server.Options {
	return srv.options
}

func (srv *NatsServer) Start() error {
	srv.Lock()
	defer srv.Unlock()

	if srv.runing {
		// Runing
		return nil
	}

	srv.wg.Add(1)
	go func() error {
		srv.wg.Done()

		return nil
	}()

	srv.runing = true

	return nil
}

func (srv *NatsServer) Stop() error {
	srv.Lock()
	defer srv.Unlock()

	if !srv.runing {
		// Not runing
		return nil
	}

	srv.wg.Wait()
	srv.runing = false

	return nil
}

func (srv *NatsServer) String() string {
	return "nats"
}

func (srv *NatsServer) Name() string {
	return srv.config.Name
}

func (srv *NatsServer) ID() string {
	return srv.options.ID()
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
