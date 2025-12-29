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
	"time"

	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/utils"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/google/uuid"
)

const (
	ControlDeadline = 5 * time.Second
)

// WebsocketServer : Server definition
type WebsocketServer struct {
	config        *Config
	ctx           context.Context
	options       *server.Options
	app           *fiber.App
	running       bool
	addr          net.Addr
	advertiseAddr net.Addr
	metadata      utils.Metadata
	handlers      []Handler
	// pool          *Pool

	sync.RWMutex
	wg sync.WaitGroup
}

// New Websocket server
func New(opts *server.Options, cfg *Config) *WebsocketServer {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	var (
		addr          net.Addr
		advertiseAddr net.Addr
		err           error
	)

	// TCP default
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
				"Advertise address resolve failed",
				"string", cfg.AdvertiseAddress,
				"error", err.Error(),
			)
		}
	} else {
		advertiseAddr = addr
	}

	srv := &WebsocketServer{
		config:        cfg,
		ctx:           opts.Context,
		addr:          addr,
		advertiseAddr: advertiseAddr,
		running:       false,
		options:       opts,
		metadata:      utils.NewMetadata(),
		// pool:          NewPool(cfg.PingDuration, cfg.MaxIdleDuration),
		handlers: make([]Handler, 0),
	}

	app := fiber.New(
		fiber.Config{
			Prefork:               false,
			DisableStartupMessage: true,
			ServerHeader:          opts.Name,
			AppName:               opts.Name,
			Network:               cfg.Network,
		},
	)
	app.Use(recover.New(recover.ConfigDefault))
	srv.app = app
	srv.options.Logger.InfoContext(
		srv.ctx,
		"Websocket server created",
		"server", srv.String(),
		"id", srv.options.ID,
		"name", srv.options.Name,
		"addr", addr.String(),
		"path", cfg.Path,
	)

	app.Use(cfg.Path, func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)

			return c.Next()
		}

		return fiber.ErrUpgradeRequired
	})
	app.Get(cfg.Path, websocket.New(srv.operator))
	server.Set(srv)

	// Generate pool
	if SessionPool == nil {
		SessionPool = NewPool(cfg.PingDuration, cfg.MaxIdleDuration)
	}

	return srv
}

func (srv *WebsocketServer) Context() context.Context {
	return srv.ctx
}

func (srv *WebsocketServer) Options() *server.Options {
	return srv.options
}

func (srv *WebsocketServer) String() string {
	return "websocket"
}

func (srv *WebsocketServer) ID() uuid.UUID {
	return srv.options.ID
}

func (srv *WebsocketServer) Name() string {
	return srv.options.Name
}

func (srv *WebsocketServer) Start() error {
	var (
		listener net.Listener
		cert     tls.Certificate
		err      error
	)

	srv.Lock()
	defer srv.Unlock()

	if srv.running {
		// running
		return nil
	}

	// Try TLS first
	if srv.config.TLSCertPEM != "" && srv.config.TLSKeyPEM != "" {
		cert, err = tls.X509KeyPair([]byte(srv.config.TLSCertPEM), []byte(srv.config.TLSKeyPEM))
		if err != nil {
			srv.options.Logger.ErrorContext(
				srv.ctx,
				"TLS certification failed",
				"server", srv.String(),
				"id", srv.options.ID,
				"name", srv.options.Name,
				"error", err.Error(),
			)
		}

		listener, err = tls.Listen(
			srv.addr.Network(),
			srv.addr.String(),
			&tls.Config{
				MinVersion:   tls.VersionTLS12,
				Certificates: []tls.Certificate{cert},
			},
		)
		if err != nil {
			srv.options.Logger.ErrorContext(
				srv.ctx,
				"Network listen with TLS certificate failed",
				"server", srv.String(),
				"id", srv.options.ID,
				"name", srv.options.Name,
				"error", err.Error(),
			)

			return err
		}
	} else {
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
	}

	srv.addr = listener.Addr()
	srv.metadata.Set("server", srv.String())
	srv.metadata.Set("network", srv.addr.Network())
	srv.metadata.Set("address", srv.addr.String())
	srv.metadata.Set("advertise_address", srv.advertiseAddr.String())
	srv.metadata.Set("name", srv.options.Name)
	srv.metadata.Set("id", srv.options.ID.String())
	srv.wg.Add(1)
	go func() error {
		err := srv.app.Listener(listener)
		if err != nil {
			srv.options.Logger.ErrorContext(
				srv.ctx, "Websocket server listen failed",
				"server", srv.String(),
				"id", srv.options.ID,
				"name", srv.options.Name,
				"error", err.Error(),
			)

			return err
		}

		srv.options.Logger.InfoContext(
			srv.ctx,
			"Websocket server closed",
			"server", srv.String(),
			"id", srv.options.ID,
			"name", srv.options.Name,
		)
		srv.wg.Done()

		return nil
	}()

	srv.options.Logger.InfoContext(
		srv.ctx,
		"Websocket server listened",
		"server", srv.String(),
		"id", srv.options.ID,
		"name", srv.options.Name,
		"addr", srv.addr.String(),
		"path", srv.config.Path,
	)
	srv.running = true

	return nil
}

func (srv *WebsocketServer) Stop() error {
	srv.Lock()
	defer srv.Unlock()

	if !srv.running {
		// Not running
		return nil
	}

	srv.app.Server().Shutdown()
	srv.wg.Wait()
	srv.options.Logger.InfoContext(
		srv.ctx,
		"Websocket server shutdown",
		"server", srv.String(),
		"id", srv.options.ID,
		"name", srv.options.Name,
		"addr", srv.addr.String(),
	)
	srv.running = false

	return nil
}

func (srv *WebsocketServer) Running() bool {
	return srv.running
}

func (srv *WebsocketServer) Addr() net.Addr {
	return srv.addr
}

func (srv *WebsocketServer) IP() net.IP {
	try := utils.AddrToIP(srv.addr)
	if try == nil || try.IsUnspecified() {
		try, _ = utils.ObtainPreferIP(true)
	}

	return try
}

func (srv *WebsocketServer) Port() int {
	return utils.AddrToPort(srv.addr)
}

func (srv *WebsocketServer) AdvertiseAddr() net.Addr {
	return srv.advertiseAddr
}

func (srv *WebsocketServer) AdvertiseIP() net.IP {
	try := utils.AddrToIP(srv.advertiseAddr)
	if try == nil || try.IsUnspecified() {
		try, _ = utils.ObtainPreferIP(true)
	}

	return try
}

func (srv *WebsocketServer) AdvertisePort() int {
	return utils.AddrToPort(srv.advertiseAddr)
}

func (srv *WebsocketServer) Metadata() utils.Metadata {
	return srv.metadata
}

func (srv *WebsocketServer) App() *fiber.App {
	return srv.app
}

func (srv *WebsocketServer) Handle(hdls ...Handler) {
	for _, hdl := range hdls {
		srv.handlers = append(srv.handlers, hdl)
		srv.options.Logger.DebugContext(
			srv.ctx,
			"Websocket handler registered",
			"server", srv.String(),
			"id", srv.options.ID,
			"name", srv.options.Name,
			"handler", hdl.Name(),
		)
	}
}

func (srv *WebsocketServer) operator(c *websocket.Conn) {
	var (
		mt   int
		body []byte
		err  error
	)

	// OnConnect
	srv.options.Logger.DebugContext(
		srv.ctx,
		"Websocket client established",
		"server", srv.String(),
		"id", srv.options.ID,
		"name", srv.options.Name,
		"client", c.RemoteAddr().String(),
	)

	sess := NewSession(c)
	if SessionPool != nil {
		SessionPool.Put(sess)
	}

	for _, hdl := range srv.handlers {
		err = hdl.OnConnect(sess)
		if err != nil {
			srv.options.Logger.ErrorContext(
				srv.ctx,
				"Websocket connect error",
				"server", srv.String(),
				"id", srv.options.ID,
				"name", srv.options.Name,
				"client", c.RemoteAddr().String(),
				"error", err.Error(),
			)
		}
	}

read:
	for {
		mt, body, err = c.ReadMessage()
		if err != nil {
			// Read error
			for _, hdl := range srv.handlers {
				hdl.OnError(sess, err)
			}

			break read
		} else {
			switch mt {
			case websocket.TextMessage, websocket.BinaryMessage:
				sess.LastActive = time.Now()
				srv.options.Logger.DebugContext(
					srv.ctx,
					"Websocket data received",
					"server", srv.String(),
					"id", srv.options.ID,
					"name", srv.options.Name,
					"client", c.RemoteAddr().String(),
					"message_type", mt,
					"message_length", len(body),
				)

				// OnData
				for _, hdl := range srv.handlers {
					err = hdl.OnData(sess, mt, body)
					if err != nil {
						srv.options.Logger.ErrorContext(
							srv.ctx,
							"Websocket data process error",
							"server", srv.String(),
							"id", srv.options.ID,
							"name", srv.options.Name,
							"client", c.RemoteAddr().String(),
							"error", err.Error(),
						)
					}
				}
			case websocket.PongMessage:
				// Ignore typo
				sess.LastActive = time.Now()
			case websocket.CloseMessage:
				// Close
				break read
			default:
				// Unnown
			}
		}
	}

	// Close
	err = sess.Close()
	if err != nil {
		srv.options.Logger.ErrorContext(
			srv.ctx,
			"Websocket connection close failed",
			"server", srv.String(),
			"id", srv.options.ID,
			"name", srv.options.Name,
			"client", c.RemoteAddr().String(),
			"session", sess.ID,
			"error", err.Error(),
		)
	} else {
		srv.options.Logger.DebugContext(
			srv.ctx,
			"Websocket connection closed",
			"server", srv.String(),
			"id", srv.options.ID,
			"name", srv.options.Name,
			"client", c.RemoteAddr().String(),
			"session", sess.ID,
		)
	}

	for _, hdl := range srv.handlers {
		err = hdl.OnClose(sess)
		if err != nil {
			srv.options.Logger.ErrorContext(
				srv.ctx,
				"Websocket close error",
				"server", srv.String(),
				"id", srv.options.ID,
				"name", srv.options.Name,
				"client", c.RemoteAddr().String(),
				"session", sess.ID,
				"error", err.Error(),
			)
		}
	}
}

/* {{{ [Handler] */
type Handler interface {
	Name() string
	Type() string
	OnConnect(*Session) error
	OnClose(*Session) error
	OnError(*Session, error) error
	OnData(*Session, int, []byte) error
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
