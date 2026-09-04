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
 * @file job.go
 * @package job
 * @author Dr.NP <np@herewe.tech>
 * @since 08/18/2024
 */

package job

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

type Job interface {
	// Get context
	Context() context.Context
	// Job options
	Options() *Options
	// Stringify
	String() string
	// Job ID
	ID() uuid.UUID
	// Job name
	Name() string
	// Start job
	Start() error
	// Stop job
	Stop() error
}

var (
	jobs       = make(map[uuid.UUID]Job)
	defaultJob Job
	jobMu      sync.RWMutex
)

func Set(js ...Job) {
	jobMu.Lock()
	defer jobMu.Unlock()

	for _, job := range js {
		jobs[job.ID()] = job
		if defaultJob == nil {
			defaultJob = job
		}
	}
}

func Get(id uuid.UUID) Job {
	jobMu.RLock()
	defer jobMu.RUnlock()

	return jobs[id]
}

func Default() Job {
	jobMu.RLock()
	defer jobMu.RUnlock()

	return defaultJob
}

func Jobs() map[uuid.UUID]Job {
	jobMu.RLock()
	defer jobMu.RUnlock()

	out := make(map[uuid.UUID]Job, len(jobs))
	for id, j := range jobs {
		out[id] = j
	}

	return out
}

// Clear resets the global registry. Intended for tests.
func Clear() {
	jobMu.Lock()
	defer jobMu.Unlock()

	jobs = make(map[uuid.UUID]Job)
	defaultJob = nil
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
