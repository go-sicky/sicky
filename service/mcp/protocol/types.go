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

const JSONRPCVersion = "2.0"

var (
	ErrInvalidRequest       = errors.New("invalid request")
	ErrParseError           = errors.New("parse error")
	ErrMethodNotFound       = errors.New("method not found")
	ErrInvalidParams        = errors.New("invalid params")
	ErrInternalError        = errors.New("internal error")
	ErrServerNotInitialized = errors.New("server not initialized")
)

// JSON-RPC 2.0 error codes
const (
	ErrCodeParseError     = -32700
	ErrCodeInvalidRequest = -32600
	ErrCodeMethodNotFound = -32601
	ErrCodeInvalidParams  = -32602
	ErrCodeInternalError  = -32603
)

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *RequestID      `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type RequestID struct {
	value string
	isNum bool
}

func NewStringID(s string) *RequestID { return &RequestID{value: s} }
func NewNumberID(n int64) *RequestID  { return &RequestID{value: strconv.FormatInt(n, 10), isNum: true} }

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

	return fmt.Errorf("RequestID must be string or number, got %s", string(data))
}

func (r *RequestID) String() string { return r.value }
func (r *RequestID) Int64() int64   { n, _ := strconv.ParseInt(r.value, 10, 64); return n }
func (r *RequestID) IsNum() bool    { return r.isNum }

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *RequestID      `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *ResponseError  `json:"error,omitempty"`
}

func NewResponse(id *RequestID, result interface{}) *Response {
	data, _ := json.Marshal(result)

	return &Response{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Result:  data,
	}
}

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

func NewErrorResponseWithData(id *RequestID, code int, message string, data interface{}) *Response {
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

type ResponseError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (e *ResponseError) Error() string {
	return fmt.Sprintf("JSON-RPC error %d: %s", e.Code, e.Message)
}

type Notification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func ParseRequest(data []byte) (*Request, error) {
	req := &Request{}
	if err := json.Unmarshal(data, req); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrParseError, err.Error())
	}

	if req.JSONRPC != JSONRPCVersion {
		return nil, fmt.Errorf("%w: invalid jsonrpc version %q", ErrInvalidRequest, req.JSONRPC)
	}

	if req.Method == "" {
		return nil, fmt.Errorf("%w: missing method", ErrInvalidRequest)
	}

	return req, nil
}

func ParseNotification(data []byte) (*Notification, error) {
	notif := &Notification{}
	if err := json.Unmarshal(data, notif); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrParseError, err.Error())
	}

	if notif.JSONRPC != JSONRPCVersion {
		return nil, fmt.Errorf("%w: invalid jsonrpc version %q", ErrInvalidRequest, notif.JSONRPC)
	}

	if notif.Method == "" {
		return nil, fmt.Errorf("%w: missing method", ErrInvalidRequest)
	}

	return notif, nil
}

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
