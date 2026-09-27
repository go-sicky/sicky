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
 * @file errors.go
 * @package http
 * @author Dr.NP <np@herewe.tech>
 * @since 09/27/2026
 */

package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/uptrace/bunrouter"

	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/utils"
)

// errInternal is the only text a plain (non-HTTPError) failure exposes to
// the client. Handler errors routinely carry DSNs, SQL or file paths, so
// echoing them would hand the internal structure to any caller.
const errInternal = "internal server error"

// NewErrorMiddleware turns handler failures into real HTTP responses and
// must sit last in the chain so it wraps the route handler directly.
//
// bunrouter's ServeHTTP discards the error returned by the handler
// (router.go: `_ = r.ServeHTTPError(...)`), so without this a failing
// handler answers 200 with an empty body and the failure never reaches
// the access logger or the RED metrics. Being innermost, the status
// recorded here is read by the access logger and the tracer above it.
func NewErrorMiddleware(l logger.GeneralLogger) bunrouter.MiddlewareFunc {
	return func(next bunrouter.HandlerFunc) bunrouter.HandlerFunc {
		return func(w http.ResponseWriter, r bunrouter.Request) (err error) {
			defer func() {
				if rec := recover(); rec != nil {
					// Stack and panic value stay server-side: the client
					// only ever sees the generic body from writeErrorResponse.
					if l != nil {
						l.ErrorContext(r.Context(),
							"http handler panicked",
							"method", r.Method,
							"path", r.URL.Path,
							"panic", fmt.Sprint(rec),
							"stack", string(debug.Stack()),
						)
					}

					err = fmt.Errorf("http handler panicked: %v", rec)
					WriteErrorResponse(w, r, err)
				}
			}()

			err = next(w, r)
			if err != nil {
				WriteErrorResponse(w, r, err)
			}

			return err
		}
	}
}

// WriteErrorResponse answers err with a status code when the handler has
// not produced one yet. It is a no-op for responses already started
// (including hijacked connections), so a handler that failed after
// writing keeps the status it chose. The error itself is still returned
// to bunrouter for the access logger above.
func WriteErrorResponse(w http.ResponseWriter, r bunrouter.Request, err error) {
	if err == nil || responseStarted(r.Context()) {
		return
	}

	code, msg := errorResponse(err)

	// 204/205/304 forbid a body: Write after WriteHeader would be
	// superfluous and only produce net/http warning noise.
	if code == http.StatusNoContent || code == http.StatusResetContent || code == http.StatusNotModified {
		w.WriteHeader(code)

		return
	}

	http.Error(w, msg, code)
}

// errorResponse maps an error to the status and the client-facing text.
// Only errors that explicitly describe a client-visible failure keep
// their own code: bunrouter's HTTPError (its contract is that Error() is
// safe to send), a *utils.CodedError (registered status and message, the
// wrapped cause stays server-side), and a body-limit breach. Everything
// else is reported as an opaque 500.
func errorResponse(err error) (code int, msg string) {
	if httpErr, ok := errors.AsType[bunrouter.HTTPError](err); ok {
		code = httpErr.StatusCode()
		if code < 400 {
			// A 4xx/5xx contract is what this path is for; a 2xx/3xx here
			// would hide the failure from the error-rate SLI.
			code = http.StatusInternalServerError
		}

		msg = httpErr.Error()
		if msg == "" {
			msg = errInternal
		}

		return code, msg
	}

	if coded, ok := errors.AsType[*utils.CodedError](err); ok && coded != nil {
		msg = coded.Msg
		if msg == "" {
			msg = utils.MessageOf(coded.Code)
		}
		if msg == "" {
			msg = errInternal
		}

		return utils.StatusOf(coded.Code), msg
	}

	// http.MaxBytesReader surfaces the breach as *http.MaxBytesError on
	// the read that crossed the limit.
	if maxBytes, ok := errors.AsType[*http.MaxBytesError](err); ok && maxBytes != nil {
		return http.StatusRequestEntityTooLarge, "request body too large"
	}

	return http.StatusInternalServerError, errInternal
}

// responseStarted reports whether a status or body already reached the
// recorder, or whether the connection left net/http's control.
func responseStarted(ctx context.Context) bool {
	rec, _ := ctx.Value(statusKey{}).(*statusRecorder)
	if rec == nil {
		// Used without NewStatusMiddleware: assume nothing was written
		// so the caller still answers instead of silently returning 200.
		return false
	}

	return rec.status != 0 || rec.hijacked
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
