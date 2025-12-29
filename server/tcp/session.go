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
 * @package tcp
 * @author Dr.NP <np@herewe.tech>
 * @since 03/08/2025
 */

package tcp

import (
	"net"
	"sync"
	"time"

	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/utils"
	"github.com/google/uuid"
)

/* {{{ [Session] */
type Session struct {
	server.SessionBase

	conn net.Conn
	pool *Pool
}

func NewSession(conn net.Conn) *Session {
	return &Session{
		SessionBase: server.SessionBase{
			ID:         uuid.New(),
			LastActive: time.Now(),
			Meta:       utils.NewMetadata(),
			Type:       server.SessionTCP,
			Valid:      true,
		},
		conn: conn,
	}
}

func (s *Session) Send(data []byte) error {
	s.LastActive = time.Now()
	_, err := s.conn.Write(data)

	return err
}

func (s *Session) Close() error {
	if s.pool != nil {
		s.pool.RemoveByID(s.ID)
	}

	return s.conn.Close()
}

func (s *Session) Conn() net.Conn {
	return s.conn
}

/* }}} */

/* {{{ [Pool] */
type Pool struct {
	sync.RWMutex

	id              uuid.UUID
	sessions        map[uuid.UUID]*Session
	conns           map[net.Conn]*Session
	keys            map[string]*Session
	maxIdleDuration time.Duration
}

func NewPool(idle int) *Pool {
	p := &Pool{
		id:              uuid.New(),
		sessions:        make(map[uuid.UUID]*Session),
		conns:           make(map[net.Conn]*Session),
		keys:            make(map[string]*Session),
		maxIdleDuration: time.Duration(idle) * time.Second,
	}

	// runtime.HandleTicker(func(t time.Time, ct uint64) error {
	// 	p.Purge()

	// 	return nil
	// })

	return p
}

func (p *Pool) Put(sess *Session) {
	p.Lock()
	defer p.Unlock()

	if sess.ID == uuid.Nil {
		sess.ID = uuid.New()
	}

	p.sessions[sess.ID] = sess
	p.conns[sess.conn] = sess
	if sess.Key != "" {
		p.keys[sess.Key] = sess
	}

	sess.pool = p
}

func (p *Pool) GetByID(id uuid.UUID) *Session {
	sess, ok := p.sessions[id]
	if !ok {
		return nil
	}

	return sess
}

func (p *Pool) GetByConn(conn *net.TCPConn) *Session {
	sess, ok := p.conns[conn]
	if !ok {
		return nil
	}

	return sess
}

func (p *Pool) GetByKey(key string) *Session {
	sess, ok := p.keys[key]
	if !ok {
		return nil
	}

	return sess
}

func (p *Pool) RemoveByID(id uuid.UUID) bool {
	sess, ok := p.sessions[id]
	if !ok {
		return false
	}

	delete(p.sessions, id)
	delete(p.conns, sess.conn)
	if sess.Key != "" {
		delete(p.keys, sess.Key)
	}

	sess.pool = nil

	return true
}

func (p *Pool) Length() int {
	p.RLock()
	defer p.RUnlock()

	return len(p.sessions)
}

func (p *Pool) Purge() {
	if p.maxIdleDuration <= 0 {
		return
	}

	p.Lock()
	defer p.Unlock()

	now := time.Now()
	for _, sess := range p.sessions {
		if now.Sub(sess.LastActive) > p.maxIdleDuration {
			logger.Logger.Debug(
				"TCP connection idle for a long time",
				"session", sess.ID,
				"remote_address", sess.conn.RemoteAddr().String(),
			)

			sess.Close()
		}
	}
}

func (p *Pool) Foreach(f func(sess *Session)) {
	p.RLock()
	defer p.RUnlock()

	for _, sess := range p.sessions {
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
