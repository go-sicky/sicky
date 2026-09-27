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
 * @file redact.go
 * @package utils
 * @author Dr.NP <np@herewe.tech>
 * @since 09/27/2026
 */

package utils

import "strings"

// secretKeyFragments mark a key (or any of its ancestors) as carrying a
// credential. Matching is by substring so `authorization`,
// `access_key_id`, `tls_key_pem` or a nested `secrets.db` all hit.
var secretKeyFragments = []string{
	"dsn", "password", "passwd", "pwd", "secret", "token", "apikey", "api_key", "api-key",
	"auth", "private_key", "privatekey", "accesskey", "access_key", "secret_key",
	"session_token", "uri", "url", "broker", "addresses", "cloud_id", "cloud_url",
	"creds_file", "nkey_file", "ca_file", "ca_cert_file", "root_ca_file",
	"tls_key", "tls_cert", "key_pem", "cert_pem", "private_key_pem", "endpoint",
}

// SanitizeValue returns val with every secret-looking entry replaced by
// "***redacted***", recursing into maps and slices.
//
// It backs both the Manager's /config endpoint and `sicky config show`:
// the two used to keep separate, diverging redaction lists, and the CLI's
// copy silently passed through private keys, access keys, arrays and
// `tracer.headers.Authorization`. A nil or empty value under a secret key
// stays as it is (nothing to hide, and replacing it would add noise).
func SanitizeValue(key string, val any) any {
	lk := strings.ToLower(key)
	for _, sub := range secretKeyFragments {
		if strings.Contains(lk, sub) {
			if s, ok := val.(string); ok && s != "" {
				return "***redacted***"
			}

			if val != nil {
				return "***redacted***"
			}

			return val
		}
	}

	switch v := val.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, vv := range v {
			out[k] = SanitizeValue(k, vv)
		}

		return out
	case []any:
		out := make([]any, len(v))
		for i, vv := range v {
			// The element inherits the parent key: `databases: [{password}]`
			// must still hide the password.
			out[i] = SanitizeValue(key, vv)
		}

		return out
	default:
		return val
	}
}
