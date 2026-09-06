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
	"errors"
	"fmt"
	"net"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/contrib/v3/websocket"
	"github.com/gofiber/fiber/v3"
	recovermiddleware "github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/google/uuid"

	"github.com/go-sicky/sicky/metrics"
	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/utils"
)

const (
	// ControlDeadline is a websocket constant.
	ControlDeadline = 5 * time.Second
)

// WebsocketServer : Server definition.
type WebsocketServer struct {
	config        *Config
	ctx           context.Context
	options       *server.Options
	app           *fiber.App
	listener      net.Listener
	running       bool
	addr          net.Addr
	advertiseAddr net.Addr
	metadata      utils.Metadata
	handlers      atomic.Pointer[[]Handler]
	reaperDone    chan struct{}

	sync.RWMutex
	wg sync.WaitGroup
}

// New Websocket server.
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
			"network address resolve failed",
			"string", cfg.Address,
			"error", err.Error(),
		)

		return nil
	}

	if cfg.AdvertiseAddress != "" {
		advertiseAddr, err = net.ResolveTCPAddr(cfg.Network, cfg.AdvertiseAddress)
		if err != nil {
			opts.Logger.Fatal(
				"advertise address resolve failed",
				"string", cfg.AdvertiseAddress,
				"error", err.Error(),
			)

			return nil
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
	}

	srv.handlers.Store(&[]Handler{})

	app := fiber.New(
		fiber.Config{
			ServerHeader:     opts.Name,
			AppName:          opts.Name,
			TrustProxy:       cfg.TrustProxy == nil || *cfg.TrustProxy,
			TrustProxyConfig: fiber.TrustProxyConfig{Loopback: true, LinkLocal: true, Private: true},
		},
	)
	app.Use(recovermiddleware.New(recovermiddleware.ConfigDefault))
	srv.app = app
	srv.options.Logger.InfoContext(
		srv.ctx,
		"websocket server created",
		"server", srv.String(),
		"id", srv.options.ID,
		"name", srv.options.Name,
		"addr", addr.String(),
		"path", cfg.Path,
	)

	app.Use(cfg.Path, func(c fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			if !srv.checkOrigin(c) {
				srv.options.Logger.WarnContext(
					srv.ctx,
					"websocket origin rejected",
					"server", srv.String(),
					"id", srv.options.ID,
					"name", srv.options.Name,
					"origin", string(c.Request().Header.Peek("Origin")),
				)
				metrics.ServerRejectedTotal.WithLabelValues("websocket", "origin").Inc()

				return fiber.ErrForbidden
			}

			fiber.Locals[bool](c, "allowed", true)

			return c.Next()
		}

		return fiber.ErrUpgradeRequired
	})
	app.Get(cfg.Path, websocket.New(srv.operator, websocket.Config{
		RecoverHandler: srv.recoverConn,
	}))
	server.Set(srv)

	// Generate pool
	poolMu.Lock()
	if SessionPool == nil {
		SessionPool = NewPool(cfg.PingDuration, cfg.MaxIdleDuration)
	}

	poolMu.Unlock()

	return srv
}

// checkOrigin validates the Origin header of an upgrade request.
// Non-browser clients (no Origin header) are always allowed. With an empty
// Origins list the default same-origin policy applies: the Origin must match
// the request Host. An explicit list whitelists exact origins; "*" allows all.
func (srv *WebsocketServer) checkOrigin(c fiber.Ctx) bool {
	origin := string(c.Request().Header.Peek("Origin"))
	if origin == "" {
		return true
	}

	for _, allowed := range srv.config.Origins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}

	if len(srv.config.Origins) == 0 {
		host := string(c.Request().Header.Peek("Host"))

		return origin == "http://"+host || origin == "https://"+host
	}

	return false
}

// recoverConn isolates operator panics: log the stack server-side and drop
// the pool entry instead of leaking the panic value to the client.
func (srv *WebsocketServer) recoverConn(conn *websocket.Conn) {
	if r := recover(); r != nil {
		metrics.ServerPanicsTotal.WithLabelValues("websocket", "operator").Inc()
		srv.options.Logger.ErrorContext(
			srv.ctx,
			"websocket operator panicked",
			"server", srv.String(),
			"id", srv.options.ID,
			"name", srv.options.Name,
			"panic", r,
			"stack", string(debug.Stack()),
		)

		if SessionPool != nil {
			SessionPool.RemoveByConn(conn)
		}
	}
}

// Context returns the component context.
func (srv *WebsocketServer) Context() context.Context {
	return srv.ctx
}

// Options returns the runtime options.
func (srv *WebsocketServer) Options() *server.Options {
	return srv.options
}

// String returns a human-readable name.
func (srv *WebsocketServer) String() string {
	return "websocket"
}

// ID returns the unique instance ID.
func (srv *WebsocketServer) ID() uuid.UUID {
	return srv.options.ID
}

// Name returns the component name.
func (srv *WebsocketServer) Name() string {
	return srv.options.Name
}

// Start starts the component.
func (srv *WebsocketServer) Start() error {
	// Half TLS configuration must never silently degrade to plaintext.
	if (srv.config.TLSCertPEM == "") != (srv.config.TLSKeyPEM == "") {
		srv.options.Logger.ErrorContext(
			srv.ctx,
			"tls certification incomplete",
			"server", srv.String(),
			"id", srv.options.ID,
			"name", srv.options.Name,
		)

		return ErrIncompleteTLSConfig
	}

	srv.RLock()
	running := srv.running
	srv.RUnlock()
	if running {
		// running
		return nil
	}

	srv.options.RunBeforeStart()

	srv.Lock()

	if srv.running {
		// double-checked: concurrent Start won the race
		srv.Unlock()

		return nil
	}

	var (
		listener net.Listener
		cert     tls.Certificate
		err      error
	)

	// Try TLS first
	if srv.config.TLSCertPEM != "" && srv.config.TLSKeyPEM != "" {
		cert, err = tls.X509KeyPair([]byte(srv.config.TLSCertPEM), []byte(srv.config.TLSKeyPEM))
		if err != nil {
			srv.Unlock()
			srv.options.Logger.ErrorContext(
				srv.ctx,
				"tls certification failed",
				"server", srv.String(),
				"id", srv.options.ID,
				"name", srv.options.Name,
				"error", err.Error(),
			)

			return err
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
			srv.Unlock()
			srv.options.Logger.ErrorContext(
				srv.ctx,
				"network listen with TLS certificate failed",
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
			srv.Unlock()
			srv.options.Logger.ErrorContext(
				srv.ctx,
				"network listen failed",
				"server", srv.String(),
				"id", srv.options.ID,
				"name", srv.options.Name,
				"error", err.Error(),
			)

			return err
		}
	}

	srv.listener = listener
	srv.addr = listener.Addr()
	srv.metadata.Set("server", srv.String())
	srv.metadata.Set("network", srv.addr.Network())
	srv.metadata.Set("address", srv.addr.String())
	srv.metadata.Set("advertise_address", srv.advertiseAddr.String())
	srv.metadata.Set("name", srv.options.Name)
	srv.metadata.Set("id", srv.options.ID.String())
	srv.wg.Go(func() {
		err := srv.app.Listener(listener, fiber.ListenConfig{
			DisableStartupMessage: true,
			ListenerNetwork:       srv.config.Network,
		})
		if err != nil {
			srv.options.Logger.ErrorContext(
				srv.ctx, "websocket server listen failed",
				"server", srv.String(),
				"id", srv.options.ID,
				"name", srv.options.Name,
				"error", err.Error(),
			)

			return
		}

		srv.options.Logger.InfoContext(
			srv.ctx,
			"websocket server closed",
			"server", srv.String(),
			"id", srv.options.ID,
			"name", srv.options.Name,
		)
	})

	srv.reaperDone = srv.startReaper()

	srv.options.Logger.InfoContext(
		srv.ctx,
		"websocket server listened",
		"server", srv.String(),
		"id", srv.options.ID,
		"name", srv.options.Name,
		"addr", srv.addr.String(),
		"path", srv.config.Path,
	)
	srv.running = true
	srv.Unlock()

	srv.options.RunAfterStart()

	return nil
}

// startReaper launches the idle-session recycler. Callers must hold the
// server Lock (Start) so the done channel cannot race with Stop.
func (srv *WebsocketServer) startReaper() chan struct{} {
	if srv.config.MaxIdleDuration <= 0 {
		return nil
	}

	interval := time.Duration(srv.config.PingDuration) * time.Second
	if interval <= 0 {
		interval = time.Duration(srv.config.MaxIdleDuration) * time.Second
	}

	done := make(chan struct{})
	srv.wg.Go(func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				if SessionPool != nil {
					SessionPool.Purge()
				}
			}
		}
	})

	return done
}

// Stop stops the component and releases resources.
func (srv *WebsocketServer) Stop() error {
	srv.Lock()
	if !srv.running {
		srv.Unlock()

		// Not running
		return nil
	}

	srv.running = false
	app := srv.app
	listener := srv.listener
	reaperDone := srv.reaperDone
	timeout := time.Duration(srv.config.ShutdownTimeout) * time.Second
	srv.Unlock()

	srv.options.RunBeforeStop()

	var stopErr error
	if reaperDone != nil {
		close(reaperDone)
	}

	if app != nil && app.Server() != nil {
		if serr := app.ShutdownWithTimeout(timeout); serr != nil {
			stopErr = errors.Join(stopErr, serr)
		}
	}

	// Backstop against the shutdown-vs-Serve registration race.
	if listener != nil {
		_ = listener.Close()
	}

	// Bound the wait: a blocking handler must never wedge shutdown forever.
	waitDone := make(chan struct{})
	go func() {
		srv.wg.Wait()
		close(waitDone)
	}()

	select {
	case <-waitDone:
	case <-time.After(timeout + time.Second):
		stopErr = errors.Join(stopErr, errors.New("websocket server shutdown timed out"))
	}

	srv.options.Logger.InfoContext(
		srv.ctx,
		"websocket server shutdown",
		"server", srv.String(),
		"id", srv.options.ID,
		"name", srv.options.Name,
	)
	srv.options.RunAfterStop()

	return stopErr
}

// Running reports whether the component is running.
func (srv *WebsocketServer) Running() bool {
	srv.RLock()
	defer srv.RUnlock()

	return srv.running
}

// Addr returns the address.
func (srv *WebsocketServer) Addr() net.Addr {
	srv.RLock()
	defer srv.RUnlock()

	return srv.addr
}

// IP returns the IP.
func (srv *WebsocketServer) IP() net.IP {
	try := utils.AddrToIP(srv.Addr())
	if try == nil || try.IsUnspecified() {
		try, _ = utils.ObtainPreferIP(true)
	}

	return try
}

// Port returns the port.
func (srv *WebsocketServer) Port() int {
	return utils.AddrToPort(srv.Addr())
}

// AdvertiseAddr returns the advertise address.
func (srv *WebsocketServer) AdvertiseAddr() net.Addr {
	srv.RLock()
	defer srv.RUnlock()

	return srv.advertiseAddr
}

// AdvertiseIP returns the advertise IP.
func (srv *WebsocketServer) AdvertiseIP() net.IP {
	try := utils.AddrToIP(srv.AdvertiseAddr())
	if try == nil || try.IsUnspecified() {
		try, _ = utils.ObtainPreferIP(true)
	}

	return try
}

// AdvertisePort returns the advertise port.
func (srv *WebsocketServer) AdvertisePort() int {
	return utils.AddrToPort(srv.AdvertiseAddr())
}

// Metadata returns the metadata.
func (srv *WebsocketServer) Metadata() utils.Metadata {
	srv.RLock()
	defer srv.RUnlock()

	return srv.metadata.Clone()
}

// App returns the app.
func (srv *WebsocketServer) App() *fiber.App {
	return srv.app
}

// Handle registers handlers.
func (srv *WebsocketServer) Handle(hdls ...Handler) {
	// Lock-free append: publish a new slice so concurrent I/O
	// goroutines keep iterating a stable snapshot.
	for {
		old := srv.snapshotHandlers()
		next := make([]Handler, 0, len(old)+len(hdls))
		next = append(next, old...)
		next = append(next, hdls...)
		if srv.handlers.CompareAndSwap(srv.handlers.Load(), &next) {
			break
		}
	}

	for _, hdl := range hdls {
		srv.options.Logger.DebugContext(
			srv.ctx,
			"websocket handler registered",
			"server", srv.String(),
			"id", srv.options.ID,
			"name", srv.options.Name,
			"handler", hdl.Name(),
		)
	}
}

// snapshotHandlers returns the current handler snapshot.
func (srv *WebsocketServer) snapshotHandlers() []Handler {
	return *srv.handlers.Load()
}

// safelyInvoke runs a handler callback with panic isolation: a panicking
// business handler must never kill the connection goroutine.
func (srv *WebsocketServer) safelyInvoke(op string, sess *Session, fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			srv.options.Logger.ErrorContext(
				srv.ctx,
				"websocket handler panicked",
				"server", srv.String(),
				"id", srv.options.ID,
				"name", srv.options.Name,
				"handler_op", op,
				"session", sess.ID,
				"panic", r,
				"stack", string(debug.Stack()),
			)
			err = fmt.Errorf("handler panicked: %v", r)
		}
	}()

	if e := fn(); e != nil {
		srv.options.Logger.ErrorContext(
			srv.ctx,
			"websocket handler error",
			"server", srv.String(),
			"id", srv.options.ID,
			"name", srv.options.Name,
			"handler_op", op,
			"session", sess.ID,
			"error", e.Error(),
		)
		err = e
	}

	return err
}

func (srv *WebsocketServer) operator(c *websocket.Conn) {
	var (
		mt   int
		body []byte
		err  error
	)

	if srv.config.MaxMessageBytes > 0 {
		c.SetReadLimit(int64(srv.config.MaxMessageBytes))
	}

	// OnConnect
	srv.options.Logger.DebugContext(
		srv.ctx,
		"websocket client established",
		"server", srv.String(),
		"id", srv.options.ID,
		"name", srv.options.Name,
		"client", c.RemoteAddr().String(),
	)

	sess := NewSession(c)
	if SessionPool != nil {
		SessionPool.Put(sess)
	}

	metrics.ServerConnections.WithLabelValues("websocket").Inc()
	defer metrics.ServerConnections.WithLabelValues("websocket").Dec()

	handlers := srv.snapshotHandlers()

	// A failing OnConnect rejects the connection instead of letting it
	// proceed into the read loop.
	connectFailed := false
	for _, hdl := range handlers {
		if herr := srv.safelyInvoke("connect", sess, func() error {
			return hdl.OnConnect(sess)
		}); herr != nil {
			connectFailed = true

			break
		}
	}

	if connectFailed {
		metrics.ServerRejectedTotal.WithLabelValues("websocket", "connect").Inc()
		_ = sess.Close()

		return
	}

read:
	for {
		mt, body, err = c.ReadMessage()
		if err != nil {
			// Read error
			for _, hdl := range handlers {
				_ = srv.safelyInvoke("error", sess, func() error {
					return hdl.OnError(sess, err)
				})
			}

			break read
		} else {
			switch mt {
			case websocket.TextMessage, websocket.BinaryMessage:
				sess.touch()
				srv.options.Logger.DebugContext(
					srv.ctx,
					"websocket data received",
					"server", srv.String(),
					"id", srv.options.ID,
					"name", srv.options.Name,
					"client", c.RemoteAddr().String(),
					"message_type", mt,
					"message_length", len(body),
				)

				// OnData
				start := time.Now()
				for _, hdl := range handlers {
					_ = srv.safelyInvoke("data", sess, func() error {
						return hdl.OnData(sess, mt, body)
					})
				}
				msgType := "binary"
				if mt == websocket.TextMessage {
					msgType = "text"
				}
				metrics.ObserveServerRequest("websocket", msgType, msgType, "ok", time.Since(start))
				metrics.ServerIOBytesTotal.WithLabelValues("websocket", "in").Add(float64(len(body)))
			case websocket.PongMessage:
				// Ignore typo
				sess.touch()
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
			"websocket connection close failed",
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
			"websocket connection closed",
			"server", srv.String(),
			"id", srv.options.ID,
			"name", srv.options.Name,
			"client", c.RemoteAddr().String(),
			"session", sess.ID,
		)
	}

	for _, hdl := range handlers {
		_ = srv.safelyInvoke("close", sess, func() error {
			return hdl.OnClose(sess)
		})
	}
}

/* {{{ [Handler]. */
type Handler interface {
	Name() string
	Type() string
	OnConnect(sess *Session) error
	OnClose(sess *Session) error
	OnError(sess *Session, err error) error
	OnData(sess *Session, msgType int, data []byte) error
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
