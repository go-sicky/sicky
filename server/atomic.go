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
 * @file atomic.go
 * @package server
 * @author Dr.NP <np@herewe.tech>
 * @since 09/27/2026
 */

package server

import "sync/atomic"

// AppendAtomicSlice publishes vals onto the slice held by p, retrying
// until the compare-and-swap succeeds.
//
// The compare value must be the very snapshot the new slice was built
// from. A loop that re-reads p for the comparison compares against the
// *latest* value instead, so it succeeds on top of a concurrent writer
// and silently drops everything that writer published - which is how
// concurrently registered handlers went missing while their registration
// was already logged.
func AppendAtomicSlice[T any](p *atomic.Pointer[[]T], vals ...T) {
	if p == nil {
		return
	}

	for {
		cur := p.Load()

		var old []T
		if cur != nil {
			old = *cur
		}

		next := make([]T, 0, len(old)+len(vals))
		next = append(next, old...)
		next = append(next, vals...)

		if p.CompareAndSwap(cur, &next) {
			return
		}
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
