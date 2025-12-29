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
 * @file tcp.go
 * @package tcp
 * @author Dr.NP <np@herewe.tech>
 * @since 01/17/2025
 */

package tcp

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/utils"
	"github.com/google/uuid"
)

type TCPServer struct {
	config        *Config
	ctx           context.Context
	options       *server.Options
	running       bool
	addr          net.Addr
	advertiseAddr net.Addr
	conn          net.Listener
	metadata      utils.Metadata
	handlers      []Handler
	pool          *Pool

	sync.RWMutex
	wg sync.WaitGroup
}

func New(opts *server.Options, cfg *Config) *TCPServer {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	var (
		addr          net.Addr
		advertiseAddr net.Addr
		err           error
	)

	addr, err = net.ResolveTCPAddr(cfg.Network, cfg.Address)
	if err != nil {
		opts.Logger.Fatal(
			"Network address resolve failed",
			"string", cfg.Address,
			"error", err.Error(),
		)
	}

	if cfg.AdvertiseAddress != "" {
		advertiseAddr, err = net.ResolveTCPAddr(cfg.Network, cfg.AdvertiseAddress)
		if err != nil {
			opts.Logger.Fatal(
				"Network address resolve failed",
				"string", cfg.AdvertiseAddress,
				"error", err.Error(),
			)
		}
	} else {
		advertiseAddr = addr
	}

	srv := &TCPServer{
		config:        cfg,
		ctx:           opts.Context,
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
		"TCP server created",
		"server", srv.String(),
		"id", srv.options.ID,
		"name", srv.options.Name,
		"network", addr.Network(),
		"address", addr.String(),
	)

	server.Set(srv)

	return srv
}

func (srv *TCPServer) Context() context.Context {
	return srv.ctx
}

func (srv *TCPServer) Options() *server.Options {
	return srv.options
}

func (srv *TCPServer) String() string {
	return "tcp"
}

func (srv *TCPServer) ID() uuid.UUID {
	return srv.options.ID
}

func (srv *TCPServer) Name() string {
	return srv.options.Name
}

func (srv *TCPServer) Running() bool {
	return srv.running
}

func (srv *TCPServer) Addr() net.Addr {
	return srv.addr
}

func (srv *TCPServer) IP() net.IP {
	try := utils.AddrToIP(srv.addr)
	if try == nil || try.IsUnspecified() {
		try, _ = utils.ObtainPreferIP(true)
	}

	return try
}

func (srv *TCPServer) Port() int {
	return utils.AddrToPort(srv.addr)
}

func (srv *TCPServer) AdvertiseAddr() net.Addr {
	return srv.advertiseAddr
}

func (srv *TCPServer) AdvertiseIP() net.IP {
	try := utils.AddrToIP(srv.advertiseAddr)
	if try == nil || try.IsUnspecified() {
		try, _ = utils.ObtainPreferIP(true)
	}

	return try
}

func (srv *TCPServer) AdvertisePort() int {
	return utils.AddrToPort(srv.advertiseAddr)
}

func (srv *TCPServer) Metadata() utils.Metadata {
	return srv.metadata
}

func (srv *TCPServer) App() net.Listener {
	return srv.conn
}

func (srv *TCPServer) Handle(hdls ...Handler) {
	for _, hdl := range hdls {
		srv.handlers = append(srv.handlers, hdl)
		srv.options.Logger.DebugContext(
			srv.ctx,
			"TCP handler registered",
			"server", srv.String(),
			"id", srv.options.ID,
			"name", srv.options.Name,
			"handler", hdl.Name(),
		)
	}
}

func (srv *TCPServer) Send(c net.Conn, data []byte) error {
	_, err := c.Write(data)

	return err
}

func (srv *TCPServer) Start() error {
	var err error

	srv.Lock()
	defer srv.Unlock()

	if srv.running {
		return nil
	}

	srv.metadata.Set("server", srv.String())
	srv.metadata.Set("network", srv.addr.Network())
	srv.metadata.Set("address", srv.addr.String())
	srv.metadata.Set("advertise_address", srv.advertiseAddr.String())
	srv.metadata.Set("name", srv.options.Name)
	srv.metadata.Set("id", srv.options.ID.String())
	srv.wg.Add(1)

	srv.conn, err = net.Listen(srv.addr.Network(), srv.addr.String())
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

	srv.addr = srv.conn.Addr()
	go func() error {
		for {
			client, err := srv.conn.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					// Network closed
					srv.options.Logger.InfoContext(
						srv.ctx,
						"TCP connection closed",
						"server", srv.String(),
						"id", srv.options.ID,
						"name", srv.options.Name,
						"network", srv.addr.Network(),
						"address", srv.addr.String(),
					)
				} else {
					srv.options.Logger.ErrorContext(
						srv.ctx,
						"TCP Accept failed",
						"server", srv.String(),
						"id", srv.options.ID,
						"name", srv.options.Name,
						"network", srv.addr.Network(),
						"address", srv.addr.String(),
						"error", err.Error(),
					)
				}

				// TODO : Exit accept
				break
			}

			sess := NewSession(client)
			srv.pool.Put(sess)
			for _, hdl := range srv.handlers {
				hdl.OnConnect(sess)
			}

			go func(c net.Conn) {
				buff := make([]byte, srv.config.BufferSize)
				reader := bufio.NewReader(c)
			read:
				for {
					n, err := reader.Read(buff)
					if err != nil {
						if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) || errors.Is(err, net.ErrClosed) {
							break read
						} else {
							srv.options.Logger.ErrorContext(
								srv.ctx,
								"TCP Read error",
								"server", srv.String(),
								"id", srv.options.ID,
								"name", srv.options.Name,
								"network", srv.addr.Network(),
								"address", srv.addr.String(),
								"remote", c.RemoteAddr().String(),
								"error", err.Error(),
							)
							for _, hdl := range srv.handlers {
								hdl.OnError(sess, err)
							}
						}
					} else {
						sess.LastActive = time.Now()
						if n > 0 {
							dst := make([]byte, n)
							copy(dst, buff)
							for _, hdl := range srv.handlers {
								err = hdl.OnData(sess, dst)
								if err != nil {
									srv.options.Logger.ErrorContext(
										srv.ctx,
										"TCP data process error",
										"server", srv.String(),
										"id", srv.options.ID,
										"name", srv.options.Name,
										"network", srv.addr.Network(),
										"address", srv.addr.String(),
										"remote", c.RemoteAddr().String(),
										"error", err.Error(),
									)
								}
							}
						}
					}
				}

				for _, hdl := range srv.handlers {
					err = hdl.OnClose(sess)
					if err != nil {
						srv.options.Logger.ErrorContext(
							srv.ctx,
							"TCP close process error",
							"server", srv.String(),
							"id", srv.options.ID,
							"name", srv.options.Name,
							"network", srv.addr.Network(),
							"address", srv.addr.String(),
							"remote", c.RemoteAddr().String(),
							"error", err.Error(),
						)
					}
				}
			}(client)
		}

		srv.wg.Done()

		return nil
	}()

	srv.options.Logger.InfoContext(
		srv.ctx,
		"TCP server listened",
		"server", srv.String(),
		"id", srv.options.ID,
		"name", srv.options.Name,
		"network", srv.addr.Network(),
		"address", srv.addr.String(),
	)
	srv.running = true

	return nil
}

func (srv *TCPServer) Stop() error {
	srv.Lock()
	defer srv.Unlock()

	if !srv.running {
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
		"TCP server shutdown",
		"server", srv.String(),
		"id", srv.options.ID,
		"name", srv.options.Name,
		"network", srv.addr.Network(),
		"address", srv.addr.String(),
	)
	srv.running = false

	return nil
}

/* {{{ [Handler] */
type Handler interface {
	Name() string
	Type() string
	OnConnect(*Session) error
	OnClose(*Session) error
	OnError(*Session, error) error
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
