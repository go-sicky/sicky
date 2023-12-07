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
	CodeOK = 0
	MsgOK  = "OK"
)

type Envelope struct {
	Code      int         `json:"code"`
	Status    int         `json:"status"`
	Timestamp time.Time   `json:"timestamp"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
}

func WrapHTTPResponse(data interface{}) *Envelope {
	e := &Envelope{
		Code:      CodeOK,
		Status:    fiber.StatusOK,
		Timestamp: time.Now(),
		Message:   MsgOK,
		Data:      data,
	}

	return e
}

func (e *Envelope) SetCode(code int) *Envelope {
	e.Code = code

	return e
}

func (e *Envelope) SetStatus(status int) *Envelope {
	e.Status = status

	return e
}

func (e *Envelope) SetMessage(msg string) *Envelope {
	e.Message = msg

	return e
}

func (e *Envelope) SetData(data interface{}) *Envelope {
	e.Data = data

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
