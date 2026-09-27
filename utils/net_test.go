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
 * @file net_test.go
 * @package utils
 * @author Dr.NP <np@herewe.tech>
 * @since 07/07/2026
 */

package utils

import (
	"errors"
	"net"
	"testing"
)

func TestAddrToIP(t *testing.T) {
	t.Run("TCPAddr", func(t *testing.T) {
		addr := &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 8080}
		ip := AddrToIP(addr)
		if ip == nil {
			t.Fatal("expected non-nil IP")
		}

		if !ip.Equal(net.ParseIP("192.168.1.1")) {
			t.Errorf("expected 192.168.1.1, got %s", ip)
		}
	})

	t.Run("TCPAddr IPv6", func(t *testing.T) {
		addr := &net.TCPAddr{IP: net.ParseIP("::1"), Port: 8080}
		ip := AddrToIP(addr)
		if ip == nil {
			t.Fatal("expected non-nil IP for IPv6")
		}

		if !ip.Equal(net.ParseIP("::1")) {
			t.Errorf("expected ::1, got %s", ip)
		}
	})

	t.Run("UDPAddr", func(t *testing.T) {
		addr := &net.UDPAddr{IP: net.ParseIP("10.0.0.1"), Port: 9090}
		ip := AddrToIP(addr)
		if ip == nil {
			t.Fatal("expected non-nil IP for UDP")
		}

		if !ip.Equal(net.ParseIP("10.0.0.1")) {
			t.Errorf("expected 10.0.0.1, got %s", ip)
		}
	})

	t.Run("IPAddr", func(t *testing.T) {
		addr := &net.IPAddr{IP: net.ParseIP("172.16.0.1")}
		ip := AddrToIP(addr)
		if ip == nil {
			t.Fatal("expected non-nil IP for IPAddr")
		}

		if !ip.Equal(net.ParseIP("172.16.0.1")) {
			t.Errorf("expected 172.16.0.1, got %s", ip)
		}
	})

	t.Run("unsupported network", func(t *testing.T) {
		addr := &net.UnixAddr{Name: "/tmp/socket", Net: "unix"}
		ip := AddrToIP(addr)
		if ip != nil {
			t.Errorf("expected nil IP for unsupported network type, got %s", ip)
		}
	})
}

func TestAddrToPort(t *testing.T) {
	t.Run("TCPAddr", func(t *testing.T) {
		addr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 6379}
		port := AddrToPort(addr)
		if port != 6379 {
			t.Errorf("expected port 6379, got %d", port)
		}
	})

	t.Run("UDPAddr", func(t *testing.T) {
		addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 5353}
		port := AddrToPort(addr)
		if port != 5353 {
			t.Errorf("expected port 5353, got %d", port)
		}
	})

	t.Run("unsupported network", func(t *testing.T) {
		addr := &net.UnixAddr{Name: "/tmp/socket", Net: "unix"}
		port := AddrToPort(addr)
		if port != 0 {
			t.Errorf("expected port 0 for unsupported network type, got %d", port)
		}
	})
}

func TestErrNilConnection(t *testing.T) {
	if ErrNilConnection.Error() != "utils: nil connection" {
		t.Errorf("expected 'utils: nil connection', got %q", ErrNilConnection.Error())
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

func TestNormalizeHost(t *testing.T) {
	for in, want := range map[string]string{
		"localhost:8888": "localhost",
		"LocalHost":      "localhost",
		"127.0.0.1:8888": "127.0.0.1",
		"[::1]:8888":     "::1",
		"::1":            "::1",
		"api.internal":   "api.internal",
		":8888":          "", // wildcard bind has no host
		"":               "",
	} {
		if got := NormalizeHost(in); got != want {
			t.Errorf("NormalizeHost(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsLoopbackHost(t *testing.T) {
	for host, want := range map[string]bool{
		"localhost":      true,
		"localhost:8888": true,
		"127.0.0.1":      true,
		"127.5.5.5:80":   true,
		"[::1]:8888":     true,
		"evil.example":   false,
		"10.0.0.5":       false,
		"":               false,
	} {
		if got := IsLoopbackHost(host); got != want {
			t.Errorf("IsLoopbackHost(%q) = %v, want %v", host, got, want)
		}
	}
}

// TestNet2fdRejectsUnsupportedConn: the reflection chain used to
// dereference whatever FieldByName returned, so a conn type without the
// expected internals (a mock, a wrapper) panicked instead of failing.
func TestNet2fdRejectsUnsupportedConn(t *testing.T) {
	if _, err := Net2fd(nil); !errors.Is(err, ErrNilConnection) {
		t.Fatalf("nil conn error = %v, want ErrNilConnection", err)
	}

	c1, c2 := net.Pipe()
	defer func() {
		_ = c1.Close()
		_ = c2.Close()
	}()

	fd, err := Net2fd(c1)
	if err == nil {
		t.Fatalf("in-memory pipe returned fd %d, want an unsupported-type error", fd)
	}

	if fd != -1 {
		t.Fatalf("fd = %d, want -1 on error", fd)
	}
}
