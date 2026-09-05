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
	"time"

	"github.com/gofiber/fiber/v2"
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
		Status:    fiber.StatusOK,
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

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
