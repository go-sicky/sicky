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
 * @file cors.go
 * @package http
 * @author Dr.NP <np@herewe.tech>
 * @since 08/31/2026
 */

package http

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/uptrace/bunrouter"
)

// DefaultCORSMaxAge is the preflight cache lifetime in seconds.
const DefaultCORSMaxAge = 86400

// corsRequestIDHeader is the response header the propagation middleware
// writes; advertised by default so a browser client can read it once
// ExposedHeaders is configured.
const corsRequestIDHeader = "X-Request-ID"

// The preflight advertisement used when AllowedMethods/AllowedHeaders are
// left unset. They exist so a preflight without an
// Access-Control-Request-Headers value still names the framework's own
// headers (tracing, request id) instead of an empty list; a client that asks
// for anything else gets its request echoed verbatim.
var (
	defaultCORSAllowedMethods = []string{
		http.MethodGet,
		http.MethodHead,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodOptions,
		"QUERY",
	}
	defaultCORSAllowedHeaders = []string{
		"Content-Type",
		"Authorization",
		corsRequestIDHeader,
		"X-B3-Traceid",
		"X-B3-Spanid",
		"X-B3-Parentspanid",
		"X-B3-Sampled",
	}
)

// CORSConfig whitelists cross-origin access. An empty AllowedOrigins
// denies all cross-origin requests (no ACAO headers emitted); use an
// explicit "*" entry only for public APIs, never with AllowCredentials.
type CORSConfig struct {
	AllowedOrigins []string `json:"allowed_origins" mapstructure:"allowed_origins" yaml:"allowed_origins"`
	// AllowedMethods is the preflight Access-Control-Allow-Methods list.
	// An empty list advertises the framework default (GET, HEAD, POST, PUT,
	// PATCH, DELETE, OPTIONS, QUERY): preflight methods are advisory, the
	// router still decides which methods a route answers.
	AllowedMethods []string `json:"allowed_methods" mapstructure:"allowed_methods" yaml:"allowed_methods"`
	// AllowedHeaders is the preflight Access-Control-Allow-Headers list.
	// An empty list echoes the request's Access-Control-Request-Headers
	// instead, so a client header the framework has never heard of (an API
	// key, a tenant id) cannot break the preflight. When the request names
	// none, the framework default (Content-Type, Authorization,
	// X-Request-ID and the B3 headers) is advertised.
	AllowedHeaders []string `json:"allowed_headers" mapstructure:"allowed_headers" yaml:"allowed_headers"`
	// ExposedHeaders lists response headers browser scripts may read
	// (Access-Control-Expose-Headers). Empty exposes none beyond the CORS
	// simple response headers; set it to hand X-Request-ID and friends to
	// a browser client.
	ExposedHeaders   []string `json:"exposed_headers"   mapstructure:"exposed_headers"   yaml:"exposed_headers"`
	AllowCredentials bool     `json:"allow_credentials" mapstructure:"allow_credentials" yaml:"allow_credentials"`
	MaxAge           int      `json:"max_age"           mapstructure:"max_age"           yaml:"max_age"`
}

// Ensure fills zero-valued fields with defaults and returns the receiver (nil-safe).
func (c *CORSConfig) Ensure() *CORSConfig {
	if c == nil {
		c = new(CORSConfig)
	}

	if c.MaxAge <= 0 {
		c.MaxAge = DefaultCORSMaxAge
	}

	// AllowedMethods/AllowedHeaders/ExposedHeaders are deliberately left
	// untouched: an empty list means "unset" and is resolved at response
	// time, so filling it here would erase the difference between an
	// operator omitting the key and explicitly asking for nothing.
	return c
}

// Validate rejects wildcard-origin with credentials (browsers forbid it
// and it leaks authenticated responses to any site) and validates the
// method/header lists, which are written into response headers.
func (c *CORSConfig) Validate() error {
	if c == nil {
		return nil
	}

	if err := validateCORSTokens("allowed_methods", c.AllowedMethods); err != nil {
		return err
	}

	if err := validateCORSTokens("allowed_headers", c.AllowedHeaders); err != nil {
		return err
	}

	if err := validateCORSTokens("exposed_headers", c.ExposedHeaders); err != nil {
		return err
	}

	if !c.AllowCredentials {
		return nil
	}

	if hasCORSWildcard(c.AllowedOrigins) {
		return errors.New("http: AllowedOrigins \"*\" cannot be combined with AllowCredentials")
	}

	// A literal "*" in a credentialed response is not a wildcard, so the
	// browser rejects the very headers it was meant to allow - the config
	// would look correct and fail every credentialed preflight.
	for _, field := range []struct {
		name   string
		values []string
	}{
		{"allowed_methods", c.AllowedMethods},
		{"allowed_headers", c.AllowedHeaders},
		{"exposed_headers", c.ExposedHeaders},
	} {
		if hasCORSWildcard(field.values) {
			return fmt.Errorf("http: %s \"*\" cannot be combined with AllowCredentials", field.name)
		}
	}

	return nil
}

// corsTokenChars is the RFC 9110 token tail: everything a header name or
// method may carry besides alphanumerics.
const corsTokenChars = "!#$%&'*+-.^_`|~" //nolint:gosec // RFC 9110 token characters, not a credential

// validCORSToken reports whether value is a valid HTTP header name/method.
func validCORSToken(value string) bool {
	if value == "" {
		return false
	}

	for i := range len(value) {
		c := value[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' {
			continue
		}

		if !strings.ContainsRune(corsTokenChars, rune(c)) {
			return false
		}
	}

	return true
}

// validateCORSTokens trims and checks one CORS list. "*" is legal for the
// three CORS response lists when it stands alone; a bare "*" mixed with
// explicit entries would be emitted literally by browsers, so it is
// rejected instead of silently ignored.
func validateCORSTokens(field string, values []string) error {
	for i, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			return fmt.Errorf("http: %s[%d] is empty", field, i)
		}

		if value == "*" {
			if i != 0 || len(values) != 1 {
				return fmt.Errorf("http: %s[%d] \"*\" must be the only entry", field, i)
			}

			continue
		}

		if strings.ContainsRune(value, '*') {
			return fmt.Errorf("http: %s[%d] %q: \"*\" must be the only entry", field, i, value)
		}

		if !validCORSToken(value) {
			return fmt.Errorf("http: %s[%d] %q is not a valid HTTP token", field, i, value)
		}
	}

	return nil
}

// hasCORSWildcard reports whether the trimmed list carries the "*" entry.
func hasCORSWildcard(values []string) bool {
	return slices.ContainsFunc(values, func(value string) bool {
		return strings.TrimSpace(value) == "*"
	})
}

// corsList joins a configured list with the separator CORS response headers
// use, dropping blank entries so a stray "" cannot produce "a, , b".
func corsList(values []string) string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			cleaned = append(cleaned, value)
		}
	}

	return strings.Join(cleaned, ", ")
}

// corsAllowMethods renders Access-Control-Allow-Methods.
func corsAllowMethods(cfg *CORSConfig) string {
	if len(cfg.AllowedMethods) == 0 {
		return strings.Join(defaultCORSAllowedMethods, ", ")
	}

	cleaned := make([]string, 0, len(cfg.AllowedMethods))
	for _, method := range cfg.AllowedMethods {
		if method = strings.ToUpper(strings.TrimSpace(method)); method != "" {
			cleaned = append(cleaned, method)
		}
	}

	if len(cleaned) == 0 {
		return strings.Join(defaultCORSAllowedMethods, ", ")
	}

	return strings.Join(cleaned, ", ")
}

// corsAllowHeaders renders Access-Control-Allow-Headers: an explicit list
// wins, otherwise the request's own Access-Control-Request-Headers is
// echoed (what the browser actually needs), and only a request that names
// no headers falls back to the framework default.
func corsAllowHeaders(cfg *CORSConfig, requested string) string {
	if len(cfg.AllowedHeaders) > 0 {
		if joined := corsList(cfg.AllowedHeaders); joined != "" {
			return joined
		}
	}

	if requested = strings.TrimSpace(requested); requested != "" {
		return requested
	}

	return strings.Join(defaultCORSAllowedHeaders, ", ")
}

// corsExposeHeaders renders Access-Control-Expose-Headers, empty when
// nothing is configured (browsers then expose only the simple headers).
func corsExposeHeaders(cfg *CORSConfig) string {
	return corsList(cfg.ExposedHeaders)
}

// writePreflightHeaders answers a preflight: the browser rejects the
// actual request unless Allow-Methods/Allow-Headers come back with the
// origin grant. The echoed header list makes the answer request-dependent,
// so the preflight varies on the request headers and method as well;
// without that a shared cache may replay one client's answer to another.
func writePreflightHeaders(h http.Header, cfg *CORSConfig, requestedHeaders string) {
	h.Set("Access-Control-Allow-Methods", corsAllowMethods(cfg))
	h.Set("Access-Control-Allow-Headers", corsAllowHeaders(cfg, requestedHeaders))
	h.Set("Access-Control-Max-Age", strconv.Itoa(cfg.MaxAge))
	h.Add("Vary", "Access-Control-Request-Method")
	h.Add("Vary", "Access-Control-Request-Headers")
}

// writeExposeHeaders grants browser scripts read access to the configured
// response headers. Only meaningful on an actual response, so preflights
// skip it.
func writeExposeHeaders(h http.Header, cfg *CORSConfig) {
	if exposed := corsExposeHeaders(cfg); exposed != "" {
		h.Set("Access-Control-Expose-Headers", exposed)
	}
}

// Deprecated: CORSMiddleware is a hardened alias that denies all
// cross-origin requests (passthrough without ACAO headers). It used to
// reflect any Origin with Allow-Credentials, which enables credential
// hijacking. Do not mount it; use NewCORSMiddleware with an explicit
// whitelist.
func CORSMiddleware(next bunrouter.HandlerFunc) bunrouter.HandlerFunc {
	denyAll := (&CORSConfig{}).Ensure()

	return NewCORSMiddleware(denyAll)(next)
}

// NewCORSMiddleware creates a new CORSMiddleware.
func NewCORSMiddleware(cfg *CORSConfig) bunrouter.MiddlewareFunc {
	cfg = cfg.Ensure()

	allowed := make(map[string]struct{}, len(cfg.AllowedOrigins))
	wildcard := false
	for _, o := range cfg.AllowedOrigins {
		if o == "*" {
			wildcard = true
			continue
		}

		allowed[o] = struct{}{}
	}

	// Fail closed: a wildcard origin must never be combined with
	// credentials (browsers reject it and it leaks authenticated
	// responses). Drop the wildcard instead of emitting the illegal
	// combination; configure explicit origins to allow credentials.
	if wildcard && cfg.AllowCredentials {
		wildcard = false
	}

	return func(next bunrouter.HandlerFunc) bunrouter.HandlerFunc {
		return func(w http.ResponseWriter, r bunrouter.Request) error {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return next(w, r)
			}

			h := w.Header()
			// Every response that consulted Origin varies on it, and Add
			// instead of Set so another middleware's Vary (e.g.
			// Accept-Encoding) survives. A shared cache serving one
			// origin's response to another is the failure mode here.
			h.Add("Vary", "Origin")

			_, ok := allowed[origin]
			if !ok {
				if wildcard && !cfg.AllowCredentials {
					h.Set("Access-Control-Allow-Origin", "*")
					if r.Method != http.MethodOptions {
						writeExposeHeaders(h, cfg)
					}
				}

				// Untrusted origin: passthrough without ACAO so the
				// browser blocks the read. Never reflect + credential.
				if r.Method == http.MethodOptions {
					if wildcard && !cfg.AllowCredentials {
						// A wildcard answer still needs the preflight
						// heads, otherwise every preflighted request
						// (anything but a simple one) fails while plain
						// GETs work - a half-working configuration.
						writePreflightHeaders(h, cfg, r.Header.Get("Access-Control-Request-Headers"))
					}

					w.WriteHeader(http.StatusNoContent)

					return nil
				}

				return next(w, r)
			}

			h.Set("Access-Control-Allow-Origin", origin)
			if cfg.AllowCredentials {
				h.Set("Access-Control-Allow-Credentials", "true")
			}

			if r.Method == http.MethodOptions {
				writePreflightHeaders(h, cfg, r.Header.Get("Access-Control-Request-Headers"))
				w.WriteHeader(http.StatusNoContent)

				return nil
			}

			writeExposeHeaders(h, cfg)

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
