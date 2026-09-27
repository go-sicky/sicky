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
 * @file duration.go
 * @package utils
 * @author Dr.NP <np@herewe.tech>
 * @since 09/27/2026
 */

package utils

import "time"

// NormalizeDuration repairs a bare second count written into a
// time.Duration config field.
//
// JSON/YAML/viper decode `read_timeout: 10` as 10 nanoseconds (the
// sibling `*_sec` fields in the same config mean seconds), which turns
// the timeout into a self-inflicted denial of service: every request
// fails its read deadline. Values below a millisecond are read as a
// count of seconds; zero (use the default) and negative values (abort in
// Validate) are left untouched.
func NormalizeDuration(d time.Duration) time.Duration {
	if d > 0 && d < time.Millisecond {
		// int64 arithmetic: multiplying two Durations reads as a
		// dimensional error to the duration checker.
		return time.Duration(int64(d) * int64(time.Second))
	}

	return d
}
