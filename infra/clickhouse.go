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
 * @file clickhouse.go
 * @package infra
 * @author Dr.NP <np@herewe.tech>
 * @since 12/20/2025
 */

package infra

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/uptrace/go-clickhouse/ch"

	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/metrics"
)

// ClickHouseConfig holds ClickHouse connection settings.
type ClickHouseConfig struct {
	DSN string `json:"dsn" mapstructure:"dsn" yaml:"dsn"`
	// DefaultInstance names the instance that also serves as the shared
	// singleton when the top-level DSN is empty.
	DefaultInstance string `json:"default_instance" mapstructure:"default_instance" yaml:"default_instance"`
	// NoDefault suppresses the shared singleton on a fresh init; only Instances are created.
	NoDefault bool `json:"no_default" mapstructure:"no_default" yaml:"no_default"`
	// Instances are complete ClickHouse configurations. They do not inherit
	// fields from the block above.
	Instances map[string]*ClickHouseConfig `json:"instances" mapstructure:"instances" yaml:"instances"`
}

// ErrClickHouseDSNEmpty aborts startup: a non-nil ClickHouseConfig means
// "enable clickhouse", and an empty DSN would otherwise fail late after a
// 5s ping timeout.
var ErrClickHouseDSNEmpty = errors.New("infra: clickhouse dsn is empty")

// Deprecated: use ErrClickHouseDSNEmpty.
var ErrClickhouseDSNEmpty = ErrClickHouseDSNEmpty

// Deprecated: use ClickHouseConfig.
type ClickhouseConfig = ClickHouseConfig

// Deprecated: use InitClickHouse.
func InitClickhouse(cfg *ClickHouseConfig) (*ch.DB, error) {
	return InitClickHouse(cfg)
}

// ClickHouse is the shared singleton.
var ClickHouse *ch.DB

// Deprecated: use ClickHouse. Kept in sync by InitClickHouse/ClearClickHouse.
var Clickhouse *ch.DB

// InitClickHouse connects and stores the shared singleton (first-wins, nil cfg disables).
func InitClickHouse(cfg *ClickHouseConfig) (*ch.DB, error) {
	if cfg == nil {
		return nil, nil
	}

	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()

	db, err := initClickHouse(cfg)
	metrics.CountInfraInit("clickhouse", err)
	if err == nil && db == nil {
		// no_default: the component initialized, but there is no shared
		// singleton to report as up.
		metrics.SetInfraUp("clickhouse", false)
	}

	return db, err
}

func initClickHouse(cfg *ClickHouseConfig) (*ch.DB, error) {
	if cfg == nil {
		return nil, nil
	}

	cfg.Ensure()
	if err := cfg.Validate(); err != nil {
		logger.Logger.Error(
			"clickhouse config invalid",
			"error", err.Error(),
		)

		return nil, err
	}

	instances, names, err := cfg.resolveInstances()
	if err != nil {
		logger.Logger.Error(
			"init clickhouse instances invalid",
			"error", err.Error(),
		)

		return nil, err
	}

	identityErr := cfg.validateIdentity()
	selection, err := resolveDefaultSelection(cfg.NoDefault, cfg.DefaultInstance, names, identityErr == nil, identityErr)
	if err != nil {
		logger.Logger.Error(
			"init clickhouse default selection invalid",
			"error", err.Error(),
		)

		return nil, err
	}

	if selection.Ignored != "" {
		logger.Logger.Warn(
			"clickhouse default_instance ignored because the top-level config is complete",
			"instance", selection.Ignored,
		)
	}

	if cfg.NoDefault {
		logger.Logger.Info("clickhouse no_default is set; this block does not create a shared instance")
	}

	if !selection.UseTopLevel && selection.Instance != "" && cfg.DefaultInstance == "" {
		if len(names) == 1 {
			logger.Logger.Info(
				"clickhouse using the only named instance as the default",
				"instance", selection.Instance,
			)
		} else {
			logger.Logger.Warn(
				"clickhouse no default_instance configured; using the first instance in sorted order",
				"instance", selection.Instance,
			)
		}
	}

	// Build first, register second: a failure never leaves a half-installed
	// component behind. Existing first-wins entries are not rebuilt.
	mu.RLock()
	existingDefault := ClickHouse
	mu.RUnlock()

	var topLevel *ch.DB

	if selection.UseTopLevel && existingDefault == nil {
		topLevel, err = newClickHouseClient(cfg)
		if err != nil {
			logger.Logger.Error(
				"init clickhouse failed",
				"dsn", redactDSN(cfg.DSN),
				"error", err.Error(),
			)

			return nil, err
		}
	}

	built := make(map[string]*ch.DB, len(names))
	for _, name := range names {
		if _, ok := clickHouseInstances.get(name); ok {
			logger.Logger.Warn("clickhouse instance already initialized, keeping existing client", "instance", name)

			continue
		}

		db, berr := newClickHouseClient(instances[name])
		if berr != nil {
			metrics.CountInfraInit("clickhouse:"+name, berr)
			logger.Logger.Error(
				"init clickhouse instance failed",
				"instance", name,
				"error", berr.Error(),
			)

			rollbackClickHouseClients(topLevel, built)

			return nil, fmt.Errorf("clickhouse instance %s: %w", name, berr)
		}

		metrics.CountInfraInit("clickhouse:"+name, nil)
		built[name] = db
	}

	// Register under the lock, release before anything that can block. This
	// mu is a single package-global write lock guarding all ten
	// singletons, so holding it across a duplicate's teardown (a 5s
	// Disconnect, a Close that flushes, paho's quiesce sleep) stalls
	// every Get* reader and every manager health probe for the duration.
	if topLevel != nil {
		stored := false

		mu.Lock()
		if ClickHouse == nil {
			ClickHouse = topLevel
			Clickhouse = topLevel
			clickHouseDefaultInstance = ""
			stored = true
		}

		mu.Unlock()

		if !stored {
			logger.Logger.Warn("clickhouse already initialized, closing duplicate connection")
			closeClickHouseClient(topLevel)
		}
	}

	for _, name := range names {
		db, ok := built[name]
		if !ok {
			continue
		}

		if _, stored := clickHouseInstances.putFirst(name, db); !stored {
			logger.Logger.Warn("clickhouse instance already initialized, closing duplicate connection", "instance", name)
			closeClickHouseClient(db)

			continue
		}

		logger.Logger.InfoContext(
			context.Background(),
			"clickhouse instance initialized",
			"instance", name,
			"dsn", redactDSN(instances[name].DSN),
		)
	}

	// A named default is installed only when the shared singleton is still
	// free; first-wins keeps an earlier top-level or alias default.
	if !selection.UseTopLevel && selection.Instance != "" {
		if db, ok := clickHouseInstances.get(selection.Instance); ok && db != nil {
			mu.Lock()
			if ClickHouse == nil {
				ClickHouse = db
				Clickhouse = db
				clickHouseDefaultInstance = selection.Instance
			}
			mu.Unlock()
		}
	}

	mu.RLock()
	db := ClickHouse
	mu.RUnlock()

	if topLevel != nil {
		logger.Logger.Info(
			"clickhouse initialized",
			"dsn", redactDSN(cfg.DSN),
		)
	}

	return db, nil
}

// newClickHouseClient connects and pings one handle. It touches no
// package-level state, so a build that fails leaves nothing registered.
func newClickHouseClient(cfg *ClickHouseConfig) (*ch.DB, error) {
	db := ch.Connect(ch.WithDSN(cfg.DSN))

	// Ping to verify connection
	ctx, cancel := context.WithTimeout(context.Background(), DefaultInitTimeoutSec*time.Second)
	defer cancel()

	if err := db.Ping(ctx); err != nil {
		logger.Logger.Error(
			"clickhouse ping failed",
			"dsn", redactDSN(cfg.DSN),
			"error", err.Error(),
		)

		// Close the handle opened above instead of leaking it.
		closeClickHouseClient(db)

		return nil, err
	}

	return db, nil
}

// closeClickHouseClient closes one handle and logs a failure instead of
// returning it: the caller is already on an error or duplicate path.
func closeClickHouseClient(db *ch.DB) {
	if db == nil {
		return
	}

	if cerr := db.Close(); cerr != nil {
		logger.Logger.Error(
			"clickhouse close failed",
			"error", cerr.Error(),
		)
	}
}

// rollbackClickHouseClients closes everything one init call built but will not
// register, so a failed build leaves no connections or "up" series behind.
func rollbackClickHouseClients(defaultDB *ch.DB, built map[string]*ch.DB) {
	closeClickHouseClient(defaultDB)

	for name, db := range built {
		closeClickHouseClient(db)
		metrics.SetInfraUp("clickhouse:"+name, false)
	}
}

// Ensure fills zero-valued fields with defaults and returns the receiver (nil-safe).
func (c *ClickHouseConfig) Ensure() *ClickHouseConfig {
	if c == nil {
		c = new(ClickHouseConfig)
	}

	return c
}

// Validate rejects half-configured or illegal values and resolves the named
// instances (without mutating the caller's config).
func (c *ClickHouseConfig) Validate() error {
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

// validateShape checks every value that does not depend on the connection
// identity. ClickHouse has no shape-only settings yet; the method exists so
// the validation split stays uniform across the infra components.
func (c *ClickHouseConfig) validateShape() error {
	return nil
}

// validateIdentity requires the field that addresses the server.
func (c *ClickHouseConfig) validateIdentity() error {
	if c == nil {
		return nil
	}

	if strings.TrimSpace(c.DSN) == "" {
		return ErrClickHouseDSNEmpty
	}

	return nil
}

func (c *ClickHouseConfig) validateBase() error {
	if err := c.validateShape(); err != nil {
		return err
	}

	return c.validateIdentity()
}

// resolveInstances validates every declared instance and returns ensured
// copies keyed by the normalized name. The caller's config is not mutated.
func (c *ClickHouseConfig) resolveInstances() (resolved map[string]*ClickHouseConfig, names []string, err error) {
	normalized, names, err := normalizeInstanceNames(c.Instances)
	if err != nil {
		return nil, nil, err
	}

	resolved = make(map[string]*ClickHouseConfig, len(names))
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

// ShutdownClickHouse closes the shared clickhouse and every named instance,
// then clears their references. Named instances are detached before closing
// so a reader cannot observe a closed handle through the registry; a default
// alias is closed exactly once.
func ShutdownClickHouse() error {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()

	instances := clickHouseInstances.clear()

	mu.Lock()
	alias := clickHouseDefaultInstance
	defaultDB := ClickHouse
	ClickHouse = nil
	Clickhouse = nil
	clickHouseDefaultInstance = ""
	mu.Unlock()

	if len(instances) == 0 && defaultDB == nil {
		return nil
	}

	var errs []error

	for name, db := range instances {
		if db == nil {
			continue
		}

		if cerr := db.Close(); cerr != nil {
			errs = append(errs, fmt.Errorf("clickhouse instance %s: %w", name, cerr))
		}

		metrics.SetInfraUp("clickhouse:"+name, false)
	}

	if defaultNeedsClose(alias, instances) && defaultDB != nil {
		if cerr := defaultDB.Close(); cerr != nil {
			errs = append(errs, fmt.Errorf("clickhouse: %w", cerr))
		}
	}

	metrics.SetInfraUp("clickhouse", false)

	return errors.Join(errs...)
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
