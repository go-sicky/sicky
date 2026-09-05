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
 * @file local.go
 * @package local
 * @author Dr.NP <np@herewe.tech>
 * @since 02/22/2026
 */

package local

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/go-sicky/sicky/metrics"
	"github.com/go-sicky/sicky/registry"
)

// Local is a local component.
type Local struct {
	config  *Config
	ctx     context.Context
	cancel  context.CancelFunc
	options *registry.Options
	watcher *Watcher
}

// New creates a new instance (nil on invalid config).
func New(opts *registry.Options, cfg *Config) *Local {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	if err := cfg.Validate(); err != nil {
		opts.Logger.ErrorContext(
			opts.Context,
			"Local registry config invalid",
			"registry", "local",
			"error", err.Error(),
		)

		return nil
	}

	ctx, cancel := context.WithCancel(opts.Context)
	rg := &Local{
		config:  cfg,
		ctx:     ctx,
		cancel:  cancel,
		options: opts,
	}

	if cfg.CleanupOnStart {
		rg.cleanupStaleFiles()
	}

	registry.Set(rg)

	rg.options.Logger.InfoContext(
		rg.ctx,
		"Registry created",
		"registry", rg.String(),
		"id", rg.options.ID,
		"name", rg.options.Name,
	)

	return rg
}

// Context returns the component context.
func (rg *Local) Context() context.Context {
	return rg.ctx
}

// Options returns the runtime options.
func (rg *Local) Options() *registry.Options {
	return rg.options
}

// String returns a human-readable name.
func (rg *Local) String() string {
	return "local"
}

// ID returns the unique instance ID.
func (rg *Local) ID() uuid.UUID {
	return rg.options.ID
}

// Name returns the component name.
func (rg *Local) Name() string {
	return rg.options.Name
}

// Register registers the collector.
func (rg *Local) Register(ins *registry.Instance) (err error) {
	start := time.Now()
	defer func() { metrics.ObserveRegistryOp("local", "register", start, err) }()

	dir := rg.config.RegistryFilePath
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			rg.options.Logger.ErrorContext(
				rg.ctx,
				"Register instance to local file failed",
				"registry", rg.String(),
				"instance_id", ins.ID.String(),
				"error", err.Error(),
			)

			return fmt.Errorf("local registry mkdir (dir %s): %w", dir, err)
		}
	}

	file := filepath.Join(dir, ins.ID.String()+".json")
	data, merr := json.Marshal(ins)
	if merr != nil {
		// Never write a 0-byte file that every later Load() cannot parse.
		return fmt.Errorf("local registry marshal (instance %s): %w", ins.ID, merr)
	}

	err = os.WriteFile(file, data, 0o600)
	if err != nil {
		rg.options.Logger.ErrorContext(
			rg.ctx,
			"Register instance to local file failed",
			"registry", rg.String(),
			"instance_id", ins.ID.String(),
			"file", file,
			"error", err.Error(),
		)

		return fmt.Errorf("local registry write (file %s): %w", file, err)
	}

	rg.options.Logger.InfoContext(
		rg.ctx,
		"Instance registered to local file",
		"registry", rg.String(),
		"id", rg.options.ID,
		"name", rg.options.Name,
		"manager_address", ins.ManagerAddress,
		"manager_port", ins.ManagerPort,
		"service_name", ins.ServiceName,
		"instance_id", ins.ID.String(),
	)

	return nil
}

// Deregister removes the registration.
func (rg *Local) Deregister(id uuid.UUID) (err error) {
	start := time.Now()
	defer func() { metrics.ObserveRegistryOp("local", "deregister", start, err) }()

	file := filepath.Join(rg.config.RegistryFilePath, id.String()+".json")
	err = os.Remove(file)
	if err != nil {
		rg.options.Logger.ErrorContext(
			rg.ctx,
			"Deregister instance from local file failed",
			"registry", rg.String(),
			"instance_id", id.String(),
			"file", file,
			"error", err.Error(),
		)

		return fmt.Errorf("local registry remove (file %s): %w", file, err)
	}

	rg.options.Logger.InfoContext(
		rg.ctx,
		"Instance deregistered from local file",
		"registry", rg.String(),
		"id", rg.options.ID,
		"name", rg.options.Name,
		"instance_id", id.String(),
	)

	return nil
}

// CheckInstance checks instance liveness.
func (rg *Local) CheckInstance(id uuid.UUID) bool {
	file := filepath.Join(rg.config.RegistryFilePath, id.String()+".json")
	_, err := os.Stat(file)
	ok := err == nil
	result := "ok"
	if !ok {
		result = "missing"
	}
	metrics.RegistryOpsTotal.WithLabelValues("local", "check", result).Inc()

	return ok
}

// Load loads persisted state.
func (rg *Local) Load() (instances []*registry.Instance, err error) {
	start := time.Now()
	defer func() {
		metrics.ObserveRegistryOp("local", "load", start, err)
		if err == nil {
			metrics.RegistryInstances.WithLabelValues("local").Set(float64(len(instances)))
		}
	}()

	dir := rg.config.RegistryFilePath
	files, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return instances, nil
		}

		return nil, fmt.Errorf("local registry read dir (dir %s): %w", dir, err)
	}

	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}

		path := filepath.Join(dir, file.Name())
		//nolint:gosec // G304: name comes from a ReadDir listing under the validated absolute registry dir, not user input
		data, err := os.ReadFile(path)
		if err != nil {
			rg.options.Logger.ErrorContext(
				rg.Context(),
				"Read instance file failed",
				"file", path,
				"error", err.Error(),
			)

			continue
		}

		var ins registry.Instance
		if err := json.Unmarshal(data, &ins); err != nil {
			rg.options.Logger.ErrorContext(
				rg.Context(),
				"Unmarshal instance failed",
				"file", path,
				"error", err.Error(),
			)

			continue
		}

		instances = append(instances, &ins)
	}

	return instances, nil
}

// Watch watches for changes.
func (rg *Local) Watch() error {
	w, err := newWatcher(rg)
	if err != nil {
		metrics.RegistryOpsTotal.WithLabelValues("local", "watch", "error").Inc()
		rg.options.Logger.ErrorContext(
			rg.ctx,
			"Create watcher failed",
			"registry", rg.String(),
			"id", rg.options.ID,
			"name", rg.options.Name,
			"error", err.Error(),
		)

		return fmt.Errorf("local registry create watcher: %w", err)
	}

	rg.watcher = w
	w.Start()
	metrics.RegistryOpsTotal.WithLabelValues("local", "watch", "ok").Inc()

	rg.options.Logger.InfoContext(
		rg.ctx,
		"Local registry watcher started",
		"registry", rg.String(),
		"id", rg.options.ID,
		"name", rg.options.Name,
	)

	return nil
}

// Stop stops the component and releases resources.
func (rg *Local) Stop() error {
	if rg.watcher != nil {
		rg.watcher.Stop()

		rg.options.Logger.InfoContext(
			rg.ctx,
			"Local registry watcher stopped",
			"registry", rg.String(),
			"id", rg.options.ID,
			"name", rg.options.Name,
		)
	}

	if rg.cancel != nil {
		rg.cancel()
	}

	return nil
}

func (rg *Local) cleanupStaleFiles() {
	dir := rg.config.RegistryFilePath
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		rg.options.Logger.WarnContext(
			rg.ctx,
			"Cleanup stale registry files failed",
			"registry", rg.String(),
			"error", err.Error(),
		)

		return
	}

	count := 0
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}

		// Only remove files this registry could have written
		// (<uuid>.json). Anything else is left alone so a misconfigured
		// directory never causes collateral deletes.
		base := file.Name()[:len(file.Name())-len(".json")]
		if _, err := uuid.Parse(base); err != nil {
			continue
		}

		path := filepath.Join(dir, file.Name())
		if err := os.Remove(path); err != nil {
			rg.options.Logger.WarnContext(
				rg.ctx,
				"Remove stale registry file failed",
				"registry", rg.String(),
				"file", path,
				"error", err.Error(),
			)
		} else {
			count++
		}
	}

	if count > 0 {
		rg.options.Logger.InfoContext(
			rg.ctx,
			"Cleaned up stale registry files",
			"registry", rg.String(),
			"count", count,
		)
	}
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
