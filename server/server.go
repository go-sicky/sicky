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
 * @file server.go
 * @package server
 * @author Dr.NP <np@herewe.tech>
 * @since 11/20/2023
 */

package server

import (
	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc"
)

type HandlerGRPC struct {
	Desc     *grpc.ServiceDesc
	Instance interface{}
}

// Handler : server handler
type Handler struct {
	grpc []*HandlerGRPC
}

func NewHandler() *Handler {
	return new(Handler)
}

func (h *Handler) RegisterGRPC(d *grpc.ServiceDesc, ins interface{}) {
	h.grpc = append(h.grpc, &HandlerGRPC{Desc: d, Instance: ins})
}

func (h *Handler) GRPC() []*HandlerGRPC {
	return h.grpc
}

type HandlerHTTP interface {
	Register(*fiber.App)
}

// Server : server abstraction
type Server interface {
	// Server options
	Options() *Options
	// Server handler register
	Handle(*Handler) error
	// Start the server
	Start() error
	// Stop the server
	Stop() error
	// Stringify
	String() string
	// Get name
	Name() string
	// RegisterService : for GRPC
	RegisterService(*grpc.ServiceDesc, any)
	// RegisterHandler : for HTTP
	RegisterHandler(HandlerHTTP)
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */