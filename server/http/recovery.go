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
 * @file recovery.go
 * @package http
 * @author Dr.NP <np@herewe.tech>
 * @since 09/04/2026
 */

package http

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/uptrace/bunrouter"

	"github.com/go-sicky/sicky/logger"
)

// NewRecoveryMiddleware recovers panicking middleware and converts the
// panic into an error. It sits directly after NewStatusMiddleware so it
// catches failures from every layer below the recorder while the
// innermost NewErrorMiddleware already handles route-handler panics
// (which must be converted below the access logger, or the access log
// and the RED metrics would never see the request).
//
// It must not sit after the route handler: net/http recovers per
// connection, but a panic escaping the chain skips the access log and
// the error response entirely.
func NewRecoveryMiddleware(l logger.GeneralLogger) bunrouter.MiddlewareFunc {
	return func(next bunrouter.HandlerFunc) bunrouter.HandlerFunc {
		return func(w http.ResponseWriter, r bunrouter.Request) (err error) {
			defer func() {
				if rec := recover(); rec != nil {
					stack := string(debug.Stack())
					if l != nil {
						l.ErrorContext(r.Context(),
							"http middleware panicked",
							"method", r.Method,
							"path", r.URL.Path,
							"panic", fmt.Sprint(rec),
							"stack", stack,
						)
					}

					err = fmt.Errorf("http middleware panicked: %v", rec)
					WriteErrorResponse(w, r, err)
				}
			}()

			return next(w, r)
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
