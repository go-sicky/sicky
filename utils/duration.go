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
// fails its read deadline. A value below a millisecond is read as a
// count of seconds by scaling its raw nanosecond value, which is what
// turns the 10 of `read_timeout: 10` into 10s.
//
// Zero and negative values are returned untouched so the caller can
// tell them apart. This function deliberately does not decide what a
// negative means, and there is no Validate that aborts on one: each
// caller owns that decision, and every caller in this repo treats a
// negative safely. server/http and server/fiber clamp a negative to 0
// and then substitute the default; server/grpc clamps its keepalive
// fields to 0, which gRPC reads as "use the client default", and its
// ShutdownTimeout to the default; client/grpc ignores a non-positive
// ConnectionTimeout at the point of use, which means "no timeout".
// So a negative duration is never honored as a deadline, and callers
// that want a different policy should clamp before calling, not expect
// this function to reject the value.
func NormalizeDuration(d time.Duration) time.Duration {
	if d > 0 && d < time.Millisecond {
		// int64 arithmetic: multiplying two Durations reads as a
		// dimensional error to the duration checker.
		return time.Duration(int64(d) * int64(time.Second))
	}

	return d
}
