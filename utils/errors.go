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
 * @file errors.go
 * @package utils
 * @author Dr.NP <np@herewe.tech>
 * @since 09/04/2026
 */

package utils

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// Well-known status codes. Applications should register their own codes
// via RegisterErrorCode (4xxx domain recommended); 0 stays success.
const (
	CodeInvalidArgument = 4000
	CodeUnauthorized    = 4001
	CodeForbidden       = 4003
	CodeNotFound        = 4004
	CodeConflict        = 4009
	CodeInternal        = 5000
)

// HTTP status codes without importing a web framework (the legacy
// Envelope in http.go still uses fiber; new code should use these).
const (
	StatusOK                  = 200
	StatusBadRequest          = 400
	StatusUnauthorized        = 401
	StatusForbidden           = 403
	StatusNotFound            = 404
	StatusConflict            = 409
	StatusInternalServerError = 500
)

type errorCodeInfo struct {
	message string
	status  int
}

var (
	codeMu    sync.RWMutex
	codeInfos = map[int]errorCodeInfo{
		CodeOK:              {MsgOK, StatusOK},
		CodeInvalidArgument: {"invalid argument", StatusBadRequest},
		CodeUnauthorized:    {"unauthorized", StatusUnauthorized},
		CodeForbidden:       {"forbidden", StatusForbidden},
		CodeNotFound:        {"not found", StatusNotFound},
		CodeConflict:        {"conflict", StatusConflict},
		CodeInternal:        {"internal error", StatusInternalServerError},
	}
)

// RegisterErrorCode adds a business code mapping. Duplicate registration
// returns an error and keeps the first entry.
func RegisterErrorCode(code int, message string, httpStatus int) error {
	codeMu.Lock()
	defer codeMu.Unlock()

	if _, exists := codeInfos[code]; exists {
		return fmt.Errorf("utils: error code %d already registered", code)
	}

	codeInfos[code] = errorCodeInfo{message: message, status: httpStatus}

	return nil
}

// MessageOf returns the registered message for code, or "" when unknown.
func MessageOf(code int) string {
	codeMu.RLock()
	defer codeMu.RUnlock()

	return codeInfos[code].message
}

// StatusOf returns the registered HTTP status for code, or 500 when unknown.
func StatusOf(code int) int {
	codeMu.RLock()
	defer codeMu.RUnlock()

	if info, ok := codeInfos[code]; ok && info.status != 0 {
		return info.status
	}

	return StatusInternalServerError
}

// CodedError is an error carrying a business code. It unwraps to Err for
// errors.Is/As checks.
type CodedError struct {
	Code int
	Msg  string
	Err  error
}

// Error returns the error string.
func (e *CodedError) Error() string {
	if e == nil {
		return "<nil>"
	}

	msg := e.Msg
	if msg == "" {
		msg = MessageOf(e.Code)
	}

	if e.Err != nil {
		return fmt.Sprintf("code %d: %s: %v", e.Code, msg, e.Err)
	}

	return fmt.Sprintf("code %d: %s", e.Code, msg)
}

// Unwrap returns the wrapped cause.
func (e *CodedError) Unwrap() error { return e.Err }

// NewCodedError builds a *CodedError with the registered message when msg
// is empty.
func NewCodedError(code int, msg string, err error) *CodedError {
	return &CodedError{Code: code, Msg: msg, Err: err}
}

// CodeOf walks the unwrap chain and returns the first *CodedError code,
// or CodeInternal when none carries one.
func CodeOf(err error) int {
	var ce *CodedError
	if errors.As(err, &ce) && ce != nil {
		return ce.Code
	}

	return CodeInternal
}

// EnvelopeT is the generic successor of Envelope (which stays untouched
// for compatibility). Data is typed; Pagination is reused as-is.
type EnvelopeT[T any] struct {
	Code       int         `json:"code"                 xml:"code"                 yaml:"code"`
	Status     int         `json:"status"               xml:"status"               yaml:"status"`
	Timestamp  time.Time   `json:"timestamp"            xml:"timestamp"            yaml:"timestamp"`
	Message    string      `json:"message"              xml:"message"              yaml:"message"`
	RequestID  string      `json:"request_id,omitempty" xml:"request_id,omitempty" yaml:"request_id,omitempty"`
	Pagination *Pagination `json:"pagination,omitempty" xml:"pagination,omitempty" yaml:"pagination,omitempty"`
	Data       T           `json:"data,omitempty"       xml:"data,omitempty"       yaml:"data,omitempty"`
}

// NewEnvelopeT builds an envelope for code, filling message/status from
// the registry when msg is empty.
func NewEnvelopeT[T any](code int, msg string, data T) *EnvelopeT[T] {
	if msg == "" {
		msg = MessageOf(code)
	}

	return &EnvelopeT[T]{
		Code:      code,
		Status:    StatusOf(code),
		Timestamp: time.Now(),
		Message:   msg,
		Data:      data,
	}
}

// OkT builds a success envelope.
func OkT[T any](data T) *EnvelopeT[T] {
	return NewEnvelopeT(CodeOK, MsgOK, data)
}

// FailT builds an error envelope for code (data stays zero).
func FailT[T any](code int, msg string) *EnvelopeT[T] {
	var zero T

	return NewEnvelopeT(code, msg, zero)
}

// WithRequestID attaches a request ID for tracing correlation.
func (e *EnvelopeT[T]) WithRequestID(id string) *EnvelopeT[T] {
	e.RequestID = id

	return e
}

// WithPagination attaches pagination metadata.
func (e *EnvelopeT[T]) WithPagination(p *Pagination) *EnvelopeT[T] {
	e.Pagination = p

	return e
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
