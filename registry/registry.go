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
 * @file registry.go
 * @package registry
 * @author Dr.NP <np@herewe.tech>
 * @since 08/04/2024
 */

package registry

import (
	"context"

	"github.com/go-sicky/sicky/server"
	"github.com/google/uuid"
)

type Registry interface {
	// Get context
	Context() context.Context
	// Registry options
	Options() *Options
	// Stringify
	String() string
	// Registry ID
	ID() uuid.UUID
	// Registry name
	Name() string
	// Register service
	Register(server.Server) error
	// Deregister service
	Deregister(server.Server) error
	// Check service instance
	CheckInstance(string) bool
	// Watch services
	Watch() error
}

var (
	registries      = make(map[uuid.UUID]Registry)
	defaultRegistry Registry
)

func Set(rgs ...Registry) {
	for _, rg := range rgs {
		registries[rg.ID()] = rg
		if defaultRegistry == nil {
			defaultRegistry = rg
		}
	}
}

func Get(id uuid.UUID) Registry {
	return registries[id]
}

func Default() Registry {
	return defaultRegistry
}

/* {{{ [Helpers] */
/* }}} */

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
