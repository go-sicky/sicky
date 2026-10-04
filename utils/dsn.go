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
 * @file dsn.go
 * @package utils
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package utils

import (
	"net"
	"net/url"
	"strings"
)

// Redacted is the placeholder substituted for a value that cannot be shown.
const Redacted = "REDACTED"

// dsnSecretQueryKeys mark a *query parameter* of a DSN as a credential.
//
// This list is deliberately separate from secretKeyFragments in redact.go:
// that one answers "is this config key secret?" and is allowed to be broad
// ("endpoint", "url"), whereas this one answers "is this query parameter a
// credential?" and must not swallow ordinary DSN options such as sslmode or
// connect_timeout. Matching is by substring so access_token, refresh_token,
// x-api-key, client_secret and aws_secret_access_key all hit. Over-redacting
// a parameter is harmless; logging one is not.
//
// Known residual: a vendor whose credential parameter contains none of these
// fragments (e.g. a bare "team=") is not redacted. Widening the list trades
// that against swallowing benign parameters, so add a fragment only when a
// real integration needs it.
var dsnSecretQueryKeys = []string{
	"token", "secret", "pass", "pwd", "key", "auth", "cred", "sig",
}

// RedactDSN strips credentials from a DSN/URI/endpoint before it is written
// to a log or an error string. It removes userinfo, replaces credential-like
// query parameters, and fails closed to Redacted when the input cannot be
// parsed or still looks like it carries a secret.
//
// Two shapes pass through untouched, because redacting them would make every
// operational log useless and neither carries a credential:
//
//   - the empty string
//   - a bare host:port such as "127.0.0.1:4317" (the default OTLP endpoint)
//
// This is the single implementation. infra.RedactDSN and tracer.RedactDSN
// both delegate here — they used to be two independent functions, one of
// which failed open and returned any DSN without "@" verbatim, sending OTLP
// API keys to the log stream in clear.
func RedactDSN(raw string) string {
	if raw == "" {
		return ""
	}

	// A bare host:port has no scheme, and url.Parse rejects it outright when
	// the host starts with a digit ("first path segment in URL cannot contain
	// colon"). Detect that shape before parsing: 127.0.0.1:4317 is the default
	// OTLP endpoint, and blanking it would make every tracer log useless. The
	// guards exclude anything carrying userinfo, a path, a query or a
	// fragment, so only a bare address takes this path.
	if !strings.ContainsAny(raw, "@/?#") {
		if _, _, err := net.SplitHostPort(raw); err == nil {
			return raw
		}
	}

	u, err := url.Parse(raw)
	if err != nil {
		return Redacted
	}

	hadUser := u.User != nil

	q := u.Query()
	swept := false

	for k := range q {
		if isDSNSecretQueryKey(k) {
			q.Set(k, Redacted)
			swept = true
		}
	}

	if swept {
		u.RawQuery = q.Encode()
	}

	u.User = nil

	// No host component and not a bare address: an opaque or path-only DSN
	// that merely happens to parse. Unvouched for, so refuse it.
	if u.Host == "" {
		// A bare host:port is not a DSN, so passing it through is correct —
		// but only when it plainly is one. Two things disqualify it:
		//
		//   - an "@", because a DSN's credential is always the userinfo
		//     before it. url.Parse puts "https//tok@host:4317/1" entirely in
		//     Path with User nil, so hadUser is false and this branch used to
		//     return the whole string, token and all.
		//   - net.SplitHostPort does not validate that the port is numeric.
		//     "14317/1" parses as a port just fine, which is what let the
		//     case above through.
		if !hadUser && !swept && u.RawQuery == "" && !strings.Contains(raw, "@") {
			if _, port, err := net.SplitHostPort(raw); err == nil && isNumericPort(port) {
				return raw
			}
		}

		return Redacted
	}

	s := u.String()

	// url.String can still render stripped userinfo as "scheme://:host" for
	// some exotic inputs; anything shaped like that is refused.
	if strings.Contains(s, "://:") {
		return Redacted
	}

	return s
}

// isDSNSecretQueryKey reports whether a DSN query parameter name denotes a
// credential. Substring match, lower-cased.
func isDSNSecretQueryKey(key string) bool {
	lk := strings.ToLower(key)
	for _, fragment := range dsnSecretQueryKeys {
		if strings.Contains(lk, fragment) {
			return true
		}
	}

	return false
}

// isNumericPort reports whether a SplitHostPort port component is digits only.
// net.SplitHostPort accepts anything without a colon, so "14317/1" comes back
// as a port — and a DSN path segment riding along in it is how a malformed
// scheme used to walk straight through the redaction.
func isNumericPort(port string) bool {
	if port == "" {
		return false
	}

	for _, r := range port {
		if r < '0' || r > '9' {
			return false
		}
	}

	return true
}
