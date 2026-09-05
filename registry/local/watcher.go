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
 * @file watcher.go
 * @package local
 * @author Dr.NP <np@herewe.tech>
 * @since 02/22/2026
 */

package local

import (
	"fmt"
	"os"
	"sync"

	"github.com/fsnotify/fsnotify"

	"github.com/go-sicky/sicky/registry"
)

// Watcher is a local component.
type Watcher struct {
	registry *Local
	watcher  *fsnotify.Watcher

	closeOnce sync.Once

	sync.RWMutex
}

func newWatcher(rg *Local) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("local registry create watcher: %w", err)
	}

	dir := rg.config.RegistryFilePath
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			_ = fw.Close()

			return nil, fmt.Errorf("local registry mkdir (dir %s): %w", dir, err)
		}
	}

	if err := fw.Add(dir); err != nil {
		_ = fw.Close()

		return nil, fmt.Errorf("local registry watch (dir %s): %w", dir, err)
	}

	return &Watcher{
		registry: rg,
		watcher:  fw,
	}, nil
}

// close releases the fsnotify handle. Safe for concurrent and repeated
// calls: Start's goroutine and Stop() both funnel through here.
func (w *Watcher) close() {
	w.closeOnce.Do(func() {
		if w.watcher != nil {
			_ = w.watcher.Close()
		}
	})
}

// Start starts the component.
func (w *Watcher) Start() {
	go func() {
		defer w.close()

		for {
			select {
			case <-w.registry.ctx.Done():
				return
			case event, ok := <-w.watcher.Events:
				if !ok {
					return
				}

				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) {
					w.registry.options.Logger.DebugContext(
						w.registry.ctx,
						"Registry directory changed",
						"registry", w.registry.String(),
						"event", event.String(),
					)

					ins, err := w.registry.Load()
					if err != nil {
						w.registry.options.Logger.ErrorContext(
							w.registry.ctx,
							"Reload services list failed",
							"registry", w.registry.String(),
							"id", w.registry.options.ID,
							"name", w.registry.options.Name,
							"error", err.Error(),
						)

						continue
					}

					w.registry.options.Logger.InfoContext(
						w.registry.ctx,
						"Watcher triggered",
						"registry", w.registry.String(),
						"id", w.registry.options.ID,
						"name", w.registry.options.Name,
					)

					registry.PurgePool(ins)
				}
			case err, ok := <-w.watcher.Errors:
				if !ok {
					return
				}

				w.registry.options.Logger.ErrorContext(
					w.registry.ctx,
					"Inotify watcher error",
					"registry", w.registry.String(),
					"error", err,
				)
			}
		}
	}()
}

// Stop stops the component and releases resources.
func (w *Watcher) Stop() {
	w.close()
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
