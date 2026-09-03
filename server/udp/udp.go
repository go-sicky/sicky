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
	"sync/atomic"
	"time"

	"github.com/go-sicky/sicky/metrics"
	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/utils"
	"github.com/google/uuid"
)

var ErrObtainUDPAddress = errors.New("obtain UDP address failed")

// ErrServerNotRunning is returned when Send is called on a stopped server.
var ErrServerNotRunning = errors.New("udp server is not running")

/* {{{ [Server] */

// UDPServer : Server definition
type UDPServer struct {
	config        *Config
	ctx           context.Context
	options       *server.Options
	running       bool
	stopping      bool
	addr          net.Addr
	advertiseAddr net.Addr
	conn          *net.UDPConn
	metadata      utils.Metadata
	// handlers is lock-free (atomic snapshot) so the packet loop never
	// blocks on registration and Stop can Wait without self-deadlock.
	handlers  atomic.Pointer[[]Handler]
	pool      *Pool
	purgeDone chan struct{}
	// rateWindow/rateCounts implement a fixed-window per-source packet
	// limiter (reflection/amplification guard), guarded by rateMu.
	rateMu     sync.Mutex
	rateWindow time.Time
	rateCounts map[string]int

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
		ctx:           opts.Context,
		addr:          addr,
		advertiseAddr: advertiseAddr,
		running:       false,
		options:       opts,
		metadata:      utils.NewMetadata(),
		pool:          NewPool(cfg.MaxIdleDuration),
		rateCounts:    make(map[string]int),
	}
	srv.handlers.Store(&[]Handler{})

	srv.options.Logger.InfoContext(
		srv.ctx,
		"UDP server created",
		"server", srv.String(),
		"id", srv.options.ID,
		"name", srv.options.Name,
		"network", addr.Network(),
		"address", addr.String(),
	)

	server.Set(srv)

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
	var err error
	srv.Lock()
	defer srv.Unlock()

	if srv.running || srv.stopping {
		// running
		return nil
	}

	srv.options.RunBeforeStart()

	srv.metadata.Set("server", srv.String())
	srv.metadata.Set("network", srv.addr.Network())
	srv.metadata.Set("address", srv.addr.String())
	srv.metadata.Set("advertise_address", srv.advertiseAddr.String())
	srv.metadata.Set("name", srv.options.Name)
	srv.metadata.Set("id", srv.options.ID.String())
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

		return ErrObtainUDPAddress
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

	srv.startReaper()
	srv.wg.Add(1)
	go func() {
		defer srv.wg.Done()

		buff := make([]byte, srv.config.BufferSize)
		for {
			if srv.config.ReadTimeout > 0 {
				_ = srv.conn.SetReadDeadline(time.Now().Add(time.Duration(srv.config.ReadTimeout) * time.Second))
			}
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

					break
				}
				// Transient errors (ENOBUFS/ICMP refused/…) must not
				// kill the packet loop: log and keep serving. Read
				// deadlines surface as timeouts: just re-arm and
				// continue; idle sessions are recycled by the reaper.
				var nerr net.Error
				if errors.As(err, &nerr) && nerr.Timeout() {
					continue
				}
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

				continue
			} else if n > 0 {
				if !srv.allowPacket(addr) {
					continue
				}
				sess := srv.pool.GetByAddr(addr)
				if sess == nil {
					if srv.config.MaxSessions > 0 && srv.pool.Length() >= srv.config.MaxSessions {
						srv.options.Logger.ErrorContext(
							srv.ctx,
							"UDP session cap reached, dropping datagram",
							"server", srv.String(),
							"id", srv.options.ID,
							"name", srv.options.Name,
							"max_sessions", srv.config.MaxSessions,
						)

						continue
					}
					var writeTimeout time.Duration
					if srv.config.WriteTimeout > 0 {
						writeTimeout = time.Duration(srv.config.WriteTimeout) * time.Second
					}
					sess = NewSessionWithTimeout(srv.conn, addr, writeTimeout)
					srv.pool.Put(sess)
					for _, hdl := range srv.snapshotHandlers() {
						h := hdl
						s := sess
						srv.safelyInvoke("OnConnect", s, addr, func() error {
							return h.OnConnect(s)
						})
					}
				}

				sess.touch()

				dst := make([]byte, n)
				copy(dst, buff)
				metrics.NumUDPServerAccessCounter.Inc()
				for _, hdl := range srv.snapshotHandlers() {
					h := hdl
					s := sess
					srv.safelyInvoke("OnData", s, addr, func() error {
						return h.OnData(s, dst)
					})
				}
			}
		}
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
	srv.options.RunAfterStart()

	return nil
}

func (srv *UDPServer) Stop() error {
	// Check-and-flag under lock, then release: holding Lock across the
	// wait would starve all RLock readers for the whole drain.
	srv.Lock()
	if !srv.running || srv.stopping {
		// Not running
		srv.Unlock()

		return nil
	}
	srv.stopping = true
	srv.options.RunBeforeStop()
	conn := srv.conn
	srv.Unlock()

	var errs error
	if err := conn.Close(); err != nil {
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
		errs = errors.Join(errs, err)
	}

	srv.stopReaper()
	srv.wg.Wait()

	srv.Lock()
	srv.running = false
	srv.stopping = false
	srv.Unlock()

	srv.options.Logger.InfoContext(
		srv.ctx,
		"UDP server shutdown",
		"server", srv.String(),
		"id", srv.options.ID,
		"name", srv.options.Name,
		"network", srv.addr.Network(),
		"address", srv.addr.String(),
	)
	srv.options.RunAfterStop()

	return errs
}

func (srv *UDPServer) Running() bool {
	srv.RLock()
	defer srv.RUnlock()

	return srv.running
}

func (srv *UDPServer) Addr() net.Addr {
	srv.RLock()
	defer srv.RUnlock()

	return srv.addr
}

func (srv *UDPServer) IP() net.IP {
	try := utils.AddrToIP(srv.Addr())
	if try == nil || try.IsUnspecified() {
		try, _ = utils.ObtainPreferIP(true)
	}

	return try
}

func (srv *UDPServer) Port() int {
	return utils.AddrToPort(srv.Addr())
}

func (srv *UDPServer) AdvertiseAddr() net.Addr {
	srv.RLock()
	defer srv.RUnlock()

	return srv.advertiseAddr
}

func (srv *UDPServer) AdvertiseIP() net.IP {
	try := utils.AddrToIP(srv.AdvertiseAddr())
	if try == nil || try.IsUnspecified() {
		try, _ = utils.ObtainPreferIP(true)
	}

	return try
}

func (srv *UDPServer) AdvertisePort() int {
	return utils.AddrToPort(srv.AdvertiseAddr())
}

func (srv *UDPServer) Metadata() utils.Metadata {
	// Snapshot: the map is written during Start while handlers may read
	// it concurrently; returning the live map would race.
	if srv.metadata == nil {
		return utils.NewMetadata()
	}

	return srv.metadata.Clone()
}

func (srv *UDPServer) App() *net.UDPConn {
	return srv.conn
}

func (srv *UDPServer) Handle(hdls ...Handler) {
	// Lock-free append: publish a new slice so the packet loop keeps
	// iterating a stable snapshot.
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
			"UDP handler registered",
			"server", srv.String(),
			"id", srv.options.ID,
			"name", srv.options.Name,
			"handler", hdl.Name(),
		)
	}
}

// snapshotHandlers returns a stable handler slice for event dispatch.
func (srv *UDPServer) snapshotHandlers() []Handler {
	if p := srv.handlers.Load(); p != nil {
		return *p
	}

	return nil
}

// startReaper launches the idle-session recycler. Callers must hold the
// server Lock (Start) so the done channel cannot race with Stop.
func (srv *UDPServer) startReaper() {
	if srv.config.MaxIdleDuration <= 0 {
		return
	}

	interval := time.Duration(srv.config.MaxIdleDuration) * time.Second / 2
	if min := time.Duration(MinReapIntervalSeconds) * time.Second; interval < min {
		interval = min
	}

	srv.purgeDone = make(chan struct{})
	done := srv.purgeDone
	srv.wg.Add(1)
	go func() {
		defer srv.wg.Done()
		srv.pool.RunReaper(interval, done)
	}()
}

// stopReaper halts the recycler started by startReaper.
func (srv *UDPServer) stopReaper() {
	if srv.purgeDone != nil {
		close(srv.purgeDone)
		srv.purgeDone = nil
	}
}

// allowPacket enforces the fixed-window per-source packet limit. A
// non-positive MaxPacketsPerSecond disables limiting. Over-limit packets
// are dropped silently: logging each drop would itself become a log-DoS.
func (srv *UDPServer) allowPacket(addr *net.UDPAddr) bool {
	limit := srv.config.MaxPacketsPerSecond
	if limit <= 0 {
		return true
	}

	key := ""
	if addr != nil {
		key = addr.String()
	}

	now := time.Now()
	srv.rateMu.Lock()
	defer srv.rateMu.Unlock()

	if now.Sub(srv.rateWindow) >= time.Second {
		srv.rateWindow = now
		clear(srv.rateCounts)
	}
	if srv.rateCounts[key] >= limit {
		return false
	}
	srv.rateCounts[key]++

	return true
}

// safelyInvoke runs a handler callback with panic isolation: a panicking
// business handler must never kill the single packet loop.
func (srv *UDPServer) safelyInvoke(op string, sess *Session, addr *net.UDPAddr, fn func() error) {
	defer func() {
		if r := recover(); r != nil {
			remote := "unknown"
			if addr != nil {
				remote = addr.String()
			}
			srv.options.Logger.ErrorContext(
				srv.ctx,
				"UDP handler panicked",
				"server", srv.String(),
				"id", srv.options.ID,
				"name", srv.options.Name,
				"handler_op", op,
				"remote", remote,
				"panic", r,
			)
		}
	}()

	if err := fn(); err != nil {
		remote := "unknown"
		if addr != nil {
			remote = addr.String()
		}
		srv.options.Logger.ErrorContext(
			srv.ctx,
			"UDP data process error",
			"server", srv.String(),
			"id", srv.options.ID,
			"name", srv.options.Name,
			"handler_op", op,
			"remote", remote,
			"error", err.Error(),
		)
	}
}

func (srv *UDPServer) Send(c *net.UDPAddr, data []byte) error {
	srv.RLock()
	conn := srv.conn
	running := srv.running
	srv.RUnlock()

	if !running || conn == nil {
		return ErrServerNotRunning
	}
	if c == nil {
		return ErrNilSessionAddr
	}

	_, err := conn.WriteToUDP(data, c)

	return err
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
