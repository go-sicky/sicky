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
	"errors"
	"maps"
	"sync"

	"github.com/google/uuid"
)

// ErrNotInitialized is returned by Load when no registry has been registered.
// sicky.ErrRegistryNotInitialized is an alias for this value, so errors.Is
// matches either spelling.
var ErrNotInitialized = errors.New("sicky: registry is not initialized")

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

// Register registers an instance. A nil return with no registry means
// there is nothing to register with, which leaves the desired state
// already satisfied: nothing was routed anywhere before either.
func Register(ins *Instance) error {
	rgMu.RLock()
	rg := defaultRegistry
	rgMu.RUnlock()
	if rg == nil {
		return nil
	}

	return rg.Register(ins)
}

// Deregister removes the registration. Like Register, a nil return with no
// registry means the registration is already absent.
func Deregister(id uuid.UUID) error {
	rgMu.RLock()
	rg := defaultRegistry
	rgMu.RUnlock()
	if rg == nil {
		return nil
	}

	return rg.Deregister(id)
}

// CheckInstance reports whether an instance is live. False covers both
// "no registry" and "not registered": the signature predates the error
// sentinels and its answer is the conservative one either way, since
// neither state justifies routing to the instance.
func CheckInstance(id uuid.UUID) bool {
	rgMu.RLock()
	rg := defaultRegistry
	rgMu.RUnlock()
	if rg == nil {
		return false
	}

	return rg.CheckInstance(id)
}

// Load loads persisted state. It returns ErrNotInitialized when no registry
// is registered: (nil, nil) there is indistinguishable from "the registry is
// reachable and holds no instances", so a misconfigured deployment would
// silently discover nothing and keep serving stale routes.
func Load() ([]*Instance, error) {
	rgMu.RLock()
	rg := defaultRegistry
	rgMu.RUnlock()
	if rg == nil {
		return nil, ErrNotInitialized
	}

	return rg.Load()
}

// Watch watches for changes. A nil return means no registry is registered,
// so there are no changes to watch for.
func Watch() error {
	rgMu.RLock()
	rg := defaultRegistry
	rgMu.RUnlock()
	if rg == nil {
		return nil
	}

	return rg.Watch()
}

// Stop stops the component and releases resources.
func Stop() error {
	rgMu.RLock()
	rg := defaultRegistry
	rgMu.RUnlock()
	if rg == nil {
		return nil
	}

	return rg.Stop()
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
