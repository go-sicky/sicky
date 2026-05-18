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

	"github.com/fsnotify/fsnotify"
	"github.com/go-sicky/sicky/registry"
	"github.com/go-sicky/sicky/utils"
	"github.com/google/uuid"
)

type Local struct {
	config  *Config
	ctx     context.Context
	options *registry.Options
}

func New(opts *registry.Options, cfg *Config) *Local {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	rg := &Local{
		config:  cfg,
		options: opts,
	}

	registry.Set(rg)

	return rg
}

func (rg *Local) Context() context.Context {
	return rg.options.Context
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
		if err := os.MkdirAll(dir, 0755); err != nil {
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
	err := os.WriteFile(file, data, 0644)
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
		"service_name", ins.ServiceMame,
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
	return true
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
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	dir := rg.config.RegistryFilePath
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			watcher.Close()

			return err
		}
	}

	go func() {
		defer watcher.Close()
		for {
			select {
			case <-rg.Context().Done():
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) {
					rg.options.Logger.DebugContext(rg.Context(), "Registry directory changed", "event", event.String())
					// Trigger reload or notify subscribers here
					ins, err := rg.Load()
					if err != nil {
						rg.options.Logger.ErrorContext(
							rg.ctx,
							"Reload services list failed",
							"registry", rg.String(),
							"id", rg.options.ID,
							"name", rg.options.Name,
							"error", err.Error(),
						)

						continue
					}

					registry.PurgePool(ins)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}

				rg.options.Logger.ErrorContext(rg.Context(), "Inotify watcher error", "error", err)
			}
		}
	}()

	err = watcher.Add(dir)
	if err != nil {
		return err
	}

	return nil
}

func (rg *Local) Stop() error {
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
