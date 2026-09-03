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
	"net/http"
	"strconv"

	"github.com/uptrace/bunrouter"
)

// DefaultCORSMaxAge is the preflight cache lifetime in seconds.
const DefaultCORSMaxAge = 86400

// CORSConfig whitelists cross-origin access. An empty AllowedOrigins
// denies all cross-origin requests (no ACAO headers emitted); use an
// explicit "*" entry only for public APIs, never with AllowCredentials.
type CORSConfig struct {
	AllowedOrigins   []string `json:"allowed_origins" yaml:"allowed_origins" mapstructure:"allowed_origins"`
	AllowCredentials bool     `json:"allow_credentials" yaml:"allow_credentials" mapstructure:"allow_credentials"`
	MaxAge           int      `json:"max_age" yaml:"max_age" mapstructure:"max_age"`
}

func (c *CORSConfig) Ensure() *CORSConfig {
	if c == nil {
		c = new(CORSConfig)
	}

	if c.MaxAge <= 0 {
		c.MaxAge = DefaultCORSMaxAge
	}

	return c
}

// Deprecated: CORSMiddleware reflects any Origin with Allow-Credentials.
// Do not mount it; use NewCORSMiddleware with an explicit whitelist.
func CORSMiddleware(next bunrouter.HandlerFunc) bunrouter.HandlerFunc {
	return func(w http.ResponseWriter, r bunrouter.Request) error {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return next(w, r)
		}

		h := w.Header()
		h.Set("Access-Control-Allow-Origin", origin)
		h.Set("Access-Control-Allow-Credentials", "true")

		if r.Method == http.MethodOptions {
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			h.Set("Access-Control-Max-Age", "85400")

			return nil
		}

		return next(w, r)
	}
}

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

	return func(next bunrouter.HandlerFunc) bunrouter.HandlerFunc {
		return func(w http.ResponseWriter, r bunrouter.Request) error {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return next(w, r)
			}

			_, ok := allowed[origin]
			if !ok {
				if wildcard && !cfg.AllowCredentials {
					h := w.Header()
					h.Set("Access-Control-Allow-Origin", "*")
					h.Set("Vary", "Origin")
				}
				// Untrusted origin: passthrough without ACAO so the
				// browser blocks the read. Never reflect + credential.
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusNoContent)

					return nil
				}

				return next(w, r)
			}

			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Vary", "Origin")
			if cfg.AllowCredentials {
				h.Set("Access-Control-Allow-Credentials", "true")
			}

			if r.Method == http.MethodOptions {
				h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				h.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID, X-B3-Traceid, X-B3-Spanid, X-B3-Parentspanid, X-B3-Sampled")
				h.Set("Access-Control-Max-Age", strconv.Itoa(cfg.MaxAge))
				w.WriteHeader(http.StatusNoContent)

				return nil
			}

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
