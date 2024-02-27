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
	"time"

	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/server"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"go.opentelemetry.io/otel/trace"
)

const (
	ControlDeadline = 5 * time.Second
)

// WebsocketServer : Server definition
type WebsocketServer struct {
	config  *Config
	options *server.Options
	ctx     context.Context
	app     *fiber.App
	runing  bool
	addr    net.Addr

	sync.RWMutex
	wg sync.WaitGroup

	tracer trace.Tracer
}

var (
	servers = make(map[string]*WebsocketServer, 0)
)

func Instance(name string, clt ...*WebsocketServer) *WebsocketServer {
	if len(clt) > 0 {
		// Set value
		servers[name] = clt[0]

		return clt[0]
	}

	return servers[name]
}

// New Websocket server
func NewServer(cfg *Config, opts ...server.Option) *WebsocketServer {
	ctx := context.Background()
	// TCP default
	addr, _ := net.ResolveTCPAddr(cfg.Network, cfg.Addr)
	srv := &WebsocketServer{
		config:  cfg,
		ctx:     ctx,
		addr:    addr,
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

	app := fiber.New(
		fiber.Config{
			Prefork:               false,
			DisableStartupMessage: true,
			ServerHeader:          cfg.Name,
			AppName:               cfg.Name,
			Network:               cfg.Network,
		},
	)
	app.Use(recover.New(recover.ConfigDefault))
	srv.app = app

	server.Instance(srv.Name(), srv)
	Instance(srv.Name(), srv)
	srv.options.Logger().InfoContext(srv.ctx, "Websocket server created", "id", srv.ID(), "name", srv.Name(), "addr", addr.String())

	return srv
}

func (srv *WebsocketServer) Options() *server.Options {
	return srv.options
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

	if srv.options.TLS() != nil {
		listener, err = tls.Listen(
			srv.addr.Network(),
			srv.addr.String(),
			srv.options.TLS(),
		)

		if err != nil {
			srv.options.Logger().ErrorContext(srv.ctx, "Websocket server with TLS listen failed", "error", err.Error())

			return err
		}
	} else {
		listener, err = net.Listen(
			srv.addr.Network(),
			srv.addr.String(),
		)

		if err != nil {
			srv.options.Logger().ErrorContext(srv.ctx, "Websocket server listen failed", "error", err.Error())

			return err
		}
	}

	srv.addr = listener.Addr()
	srv.wg.Add(1)
	go func() error {
		err := srv.app.Listener(listener)
		if err != nil {
			srv.options.Logger().ErrorContext(srv.ctx, "HTTP server listen failed", "error", err.Error())

			return err
		}

		srv.options.Logger().InfoContext(
			srv.ctx,
			"HTTP server closed",
			"id", srv.options.ID(),
			"server", srv.config.Name,
		)
		srv.wg.Done()

		return nil
	}()

	srv.options.Logger().InfoContext(
		srv.ctx,
		"HTTP server listened",
		"id", srv.options.ID(),
		"server", srv.config.Name,
		"addr", srv.addr.String(),
	)
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

	srv.app.Server().Shutdown()
	srv.wg.Wait()
	srv.runing = false

	return nil
}

func (srv *WebsocketServer) String() string {
	return "websocket"
}

func (srv *WebsocketServer) Name() string {
	return srv.config.Name
}

func (srv *WebsocketServer) ID() string {
	return srv.options.ID()
}

func (srv *WebsocketServer) App() *fiber.App {
	return srv.app
}

func (srv *WebsocketServer) Handle(hdl WebsocketHandler) {
	srv.app.Use(hdl.Path(), func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)

			return c.Next()
		}

		return fiber.ErrUpgradeRequired
	})

	srv.app.Get(hdl.Path()+"/:tag", websocket.New(func(c *websocket.Conn) {
		var (
			tag  = c.Params("tag")
			mt   int
			body []byte
			err  error
		)

		// On connect
		srv.options.Logger().Debug("websocket client established", "tag", tag)
		NewSession(tag, c)
		hdl.OnConnect(tag)
	read:
		for {
			mt, body, err = c.ReadMessage()
			if err != nil {
				// On error
				srv.options.Logger().Warn("websocket read error", "tag", tag, "error", err.Error())
				hdl.OnError(tag, err)
				break read
			} else {
				// Data
				switch mt {
				case websocket.TextMessage, websocket.BinaryMessage:
					hdl.OnData(tag, body)
				case websocket.PingMessage:
					//c.WriteControl(websocket.PongMessage, nil, time.Now().Add(ControlDeadline))
					break
				case websocket.PongMessage:
					// Ignore
					break
				case websocket.CloseMessage:
					srv.options.Logger().Info("websocket close message from tag", "tag", tag)
					//c.Close()
					break read
				default:
					// Unknown
					srv.options.Logger().Warn("unsupportted websocket data type", "type", mt)
				}
			}
		}

		// On close
		srv.options.Logger().Debug("websocket client closed", "tag", tag)
		DeleteSession(tag)
		hdl.OnClose(tag)
	}))
}

/* {{{ [Handler] */
type WebsocketHandler interface {
	Name() string
	Type() string
	Path() string
	OnConnect(string) error
	OnClose(string) error
	OnError(string, error) error
	OnData(string, []byte) error
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
