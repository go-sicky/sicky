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
 * @file http.go
 * @package utils
 * @author Dr.NP <np@herewe.tech>
 * @since 11/29/2023
 */

package utils

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/uptrace/bunrouter"
	"github.com/vmihailenco/msgpack/v5"
	"go.yaml.in/yaml/v3"
)

const (
	// CodeOK is a utils constant.
	CodeOK = 0
	// MsgOK is a utils constant.
	MsgOK = "OK"
)

// Envelope is a utils component.
type Envelope struct {
	Code       int         `json:"code"                 xml:"code"                 yaml:"code"`
	Status     int         `json:"status"               xml:"status"               yaml:"status"`
	Timestamp  time.Time   `json:"timestamp"            xml:"timestamp"            yaml:"timestamp"`
	Message    string      `json:"message"              xml:"message"              yaml:"message"`
	RequestID  string      `json:"request_id,omitempty" xml:"request_id,omitempty" yaml:"request_id,omitempty"`
	Pagination *Pagination `json:"pagination,omitempty" xml:"pagination,omitempty" yaml:"pagination,omitempty"`
	Data       any         `json:"data,omitempty"       xml:"data,omitempty"       yaml:"data,omitempty"`
}

// WrapHTTPResponse wraps data in a response envelope.
func WrapHTTPResponse(data any) *Envelope {
	e := &Envelope{
		Code:      CodeOK,
		Status:    http.StatusOK,
		Timestamp: time.Now(),
		Message:   MsgOK,
		Data:      data,
	}

	return e
}

// SetCode sets the response code.
func (e *Envelope) SetCode(code int) *Envelope {
	e.Code = code

	return e
}

// SetStatus sets the HTTP status.
func (e *Envelope) SetStatus(status int) *Envelope {
	e.Status = status

	return e
}

// SetMessage sets the response message.
func (e *Envelope) SetMessage(msg string) *Envelope {
	e.Message = msg

	return e
}

// SetRequestID sets the request ID.
func (e *Envelope) SetRequestID(requestID string) *Envelope {
	e.RequestID = requestID

	return e
}

// SetData sets the response payload.
func (e *Envelope) SetData(data any) *Envelope {
	e.Data = data

	return e
}

// Pagination is a utils component.
type Pagination struct {
	Total   int64 `json:"total"   xml:"total"   yaml:"total"`
	Limit   int64 `json:"limit"   xml:"limit"   yaml:"limit"`
	Offset  int64 `json:"offset"  xml:"offset"  yaml:"offset"`
	Current int64 `json:"current" xml:"current" yaml:"current"`
	Pages   int64 `json:"pages"   xml:"pages"   yaml:"pages"`
}

const (
	// mediaTypeJSON is both the negotiated default and the fallback for
	// an unknown Accept header.
	mediaTypeJSON    = "application/json"
	mediaTypeXML     = "application/xml"
	mediaTypeYAML    = "application/x-yaml"
	mediaTypeMsgpack = "application/x-msgpack"
	// mediaTypePlainText keeps scalar payloads verbatim.
	mediaTypePlainText = "text/plain; charset=utf-8"
)

// ErrUnsupportedBindTarget signals AutoBind was handed a target the media
// type cannot decode into: a text/plain body with anything but a *string or
// *[]byte destination. Match with errors.Is.
var ErrUnsupportedBindTarget = errors.New("utils: auto bind target unsupported for media type")

// AutoFormatData renders data for the request's Accept header.
//
// Negotiation runs on the media type: parameters (charset) and quality
// values are stripped, so an ordinary header like
// "application/json; charset=utf-8" finally matches instead of falling
// through to the plain-text branch. JSON is the fallback for an empty,
// wildcard or unknown Accept: the old default was fmt.Sprintf("%v"),
// which prints unexported fields that json.Marshal would never expose -
// private state holding credentials would have gone over the wire.
func AutoFormatData(req *http.Request, data any) (contentType string, body []byte, err error) {
	switch negotiateAccept(req.Header.Get("Accept")) {
	case mediaTypeXML, "text/xml":
		xmlData, err := xml.Marshal(data)
		if err != nil {
			return "", nil, err
		}

		return mediaTypeXML, xmlData, nil
	case mediaTypeYAML, "text/yaml":
		yamlData, err := yaml.Marshal(data)
		if err != nil {
			return "", nil, err
		}

		return mediaTypeYAML, yamlData, nil
	case "text/plain":
		// Negotiation strips parameters, so the case is the bare media
		// type while the answer carries the charset.
		// Scalars stay verbatim; anything structured goes through JSON
		// rather than %v so unexported fields never leak.
		switch v := data.(type) {
		case string:
			return mediaTypePlainText, []byte(v), nil
		case []byte:
			return mediaTypePlainText, v, nil
		}

		fallthrough
	default:
		jsonData, err := json.Marshal(data)
		if err != nil {
			return "", nil, err
		}

		return mediaTypeJSON, jsonData, nil
	}
}

// negotiateAccept returns the media type the client asked for: the first
// usable entry of the Accept header, lower-cased and with its parameters
// and q-value removed. An empty header or a wildcard resolves to JSON.
func negotiateAccept(accept string) string {
	for raw := range strings.SplitSeq(accept, ",") {
		media := strings.ToLower(strings.TrimSpace(strings.SplitN(raw, ";", 2)[0]))
		switch media {
		case "":
			// Empty entry (trailing comma): keep looking.
		case "*/*", "text/*":
			// A wildcard prefers the structured default.
			return mediaTypeJSON
		default:
			return media
		}
	}

	return mediaTypeJSON
}

func AutoFormatBytes(req *http.Request, data any) []byte {
	_, bytes, _ := AutoFormatData(req, data)

	return bytes
}

func BunAutoFormat(w http.ResponseWriter, r bunrouter.Request, data any) error {
	contentType, bytes, err := AutoFormatData(r.Request, data)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", contentType)
	_, err = w.Write(bytes)

	return err
}

// AutoBind decodes the request body into v according to the Content-Type
// header. It is the request-side inverse of AutoFormatData: negotiation runs
// on the media type only - parameters (charset) are stripped - and JSON is
// the fallback for an empty or unknown Content-Type, matching the response
// side's default. Supported codecs are JSON, XML, YAML and msgpack; a
// text/plain body is verbatim and decodes only into *string or *[]byte -
// any other target reports ErrUnsupportedBindTarget.
//
// The body is read once with io.ReadAll and is not size-limited here: the
// server BodyLimit middleware owns request caps.
func AutoBind(req *http.Request, v any) error {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return fmt.Errorf("utils: auto bind read body: %w", err)
	}

	switch mediaTypeOf(req.Header.Get("Content-Type")) {
	case mediaTypeXML, "text/xml":
		if err := xml.Unmarshal(body, v); err != nil {
			return fmt.Errorf("utils: auto bind xml: %w", err)
		}

	case mediaTypeYAML, "text/yaml":
		if err := yaml.Unmarshal(body, v); err != nil {
			return fmt.Errorf("utils: auto bind yaml: %w", err)
		}

	case mediaTypeMsgpack:
		if err := msgpack.Unmarshal(body, v); err != nil {
			return fmt.Errorf("utils: auto bind msgpack: %w", err)
		}

	case "text/plain":
		// No codec to run: the payload stays exactly as sent.
		switch dst := v.(type) {
		case *string:
			*dst = string(body)

		case *[]byte:
			*dst = body

		default:
			return fmt.Errorf("utils: auto bind text/plain: %w", ErrUnsupportedBindTarget)
		}

	default:
		// JSON: the negotiated default and unknown-header fallback.
		if err := json.Unmarshal(body, v); err != nil {
			return fmt.Errorf("utils: auto bind json: %w", err)
		}
	}

	return nil
}

// mediaTypeOf returns the lower-cased media type of a Content-Type header
// value with its parameters (charset etc.) removed. Content-Type is a single
// value, so unlike negotiateAccept there is no list or q-value handling.
func mediaTypeOf(contentType string) string {
	return strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
}

// BunAutoBind decodes the bunrouter request body into v per its
// Content-Type, mirroring BunAutoFormat on the request side.
func BunAutoBind(r bunrouter.Request, v any) error {
	return AutoBind(r.Request, v)
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
