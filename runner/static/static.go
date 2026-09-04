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

	"github.com/go-sicky/sicky/runner"
	"github.com/go-sicky/sicky/utils"
	"github.com/google/uuid"
)

type Static struct {
	config  *Config
	ctx     context.Context
	options *runner.Options

	mu      sync.Mutex
	wg      sync.WaitGroup
	task    chan *runner.Task
	started bool
	stopped bool
}

// New static runner (pool)
func New(opts *runner.Options, cfg *Config) *Static {
	opts = opts.Ensure()
	cfg = cfg.Ensure()

	r := &Static{
		config:  cfg,
		ctx:     opts.Context,
		options: opts,
		task:    make(chan *runner.Task, opts.BufferSize),
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

func (r *Static) Context() context.Context {
	return r.ctx
}

func (r *Static) Options() *runner.Options {
	return r.options
}

func (r *Static) String() string {
	return "static"
}

func (r *Static) ID() uuid.UUID {
	return r.options.ID
}

func (r *Static) Name() string {
	return r.options.Name
}

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
	for idx := 0; idx < n; idx++ {
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

func (r *Static) Stop() error {
	r.mu.Lock()
	ch := r.task
	if !r.stopped && ch != nil {
		r.stopped = true
		r.task = nil
		close(ch)
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

func (r *Static) Task(t *runner.Task) {
	if t == nil {
		return
	}
	// Hold the lock across the send: Stop closes the channel under the
	// same lock, so sending unlocked could panic on a closed channel.
	// Workers drain the queue without this lock, so a blocked send only
	// waits for a free slot (and Stop waits behind it) — never deadlocks
	// on the mutex itself.
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.task == nil || r.stopped || !r.started {
		return
	}
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}

	// Blocking send: back-pressures the caller when the queue is full.
	// Never call Task from inside the runner Handler itself — with all
	// workers blocked in Handler the queue can never drain (deadlock).
	// Use TryTask for a non-blocking or bounded-wait enqueue.
	r.task <- t
}

// TryTask enqueues t without indefinite blocking. A non-positive timeout
// tries once and returns runner.ErrPoolFull when the queue is full;
// a positive timeout bounds the wait. It returns runner.ErrPoolFull
// when the runner is stopped.
//
// The lock is held across the send (see Task): an in-flight TryTask
// delays Stop by at most the caller's timeout.
func (r *Static) TryTask(t *runner.Task, timeout time.Duration) error {
	if t == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.task == nil || r.stopped || !r.started {
		return runner.ErrPoolFull
	}
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}

	if timeout <= 0 {
		select {
		case r.task <- t:
			return nil
		default:
			return runner.ErrPoolFull
		}
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case r.task <- t:
		return nil
	case <-timer.C:
		return runner.ErrPoolFull
	}
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
