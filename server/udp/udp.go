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
	"errors"
	"net"
	"sync"
	"time"

	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/utils"
	"github.com/google/uuid"
)

/* {{{ [Server] */

// UDPServer : Server definition
type UDPServer struct {
	config        *Config
	ctx           context.Context
	options       *server.Options
	running       bool
	addr          net.Addr
	advertiseAddr net.Addr
	conn          *net.UDPConn
	metadata      utils.Metadata
	handlers      []Handler
	pool          *Pool

	sync.RWMutex
	wg sync.WaitGroup
}

// New UDP server
func New(opts *server.Options, cfg *Config) *UDPServer {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	var (
		addr          net.Addr
		advertiseAddr net.Addr
		err           error
	)

	addr, err = net.ResolveUDPAddr(cfg.Network, cfg.Address)
	if err != nil {
		opts.Logger.Fatal(
			"Network address resolve failed",
			"string", cfg.Address,
			"error", err.Error(),
		)
	}

	if cfg.AdvertiseAddress != "" {
		advertiseAddr, err = net.ResolveUDPAddr(cfg.Network, cfg.AdvertiseAddress)
		if err != nil {
			opts.Logger.Fatal(
				"Advertise network address resolve failed",
				"string", cfg.AdvertiseAddress,
				"error", err.Error(),
			)
		}
	} else {
		advertiseAddr = addr
	}

	srv := &UDPServer{
		config:        cfg,
		ctx:           context.Background(),
		addr:          addr,
		advertiseAddr: advertiseAddr,
		running:       false,
		options:       opts,
		metadata:      utils.NewMetadata(),
		pool:          NewPool(cfg.MaxIdleDuration),
		handlers:      make([]Handler, 0),
	}

	srv.options.Logger.InfoContext(
		srv.ctx,
		"UDP server created",
		"server", srv.String(),
		"id", srv.options.ID,
		"name", srv.options.Name,
		"network", addr.Network(),
		"address", addr.String(),
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
		err error
	)
	srv.Lock()
	defer srv.Unlock()

	if srv.running {
		// running
		return nil
	}

	srv.metadata.Set("server", srv.String())
	srv.metadata.Set("network", srv.addr.Network())
	srv.metadata.Set("address", srv.addr.String())
	srv.metadata.Set("advertise_address", srv.advertiseAddr.String())
	srv.metadata.Set("name", srv.options.Name)
	srv.metadata.Set("id", srv.options.ID.String())
	srv.wg.Add(1)
	c, ok := srv.addr.(*net.UDPAddr)
	if !ok {
		srv.options.Logger.ErrorContext(
			srv.ctx,
			"Obtain UDP address failed",
			"server", srv.String(),
			"id", srv.options.ID,
			"name", srv.options.Name,
			"network", srv.addr.Network(),
			"address", srv.addr.String(),
		)

		return errors.New("obtain UDP address failed")
	}

	srv.conn, err = net.ListenUDP(
		srv.addr.Network(),
		c,
	)
	if err != nil {
		srv.options.Logger.ErrorContext(
			srv.ctx,
			"Network listen failed",
			"server", srv.String(),
			"id", srv.options.ID,
			"name", srv.options.Name,
			"network", srv.addr.Network(),
			"address", srv.addr.String(),
			"error", err.Error(),
		)

		return err
	}

	go func() error {
		buff := make([]byte, srv.config.BufferSize)
		for {
			n, addr, err := srv.conn.ReadFromUDP(buff)
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					// Network closed
					srv.options.Logger.InfoContext(
						srv.ctx,
						"UDP connection closed",
						"server", srv.String(),
						"id", srv.options.ID,
						"name", srv.options.Name,
						"network", srv.addr.Network(),
						"address", srv.addr.String(),
					)
				} else {
					srv.options.Logger.ErrorContext(
						srv.ctx,
						"UDP ReadFromUDP failed",
						"server", srv.String(),
						"id", srv.options.ID,
						"name", srv.options.Name,
						"network", srv.addr.Network(),
						"address", srv.addr.String(),
						"error", err.Error(),
					)
				}

				// TODO : Exit read ???
				break
			} else if n >= 0 {
				sess := srv.pool.GetByAddr(addr)
				if sess == nil {
					sess = NewSession(srv.conn, addr)
					srv.pool.Put(sess)
					for _, hdl := range srv.handlers {
						hdl.OnConnect(sess)
					}
				}

				sess.LastActive = time.Now()

				dst := make([]byte, n)
				copy(dst, buff)
				for _, hdl := range srv.handlers {
					hdl.OnData(sess, dst)
				}
			}
		}

		srv.wg.Done()

		return nil
	}()

	srv.options.Logger.InfoContext(
		srv.ctx,
		"UDP server listened",
		"server", srv.String(),
		"id", srv.options.ID,
		"name", srv.options.Name,
		"network", srv.addr.Network(),
		"address", srv.addr.String(),
	)
	srv.running = true

	return nil
}

func (srv *UDPServer) Stop() error {
	srv.Lock()
	defer srv.Unlock()

	if !srv.running {
		// Not running
		return nil
	}

	err := srv.conn.Close()
	if err != nil {
		srv.options.Logger.ErrorContext(
			srv.ctx,
			"Network close failed",
			"server", srv.String(),
			"id", srv.options.ID,
			"name", srv.options.Name,
			"network", srv.addr.Network(),
			"address", srv.addr.String(),
			"error", err.Error(),
		)

		return err
	}

	srv.wg.Wait()
	srv.options.Logger.InfoContext(
		srv.ctx,
		"UDP server shutdown",
		"server", srv.String(),
		"id", srv.options.ID,
		"name", srv.options.Name,
		"network", srv.addr.Network(),
		"address", srv.addr.String(),
	)
	srv.running = false

	return nil
}

func (srv *UDPServer) Running() bool {
	return srv.running
}

func (srv *UDPServer) Addr() net.Addr {
	return srv.addr
}

func (srv *UDPServer) IP() net.IP {
	try := utils.AddrToIP(srv.addr)
	if try == nil || try.IsUnspecified() {
		try, _ = utils.ObtainPreferIP(true)
	}

	return try
}

func (srv *UDPServer) Port() int {
	return utils.AddrToPort(srv.addr)
}

func (srv *UDPServer) AdvertiseAddr() net.Addr {
	return srv.advertiseAddr
}

func (srv *UDPServer) AdvertiseIP() net.IP {
	try := utils.AddrToIP(srv.advertiseAddr)
	if try == nil || try.IsUnspecified() {
		try, _ = utils.ObtainPreferIP(true)
	}

	return try
}

func (srv *UDPServer) AdvertisePort() int {
	return utils.AddrToPort(srv.advertiseAddr)
}

func (srv *UDPServer) Metadata() utils.Metadata {
	return srv.metadata
}

func (srv *UDPServer) App() *net.UDPConn {
	return srv.conn
}

func (srv *UDPServer) Handle(hdls ...Handler) {
	for _, hdl := range hdls {
		srv.handlers = append(srv.handlers, hdl)
		srv.options.Logger.DebugContext(
			srv.ctx,
			"UDP handler registered",
			"server", srv.String(),
			"id", srv.options.ID,
			"name", srv.options.Name,
			"handler", hdl.Name(),
		)
	}
}

func (srv *UDPServer) Send(c *net.UDPAddr, data []byte) error {
	return nil
}

/* }}} */

/* {{{ [Handler] */
type Handler interface {
	Name() string
	Type() string
	OnConnect(*Session) error
	OnData(*Session, []byte) error
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
