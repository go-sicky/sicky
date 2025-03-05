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
 * @file server.go
 * @package server
 * @author Dr.NP <np@herewe.tech>
 * @since 11/20/2023
 */

package server

import (
	"context"
	"net"

	"github.com/go-sicky/sicky/utils"
	"github.com/google/uuid"
)

// Server : server abstraction
type Server interface {
	// Get context
	Context() context.Context
	// Server options
	Options() *Options
	// Stringify
	String() string
	// Server ID
	ID() uuid.UUID
	// Server name
	Name() string
	// Start the server
	Start() error
	// Stop the server
	Stop() error
	// Server is running
	Running() bool
	// Obtain address
	Addr() net.Addr
	// Obtain IP
	IP() net.IP
	// Optain port
	Port() int
	// Obtain advertise address
	AdvertiseAddr() net.Addr
	// Advertise IP
	AdvertiseIP() net.IP
	// Advertise port
	AdvertisePort() int
	// Metadata
	Metadata() utils.Metadata
}

var (
	servers = make(map[uuid.UUID]Server)
)

func Instance(id uuid.UUID, srv ...Server) Server {
	if len(srv) > 0 {
		servers[id] = srv[0]

		return srv[0]
	}

	return servers[id]
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
