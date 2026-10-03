/*
 * The MIT License (MIT)
 *
 * Copyright (c) 2024 HereweTech Co.LTD
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */

/**
 * @file leakcheck.go
 * @package leakcheck
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

// Package leakcheck holds the goroutine-leak policy shared by every test
// package in this module. It exists so the exclusions below are written and
// justified once: a leak check that quietly stops running is worse than no
// leak check, and a long justification duplicated across a dozen TestMain
// functions is a comment that will drift.
//
// It is imported only from _test.go files, so it is never linked into a
// binary built from this module.
package leakcheck

import "go.uber.org/goleak"

// fasthttp goroutines that no stop path reaches.
//
// server/fiber and server/websocket both sit on fasthttp (fiber v3 uses it
// directly; websocket pulls it in for its hand-built RequestCtx test helper).
// Two of its goroutines have no public stop:
//
//   - the prefork worker pool, created once per Server.Serve and slept on by
//     Server.Shutdown with nothing to wake it;
//   - the ServerDate refresher, a sync.Once-guarded ticker.
//
// Neither is ours and neither is reachable by a Stop, so a check that
// included them would be red from the first run and every exclusion added
// afterwards would erode the check itself.
//
// IgnoreAnyFunction, not IgnoreTopFunction: both goroutines sit in
// time.Sleep at the top of their stack and only reach fasthttp underneath, so
// matching on the top frame never matches.
//
// The names are fully qualified and the options are returned from one place,
// so a leak from any other origin — including one inside the websocket or
// fiber server itself — is still reported.
var fasthttpExemptions = []goleak.Option{
	goleak.IgnoreAnyFunction("github.com/valyala/fasthttp.(*workerPool).Start.func2"),
	goleak.IgnoreAnyFunction("github.com/valyala/fasthttp.updateServerDate.func1"),
}

// net/http's post-Close linger.
//
// http.Server.Close force-closes every connection, and each one that had a
// pending response parks in conn.closeWriteAndWait first. That function is a
// single unconditional `time.Sleep(rstAvoidanceDelay)` — 500ms as of Go 1.26
// — after which the connection is dropped for real. The sleep does not
// consult the peer, so there is nothing a caller can do to shorten it and
// nothing it can do to hang it.
//
// Exempted for that reason: it is bounded by construction, it holds no state
// of ours, and no code from this module appears anywhere in its stack, so it
// cannot mask a leak here. Anything reachable only through it is a stdlib
// connection being torn down.
var netHTTPExemptions = []goleak.Option{
	goleak.IgnoreAnyFunction("net/http.(*conn).closeWriteAndWait"),
}

// Options returns the goleak options a package's TestMain should use:
//
//	goleak.VerifyTestMain(m, leakcheck.Options()...)
//
// IgnoreCurrent is always first, so a test never has to account for whatever
// testing and the runtime already had running before the suite started.
// Anything a test starts must still be gone when the package run finishes.
func Options(extra ...goleak.Option) []goleak.Option {
	opts := make([]goleak.Option, 0, 1+len(fasthttpExemptions)+len(netHTTPExemptions)+len(extra))
	opts = append(opts, goleak.IgnoreCurrent())
	opts = append(opts, fasthttpExemptions...)
	opts = append(opts, netHTTPExemptions...)

	return append(opts, extra...)
}
