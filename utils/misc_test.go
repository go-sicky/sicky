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
 * @file misc_test.go
 * @package utils
 * @author Dr.NP <np@herewe.tech>
 * @since 07/07/2026
 */

package utils

import (
	"math"
	"testing"
)

func TestRandomString(t *testing.T) {
	t.Run("length zero", func(t *testing.T) {
		s := RandomString(0)
		if len(s) != 0 {
			t.Errorf("expected empty string, got %q", s)
		}
	})

	t.Run("positive length", func(t *testing.T) {
		for _, length := range []int{1, 8, 32, 128} {
			s := RandomString(length)
			if len(s) != length {
				t.Errorf("expected length %d, got %d: %q", length, len(s), s)
			}
		}
	})

	t.Run("randomness", func(t *testing.T) {
		s1 := RandomString(32)
		s2 := RandomString(32)
		if s1 == s2 {
			t.Error("two RandomString calls produced same result (unlikely)")
		}
	})
}

func TestRandomHex(t *testing.T) {
	t.Run("zero length", func(t *testing.T) {
		b := RandomHex(0)
		if len(b) != 0 {
			t.Errorf("expected empty slice, got %d bytes", len(b))
		}
	})

	t.Run("positive length", func(t *testing.T) {
		for _, length := range []int{1, 8, 16, 64} {
			b := RandomHex(length)
			if len(b) != length {
				t.Errorf("expected length %d, got %d", length, len(b))
			}
		}
	})
}

func TestMD5String(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty string", "", "d41d8cd98f00b204e9800998ecf8427e"},
		{"hello", "hello", "5d41402abc4b2a76b9719d911017c592"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MD5String(tt.input)
			if got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, got)
			}
		})
	}

	t.Run("deterministic", func(t *testing.T) {
		input := "test-input"
		h1 := MD5String(input)
		h2 := MD5String(input)
		if h1 != h2 {
			t.Error("same input produced different hashes")
		}
	})
}

func TestCryptoPassword(t *testing.T) {
	t.Run("deterministic", func(t *testing.T) {
		h1 := CryptoPassword("mypassword", "salt123")
		h2 := CryptoPassword("mypassword", "salt123")
		if h1 != h2 {
			t.Error("same inputs produced different outputs")
		}
	})

	t.Run("different password", func(t *testing.T) {
		h1 := CryptoPassword("password1", "salt")
		h2 := CryptoPassword("password2", "salt")
		if h1 == h2 {
			t.Error("different passwords produced same hash")
		}
	})

	t.Run("different salt", func(t *testing.T) {
		h1 := CryptoPassword("password", "salt1")
		h2 := CryptoPassword("password", "salt2")
		if h1 == h2 {
			t.Error("different salts produced same hash")
		}
	})

	t.Run("output format", func(t *testing.T) {
		h := CryptoPassword("password", "salt")
		if len(h) != 64 {
			t.Errorf("expected 64-char hex string, got %d chars", len(h))
		}
	})
}

func TestEnsureStatus(t *testing.T) {
	tests := []struct {
		name   string
		status int64
		bit    int64
		want   bool
	}{
		{"bit 0 set", 1, 0, true},
		{"bit 0 clear", 2, 0, false},
		{"bit 1 set", 2, 1, true},
		{"bit 2 set", 4, 2, true},
		{"multiple bits", 5, 0, true},
		{"multiple bits check 2", 5, 2, true},
		{"bit 63 set", int64(math.MinInt64), 63, true},
		{"bit -1 invalid", 0, -1, false},
		{"bit 64 invalid", 0, 64, false},
		{"all bits set", -1, 35, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EnsureStatus(tt.status, tt.bit)
			if got != tt.want {
				t.Errorf("EnsureStatus(%d, %d) = %v, want %v", tt.status, tt.bit, got, tt.want)
			}
		})
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
