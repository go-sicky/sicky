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
 * @file websocket.go
 * @package websocket
 * @author Dr.NP <np@herewe.tech>
 * @since 11/20/2023
 */

package websocket

import (
	"context"
	"crypto/tls"
	"net"
	"sync"

	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/server"
	"github.com/google/uuid"
	"google.golang.org/grpc"
)

// WebsocketServer : Server definition
type WebsocketServer struct {
	ctx     context.Context
	runing  bool
	logger  logger.GeneralLogger
	options *server.Options

	sync.RWMutex
	wg sync.WaitGroup
}

// New Websocket server
func NewServer(cfg *Config, opts ...server.Option) server.Server {
	ctx := context.Background()
	logger := logger.Logger
	// TCP default
	addr, _ := net.ResolveTCPAddr(cfg.Network, cfg.Addr)
	srv := &WebsocketServer{
		ctx:    ctx,
		runing: false,
		logger: logger,
		options: &server.Options{
			Name: cfg.Name,
			Addr: addr,
		},
	}

	for _, opt := range opts {
		opt(srv.options)
	}

	// Set logger
	if srv.options.Logger != nil {
		srv.logger = srv.options.Logger
	} else {
		srv.options.Logger = logger
	}

	// Set global context
	if srv.options.Context != nil {
		srv.ctx = srv.options.Context
	} else {
		srv.options.Context = ctx
	}

	// Set ID
	srv.options.ID = uuid.New().String()

	return srv
}

func (srv *WebsocketServer) Options() *server.Options {
	return srv.options
}

func (srv *WebsocketServer) Handle(*server.Handler) error {
	return nil
}

func (srv *WebsocketServer) Start() error {
	var (
		listener net.Listener
		err      error
	)

	srv.Lock()
	defer srv.Unlock()

	if srv.runing {
		// Runing
		return nil
	}

	if srv.options.TLS != nil {
		listener, err = tls.Listen(
			srv.options.Addr.Network(),
			srv.options.Addr.String(),
			srv.options.TLS,
		)

		if err != nil {
			srv.logger.Error("HTTP server with TLS listen failed", "error", err.Error())

			return err
		}
	} else {
		listener, err = net.Listen(
			srv.options.Addr.Network(),
			srv.options.Addr.String(),
		)

		if err != nil {
			srv.logger.Error("HTTP server listen failed", "error", err.Error())

			return err
		}
	}

	srv.options.Addr = listener.Addr()
	srv.wg.Add(1)
	go func() error {
		srv.wg.Done()

		return nil
	}()

	srv.runing = true

	return nil
}

func (srv *WebsocketServer) Stop() error {
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

func (srv *WebsocketServer) String() string {
	return "websocket"
}

func (srv *WebsocketServer) Name() string {
	return srv.options.Name
}

func (srv *WebsocketServer) RegisterService(desc *grpc.ServiceDesc, impl any) {
	// Did nothing
}

func (srv *WebsocketServer) RegisterHandler(hdl server.HandlerHTTP) {
	// Did nothing
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
