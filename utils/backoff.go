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
 * @file backoff.go
 * @package utils
 * @author Dr.NP <np@herewe.tech>
 * @since 09/04/2026
 */

package utils

import (
	"sync"
	"time"
)

// Backoff is a capped exponential backoff with deterministic ±25% jitter.
// The zero value is invalid: build one with NewBackoff. It is safe for
// concurrent use by a single loop (one goroutine calls Next/Reset).
type Backoff struct {
	base time.Duration
	cap  time.Duration

	mu       sync.Mutex
	attempts uint64
}

// NewBackoff returns a backoff starting at base and doubling (with jitter)
// up to maxDelay. Non-positive inputs fall back to 50ms base / 1s cap.
func NewBackoff(base, maxDelay time.Duration) *Backoff {
	if base <= 0 {
		base = 50 * time.Millisecond
	}

	if maxDelay <= 0 {
		maxDelay = time.Second
	}

	return &Backoff{base: base, cap: maxDelay}
}

// Next returns the sleep for the current consecutive failure and advances
// the sequence. Call Reset after a success.
func (b *Backoff) Next() time.Duration {
	b.mu.Lock()
	a := b.attempts
	b.attempts++
	b.mu.Unlock()

	shift := min(a,
		// base*32 already exceeds any sane cap; avoid overflow
		5)

	d := b.base << shift
	if d <= 0 || d > b.cap {
		d = b.cap
	}

	// Jitter needs a positive halved magnitude: d==1 would divide by zero,
	// and the int64->uint64 conversion below is only sound for d > 0
	// (guaranteed by the cap guard above).
	half := d / 2
	if half <= 0 {
		return d
	}

	// Deterministic splitmix64 jitter in [-25%, +25%]: decorrelates
	// lock-stepped restarts without importing math/rand or locking.
	x := a + 0x9E3779B97F4A7C15
	x = (x ^ (x >> 30)) * 0xBF58476D1CE4E5B9
	x = (x ^ (x >> 27)) * 0x94D049BB133111EB
	x ^= x >> 31
	j := int64(x%uint64(half)) - int64(d/4) //nolint:gosec // G115: half is guarded positive above, conversion cannot overflow

	return d + time.Duration(j)
}

// Reset restarts the sequence after a success.
func (b *Backoff) Reset() {
	b.mu.Lock()
	b.attempts = 0
	b.mu.Unlock()
}

// LogSampler gates repetitive error logs: at most max full logs per
// window, the rest suppressed (counted). The zero value is invalid:
// build one with NewLogSampler. Safe for concurrent use.
type LogSampler struct {
	window time.Duration
	max    int

	mu         sync.Mutex
	start      time.Time
	count      int
	suppressed int64
}

// NewLogSampler allows maxLogs full logs per window. Non-positive inputs fall
// back to 5 logs per second.
func NewLogSampler(maxLogs int, window time.Duration) *LogSampler {
	if maxLogs <= 0 {
		maxLogs = 5
	}

	if window <= 0 {
		window = time.Second
	}

	return &LogSampler{window: window, max: maxLogs, start: time.Now()}
}

// Allow reports whether the caller should emit the full log. When it
// returns allow=false the log was suppressed. When allow=true, suppressed
// carries how many logs were dropped since the last emitted one (0 on a
// fresh window) so the emitted log can note the gap.
func (s *LogSampler) Allow() (allow bool, suppressed int64) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	if now.Sub(s.start) >= s.window {
		s.start = now
		s.count = 0
		s.suppressed = 0
	}

	if s.count < s.max {
		s.count++
		suppressed = s.suppressed
		s.suppressed = 0

		return true, suppressed
	}

	s.suppressed++

	return false, 0
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
