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
	"errors"
	"fmt"

	"github.com/dgraph-io/ristretto/v2"

	"github.com/go-sicky/sicky/logger"
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
}

// Ristretto is a shared infra value.
var Ristretto *ristretto.Cache[string, any]

// InitRistretto is part of the public API.
func InitRistretto(cfg *RistrettoConfig) (*ristretto.Cache[string, any], error) {
	if cfg == nil {
		return nil, nil
	}

	cfg = cfg.Ensure()
	if err := cfg.Validate(); err != nil {
		logger.Logger.Error(
			"Ristretto config invalid",
			"error", err.Error(),
		)

		return nil, err
	}

	cache, err := ristretto.NewCache(
		&ristretto.Config[string, any]{
			NumCounters: cfg.NumCounters,
			MaxCost:     cfg.MaxCost,
			BufferItems: cfg.BufferItems,
		},
	)
	if err != nil {
		logger.Logger.Error(
			"Ristretto cache initialize failed",
			"error", err.Error(),
		)

		return nil, fmt.Errorf("infra: ristretto new cache: %w", err)
	}

	logger.Logger.Info(
		"Ristretto cache initialized",
		"num_counters", cfg.NumCounters,
		"max_cost", cfg.MaxCost,
		"buffer_items", cfg.BufferItems,
	)

	mu.Lock()
	defer mu.Unlock()
	if Ristretto != nil {
		// First-wins: keep the existing singleton and drop the duplicate
		// instead of leaking it.
		logger.Logger.Warn("Ristretto already initialized, closing duplicate cache")
		cache.Close()

		return Ristretto, nil
	}

	Ristretto = cache

	return cache, nil
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

// Validate aborts on negative sizing (BREAKING: previously swallowed).
func (c *RistrettoConfig) Validate() error {
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

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
