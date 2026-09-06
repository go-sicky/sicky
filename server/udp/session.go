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
 * @file session.go
 * @package udp
 * @author Dr.NP <np@herewe.tech>
 * @since 03/08/2025
 */

package udp

import (
	"errors"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/utils"
)

// ErrNilSessionAddr is returned when replying to a session without a peer.
var ErrNilSessionAddr = errors.New("udp session has no peer address")

/* {{{ [Session]. */
type Session struct {
	server.SessionBase

	conn *net.UDPConn
	addr *net.UDPAddr
	pool *Pool

	// mu guards LastActive/Valid/Key. Meta stays handler-owned after
	// OnConnect, matching the pre-existing exported-field contract.
	mu sync.RWMutex
	// sendMu serializes concurrent WriteToUDP calls.
	sendMu sync.Mutex

	writeTimeout time.Duration
}

// NewSession creates a new Session.
func NewSession(conn *net.UDPConn, addr *net.UDPAddr) *Session {
	return NewSessionWithTimeout(conn, addr, 0)
}

// NewSessionWithTimeout builds a session whose Send applies a per-write
// deadline. A non-positive timeout disables the deadline.
func NewSessionWithTimeout(conn *net.UDPConn, addr *net.UDPAddr, writeTimeout time.Duration) *Session {
	return &Session{
		SessionBase: server.SessionBase{
			ID:         uuid.New(),
			LastActive: time.Now(),
			Meta:       utils.NewMetadata(),
			Type:       server.SessionUDP,
			Valid:      true,
		},
		conn:         conn,
		addr:         addr,
		writeTimeout: writeTimeout,
	}
}

func (s *Session) touch() {
	s.mu.Lock()
	s.LastActive = time.Now()
	s.mu.Unlock()
}

func (s *Session) lastActive() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.LastActive
}

// Send sends data.
func (s *Session) Send(data []byte) error {
	dst := s.Addr()
	if dst == nil {
		return ErrNilSessionAddr
	}

	s.touch()
	s.sendMu.Lock()
	defer s.sendMu.Unlock()

	if s.writeTimeout > 0 {
		_ = s.conn.SetWriteDeadline(time.Now().Add(s.writeTimeout))
	}

	_, err := s.conn.WriteToUDP(data, dst)

	return err
}

// Close closes the resource.
func (s *Session) Close() error {
	s.mu.Lock()
	if !s.Valid {
		s.mu.Unlock()

		return nil
	}

	s.Valid = false
	p := s.pool
	s.mu.Unlock()

	// Detach outside the session lock: RemoveByID takes the pool lock,
	// so holding mu here would invert the pool->session lock order.
	if p != nil {
		p.RemoveByID(s.ID)
	}

	// The UDP conn is shared by all sessions; only the binding is dropped.
	return nil
}

// Conn returns the connection.
func (s *Session) Conn() *net.UDPConn {
	return s.conn
}

// SetKey assigns the affinity key and (re)indexes the session in the
// pool, replacing any previous key. Unattached or closed sessions only
// retain the value locally.
func (s *Session) SetKey(key string) {
	s.mu.Lock()
	old := s.Key
	s.Key = key
	p := s.pool
	id := s.ID
	s.mu.Unlock()

	if p == nil {
		return
	}

	p.Lock()
	defer p.Unlock()

	if old != "" {
		delete(p.keys, old)
	}

	if _, ok := p.sessions[id]; ok && key != "" {
		p.keys[key] = s
	}
}

// Addr returns the address.
func (s *Session) Addr() *net.UDPAddr {
	return s.addr
}

/* }}} */

/* {{{ [Pool]. */
type Pool struct {
	sync.RWMutex

	id              uuid.UUID
	sessions        map[uuid.UUID]*Session
	conns           map[*net.UDPConn]*Session
	addrs           map[string]*Session
	keys            map[string]*Session
	maxIdleDuration time.Duration
}

// NewPool creates a new Pool.
func NewPool(idle int) *Pool {
	p := &Pool{
		id:              uuid.New(),
		sessions:        make(map[uuid.UUID]*Session),
		conns:           make(map[*net.UDPConn]*Session),
		addrs:           make(map[string]*Session),
		keys:            make(map[string]*Session),
		maxIdleDuration: time.Duration(idle) * time.Second,
	}

	// runtime.HandleTicker(func(t time.Time, ct uint64) error {
	// 	p.Purge()

	// 	return nil
	// })

	return p
}

// addrKey normalizes the peer address. Pointers must never be map keys:
// ReadFromUDP returns a fresh *UDPAddr per datagram, so pointer equality
// never matches and every packet would allocate a new session.
func addrKey(addr *net.UDPAddr) string {
	if addr == nil {
		return ""
	}

	return addr.String()
}

// Put stores the entry.
func (p *Pool) Put(sess *Session) {
	p.Lock()
	defer p.Unlock()

	if sess.ID == uuid.Nil {
		sess.ID = uuid.New()
	}

	// Publish the back-pointer before the session becomes reachable via
	// the maps so concurrent getters never observe a torn registration.
	sess.pool = p
	p.sessions[sess.ID] = sess
	if key := addrKey(sess.addr); key != "" {
		p.addrs[key] = sess
	}

	if sess.Key != "" {
		p.keys[sess.Key] = sess
	}
}

// GetByID looks up by ID.
func (p *Pool) GetByID(id uuid.UUID) *Session {
	p.RLock()
	defer p.RUnlock()

	sess, ok := p.sessions[id]
	if !ok {
		return nil
	}

	return sess
}

// GetByConn looks up by connection.
func (p *Pool) GetByConn(conn *net.UDPConn) *Session {
	p.RLock()
	defer p.RUnlock()

	sess, ok := p.conns[conn]
	if !ok {
		return nil
	}

	return sess
}

// GetByAddr returns the byaddr.
func (p *Pool) GetByAddr(addr *net.UDPAddr) *Session {
	p.RLock()
	defer p.RUnlock()

	sess, ok := p.addrs[addrKey(addr)]
	if !ok {
		return nil
	}

	return sess
}

// GetByKey looks up by key.
func (p *Pool) GetByKey(key string) *Session {
	p.RLock()
	defer p.RUnlock()

	sess, ok := p.keys[key]
	if !ok {
		return nil
	}

	return sess
}

// RemoveByID removes by ID.
func (p *Pool) RemoveByID(id uuid.UUID) bool {
	p.Lock()
	defer p.Unlock()

	sess, ok := p.sessions[id]
	if !ok {
		return false
	}

	delete(p.sessions, id)
	delete(p.addrs, addrKey(sess.addr))
	if sess.Key != "" {
		delete(p.keys, sess.Key)
	}

	return true
}

// Length returns the entry count.
func (p *Pool) Length() int {
	p.RLock()
	defer p.RUnlock()

	return len(p.sessions)
}

// Purge removes expired entries.
func (p *Pool) Purge() {
	if p.maxIdleDuration <= 0 {
		return
	}

	// Collect under a read lock, close outside it: Close detaches via
	// RemoveByID (pool write lock), so holding the pool lock across it
	// would self-deadlock and stall readers.
	now := time.Now()
	var idle []*Session
	p.RLock()
	for _, sess := range p.sessions {
		if now.Sub(sess.lastActive()) > p.maxIdleDuration {
			idle = append(idle, sess)
		}
	}

	p.RUnlock()

	for _, sess := range idle {
		remote := unknownRemote
		if addr := sess.Addr(); addr != nil {
			remote = addr.String()
		}

		logger.Logger.Debug(
			"udp connection idle for a long time",
			"session", sess.ID,
			"remote_address", remote,
		)

		_ = sess.Close()
	}
}

// PurgeForce drops every tracked session unconditionally. Used by Stop:
// after the server conn is closed every pooled session is dead, and on
// restart a returning client must re-fire OnConnect.
func (p *Pool) PurgeForce() {
	p.RLock()
	snapshot := make([]*Session, 0, len(p.sessions))
	for _, sess := range p.sessions {
		snapshot = append(snapshot, sess)
	}

	p.RUnlock()

	for _, sess := range snapshot {
		_ = sess.Close()
	}
}

// RunReaper purges idle sessions on every tick until stop is closed.
// The caller owns the goroutine lifecycle (add to WaitGroup before go).
func (p *Pool) RunReaper(tick time.Duration, stop <-chan struct{}) {
	t := time.NewTicker(tick)
	defer t.Stop()

	for {
		select {
		case <-stop:
			return
		case <-t.C:
			p.Purge()
		}
	}
}

// Foreach iterates all entries.
func (p *Pool) Foreach(f func(sess *Session)) {
	// Snapshot first: invoking external callbacks under RLock would
	// deadlock as soon as a callback calls Put/Remove/Purge.
	p.RLock()
	snapshot := make([]*Session, 0, len(p.sessions))
	for _, sess := range p.sessions {
		snapshot = append(snapshot, sess)
	}

	p.RUnlock()

	for _, sess := range snapshot {
		f(sess)
	}
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
