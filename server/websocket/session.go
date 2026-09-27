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
 * @package websocket
 * @author Dr.NP <np@herewe.tech>
 * @since 02/11/2023
 */

package websocket

import (
	"errors"
	"sync"
	"time"

	"github.com/gofiber/contrib/v3/websocket"
	"github.com/google/uuid"

	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/utils"
)

/* {{{ [Session]. */
type Session struct {
	server.SessionBase

	conn *websocket.Conn
	pool *Pool

	// mu guards LastActive/Valid/Key. Meta stays handler-owned after
	// OnConnect, matching the pre-existing exported-field contract.
	mu sync.RWMutex
	// sendMu serializes concurrent Write calls so frames never interleave.
	sendMu sync.Mutex
}

// NewSession creates a new Session.
func NewSession(conn *websocket.Conn) *Session {
	return &Session{
		SessionBase: server.SessionBase{
			ID:         uuid.New(),
			LastActive: time.Now(),
			Meta:       utils.NewMetadata(),
			Type:       server.SessionWebsocket,
			Valid:      true,
		},
		conn: conn,
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
func (s *Session) Send(mt int, data []byte) error {
	s.touch()
	if mt <= 0 {
		mt = websocket.TextMessage
	}

	if s.conn == nil {
		return errors.New("websocket session has no connection")
	}

	// Serialize writes: the underlying websocket connection requires a
	// single concurrent writer.
	s.sendMu.Lock()
	defer s.sendMu.Unlock()

	if err := s.conn.SetWriteDeadline(time.Now().Add(ControlDeadline)); err != nil {
		return err
	}

	return s.conn.WriteMessage(mt, data)
}

// Close closes the resource. It is idempotent: the reaper and the
// operator both call it, and a second call must not write another close
// frame on a connection the first call already tore down.
func (s *Session) Close() error {
	s.mu.Lock()
	if !s.Valid {
		s.mu.Unlock()

		return nil
	}

	s.Valid = false
	pool := s.pool
	conn := s.conn
	id := s.ID
	s.mu.Unlock()

	// Detach outside the session lock: RemoveByID takes the pool lock,
	// so holding mu here would invert the pool->session lock order.
	if pool != nil {
		pool.RemoveByID(id)
	}

	if conn == nil {
		return nil
	}

	// The close frame is a write like any other: it must not interleave
	// with a concurrent Send or with the reaper's ping.
	s.sendMu.Lock()
	defer s.sendMu.Unlock()

	if err := conn.SetWriteDeadline(time.Now().Add(ControlDeadline)); err == nil {
		// A failed frame write falls through to the connection close
		// below, which is what actually releases the socket.
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "closed"))
	}

	return conn.Close()
}

// ping writes a keepalive frame under the write lock so it can never
// interleave with Send.
func (s *Session) ping() {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()

	if s.conn == nil {
		return
	}

	if err := s.conn.SetWriteDeadline(time.Now().Add(ControlDeadline)); err != nil {
		return
	}

	_ = s.conn.WriteMessage(websocket.PingMessage, nil)
}

// Conn returns the connection.
func (s *Session) Conn() *websocket.Conn {
	return s.conn
}

// SetKey assigns the affinity key and (re)indexes the session in the
// pool, replacing any previous key. Unattached or closed sessions only
// retain the value locally. Key is written under s.mu - RemoveByID reads
// it there while holding the pool lock.
func (s *Session) SetKey(key string) {
	s.mu.Lock()
	old := s.Key
	s.Key = key
	pool := s.pool
	id := s.ID
	s.mu.Unlock()

	if pool == nil {
		return
	}

	pool.Lock()
	defer pool.Unlock()

	// Last writer wins; migration path will replace keying entirely.
	// Only drop the old mapping when this session still owns it - a
	// concurrent SetKey may have handed the key to somebody else.
	if old != "" && pool.keys[old] == s {
		delete(pool.keys, old)
	}

	// The insert is skipped when the session already left the pool, so a
	// concurrent removal cannot be resurrected by a late SetKey.
	if _, ok := pool.sessions[id]; ok && key != "" {
		pool.keys[key] = s
	}
}

/* }}} */

/* {{{ [Pool]. */
var (
	// SessionPool is the package-wide session pool singleton.
	SessionPool *Pool
	// poolMu serializes SessionPool initialization across concurrent New calls.
	poolMu sync.Mutex
)

// Pool is a websocket component.
type Pool struct {
	sync.RWMutex

	id              uuid.UUID
	sessions        map[uuid.UUID]*Session
	conns           map[*websocket.Conn]*Session
	keys            map[string]*Session
	pingDuration    time.Duration
	maxIdleDuration time.Duration
}

// NewPool creates a new Pool.
func NewPool(ping, idle int) *Pool {
	p := &Pool{
		id:              uuid.New(),
		sessions:        make(map[uuid.UUID]*Session),
		conns:           make(map[*websocket.Conn]*Session),
		keys:            make(map[string]*Session),
		pingDuration:    time.Duration(ping) * time.Second,
		maxIdleDuration: time.Duration(idle) * time.Second,
	}

	// runtime.HandleTicker(func(t time.Time, ct uint64) error {
	// 	p.Purge()

	// 	return nil
	// })

	if SessionPool == nil {
		SessionPool = p
	}

	return p
}

// Put stores the entry.
func (p *Pool) Put(sess *Session) {
	p.Lock()
	defer p.Unlock()

	// Key and pool are written by SetKey/Close under s.mu: read and write
	// them under the same lock (pool lock -> session lock, the order
	// SetKey releases) or the two race on every re-attach.
	sess.mu.Lock()
	if sess.ID == uuid.Nil {
		sess.ID = uuid.New()
	}

	id := sess.ID
	key := sess.Key
	conn := sess.conn
	sess.pool = p
	sess.mu.Unlock()

	p.sessions[id] = sess
	p.conns[conn] = sess
	if key != "" {
		p.keys[key] = sess
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
func (p *Pool) GetByConn(conn *websocket.Conn) *Session {
	p.RLock()
	defer p.RUnlock()
	sess, ok := p.conns[conn]
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
	delete(p.conns, sess.conn)

	// Key is written by SetKey under s.mu; read it under the same lock
	// (pool lock -> session lock, the order SetKey releases).
	sess.mu.RLock()
	key := sess.Key
	sess.mu.RUnlock()

	if key != "" {
		delete(p.keys, key)
	}

	return true
}

// RemoveByConn is part of the public API.
func (p *Pool) RemoveByConn(conn *websocket.Conn) bool {
	p.Lock()
	defer p.Unlock()
	sess, ok := p.conns[conn]
	if !ok {
		return false
	}

	delete(p.sessions, sess.ID)
	delete(p.conns, conn)

	sess.mu.RLock()
	key := sess.Key
	sess.mu.RUnlock()

	if key != "" {
		delete(p.keys, key)
	}

	return true
}

// RemoveByKey is part of the public API.
func (p *Pool) RemoveByKey(key string) bool {
	p.Lock()
	defer p.Unlock()
	sess, ok := p.keys[key]
	if !ok {
		return false
	}

	delete(p.sessions, sess.ID)
	delete(p.conns, sess.conn)
	delete(p.keys, key)

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

	// Snapshot under lock, do blocking I/O outside lock (migration to
	// gobwas/ws will replace this entirely; minimal fix only).
	p.RLock()
	snapshot := make([]*Session, 0, len(p.sessions))
	for _, sess := range p.sessions {
		snapshot = append(snapshot, sess)
	}

	p.RUnlock()

	now := time.Now()
	for _, sess := range snapshot {
		last := sess.lastActive()
		if now.Sub(last) > p.pingDuration {
			// Write ping under the session write lock: the reaper runs
			// concurrently with handlers calling Send.
			sess.ping()
		}

		if now.Sub(last) > p.maxIdleDuration {
			_ = sess.Close()
		}
	}
}

// Foreach iterates all entries.
func (p *Pool) Foreach(f func(sess *Session)) {
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
