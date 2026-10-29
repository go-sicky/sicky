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
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dgraph-io/badger/v4"

	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/metrics"
)

// BadgerConfig is a infra component.
type BadgerConfig struct {
	Path string `json:"path" mapstructure:"path" yaml:"path"`
	// DefaultInstance names the instance that also serves as the shared
	// singleton when the top-level fields do not form a complete config.
	DefaultInstance string `json:"default_instance" mapstructure:"default_instance" yaml:"default_instance"`
	// NoDefault suppresses the shared singleton on a fresh init; only Instances are created.
	NoDefault bool `json:"no_default" mapstructure:"no_default" yaml:"no_default"`
	// Instances are complete Badger configurations. They do not inherit fields
	// from the block above.
	Instances map[string]*BadgerConfig `json:"instances" mapstructure:"instances" yaml:"instances"`
}

// ErrBadgerPathEmpty aborts startup: a non-nil BadgerConfig means "enable
// badger", and an empty path would otherwise open in the CWD or fail late.
// ErrBadgerPathDuplicate aborts before anything opens: Badger locks its
// directory, so a second open of the same path fails with a file-lock error
// that says nothing about which two config blocks collided.
var (
	ErrBadgerPathEmpty     = errors.New("infra: badger path is empty")
	ErrBadgerPathDuplicate = errors.New("infra: badger path is duplicated across instances")
)

// Badger is a shared infra value.
var Badger *badger.DB

// InitBadger is part of the public API.
func InitBadger(cfg *BadgerConfig) (*badger.DB, error) {
	if cfg == nil {
		return nil, nil
	}

	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()

	db, err := initBadger(cfg)
	metrics.CountInfraInit("badger", err)
	if err == nil && db == nil {
		// no_default: the component initialized, but there is no shared
		// singleton to report as up.
		metrics.SetInfraUp("badger", false)
	}

	return db, err
}

func initBadger(cfg *BadgerConfig) (*badger.DB, error) {
	if cfg == nil {
		return nil, nil
	}

	cfg.Ensure()
	if err := cfg.Validate(); err != nil {
		logger.Logger.Error(
			"badger config invalid",
			"error", err.Error(),
		)

		return nil, err
	}

	instances, names, err := cfg.resolveInstances()
	if err != nil {
		logger.Logger.Error(
			"init badger instances invalid",
			"error", err.Error(),
		)

		return nil, err
	}

	identityErr := cfg.validateIdentity()
	selection, err := resolveDefaultSelection(cfg.NoDefault, cfg.DefaultInstance, names, identityErr == nil, identityErr)
	if err != nil {
		logger.Logger.Error(
			"init badger default selection invalid",
			"error", err.Error(),
		)

		return nil, err
	}

	if selection.Ignored != "" {
		logger.Logger.Warn(
			"badger default_instance ignored because the top-level config is complete",
			"instance", selection.Ignored,
		)
	}

	if cfg.NoDefault {
		logger.Logger.Info("badger no_default is set; this block does not create a shared instance")
	}

	if !selection.UseTopLevel && selection.Instance != "" && cfg.DefaultInstance == "" {
		if len(names) == 1 {
			logger.Logger.Info(
				"badger using the only named instance as the default",
				"instance", selection.Instance,
			)
		} else {
			logger.Logger.Warn(
				"badger no default_instance configured; using the first instance in sorted order",
				"instance", selection.Instance,
			)
		}
	}

	// Reject a duplicated directory before anything opens: Badger's own
	// failure is a lock error that does not name the colliding blocks.
	if err := cfg.validatePaths(selection, instances, names); err != nil {
		logger.Logger.Error(
			"init badger path conflict",
			"error", err.Error(),
		)

		return nil, err
	}

	// Build first, register second: a failure never leaves a half-installed
	// component behind. Existing first-wins entries are not rebuilt.
	mu.RLock()
	existingDefault := Badger
	mu.RUnlock()

	var topLevel *badger.DB

	if selection.UseTopLevel && existingDefault == nil {
		topLevel, err = newBadgerClient(cfg, "badger")
		if err != nil {
			// newBadgerClient already logged the failure with the same
			// instance label; a second identical line adds nothing.
			return nil, err
		}
	}

	built := make(map[string]badgerInstance, len(names))
	for _, name := range names {
		if _, ok := badgerInstances.get(name); ok {
			logger.Logger.Warn("badger instance already initialized, keeping existing storage", "instance", name)

			continue
		}

		db, berr := newBadgerClient(instances[name], "badger:"+name)
		if berr != nil {
			metrics.CountInfraInit("badger:"+name, berr)
			logger.Logger.Error(
				"init badger instance failed",
				"instance", name,
				"error", berr.Error(),
			)

			rollbackBadgerClients(topLevel, built)

			return nil, fmt.Errorf("badger instance %s: %w", name, berr)
		}

		metrics.CountInfraInit("badger:"+name, nil)
		built[name] = badgerInstance{db: db, path: resolveBadgerPath(instances[name].Path)}
	}

	// Register under the lock, release before anything that can block. This
	// mu is a single package-global write lock guarding all ten
	// singletons, so holding it across a duplicate's teardown (a 5s
	// Disconnect, a Close that flushes, paho's quiesce sleep) stalls
	// every Get* reader and every manager health probe for the duration.
	if topLevel != nil {
		stored := false

		mu.Lock()
		if Badger == nil {
			Badger = topLevel
			badgerDefaultInstance = ""
			stored = true
		}

		mu.Unlock()

		if !stored {
			logger.Logger.Warn("badger already initialized, closing duplicate connection")
			closeBadgerClient(topLevel, "badger")
		}
	}

	for _, name := range names {
		instance, ok := built[name]
		if !ok {
			continue
		}

		if _, stored := badgerInstances.putFirst(name, instance); !stored {
			logger.Logger.Warn("badger instance already initialized, closing duplicate connection", "instance", name)
			closeBadgerClient(instance.db, "badger:"+name)

			continue
		}

		logger.Logger.InfoContext(
			context.Background(),
			"badger instance initialized",
			"instance", name,
			"path", instances[name].Path,
		)
	}

	// A named default is installed only when the shared singleton is still
	// free; first-wins keeps an earlier top-level or alias default.
	if !selection.UseTopLevel && selection.Instance != "" {
		if instance, ok := badgerInstances.get(selection.Instance); ok && instance.db != nil {
			mu.Lock()
			if Badger == nil {
				Badger = instance.db
				badgerDefaultInstance = selection.Instance
			}
			mu.Unlock()
		}
	}

	mu.RLock()
	db := Badger
	mu.RUnlock()

	if topLevel != nil {
		logger.Logger.Info(
			"badger storage initialized",
			"instance", "badger",
			"path", cfg.Path,
		)
	}

	return db, nil
}

// validatePaths rejects a directory claimed by more than one of the clients
// this config block describes. The top-level config is part of the set only
// when it is the selected default.
func (c *BadgerConfig) validatePaths(selection defaultSelection, instances map[string]*BadgerConfig, names []string) error {
	seen := make(map[string]string, len(names)+1)

	check := func(path, owner string) error {
		resolved := resolveBadgerPath(path)
		if previous, ok := seen[resolved]; ok {
			return fmt.Errorf("%w: %q is used by %s and %s", ErrBadgerPathDuplicate, resolved, previous, owner)
		}

		seen[resolved] = owner

		return nil
	}

	// Live registered storages still hold their directory lock even though
	// this call will not rebuild them. Seed their effective paths so a new
	// block cannot collide with one and only discover it at Open.
	for name, instance := range badgerInstances.snapshot() {
		if instance.path == "" {
			continue
		}

		seen[instance.path] = "instances." + name
	}

	if selection.UseTopLevel {
		mu.RLock()
		existingDefault := Badger
		mu.RUnlock()

		// A singleton that is already open will not be rebuilt, so its path
		// is not part of this call's conflict set.
		if existingDefault == nil {
			if err := check(c.Path, "the top-level config"); err != nil {
				return err
			}
		}
	}

	for _, name := range names {
		// Already-registered instances are kept first-wins and are not
		// reopened; skipping them avoids a false conflict on a second Init.
		if _, ok := badgerInstances.get(name); ok {
			continue
		}

		if err := check(instances[name].Path, "instances."+name); err != nil {
			return err
		}
	}

	return nil
}

// resolveBadgerPath normalizes a configured path so two spellings of the same
// directory are compared as one. Badger itself locks the absolute path.
func resolveBadgerPath(path string) string {
	cleaned := filepath.Clean(strings.TrimSpace(path))

	abs, err := filepath.Abs(cleaned)
	if err != nil {
		// Getwd failed: the cleaned path is the best available answer and
		// Badger reports the real problem when it opens.
		return cleaned
	}

	return abs
}

// newBadgerClient opens one storage. It touches no package-level state, so a
// build that fails leaves nothing registered. label names the instance in
// logs: "badger" for the shared singleton, "badger:<name>" for a named one.
func newBadgerClient(cfg *BadgerConfig, label string) (*badger.DB, error) {
	db, err := badger.Open(badger.DefaultOptions(cfg.Path))
	if err != nil {
		logger.Logger.Error(
			"badger storage initialize failed",
			"instance", label,
			"path", cfg.Path,
			"error", err.Error(),
		)

		return nil, err
	}

	return db, nil
}

// closeBadgerClient closes one storage and logs a failure instead of returning
// it: the caller is already on an error or duplicate path.
func closeBadgerClient(db *badger.DB, label string) {
	if db == nil {
		return
	}

	if cerr := db.Close(); cerr != nil {
		logger.Logger.Error(
			"badger close failed",
			"instance", label,
			"error", cerr.Error(),
		)
	}
}

// rollbackBadgerClients closes everything one init call built but will not
// register, so a failed build leaves no open directories or "up" series
// behind.
func rollbackBadgerClients(defaultDB *badger.DB, built map[string]badgerInstance) {
	closeBadgerClient(defaultDB, "badger")

	for name, instance := range built {
		closeBadgerClient(instance.db, "badger:"+name)
		metrics.SetInfraUp("badger:"+name, false)
	}
}

// Ensure fills zero-valued fields with defaults and returns the receiver (nil-safe).
func (c *BadgerConfig) Ensure() *BadgerConfig {
	if c == nil {
		c = new(BadgerConfig)
	}

	return c
}

// Validate rejects half-configured or illegal values and resolves the named
// instances (without mutating the caller's config).
func (c *BadgerConfig) Validate() error {
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

// validateShape checks every value that does not depend on the storage
// identity. Badger has no shape-only settings; the method exists so the
// validation split stays uniform across the infra components.
func (c *BadgerConfig) validateShape() error {
	return nil
}

// validateIdentity requires the directory that holds the storage.
func (c *BadgerConfig) validateIdentity() error {
	if c == nil {
		return nil
	}

	if strings.TrimSpace(c.Path) == "" {
		return ErrBadgerPathEmpty
	}

	return nil
}

func (c *BadgerConfig) validateBase() error {
	if err := c.validateShape(); err != nil {
		return err
	}

	return c.validateIdentity()
}

// resolveInstances validates every declared instance and returns ensured
// copies keyed by the normalized name. The caller's config is not mutated.
func (c *BadgerConfig) resolveInstances() (resolved map[string]*BadgerConfig, names []string, err error) {
	normalized, names, err := normalizeInstanceNames(c.Instances)
	if err != nil {
		return nil, nil, err
	}

	resolved = make(map[string]*BadgerConfig, len(names))
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

// ShutdownBadger closes the shared badger and every named instance, then
// clears their references. Named instances are detached before closing so a
// reader cannot observe a closed handle through the registry; a default alias
// is closed exactly once.
func ShutdownBadger() error {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()

	instances := badgerInstances.clear()

	mu.Lock()
	alias := badgerDefaultInstance
	defaultDB := Badger
	Badger = nil
	badgerDefaultInstance = ""
	mu.Unlock()

	if len(instances) == 0 && defaultDB == nil {
		return nil
	}

	var errs []error

	for name, instance := range instances {
		if instance.db == nil {
			continue
		}

		if cerr := instance.db.Close(); cerr != nil {
			errs = append(errs, fmt.Errorf("badger instance %s: %w", name, cerr))
		}

		metrics.SetInfraUp("badger:"+name, false)
	}

	if defaultNeedsClose(alias, instances) && defaultDB != nil {
		if cerr := defaultDB.Close(); cerr != nil {
			errs = append(errs, fmt.Errorf("badger: %w", cerr))
		}
	}

	metrics.SetInfraUp("badger", false)

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
