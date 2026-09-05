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
 * @file static.go
 * @package static
 * @author Dr.NP <np@herewe.tech>
 * @since 12/18/2024
 */

package static

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/go-sicky/sicky/runner"
	"github.com/go-sicky/sicky/utils"
)

// Static is a static component.
type Static struct {
	config  *Config
	ctx     context.Context
	options *runner.Options

	mu      sync.Mutex
	wg      sync.WaitGroup
	task    chan *runner.Task
	done    chan struct{}
	started bool
	stopped bool
}

// New static runner (pool).
func New(opts *runner.Options, cfg *Config) *Static {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	r := &Static{
		config:  cfg,
		ctx:     opts.Context,
		options: opts,
		task:    make(chan *runner.Task, opts.BufferSize),
		done:    make(chan struct{}),
	}

	r.options.Logger.InfoContext(
		r.ctx,
		"Runner created",
		"runner", r.String(),
		"id", r.options.ID,
		"name", r.options.Name,
	)

	runner.Set(r)

	return r
}

// Context returns the component context.
func (r *Static) Context() context.Context {
	return r.ctx
}

// Options returns the runtime options.
func (r *Static) Options() *runner.Options {
	return r.options
}

// String returns a human-readable name.
func (r *Static) String() string {
	return "static"
}

// ID returns the unique instance ID.
func (r *Static) ID() uuid.UUID {
	return r.options.ID
}

// Name returns the component name.
func (r *Static) Name() string {
	return r.options.Name
}

// Start starts the component.
func (r *Static) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started {
		return nil
	}

	n := r.options.NThreads
	if n <= 0 {
		n = 1
	}

	for range n {
		r.wg.Add(1)
		go r._worker()
	}

	r.started = true

	r.options.Logger.InfoContext(
		r.ctx,
		"Runner started",
		"runner", r.String(),
		"id", r.options.ID,
		"name", r.options.Name,
		"threads", n,
	)

	return nil
}

// Stop stops the component and releases resources.
func (r *Static) Stop() error {
	r.mu.Lock()
	ch := r.task
	if !r.stopped {
		r.stopped = true
		r.task = nil
		close(r.done)
		if ch != nil {
			close(ch)
		}
	}

	r.mu.Unlock()

	r.wg.Wait()

	r.options.Logger.InfoContext(
		r.ctx,
		"Runner stopped",
		"runner", r.String(),
		"id", r.options.ID,
		"name", r.options.Name,
	)

	return nil
}

// Task submits a task (blocks when the queue is full).
// The queue channel is snapshotted under lock, so a blocked send never
// holds the mutex: Stop closes the channels and returns promptly instead
// of waiting behind a full queue. If Stop wins the race the task is
// dropped (there is no error return to report it — use TryTask when the
// caller must know). Never call Task from inside the runner Handler
// itself — with all workers blocked in Handler the queue can never
// drain (deadlock).
func (r *Static) Task(t *runner.Task) {
	if t == nil {
		return
	}

	r.mu.Lock()
	if r.task == nil || r.stopped || !r.started {
		r.mu.Unlock()

		return
	}

	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}

	ch, done := r.task, r.done
	r.mu.Unlock()

	// Fast path: already stopping.
	select {
	case <-done:
		return
	default:
	}

	// A concurrent Stop closes ch while we send; the recover converts the
	// resulting panic into a drop (same outcome as losing the race above).
	defer func() {
		_ = recover()
	}()

	select {
	case ch <- t:
	case <-done:
	}
}

// TryTask enqueues t without indefinite blocking. A non-positive timeout
// tries once and returns runner.ErrPoolFull when the queue is full;
// a positive timeout bounds the wait. It returns runner.ErrPoolFull
// when the runner is stopped or stopping.
func (r *Static) TryTask(t *runner.Task, timeout time.Duration) error {
	if t == nil {
		return nil
	}

	r.mu.Lock()
	if r.task == nil || r.stopped || !r.started {
		r.mu.Unlock()

		return runner.ErrPoolFull
	}

	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}

	ch, done := r.task, r.done
	r.mu.Unlock()

	send := func() (ok bool) {
		defer func() {
			// A concurrent Stop closes ch mid-send; report it as full.
			_ = recover()
		}()

		if timeout <= 0 {
			select {
			case ch <- t:
				return true
			case <-done:
				return false
			default:
				return false
			}
		}

		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case ch <- t:
			return true
		case <-done:
			return false
		case <-timer.C:
			return false
		}
	}

	if !send() {
		return runner.ErrPoolFull
	}

	return nil
}

// Len reports the current number of queued (not yet picked-up) tasks.
func (r *Static) Len() int {
	r.mu.Lock()
	ch := r.task
	r.mu.Unlock()
	if ch == nil {
		return 0
	}

	return len(ch)
}

func (r *Static) _worker() {
	defer r.wg.Done()
	self := utils.GoroutineID()

	// Snapshot the channel: ranging the r.task field directly races with
	// Stop (which nils it) — a worker that first evaluates the range after
	// Stop would range a nil channel and block forever.
	r.mu.Lock()
	ch := r.task
	r.mu.Unlock()
	if ch == nil {
		return
	}

	for t := range ch {
		func() {
			defer func() {
				_ = recover()
			}()
			r.options.Logger.TraceContext(
				r.ctx,
				"Runner task created",
				"runner", r.String(),
				"id", r.options.ID,
				"name", r.options.Name,
				"worker", self,
				"task", t.ID.String(),
			)

			// Call handler
			if r.options.Handler != nil {
				err := r.options.Handler(t)
				if err != nil {
					r.options.Logger.ErrorContext(
						r.ctx,
						"Runner worker run failed",
						"runner", r.String(),
						"id", r.options.ID,
						"name", r.options.Name,
						"worker", self,
						"task", t.ID.String(),
						"error", err.Error(),
					)
				}
			}
		}()
	}

	r.options.Logger.DebugContext(
		r.ctx,
		"Runner worker created",
		"runner", r.String(),
		"id", r.options.ID,
		"name", r.options.Name,
		"worker", self,
	)
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
