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
 * @file errors.go
 * @package server
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package server

import "errors"

// ErrIncompleteTLSConfig is returned when only one of tls_cert_pem and
// tls_key_pem is set. Falling back to plaintext silently would be a security
// hole, so startup fails fast instead.
//
// This is the one value for the condition. The four server implementations
// each used to declare their own with this same name, so a caller switching
// on it needed one errors.Is arm per sub-package — and
// errors.Is(httpErr, grpc.ErrIncompleteTLSConfig) was false, because
// errors.New allocates a distinct value per call site. They are now aliases
// of this one, so their exported names still resolve and any of them matches.
var ErrIncompleteTLSConfig = errors.New("incomplete TLS configuration: both tls_cert_pem and tls_key_pem must be set")
