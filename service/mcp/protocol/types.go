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
 * @file types.go
 * @package protocol
 * @author Dr.NP <np@herewe.tech>
 * @since 07/06/2026
 */

package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

// JSONRPCVersion is a protocol constant.
const JSONRPCVersion = "2.0"

var (
	// ErrInvalidRequest is a shared protocol value.
	ErrInvalidRequest = errors.New("invalid request")
	// ErrParseError is a shared protocol value.
	ErrParseError = errors.New("parse error")
	// ErrMethodNotFound is a shared protocol value.
	ErrMethodNotFound = errors.New("method not found")
	// ErrInvalidParams is a shared protocol value.
	ErrInvalidParams = errors.New("invalid params")
	// ErrInternalError is a shared protocol value.
	ErrInternalError = errors.New("internal error")
	// ErrServerNotInitialized is a shared protocol value.
	ErrServerNotInitialized = errors.New("server not initialized")
)

// JSON-RPC 2.0 error codes.
const (
	ErrCodeParseError     = -32700
	ErrCodeInvalidRequest = -32600
	ErrCodeMethodNotFound = -32601
	ErrCodeInvalidParams  = -32602
	ErrCodeInternalError  = -32603
)

// Request is a protocol component.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *RequestID      `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// RequestID is a protocol component.
type RequestID struct {
	value string
	isNum bool
}

// NewStringID creates a new StringID.
func NewStringID(s string) *RequestID { return &RequestID{value: s} }

// NewNumberID creates a new NumberID.
func NewNumberID(n int64) *RequestID { return &RequestID{value: strconv.FormatInt(n, 10), isNum: true} }

// MarshalJSON encodes to JSON.
func (r *RequestID) MarshalJSON() ([]byte, error) {
	if r.isNum {
		n, err := strconv.ParseInt(r.value, 10, 64)
		if err != nil {
			return nil, err
		}

		return json.Marshal(n)
	}

	return json.Marshal(r.value)
}

// UnmarshalJSON decodes from JSON.
func (r *RequestID) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		r.value = ""

		return nil
	}

	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		r.value = s
		r.isNum = false

		return nil
	}

	var n int64
	if err := json.Unmarshal(data, &n); err == nil {
		r.value = strconv.FormatInt(n, 10)
		r.isNum = true

		return nil
	}

	return fmt.Errorf("requestID must be string or number, got %s", string(data))
}

// String returns a human-readable name.
func (r *RequestID) String() string { return r.value }

// Int64 returns the numeric ID.
func (r *RequestID) Int64() int64 { n, _ := strconv.ParseInt(r.value, 10, 64); return n }

// IsNum reports whether the ID is numeric.
func (r *RequestID) IsNum() bool { return r.isNum }

// Response is a protocol component.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *RequestID      `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *ResponseError  `json:"error,omitempty"`
}

// NewResponse creates a new Response.
func NewResponse(id *RequestID, result any) *Response {
	data, _ := json.Marshal(result)

	return &Response{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Result:  data,
	}
}

// NewErrorResponse creates a new ErrorResponse.
func NewErrorResponse(id *RequestID, code int, message string) *Response {
	return &Response{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Error: &ResponseError{
			Code:    code,
			Message: message,
		},
	}
}

// NewErrorResponseWithData creates a new ErrorResponseWithData.
func NewErrorResponseWithData(id *RequestID, code int, message string, data any) *Response {
	return &Response{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Error: &ResponseError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}

// ResponseError is a protocol component.
type ResponseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Error returns the error string.
func (e *ResponseError) Error() string {
	return fmt.Sprintf("JSON-RPC error %d: %s", e.Code, e.Message)
}

// Notification is a protocol component.
type Notification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// ParseRequest parses a JSON-RPC request.
func ParseRequest(data []byte) (*Request, error) {
	req := &Request{}
	if err := json.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrParseError, err)
	}

	if req.JSONRPC != JSONRPCVersion {
		return nil, fmt.Errorf("%w: invalid jsonrpc version %q", ErrInvalidRequest, req.JSONRPC)
	}

	if req.Method == "" {
		return nil, fmt.Errorf("%w: missing method", ErrInvalidRequest)
	}

	return req, nil
}

// ParseNotification parses a JSON-RPC notification.
func ParseNotification(data []byte) (*Notification, error) {
	notif := &Notification{}
	if err := json.Unmarshal(data, notif); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrParseError, err)
	}

	if notif.JSONRPC != JSONRPCVersion {
		return nil, fmt.Errorf("%w: invalid jsonrpc version %q", ErrInvalidRequest, notif.JSONRPC)
	}

	if notif.Method == "" {
		return nil, fmt.Errorf("%w: missing method", ErrInvalidRequest)
	}

	return notif, nil
}

// IsNotification reports whether the message is a notification.
func IsNotification(data []byte) bool {
	var check struct {
		ID *json.RawMessage `json:"id"`
	}

	_ = json.Unmarshal(data, &check)

	return check.ID == nil
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
