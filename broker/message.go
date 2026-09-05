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
	"errors"
	"fmt"

	"github.com/vmihailenco/msgpack/v5"
	"google.golang.org/protobuf/proto"

	"github.com/go-sicky/sicky/utils"
)

var (
	// ErrScanNotProtoMessage signals Scan was given a non-proto.Message
	// target for a protobuf payload. Match with errors.Is.
	ErrScanNotProtoMessage = errors.New("broker: scan target is not a proto.Message")

	// ErrFormatNotProtoMessage signals Format was given a non-proto.Message
	// value for a protobuf mime. Match with errors.Is.
	ErrFormatNotProtoMessage = errors.New("broker: format value is not a proto.Message")

	// ErrFormatNotBytes signals Format was given a non-[]byte value for
	// the raw mime. Match with errors.Is.
	ErrFormatNotBytes = errors.New("broker: raw format value is not []byte")
)

const (
	// Message data type
	// MsgRaw is a broker constant.
	MsgRaw = iota
	// MsgJSON is a broker constant.
	MsgJSON
	// MsgMessagePack is a broker constant.
	MsgMessagePack
	// MsgProtobuf is a broker constant.
	MsgProtobuf
)

const (
	// Deprecated: misspelled, use MsgJSON.
	MsgJson = MsgJSON
	// Deprecated: misspelled, use MsgMessagePack.
	MsgMsgpack = MsgMessagePack
)

const (
	// Message mime
	// MsgRawMime is a broker constant.
	MsgRawMime = "application/octet-stream"
	// MsgJSONMime is a broker constant.
	MsgJSONMime = "application/json"
	// MsgMessagePackMime is a broker constant.
	MsgMessagePackMime = "application/x-msgpack"
	// MsgProtobufMime is a broker constant.
	MsgProtobufMime = "application/x-protobuf"
)

const (
	// Deprecated: misspelled, use MsgJSONMime.
	MsgJsonMime = MsgJSONMime
	// Deprecated: misspelled, use MsgMessagePackMime.
	MsgMsgpackMime = MsgMessagePackMime
)

// Message is a broker component.
type Message struct {
	// Header
	Metadata utils.Metadata `json:"metadata,omitempty" msgpack:"metadata,omitempty"`
	Mime     int            `json:"mime"               msgpack:"mime"`
	TraceID  string         `json:"trace_id,omitempty" msgpack:"trace_id,omitempty"`
	Topic    string         `json:"topic,omitempty"    msgpack:"topic,omitempty"`

	// Content
	Body []byte `json:"body,omitempty" msgpack:"body,omitempty"`
}

// Scan decodes the message body into v according to the message mime.
// Raw messages are a passthrough and return nil. Decoding failures are
// wrapped with topic/mime context; use errors.Is/As on the cause.
func (m *Message) Scan(v any) error {
	switch m.Mime {
	case MsgJSON:
		if err := json.Unmarshal(m.Body, v); err != nil {
			return fmt.Errorf("broker: scan json (topic %q mime %d): %w", m.Topic, m.Mime, err)
		}

		return nil
	case MsgProtobuf:
		pm, ok := v.(proto.Message)
		if !ok {
			return fmt.Errorf("broker: scan protobuf (topic %q): %w", m.Topic, ErrScanNotProtoMessage)
		}

		if err := proto.Unmarshal(m.Body, pm); err != nil {
			return fmt.Errorf("broker: scan protobuf (topic %q mime %d): %w", m.Topic, m.Mime, err)
		}

		return nil
	case MsgMessagePack:
		if err := msgpack.Unmarshal(m.Body, v); err != nil {
			return fmt.Errorf("broker: scan msgpack (topic %q mime %d): %w", m.Topic, m.Mime, err)
		}

		return nil
	default:
		// Raw
	}

	return nil
}

// Format encodes v into the message body and records the effective mime.
// A non-proto.Message value with the protobuf mime, or a non-[]byte value
// with the raw mime, now returns an error instead of silently producing
// an empty body.
func (m *Message) Format(v any, mime ...int) error {
	tm := MsgJSON
	if len(mime) > 0 {
		tm = mime[0]
	}

	var err error
	switch tm {
	case MsgJSON:
		m.Body, err = json.Marshal(v)
	case MsgProtobuf:
		pm, ok := v.(proto.Message)
		if !ok {
			return fmt.Errorf("broker: format protobuf (topic %q): %w", m.Topic, ErrFormatNotProtoMessage)
		}

		m.Body, err = proto.Marshal(pm)
	case MsgMessagePack:
		m.Body, err = msgpack.Marshal(v)
	default:
		// Raw
		if b, ok := v.([]byte); ok {
			m.Body = b
			tm = MsgRaw
		} else {
			return fmt.Errorf("broker: format raw (topic %q): %w", m.Topic, ErrFormatNotBytes)
		}
	}

	// Always record the effective mime: previously the JSON/msgpack
	// paths returned early with Mime left at zero (Raw), so Scan on the
	// receiving side silently skipped decoding (data loss with nil error).
	m.Mime = tm

	return err
}

// Raw is part of the public API.
//
// A msgpack marshal failure (e.g. an unmarshalable Body) falls back to the
// original wire bytes so publishers never silently send an empty payload.
func (m *Message) Raw() []byte {
	b, err := msgpack.Marshal(m)
	if err != nil && m.Body != nil {
		return m.Body
	}

	return b
}

// NewMessage creates a new Message.
// Corrupt wire bytes fall back to a raw message preserving the input
// instead of a half-zero struct, so no data is silently dropped.
func NewMessage(raw []byte) *Message {
	m := new(Message)
	if raw != nil {
		if err := msgpack.Unmarshal(raw, m); err != nil {
			return &Message{Body: raw, Mime: MsgRaw}
		}
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
