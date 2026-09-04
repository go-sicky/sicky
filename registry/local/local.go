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
	"os"
	"path/filepath"

	"github.com/go-sicky/sicky/registry"
	"github.com/go-sicky/sicky/utils"
	"github.com/google/uuid"
)

type Local struct {
	config  *Config
	ctx     context.Context
	cancel  context.CancelFunc
	options *registry.Options
	watcher *Watcher
}

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

func (rg *Local) Context() context.Context {
	return rg.ctx
}

func (rg *Local) Options() *registry.Options {
	return rg.options
}

func (rg *Local) String() string {
	return "local"
}

func (rg *Local) ID() uuid.UUID {
	return rg.options.ID
}

func (rg *Local) Name() string {
	return rg.options.Name
}

func (rg *Local) Register(ins *registry.Instance) error {
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

			return err
		}
	}

	file := filepath.Join(dir, ins.ID.String()+".json")
	data := utils.JSONAnyBytes(ins)
	err := os.WriteFile(file, data, 0o600)
	if err != nil {
		rg.options.Logger.ErrorContext(
			rg.ctx,
			"Register instance to local file failed",
			"registry", rg.String(),
			"instance_id", ins.ID.String(),
			"file", file,
			"error", err.Error(),
		)

		return err
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

func (rg *Local) Deregister(id uuid.UUID) error {
	file := filepath.Join(rg.config.RegistryFilePath, id.String()+".json")
	err := os.Remove(file)
	if err != nil {
		rg.options.Logger.ErrorContext(
			rg.ctx,
			"Deregister instance from local file failed",
			"registry", rg.String(),
			"instance_id", id.String(),
			"file", file,
			"error", err.Error(),
		)

		return err
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

func (rg *Local) CheckInstance(id uuid.UUID) bool {
	file := filepath.Join(rg.config.RegistryFilePath, id.String()+".json")
	_, err := os.Stat(file)

	return err == nil
}

func (rg *Local) Load() ([]*registry.Instance, error) {
	var instances []*registry.Instance
	dir := rg.config.RegistryFilePath
	files, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return instances, nil
		}

		return nil, err
	}

	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}

		path := filepath.Join(dir, file.Name())
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

func (rg *Local) Watch() error {
	w, err := newWatcher(rg)
	if err != nil {
		rg.options.Logger.ErrorContext(
			rg.ctx,
			"Create watcher failed",
			"registry", rg.String(),
			"id", rg.options.ID,
			"name", rg.options.Name,
			"error", err.Error(),
		)

		return err
	}

	rg.watcher = w
	w.Start()

	rg.options.Logger.InfoContext(
		rg.ctx,
		"Local registry watcher started",
		"registry", rg.String(),
		"id", rg.options.ID,
		"name", rg.options.Name,
	)

	return nil
}

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
