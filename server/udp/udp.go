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
 * @file udp.go
 * @package udp
 * @author Dr.NP <np@herewe.tech>
 * @since 09/17/2024
 */

package udp

import (
	"context"
	"net"
	"sync"

	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/utils"
	"github.com/google/uuid"
)

/* {{{ [Server] */

// UDPServer : Server definition
type UDPServer struct {
	config   *Config
	ctx      context.Context
	options  *server.Options
	runing   bool
	addr     net.Addr
	metadata utils.Metadata

	sync.RWMutex
	wg sync.WaitGroup
}

// New UDP server
func New(opts *server.Options, cfg *Config) *UDPServer {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	addr, _ := net.ResolveUDPAddr(cfg.Network, cfg.Addr)
	srv := &UDPServer{
		config:   cfg,
		ctx:      context.Background(),
		addr:     addr,
		runing:   false,
		options:  opts,
		metadata: utils.NewMetadata(),
	}

	srv.options.Logger.InfoContext(
		srv.ctx,
		"Server created",
		"server", srv.addr.String(),
		"id", srv.options.ID,
		"name", srv.options.Name,
		"addr", addr.String(),
	)

	server.Instance(opts.ID, srv)

	return srv
}

func (srv *UDPServer) Context() context.Context {
	return srv.ctx
}

func (srv *UDPServer) Options() *server.Options {
	return srv.options
}

func (srv *UDPServer) String() string {
	return "udp"
}

func (srv *UDPServer) ID() uuid.UUID {
	return srv.options.ID
}

func (srv *UDPServer) Name() string {
	return srv.options.Name
}

func (srv *UDPServer) Start() error {
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

	listener, err = net.Listen(
		srv.addr.Network(),
		srv.addr.String(),
	)

	if err != nil {
		srv.options.Logger.ErrorContext(
			srv.ctx,
			"Network listen failed",
			"server", srv.String(),
			"id", srv.options.ID,
			"name", srv.options.Name,
			"error", err.Error(),
		)

		return err
	}

	srv.addr = listener.Addr()
	srv.metadata.Set("server", srv.String())
	srv.metadata.Set("network", srv.addr.Network())
	srv.metadata.Set("address", srv.addr.String())
	srv.metadata.Set("name", srv.options.Name)
	srv.metadata.Set("id", srv.options.ID.String())
	srv.wg.Add(1)
	go func() error {
		srv.wg.Done()

		return nil
	}()

	srv.options.Logger.InfoContext(
		srv.ctx,
		"Server listened",
		"server", srv.String(),
		"id", srv.options.ID,
		"name", srv.options.Name,
		"addr", srv.addr.String(),
	)
	srv.runing = true

	return nil
}

func (srv *UDPServer) Stop() error {
	srv.Lock()
	defer srv.Unlock()

	if srv.runing {
		// Not runing
		return nil
	}

	srv.wg.Wait()
	srv.runing = false

	return nil
}

func (srv *UDPServer) Runing() bool {
	return srv.runing
}

func (srv *UDPServer) Addr() net.Addr {
	return srv.addr
}

func (srv *UDPServer) IP() net.IP {
	try := utils.AddrToIP(srv.addr)
	if try.IsUnspecified() {
		try, _ = utils.ObtainPreferIP(true)
	}

	return try
}

func (srv *UDPServer) Port() int {
	return utils.AddrToPort(srv.addr)
}

func (srv *UDPServer) Metadata() utils.Metadata {
	return srv.metadata
}

func (srv *UDPServer) Handle(hdls ...Handler) {
	for _, hdl := range hdls {
		hdl.Register()
		srv.options.Logger.InfoContext(
			srv.ctx,
			"UDP handler registered",
			"server", srv.String(),
			"id", srv.options.ID,
			"name", srv.options.Name,
			"handler", hdl.Name(),
		)
	}
}

/* }}} */

/* {{{ [Handler] */
type Handler interface {
	Name() string
	Type() string
	Register()
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
