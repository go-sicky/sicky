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
 * @file ristretto.go
 * @package infra
 * @author Dr.NP <np@herewe.tech>
 * @since 03/08/2025
 */

package infra

import (
	"context"
	"errors"
	"fmt"

	"github.com/dgraph-io/ristretto/v2"

	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/metrics"
)

var (
	// ErrRistrettoNegativeNumCounters is a shared infra value.
	ErrRistrettoNegativeNumCounters = errors.New("infra: ristretto num_counters is negative")
	// ErrRistrettoNegativeMaxCost is a shared infra value.
	ErrRistrettoNegativeMaxCost = errors.New("infra: ristretto max_cost is negative")
	// ErrRistrettoNegativeBufferItems is a shared infra value.
	ErrRistrettoNegativeBufferItems = errors.New("infra: ristretto buffer_items is negative")
)

// RistrettoConfig is a infra component.
type RistrettoConfig struct {
	NumCounters int64 `json:"num_counters" mapstructure:"num_counters" yaml:"num_counters"`
	MaxCost     int64 `json:"max_cost"     mapstructure:"max_cost"     yaml:"max_cost"`
	BufferItems int64 `json:"buffer_items" mapstructure:"buffer_items" yaml:"buffer_items"`
	// DefaultInstance is accepted for config symmetry but always ignored:
	// ristretto has no required identity, so the top-level config is always
	// the shared singleton. A no_default block refuses it.
	DefaultInstance string `json:"default_instance" mapstructure:"default_instance" yaml:"default_instance"`
	// NoDefault suppresses the top-level cache on a fresh init; only Instances are created.
	NoDefault bool `json:"no_default" mapstructure:"no_default" yaml:"no_default"`
	// Instances are complete Ristretto configurations. They do not inherit
	// fields from the block above.
	Instances map[string]*RistrettoConfig `json:"instances" mapstructure:"instances" yaml:"instances"`
}

// Ristretto is a shared infra value.
var Ristretto *ristretto.Cache[string, any]

// InitRistretto is part of the public API.
func InitRistretto(cfg *RistrettoConfig) (*ristretto.Cache[string, any], error) {
	if cfg == nil {
		return nil, nil
	}

	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()

	cache, err := initRistretto(cfg)
	metrics.CountInfraInit("ristretto", err)
	if err == nil && cache == nil {
		// no_default: the component initialized, but there is no shared
		// singleton to report as up.
		metrics.SetInfraUp("ristretto", false)
	}

	return cache, err
}

func initRistretto(cfg *RistrettoConfig) (*ristretto.Cache[string, any], error) {
	if cfg == nil {
		return nil, nil
	}

	cfg.Ensure()
	if err := cfg.Validate(); err != nil {
		logger.Logger.Error(
			"ristretto config invalid",
			"error", err.Error(),
		)

		return nil, err
	}

	// Report the implicit sizing before resolveInstances fills the copies:
	// once Ensure has run, a zero is indistinguishable from a deliberate
	// default and the warning could never be derived again.
	for name, raw := range cfg.Instances {
		if raw == nil {
			continue
		}

		if raw.NumCounters == 0 || raw.MaxCost == 0 {
			logger.Logger.Warn(
				"ristretto instance uses the default sizing; set num_counters and max_cost explicitly",
				"instance", normalizeInstanceName(name),
				"num_counters", raw.NumCounters,
				"max_cost", raw.MaxCost,
			)
		}
	}

	instances, names, err := cfg.resolveInstances()
	if err != nil {
		logger.Logger.Error(
			"init ristretto instances invalid",
			"error", err.Error(),
		)

		return nil, err
	}

	// validateIdentity always succeeds for ristretto, so the selection is
	// always the top-level config and default_instance is ignored. The
	// framework-wide rules still govern no_default.
	identityErr := cfg.validateIdentity()
	selection, err := resolveDefaultSelection(cfg.NoDefault, cfg.DefaultInstance, names, identityErr == nil, identityErr)
	if err != nil {
		logger.Logger.Error(
			"init ristretto default selection invalid",
			"error", err.Error(),
		)

		return nil, err
	}

	if selection.Ignored != "" {
		logger.Logger.Warn(
			"default_instance is ignored for ristretto; the top-level config is always the default",
			"instance", selection.Ignored,
		)
	}

	if cfg.NoDefault {
		logger.Logger.Info("ristretto no_default is set; this block does not create a shared instance")
	}

	if !cfg.NoDefault && len(names) > 0 {
		logger.Logger.Warn(
			"a default ristretto cache with the top-level sizing will also be created; set no_default to avoid it",
		)
	}

	// Build first, register second: a failure never leaves a half-installed
	// component behind. Existing first-wins entries are not rebuilt.
	mu.RLock()
	existingDefault := Ristretto
	mu.RUnlock()

	var topLevel *ristretto.Cache[string, any]

	if selection.UseTopLevel {
		if existingDefault == nil {
			topLevel, err = newRistrettoClient(cfg)
			if err != nil {
				// newRistrettoClient already logged the failure; a second
				// identical line adds nothing.
				return nil, err
			}
		} else {
			// A cache is cheap to keep but expensive to build twice: a
			// default sizing allocates for ten million counters. Keep the
			// first-wins singleton instead of building and dropping a
			// duplicate.
			logger.Logger.Warn("ristretto already initialized, keeping existing cache")
		}
	}

	built := make(map[string]*ristretto.Cache[string, any], len(names))
	for _, name := range names {
		if _, ok := ristrettoInstances.get(name); ok {
			logger.Logger.Warn("ristretto instance already initialized, keeping existing cache", "instance", name)

			continue
		}

		cache, berr := newRistrettoClient(instances[name])
		if berr != nil {
			metrics.CountInfraInit("ristretto:"+name, berr)
			logger.Logger.Error(
				"init ristretto instance failed",
				"instance", name,
				"error", berr.Error(),
			)

			rollbackRistrettoClients(topLevel, built)

			return nil, fmt.Errorf("ristretto instance %s: %w", name, berr)
		}

		metrics.CountInfraInit("ristretto:"+name, nil)
		built[name] = cache
	}

	// Register under the lock, release before anything that can block. This
	// mu is a single package-global write lock guarding all ten
	// singletons, so holding it across a duplicate's teardown (a 5s
	// Disconnect, a Close that flushes, paho's quiesce sleep) stalls
	// every Get* reader and every manager health probe for the duration.
	if topLevel != nil {
		stored := false

		mu.Lock()
		if Ristretto == nil {
			Ristretto = topLevel
			ristrettoDefaultInstance = ""
			stored = true
		}

		mu.Unlock()

		if !stored {
			logger.Logger.Warn("ristretto already initialized, closing duplicate cache")
			closeRistrettoClient(topLevel)
		}
	}

	for _, name := range names {
		cache, ok := built[name]
		if !ok {
			continue
		}

		if _, stored := ristrettoInstances.putFirst(name, cache); !stored {
			logger.Logger.Warn("ristretto instance already initialized, closing duplicate cache", "instance", name)
			closeRistrettoClient(cache)

			continue
		}

		logger.Logger.InfoContext(
			context.Background(),
			"ristretto instance initialized",
			"instance", name,
			"num_counters", instances[name].NumCounters,
			"max_cost", instances[name].MaxCost,
			"buffer_items", instances[name].BufferItems,
		)
	}

	// ristrettoDefaultInstance stays empty by design: the top-level config
	// is always the shared singleton, so a named alias cannot exist.

	mu.RLock()
	cache := Ristretto
	mu.RUnlock()

	if topLevel != nil {
		logger.Logger.Info(
			"ristretto cache initialized",
			"instance", "ristretto",
			"num_counters", cfg.NumCounters,
			"max_cost", cfg.MaxCost,
			"buffer_items", cfg.BufferItems,
		)
	}

	return cache, nil
}

// newRistrettoClient builds one cache. It touches no package-level state, so a
// build that fails leaves nothing registered.
func newRistrettoClient(cfg *RistrettoConfig) (*ristretto.Cache[string, any], error) {
	cache, err := ristretto.NewCache(
		&ristretto.Config[string, any]{
			NumCounters: cfg.NumCounters,
			MaxCost:     cfg.MaxCost,
			BufferItems: cfg.BufferItems,
		},
	)
	if err != nil {
		logger.Logger.Error(
			"ristretto cache initialize failed",
			"error", err.Error(),
		)

		return nil, fmt.Errorf("infra: ristretto new cache: %w", err)
	}

	return cache, nil
}

// closeRistrettoClient closes one cache. Close returns nothing, so there is no
// failure to log or return.
func closeRistrettoClient(cache *ristretto.Cache[string, any]) {
	if cache == nil {
		return
	}

	cache.Close()
}

// rollbackRistrettoClients closes everything one init call built but will not
// register, so a failed build leaves no caches or "up" series behind.
func rollbackRistrettoClients(defaultCache *ristretto.Cache[string, any], built map[string]*ristretto.Cache[string, any]) {
	closeRistrettoClient(defaultCache)

	for name, cache := range built {
		closeRistrettoClient(cache)
		metrics.SetInfraUp("ristretto:"+name, false)
	}
}

const (
	// DefaultRistrettoNumCounters is a infra constant.
	DefaultRistrettoNumCounters = 10000000
	// DefaultRistrettoMaxCost is a infra constant.
	DefaultRistrettoMaxCost = 100000000
	// DefaultRistrettoBufferItems is a infra constant.
	DefaultRistrettoBufferItems = 64
)

// Ensure fills zero-valued fields with defaults and returns the receiver (nil-safe).
func (c *RistrettoConfig) Ensure() *RistrettoConfig {
	if c == nil {
		c = new(RistrettoConfig)
	}

	// Zero fills default; negative stays for Validate to abort.
	if c.NumCounters == 0 {
		c.NumCounters = DefaultRistrettoNumCounters
	}

	if c.MaxCost == 0 {
		c.MaxCost = DefaultRistrettoMaxCost
	}

	if c.BufferItems == 0 {
		c.BufferItems = DefaultRistrettoBufferItems
	}

	return c
}

// Validate rejects half-configured or illegal values and resolves the named
// instances (without mutating the caller's config).
func (c *RistrettoConfig) Validate() error {
	if c == nil {
		return nil
	}

	if err := c.validateShape(); err != nil {
		return err
	}

	_, names, err := c.resolveInstances()
	if err != nil {
		return err
	}

	identityErr := c.validateIdentity()
	if _, err := resolveDefaultSelection(c.NoDefault, c.DefaultInstance, names, identityErr == nil, identityErr); err != nil {
		return err
	}

	return nil
}

// validateShape aborts on negative sizing (BREAKING: previously swallowed).
func (c *RistrettoConfig) validateShape() error {
	if c == nil {
		return nil
	}

	if c.NumCounters < 0 {
		return ErrRistrettoNegativeNumCounters
	}

	if c.MaxCost < 0 {
		return ErrRistrettoNegativeMaxCost
	}

	if c.BufferItems < 0 {
		return ErrRistrettoNegativeBufferItems
	}

	return nil
}

// validateIdentity always succeeds: an in-process cache has no address or
// credentials, so the top-level block is always a complete default.
func (c *RistrettoConfig) validateIdentity() error {
	return nil
}

func (c *RistrettoConfig) validateBase() error {
	if err := c.validateShape(); err != nil {
		return err
	}

	return c.validateIdentity()
}

// resolveInstances validates every declared instance and returns ensured
// copies keyed by the normalized name. The caller's config is not mutated.
func (c *RistrettoConfig) resolveInstances() (resolved map[string]*RistrettoConfig, names []string, err error) {
	normalized, names, err := normalizeInstanceNames(c.Instances)
	if err != nil {
		return nil, nil, err
	}

	resolved = make(map[string]*RistrettoConfig, len(names))
	for _, name := range names {
		raw := normalized[name]
		if raw.Instances != nil || raw.DefaultInstance != "" || raw.NoDefault {
			return nil, nil, fmt.Errorf("%w: instances.%s", ErrInstanceNested, name)
		}

		instance := *raw
		instance.Instances = nil
		instance.DefaultInstance = ""
		instance.NoDefault = false
		instance.Ensure()

		if err := instance.validateBase(); err != nil {
			return nil, nil, fmt.Errorf("instances.%s: %w", name, err)
		}

		resolved[name] = &instance
	}

	return resolved, names, nil
}

// ShutdownRistretto closes the shared cache and every named instance, then
// clears their references. Named instances close first so a default alias is
// closed exactly once.
func ShutdownRistretto() error {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()

	instances := ristrettoInstances.clear()

	mu.Lock()
	alias := ristrettoDefaultInstance
	defaultCache := Ristretto
	Ristretto = nil
	ristrettoDefaultInstance = ""
	mu.Unlock()

	if len(instances) == 0 && defaultCache == nil {
		return nil
	}

	for name, cache := range instances {
		closeRistrettoClient(cache)
		metrics.SetInfraUp("ristretto:"+name, false)
	}

	if defaultNeedsClose(alias, instances) {
		closeRistrettoClient(defaultCache)
	}

	metrics.SetInfraUp("ristretto", false)

	return nil
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
