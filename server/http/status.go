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
 * @file status.go
 * @package http
 * @author Dr.NP <np@herewe.tech>
 * @since 09/04/2026
 */

package http

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/uptrace/bunrouter"
)

// statusKey carries the recorder through the request context so
// downstream middlewares (access logger) can read the real status even
// when intermediate layers rebuild the Request value.
type statusKey struct{}

// statusRecorder captures the status code. net/http never exposes it on
// the ResponseWriter, so without this the access logger cannot classify
// 4xx/5xx responses.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

// WriteHeader is part of the public API.
func (w *statusRecorder) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}

	w.ResponseWriter.WriteHeader(code)
}

// Write writes data.
func (w *statusRecorder) Write(b []byte) (int, error) {
	// Implicit 200 on first Write without an explicit WriteHeader.
	if w.status == 0 {
		w.status = http.StatusOK
	}

	return w.ResponseWriter.Write(b)
}

// Flush is part of the public API.
func (w *statusRecorder) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack is part of the public API.
func (w *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}

	return nil, nil, fmt.Errorf("hijack not supported by wrapped ResponseWriter %T", w.ResponseWriter)
}

// Push is part of the public API.
func (w *statusRecorder) Push(target string, opts *http.PushOptions) error {
	if p, ok := w.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}

	return http.ErrNotSupported
}

// ReadFrom is part of the public API.
func (w *statusRecorder) ReadFrom(r io.Reader) (int64, error) {
	if rf, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		return rf.ReadFrom(r)
	}

	return io.Copy(w, r)
}

// NewStatusMiddleware records the response status into the request
// context. It must run before the access logger (which reads it) and
// after nothing that terminates the chain early for logged requests.
func NewStatusMiddleware() bunrouter.MiddlewareFunc {
	return func(next bunrouter.HandlerFunc) bunrouter.HandlerFunc {
		return func(w http.ResponseWriter, r bunrouter.Request) error {
			rec := &statusRecorder{ResponseWriter: w}
			r = r.WithContext(context.WithValue(r.Context(), statusKey{}, rec))

			return next(rec, r)
		}
	}
}

// StatusFromContext returns the recorded response status, or 0 when the
// status middleware did not run (e.g. a middleware answered early).
func StatusFromContext(ctx context.Context) int {
	if rec, ok := ctx.Value(statusKey{}).(*statusRecorder); ok && rec != nil {
		return rec.status
	}

	return 0
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
