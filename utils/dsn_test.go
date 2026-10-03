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
 * @file dsn_test.go
 * @package utils
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package utils

import (
	"strings"
	"testing"
)

// RedactDSN is the single redaction implementation; infra.RedactDSN and
// tracer.RedactDSN delegate to it. Its contract has two halves that pull in
// opposite directions: never emit a credential, but never blank out an address
// that carries none.
func TestRedactDSNStripsCredentials(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		secrets []string
		keep    string
	}{
		{"userinfo", "clickhouse://user:s3cret@localhost:9000/default", []string{"s3cret", "user:"}, "localhost:9000"},
		{"encoded userinfo", "mongodb://admin:p%40ss@mongo:27017/app", []string{"p%40ss"}, "mongo:27017"},
		{"api key query", "https://otlp.example.com/v1/traces?api-key=SECRET", []string{"SECRET"}, "otlp.example.com"},
		{"token query", "uptrace://host/1?token=abc", []string{"abc"}, "uptrace://host/1"},
		{"secret query", "s3://storage/bucket?aws_secret_access_key=AKIA", []string{"AKIA"}, "storage"},
		{"benign params survive", "postgres://h:5432/db?sslmode=disable&connect_timeout=5", nil, "sslmode=disable&connect_timeout=5"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := RedactDSN(c.in)
			for _, secret := range c.secrets {
				if strings.Contains(got, secret) {
					t.Errorf("RedactDSN(%q) = %q, leaks %q", c.in, got, secret)
				}
			}

			if c.keep != "" && !strings.Contains(got, c.keep) {
				t.Errorf("RedactDSN(%q) = %q, lost %q", c.in, got, c.keep)
			}
		})
	}
}

// A credential-free address must survive verbatim. 127.0.0.1:4317 is the
// default OTLP endpoint and does not even parse as a URL, because a scheme
// cannot start with a digit — the pre-parse check exists for exactly this.
func TestRedactDSNKeepsBareAddresses(t *testing.T) {
	cases := []string{"127.0.0.1:4317", "localhost:4317", "otel-collector:4317", "example.com:4317"}

	for _, in := range cases {
		if got := RedactDSN(in); got != in {
			t.Errorf("RedactDSN(%q) = %q, want it unchanged: it carries no credential", in, got)
		}
	}

	if got := RedactDSN(""); got != "" {
		t.Errorf("RedactDSN(\"\") = %q, want empty", got)
	}
}

// Failing closed is the whole point: anything we cannot vouch for is
// replaced rather than logged. The permissive predecessor returned every
// input without "@" verbatim, which is how OTLP API keys reached the logs.
func TestRedactDSNFailsClosed(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"unparseable", "://not a url", Redacted},
		{"path-only with a secret", "host/1?token=abc", Redacted},
		{"opaque with a secret", "collector:4317?access_token=TOK", Redacted},
		{"no host, no port", "just-a-bare-word", Redacted},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := RedactDSN(c.in); got != c.want {
				t.Errorf("RedactDSN(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// Widening dsnSecretQueryKeys is how a real integration gets covered, so the
// substring semantics are pinned: a longer parameter name containing a
// fragment must match, and a benign one must not.
func TestDSNSecretQueryKeyMatching(t *testing.T) {
	matched := []string{
		"api-key", "x-api-key", "api_key", "access_token", "refresh_token",
		"client_secret", "aws_secret_access_key", "password", "pwd", "sig", "signature",
	}

	for _, k := range matched {
		if !isDSNSecretQueryKey(k) {
			t.Errorf("isDSNSecretQueryKey(%q) = false, want true", k)
		}
	}

	benign := []string{"sslmode", "connect_timeout", "database", "replicaSet", "appName", "max_pool_size"}

	for _, k := range benign {
		if isDSNSecretQueryKey(k) {
			t.Errorf("isDSNSecretQueryKey(%q) = true, want false: redacting it makes the DSN useless", k)
		}
	}
}
