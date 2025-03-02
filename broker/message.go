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

	"github.com/go-sicky/sicky/utils"
	"github.com/vmihailenco/msgpack/v5"
	"google.golang.org/protobuf/proto"
)

const (
	// Message data type
	MsgRaw = iota
	MsgJson
	MsgMsgpack
	MsgProtobuf
)

const (
	// Message mime
	MsgRawMime      = "application/octet-stream"
	MsgJsonMime     = "application/json"
	MsgMsgpackMime  = "application/x-msgpack"
	MsgProtobufMime = "application/x-protobuf"
)

type Message struct {
	// Header
	Metadata utils.Metadata `msgpack:"metadata,omitempty" json:"metadata,omitempty"`
	Mime     int            `msgpack:"mime" json:"mime"`
	TraceID  string         `msgpack:"trace_id,omitempty" json:"trace_id,omitempty"`
	Topic    string         `msgpack:"topic,omitempty" json:"topic,omitempty"`

	// Content
	Body []byte `msgpack:"body,omitempty" json:"body,omitempty"`
}

func (m *Message) Scan(v any) {
	switch m.Mime {
	case MsgJson:
		json.Unmarshal(m.Body, v)
	case MsgProtobuf:
		pm, ok := v.(proto.Message)
		if ok {
			proto.Unmarshal(m.Body, pm)
		}
	case MsgMsgpack:
		msgpack.Unmarshal(m.Body, v)
	default:
		// Raw
	}
}

func (m *Message) Format(v any, mime ...int) {
	tm := MsgJson
	if len(mime) > 0 {
		tm = mime[0]
	}

	switch tm {
	case MsgJson:
		m.Body, _ = json.Marshal(v)
	case MsgProtobuf:
		pm, ok := v.(proto.Message)
		if ok {
			m.Body, _ = proto.Marshal(pm)
		}
	case MsgMsgpack:
		m.Body, _ = msgpack.Marshal(v)
	default:
		// Raw
		b, ok := v.([]byte)
		if ok {
			m.Body = b
			tm = MsgRaw
		}
	}

	m.Mime = tm
}

func (m *Message) Raw() []byte {
	b, _ := msgpack.Marshal(m)

	return b
}

func NewMessage(raw []byte) *Message {
	m := new(Message)
	if raw != nil {
		msgpack.Unmarshal(raw, m)
	} else {
		m.Metadata = utils.NewMetadata()
		m.Mime = MsgRaw
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
