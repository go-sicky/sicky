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
	"sync"
	"time"

	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/runtime"
	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/utils"
	"github.com/gofiber/contrib/websocket"
	"github.com/google/uuid"
)

/* {{{ [Session] */
type Session struct {
	server.SessionBase

	conn *websocket.Conn
	pool *Pool
}

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

func (s *Session) Send(mt int, data []byte) error {
	s.LastActive = time.Now()
	if mt <= 0 {
		mt = websocket.TextMessage
	}

	return s.conn.WriteMessage(mt, data)
}

func (s *Session) Close() error {
	if s.pool != nil {
		s.pool.RemoveByID(s.ID)
	}

	// Send close frame
	err := s.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "timeout"))
	if err != nil {
		// Close force
		nc := s.conn.NetConn()
		if nc != nil {
			nc.Close()
		}
	}

	return s.conn.Close()
}

func (s *Session) Conn() *websocket.Conn {
	return s.conn
}

func (s *Session) SetKey(key string) {
	if s.pool != nil {
		// Refresh pool
		if s.Key != "" {
			s.pool.Lock()
			delete(s.pool.keys, s.Key)
			s.pool.Unlock()
		}

		s.pool.keys[key] = s
	}

	s.Key = key
}

/* }}} */

/* {{{ [Pool] */
var SessionPool *Pool

type Pool struct {
	sync.RWMutex

	id              uuid.UUID
	sessions        map[uuid.UUID]*Session
	conns           map[*websocket.Conn]*Session
	keys            map[string]*Session
	pingDuration    time.Duration
	maxIdleDuration time.Duration
}

func NewPool(ping, idle int) *Pool {
	p := &Pool{
		id:              uuid.New(),
		sessions:        make(map[uuid.UUID]*Session),
		conns:           make(map[*websocket.Conn]*Session),
		keys:            make(map[string]*Session),
		pingDuration:    time.Duration(ping) * time.Second,
		maxIdleDuration: time.Duration(idle) * time.Second,
	}

	runtime.HandleTicker(func(t time.Time, ct uint64) error {
		p.Purge()

		return nil
	})

	if SessionPool == nil {
		SessionPool = p
	}

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

func (p *Pool) GetByConn(conn *websocket.Conn) *Session {
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

func (p *Pool) RemoveByConn(conn *websocket.Conn) bool {
	sess, ok := p.conns[conn]
	if !ok {
		return false
	}

	delete(p.sessions, sess.ID)
	delete(p.conns, conn)
	if sess.Key != "" {
		delete(p.keys, sess.Key)
	}

	sess.pool = nil

	return true
}

func (p *Pool) RemoveByKey(key string) bool {
	sess, ok := p.keys[key]
	if !ok {
		return false
	}

	delete(p.sessions, sess.ID)
	delete(p.conns, sess.conn)
	delete(p.keys, key)

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
		if now.Sub(sess.LastActive) > p.pingDuration {
			// Write ping
			err := sess.conn.WriteMessage(websocket.PingMessage, nil)
			if err != nil {
				logger.Logger.Error(
					"Websocket write ping message failed",
					"session", sess.ID,
					"error", err.Error(),
				)
			}
		}

		if now.Sub(sess.LastActive) > p.maxIdleDuration {
			logger.Logger.Debug(
				"Websocket connection idle for a long time",
				"session", sess.ID,
				"remote_address", sess.conn.RemoteAddr().String(),
			)

			err := sess.Close()
			if err != nil {
				logger.Logger.Error(
					"Websocket connection close failed",
					"session", sess.ID,
					"error", err.Error(),
				)
			}
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
