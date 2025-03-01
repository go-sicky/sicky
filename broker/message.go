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
 * @file message.go
 * @package broker
 * @author Dr.NP <np@herewe.tech>
 * @since 08/18/2024
 */

package broker

import (
	"encoding/json"
	"strings"

	"github.com/go-sicky/sicky/utils"
	"github.com/vmihailenco/msgpack/v5"
	"google.golang.org/protobuf/proto"
)

type Message struct {
	// Header
	Metadata utils.Metadata `msgpack:"metadata,omitempty" json:"metadata,omitempty"`
	Mime     string         `msgpack:"mime,omitempty" json:"mime,omitempty"`
	TraceID  string         `msgpack:"trace_id,omitempty" json:"trace_id,omitempty"`
	Topic    string         `msgpack:"topic,omitempty" json:"topic,omitempty"`

	// Content
	Body []byte `msgpack:"body,omitempty" json:"body,omitempty"`
}

func (m *Message) Scan(v any) {
	switch strings.ToLower(m.Mime) {
	case "application/json":
		json.Unmarshal(m.Body, v)
	case "application/vnd.google.protobuf", "application/x-protobuf", "application/protobuf":
		pm, ok := v.(proto.Message)
		if ok {
			proto.Unmarshal(m.Body, pm)
		}
	case "application/x-msgpack", "application/msgpack":
		msgpack.Unmarshal(m.Body, v)
	default:
		// Raw
	}
}

func (m *Message) Format(v any, mime ...string) {
	tm := "application/json"
	if len(mime) > 0 {
		tm = mime[0]
	}

	tm = strings.ToLower(tm)
	switch tm {
	case "application/json":
		m.Body, _ = json.Marshal(v)
	case "application/vnd.google.protobuf", "application/x-protobuf", "application/protobuf":
		pm, ok := v.(proto.Message)
		if ok {
			m.Body, _ = proto.Marshal(pm)
		}
	case "application/x-msgpack", "application/msgpack":
		m.Body, _ = msgpack.Marshal(v)
	default:
		// Raw
	}

	m.Mime = tm
}

func (m *Message) Raw() []byte {
	b, _ := msgpack.Marshal(m)

	return b
}

func NewMessage(raw []byte) *Message {
	m := new(Message)
	m.Metadata = utils.NewMetadata()
	if raw != nil {
		msgpack.Unmarshal(raw, m)
	}

	return m
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
