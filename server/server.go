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
	"maps"
	"net"
	"sync"

	"github.com/google/uuid"

	"github.com/go-sicky/sicky/utils"
)

// Server : server abstraction.
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
	// Obtain port
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
	servers       = make(map[uuid.UUID]Server)
	defaultServer Server
	srvMu         sync.RWMutex
)

// Set registers server instances; the first one becomes the default.
func Set(srvs ...Server) {
	srvMu.Lock()
	defer srvMu.Unlock()

	for _, srv := range srvs {
		servers[srv.ID()] = srv
		if defaultServer == nil {
			defaultServer = srv
		}
	}
}

// Get looks up a server instance by ID.
func Get(id uuid.UUID) Server {
	srvMu.RLock()
	defer srvMu.RUnlock()

	return servers[id]
}

// Default returns the default server instance.
func Default() Server {
	srvMu.RLock()
	defer srvMu.RUnlock()

	return defaultServer
}

// Servers returns a copy of the server registry.
func Servers() map[uuid.UUID]Server {
	srvMu.RLock()
	defer srvMu.RUnlock()

	out := make(map[uuid.UUID]Server, len(servers))
	maps.Copy(out, servers)

	return out
}

// Clear resets the global registry. Intended for tests.
func Clear() {
	srvMu.Lock()
	defer srvMu.Unlock()

	servers = make(map[uuid.UUID]Server)
	defaultServer = nil
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
