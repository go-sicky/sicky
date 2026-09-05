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
 * @file badger.go
 * @package infra
 * @author Dr.NP <np@herewe.tech>
 * @since 03/08/2025
 */

package infra

import (
	"errors"
	"strings"

	"github.com/dgraph-io/badger/v4"

	"github.com/go-sicky/sicky/logger"
)

// BadgerConfig is a infra component.
type BadgerConfig struct {
	Path string `json:"path" mapstructure:"path" yaml:"path"`
}

// ErrBadgerPathEmpty aborts startup: a non-nil BadgerConfig means "enable
// badger", and an empty path would otherwise open in the CWD or fail late.
var ErrBadgerPathEmpty = errors.New("infra: badger path is empty")

// Badger is a shared infra value.
var Badger *badger.DB

// InitBadger is part of the public API.
func InitBadger(cfg *BadgerConfig) (*badger.DB, error) {
	if cfg == nil {
		return nil, nil
	}

	cfg.Ensure()
	if err := cfg.Validate(); err != nil {
		logger.Logger.Error(
			"Badger config invalid",
			"error", err.Error(),
		)

		return nil, err
	}

	kv, err := badger.Open(badger.DefaultOptions(cfg.Path))
	if err != nil {
		logger.Logger.Error(
			"Badger storage initialize failed",
			"error", err.Error(),
		)

		return nil, err
	}

	logger.Logger.Info(
		"Badger storage initialized",
		"path", cfg.Path,
	)

	mu.Lock()
	defer mu.Unlock()
	if Badger != nil {
		// First-wins: keep the existing singleton and drop the duplicate
		// instead of leaking it.
		logger.Logger.Warn("Badger already initialized, closing duplicate connection")
		if cerr := kv.Close(); cerr != nil {
			logger.Logger.Error(
				"Badger duplicate close failed",
				"error", cerr.Error(),
			)
		}

		return Badger, nil
	}

	Badger = kv

	return kv, nil
}

// Ensure fills zero-valued fields with defaults and returns the receiver (nil-safe).
func (c *BadgerConfig) Ensure() *BadgerConfig {
	if c == nil {
		c = new(BadgerConfig)
	}

	return c
}

// Validate rejects half-configured or illegal values.
func (c *BadgerConfig) Validate() error {
	if c == nil {
		return nil
	}

	if strings.TrimSpace(c.Path) == "" {
		return ErrBadgerPathEmpty
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
