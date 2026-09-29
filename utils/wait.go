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
 * @file wait.go
 * @package utils
 * @author Dr.NP <np@herewe.tech>
 * @since 09/27/2026
 */

package utils

import (
	"errors"
	"sync"
	"time"
)

// ErrStopTimeout reports that a Stop gave up waiting for running work.
// Callers keep shutting down; the work that outlived the wait must not be
// restarted until it drains (components expose that as a refused Start).
var ErrStopTimeout = errors.New("stop timed out waiting for running handlers")

// StopTimeout bounds every Stop() that waits on user code: runner
// workers, job loops and the server accept loop. A handler that never
// returns would otherwise wedge the whole shutdown sequence - the
// orchestrator's force timeout cancels a context, which none of these
// waits observe.
//
// It is a variable so tests can shorten it; production code should not
// write it.
var StopTimeout = 10 * time.Second

// WaitGroupTimeout waits for wg to reach zero and gives up after timeout
// (a non-positive timeout waits indefinitely). onDrained runs exactly
// once when the group does drain - including when that happens after the
// timeout already fired - so callers can use it to clear a "still
// draining" flag without losing the race between the two outcomes.
//
// The helper goroutine outlives a timeout: it is waiting on wg, which is
// what the caller wanted anyway.
func WaitGroupTimeout(wg *sync.WaitGroup, timeout time.Duration, onDrained func()) bool {
	return WaitTimeout(wg.Wait, timeout, onDrained)
}

// WaitTimeout is WaitGroupTimeout for any blocking wait: it runs wait in
// a helper goroutine and gives up after timeout (a non-positive timeout
// waits indefinitely). onDrained runs exactly once when wait returns,
// including after a timeout already fired.
func WaitTimeout(wait func(), timeout time.Duration, onDrained func()) bool {
	done := make(chan struct{})

	go func() {
		wait()

		// onDrained must run before close(done): the waiter resumes on
		// close and would otherwise observe the still-draining flag it
		// was just told to expect cleared, turning a clean restart into
		// a spurious ErrStopTimeout.
		if onDrained != nil {
			onDrained()
		}

		close(done)
	}()

	if timeout <= 0 {
		<-done

		return true
	}

	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}
