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
	"maps"
	"sync"

	"github.com/google/uuid"
)

// Registry is a registry component.
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
	Register(ins *Instance) error
	// Deregister service
	Deregister(id uuid.UUID) error
	// Check service instance
	CheckInstance(id uuid.UUID) bool
	// Load instances
	Load() ([]*Instance, error)
	// Watch services
	Watch() error
	// Stop registry watcher
	Stop() error
}

var (
	registries      = make(map[uuid.UUID]Registry)
	defaultRegistry Registry
	rgMu            sync.RWMutex
)

// Set registers registry instances; the first one becomes the default.
func Set(rgs ...Registry) {
	rgMu.Lock()
	defer rgMu.Unlock()

	for _, rg := range rgs {
		registries[rg.ID()] = rg
		if defaultRegistry == nil {
			defaultRegistry = rg
		}
	}
}

// Get looks up a registry instance by ID.
func Get(id uuid.UUID) Registry {
	rgMu.RLock()
	defer rgMu.RUnlock()

	return registries[id]
}

// Default returns the default registry instance.
func Default() Registry {
	rgMu.RLock()
	defer rgMu.RUnlock()

	return defaultRegistry
}

// Registries returns the managed registries.
func Registries() map[uuid.UUID]Registry {
	rgMu.RLock()
	defer rgMu.RUnlock()

	out := make(map[uuid.UUID]Registry, len(registries))
	maps.Copy(out, registries)

	return out
}

// Clear resets the global registry. Intended for tests.
func Clear() {
	rgMu.Lock()
	defer rgMu.Unlock()

	registries = make(map[uuid.UUID]Registry)
	defaultRegistry = nil
}

/* {{{ [Helpers]. */
func Register(ins *Instance) error {
	if defaultRegistry == nil {
		return nil
	}

	return defaultRegistry.Register(ins)
}

// Deregister removes the registration.
func Deregister(id uuid.UUID) error {
	if defaultRegistry == nil {
		return nil
	}

	return defaultRegistry.Deregister(id)
}

// CheckInstance checks instance liveness.
func CheckInstance(id uuid.UUID) bool {
	if defaultRegistry == nil {
		return false
	}

	return defaultRegistry.CheckInstance(id)
}

// Load loads persisted state.
func Load() ([]*Instance, error) {
	if defaultRegistry == nil {
		return nil, nil
	}

	return defaultRegistry.Load()
}

// Watch watches for changes.
func Watch() error {
	if defaultRegistry == nil {
		return nil
	}

	return defaultRegistry.Watch()
}

// Stop stops the component and releases resources.
func Stop() error {
	if defaultRegistry == nil {
		return nil
	}

	return defaultRegistry.Stop()
}

/* }}} */

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
