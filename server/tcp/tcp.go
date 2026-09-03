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
	"sync/atomic"
	"time"

	"github.com/go-sicky/sicky/metrics"
	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/utils"
	"github.com/google/uuid"
)

type TCPServer struct {
	config        *Config
	ctx           context.Context
	options       *server.Options
	running       bool
	stopping      bool
	addr          net.Addr
	advertiseAddr net.Addr
	conn          net.Listener
	metadata      utils.Metadata
	// handlers is lock-free (atomic snapshot) so I/O goroutines never
	// block on registration and Stop can Wait without self-deadlock.
	handlers  atomic.Pointer[[]Handler]
	pool      *Pool
	purgeDone chan struct{}
	conns     map[net.Conn]struct{}
	connsMu   sync.Mutex

	sync.RWMutex
	// wg tracks live connections only. acceptWg tracks the accept loop
	// and the reaper: Stop waits for acceptWg first, after which no new
	// conn Add can occur, so the later wg.Wait is race-free. (A single
	// WaitGroup would allow Add concurrent with Wait.)
	wg       sync.WaitGroup
	acceptWg sync.WaitGroup
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
		conns:         make(map[net.Conn]struct{}),
	}
	srv.handlers.Store(&[]Handler{})

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
	srv.RLock()
	defer srv.RUnlock()

	return srv.running
}

func (srv *TCPServer) Addr() net.Addr {
	srv.RLock()
	defer srv.RUnlock()

	return srv.addr
}

func (srv *TCPServer) IP() net.IP {
	try := utils.AddrToIP(srv.Addr())
	if try == nil || try.IsUnspecified() {
		try, _ = utils.ObtainPreferIP(true)
	}

	return try
}

func (srv *TCPServer) Port() int {
	return utils.AddrToPort(srv.Addr())
}

func (srv *TCPServer) AdvertiseAddr() net.Addr {
	srv.RLock()
	defer srv.RUnlock()

	return srv.advertiseAddr
}

func (srv *TCPServer) AdvertiseIP() net.IP {
	try := utils.AddrToIP(srv.AdvertiseAddr())
	if try == nil || try.IsUnspecified() {
		try, _ = utils.ObtainPreferIP(true)
	}

	return try
}

func (srv *TCPServer) AdvertisePort() int {
	return utils.AddrToPort(srv.AdvertiseAddr())
}

func (srv *TCPServer) Metadata() utils.Metadata {
	// Snapshot: the map is written during Start while handlers may read
	// it concurrently; returning the live map would race.
	if srv.metadata == nil {
		return utils.NewMetadata()
	}

	return srv.metadata.Clone()
}

func (srv *TCPServer) App() net.Listener {
	return srv.conn
}

func (srv *TCPServer) Handle(hdls ...Handler) {
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
			"TCP handler registered",
			"server", srv.String(),
			"id", srv.options.ID,
			"name", srv.options.Name,
			"handler", hdl.Name(),
		)
	}
}

// startReaper launches the idle-session recycler. Callers must hold the
// server Lock (Start) so the done channel cannot race with Stop.
func (srv *TCPServer) startReaper() {
	if srv.config.MaxIdleDuration <= 0 {
		return
	}

	interval := time.Duration(srv.config.MaxIdleDuration) * time.Second / 2
	if min := time.Duration(MinReapIntervalSeconds) * time.Second; interval < min {
		interval = min
	}

	srv.purgeDone = make(chan struct{})
	done := srv.purgeDone
	srv.acceptWg.Add(1)
	go func() {
		defer srv.acceptWg.Done()
		srv.pool.RunReaper(interval, done)
	}()
}

// stopReaper halts the recycler started by startReaper.
func (srv *TCPServer) stopReaper() {
	if srv.purgeDone != nil {
		close(srv.purgeDone)
		srv.purgeDone = nil
	}
}

// snapshotHandlers returns a stable handler slice for event dispatch.
func (srv *TCPServer) snapshotHandlers() []Handler {
	if p := srv.handlers.Load(); p != nil {
		return *p
	}

	return nil
}

// remoteAddrString nil-guards addresses of half-closed connections.
func remoteAddrString(c net.Conn) string {
	if c == nil {
		return "unknown"
	}
	if addr := c.RemoteAddr(); addr != nil {
		return addr.String()
	}

	return "unknown"
}

// safelyInvoke runs a handler callback with panic isolation: a panicking
// business handler must never kill the accept loop or a connection.
func (srv *TCPServer) safelyInvoke(op string, sess *Session, fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			srv.options.Logger.ErrorContext(
				srv.ctx,
				"TCP handler panicked",
				"server", srv.String(),
				"id", srv.options.ID,
				"name", srv.options.Name,
				"handler_op", op,
				"remote", remoteAddrString(sess.Conn()),
				"panic", r,
			)
			err = nil
		}
	}()

	if e := fn(); e != nil {
		srv.options.Logger.ErrorContext(
			srv.ctx,
			"TCP data process error",
			"server", srv.String(),
			"id", srv.options.ID,
			"name", srv.options.Name,
			"network", srv.Addr().Network(),
			"address", srv.Addr().String(),
			"remote", remoteAddrString(sess.Conn()),
			"error", e.Error(),
		)
	}

	return nil
}

func (srv *TCPServer) Send(c net.Conn, data []byte) error {
	_, err := c.Write(data)

	return err
}

func (srv *TCPServer) Start() error {
	var err error

	srv.Lock()
	defer srv.Unlock()

	if srv.running || srv.stopping {
		return nil
	}

	srv.options.RunBeforeStart()

	srv.metadata.Set("server", srv.String())
	srv.metadata.Set("network", srv.addr.Network())
	srv.metadata.Set("address", srv.addr.String())
	srv.metadata.Set("advertise_address", srv.advertiseAddr.String())
	srv.metadata.Set("name", srv.options.Name)
	srv.metadata.Set("id", srv.options.ID.String())

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
	srv.startReaper()
	srv.acceptWg.Add(1)
	go func() {
		defer srv.acceptWg.Done()

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

					break
				}
				// Transient errors (EMFILE/EINTR/…) must not kill the
				// accept loop: back off briefly and keep serving.
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
				time.Sleep(50 * time.Millisecond)

				continue
			}

			var writeTimeout time.Duration
			if srv.config.WriteTimeout > 0 {
				writeTimeout = time.Duration(srv.config.WriteTimeout) * time.Second
			}
			// Enforce the session cap before allocating anything for
			// the peer (mirrors the UDP datagram-drop policy).
			if srv.config.MaxSessions > 0 && srv.pool.Length() >= srv.config.MaxSessions {
				srv.options.Logger.ErrorContext(
					srv.ctx,
					"TCP session cap reached, rejecting connection",
					"server", srv.String(),
					"id", srv.options.ID,
					"name", srv.options.Name,
					"remote", remoteAddrString(client),
					"max_sessions", srv.config.MaxSessions,
				)
				client.Close()

				continue
			}
			sess := NewSessionWithTimeout(client, writeTimeout)
			srv.pool.Put(sess)
			hdls := srv.snapshotHandlers()
			for _, hdl := range hdls {
				h := hdl
				_ = srv.safelyInvoke("OnConnect", sess, func() error {
					return h.OnConnect(sess)
				})
			}

			// Register before spawning so Stop's Wait cannot miss the
			// connection: Stop holds the server Lock across Wait, and
			// the Add below is sequenced before the goroutine starts.
			srv.connsMu.Lock()
			srv.conns[client] = struct{}{}
			srv.connsMu.Unlock()
			srv.wg.Add(1)
			go func(c net.Conn) {
				defer srv.wg.Done()
				defer func() {
					srv.connsMu.Lock()
					delete(srv.conns, c)
					srv.connsMu.Unlock()
				}()
				defer c.Close()
				// Last-resort panic guard; per-callback guards above
				// already isolate handler panics.
				defer func() {
					if r := recover(); r != nil {
						srv.options.Logger.ErrorContext(
							srv.ctx,
							"TCP connection panicked",
							"server", srv.String(),
							"id", srv.options.ID,
							"name", srv.options.Name,
							"remote", remoteAddrString(c),
							"panic", r,
						)
					}
				}()

				buff := make([]byte, srv.config.BufferSize)
				reader := bufio.NewReader(c)
			read:
				for {
					if srv.config.ReadTimeout > 0 {
						_ = c.SetReadDeadline(time.Now().Add(time.Duration(srv.config.ReadTimeout) * time.Second))
					}
					n, err := reader.Read(buff)
					if err != nil {
						if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) || errors.Is(err, net.ErrClosed) {
							break read
						}
						var nerr net.Error
						if errors.As(err, &nerr) && nerr.Timeout() {
							// Idle read deadline: keep the connection
							// until the pool reaper recycles it.
							continue
						}
						srv.options.Logger.ErrorContext(
							srv.ctx,
							"TCP Read error",
							"server", srv.String(),
							"id", srv.options.ID,
							"name", srv.options.Name,
							"network", srv.addr.Network(),
							"address", srv.addr.String(),
							"remote", remoteAddrString(c),
							"error", err.Error(),
						)
						for _, hdl := range srv.snapshotHandlers() {
							h := hdl
							_ = srv.safelyInvoke("OnError", sess, func() error {
								return h.OnError(sess, err)
							})
						}

						break read
					} else {
						sess.touch()
						if n > 0 {
							dst := make([]byte, n)
							copy(dst, buff)
							metrics.NumTCPServerAccessCounter.Inc()
							for _, hdl := range srv.snapshotHandlers() {
								h := hdl
								_ = srv.safelyInvoke("OnData", sess, func() error {
									return h.OnData(sess, dst)
								})
							}
						}
					}
				}

				for _, hdl := range srv.snapshotHandlers() {
					h := hdl
					_ = srv.safelyInvoke("OnClose", sess, func() error {
						return h.OnClose(sess)
					})
				}
			}(client)
		}
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
	srv.options.RunAfterStart()

	return nil
}

func (srv *TCPServer) Stop() error {
	// Check-and-flag under lock, then release: holding Lock across the
	// waits would starve all RLock readers for the whole drain.
	srv.Lock()
	if !srv.running || srv.stopping {
		srv.Unlock()

		return nil
	}
	srv.stopping = true
	srv.options.RunBeforeStop()
	listener := srv.conn
	srv.Unlock()

	var errs []error
	if err := listener.Close(); err != nil {
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
		errs = append(errs, err)
	}

	// Phase 1: stop intake. Waiting for acceptWg first guarantees no
	// further conn wg.Add can occur, keeping the phase-2 Wait race-free.
	srv.stopReaper()
	srv.acceptWg.Wait()

	// Phase 2: close tracked connections so handler goroutines observe
	// EOF and exit, then wait for them.
	srv.connsMu.Lock()
	for c := range srv.conns {
		if err := c.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	srv.connsMu.Unlock()

	srv.wg.Wait()

	srv.Lock()
	srv.running = false
	srv.stopping = false
	srv.Unlock()

	srv.options.Logger.InfoContext(
		srv.ctx,
		"TCP server shutdown",
		"server", srv.String(),
		"id", srv.options.ID,
		"name", srv.options.Name,
		"network", srv.addr.Network(),
		"address", srv.addr.String(),
	)
	srv.options.RunAfterStop()

	return errors.Join(errs...)
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
