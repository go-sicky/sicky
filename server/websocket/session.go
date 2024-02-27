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
 * @file session.go
 * @package websocket
 * @author Dr.NP <np@herewe.tech>
 * @since 02/11/2023
 */

package websocket

import (
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
)

type Session struct {
	tag      string
	conn     *websocket.Conn
	linkTime time.Time
}

var store sync.Map

func NewSession(tag string, conn *websocket.Conn) *Session {
	sess := &Session{
		tag:      tag,
		conn:     conn,
		linkTime: time.Now(),
	}

	store.Store(tag, sess)

	return sess
}

func GetSession(tag string) *Session {
	d, ok := store.Load(tag)
	if ok {
		sess, sok := d.(*Session)
		if sok {
			return sess
		}
	}

	return nil
}

func DeleteSession(tag string) {
	store.Delete(tag)
}

func (s *Session) Send(data []byte) error {
	return s.conn.WriteMessage(websocket.BinaryMessage, data)
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
