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
 * @file client.go
 * @package client
 * @author Dr.NP <np@herewe.tech>
 * @since 11/20/2023
 */

package client

import (
	"context"
	"maps"
	"sync"

	"github.com/google/uuid"
)

// Client : service callee.
type Client interface {
	// Get context
	Context() context.Context
	// Client options
	Options() *Options
	// Connect
	Connect() error
	// Disconnect
	Disconnect() error
	// Call handle
	Call() error
	// Stringify
	String() string
	// Get name
	Name() string
	// Get ID
	ID() uuid.UUID
}

var (
	clients       = make(map[uuid.UUID]Client, 0)
	defaultClient Client
	cltMu         sync.RWMutex
)

// Set registers client instances; the first one becomes the default.
func Set(clts ...Client) {
	cltMu.Lock()
	defer cltMu.Unlock()

	for _, clt := range clts {
		clients[clt.ID()] = clt
		if defaultClient == nil {
			defaultClient = clt
		}
	}
}

// Get looks up a client instance by ID.
func Get(id uuid.UUID) Client {
	cltMu.RLock()
	defer cltMu.RUnlock()

	return clients[id]
}

// Default returns the default client instance.
func Default() Client {
	cltMu.RLock()
	defer cltMu.RUnlock()

	return defaultClient
}

// Clients returns a copy of the client registry.
func Clients() map[uuid.UUID]Client {
	cltMu.RLock()
	defer cltMu.RUnlock()

	out := make(map[uuid.UUID]Client, len(clients))
	maps.Copy(out, clients)

	return out
}

// Clear resets the global registry. Intended for tests.
func Clear() {
	cltMu.Lock()
	defer cltMu.Unlock()

	clients = make(map[uuid.UUID]Client, 0)
	defaultClient = nil
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
