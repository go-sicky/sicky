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
 * @package fiber
 * @author Dr.NP <np@herewe.tech>
 * @since 09/27/2026
 */

package fiber

import (
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/gofiber/fiber/v3"

	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/metrics"
	"github.com/go-sicky/sicky/utils"
)

// errInternal is the only text a plain failure exposes to the client.
// Handler errors routinely carry DSNs, SQL or file paths, so echoing
// them would hand the internal structure to any caller - the detail
// belongs to the access log, which records the error itself.
const errInternal = "internal server error"

// mapFiberError maps an error to the status and the client-facing text.
//
// Only errors that explicitly describe a client-visible failure keep
// their own payload: fiber's own *fiber.Error (whose contract is that
// Message is safe to send - it also carries the framework's 404/405
// answers) and a *utils.CodedError (registered status and message, the
// wrapped cause stays server-side). Everything else becomes an opaque
// 500.
func mapFiberError(err error) (code int, msg string) {
	if err == nil {
		return http.StatusOK, ""
	}

	if ferr, ok := errors.AsType[*fiber.Error](err); ok && ferr != nil {
		code = ferr.Code
		if code < http.StatusBadRequest {
			// A 4xx/5xx contract is what this path is for; a 2xx/3xx here
			// would hide the failure from the error-rate SLI.
			code = http.StatusInternalServerError
		}

		msg = ferr.Message
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

	return http.StatusInternalServerError, errInternal
}

// NewErrorHandler answers failing requests instead of echoing the error.
//
// fiber's DefaultErrorHandler sends err.Error() verbatim (app.go
// DefaultErrorHandler), so a handler returning
// fmt.Errorf("query: %w", dbErr) would leak the SQL, the DSN or the file
// path as the response body. It also keeps the gRPC stack's promise:
// clients only ever see a generic message.
func NewErrorHandler() fiber.ErrorHandler {
	return func(c fiber.Ctx, err error) error {
		code, msg := mapFiberError(err)

		c.Set(fiber.HeaderContentType, fiber.MIMETextPlainCharsetUTF8)

		return c.Status(code).SendString(msg)
	}
}

// NewPanicHandler converts a recovered panic into the same generic 500
// and keeps the panic value plus the stack on the server side.
//
// fiber's default handler returns the panic value as the error, which
// DefaultErrorHandler then ships to the client.
func NewPanicHandler(l logger.GeneralLogger) func(c fiber.Ctx, r any) error {
	return func(c fiber.Ctx, r any) error {
		metrics.ServerPanicsTotal.WithLabelValues("fiber", metrics.NormalizeHTTPMethod(c.Method())).Inc()

		if l != nil {
			l.ErrorContext(c.Context(),
				"fiber handler panicked",
				"method", c.Method(),
				"path", c.Path(),
				"panic", fmt.Sprint(r),
				"stack", string(debug.Stack()),
			)
		}

		return fiber.NewError(http.StatusInternalServerError, errInternal)
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
