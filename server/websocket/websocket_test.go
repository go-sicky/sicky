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
 * @file websocket_test.go
 * @package websocket
 * @author Dr.NP <np@herewe.tech>
 * @since 09/06/2026
 */

package websocket

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"
)

func TestConfigEnsureDefaults(t *testing.T) {
	cfg := (&Config{}).Ensure()
	if cfg.Network != DefaultNetwork {
		t.Errorf("expected network %q, got %q", DefaultNetwork, cfg.Network)
	}

	if cfg.PingDuration != DefaultPingDuration {
		t.Errorf("expected ping duration %d, got %d", DefaultPingDuration, cfg.PingDuration)
	}

	if cfg.MaxIdleDuration != DefaultMaxIdleDuration {
		t.Errorf("expected max idle duration %d, got %d", DefaultMaxIdleDuration, cfg.MaxIdleDuration)
	}

	if cfg.ShutdownTimeout != DefaultShutdownTimeout {
		t.Errorf("expected shutdown timeout %d, got %d", DefaultShutdownTimeout, cfg.ShutdownTimeout)
	}

	// MaxMessageBytes must stay 0 (unlimited) and Origins stay nil (same-origin).
	if cfg.MaxMessageBytes != 0 {
		t.Errorf("expected max message bytes 0, got %d", cfg.MaxMessageBytes)
	}

	if cfg.Origins != nil {
		t.Errorf("expected nil origins, got %v", cfg.Origins)
	}

	if (*Config)(nil).Ensure() == nil {
		t.Error("nil Ensure must return a default config")
	}

	if cfg.TrustProxy == nil || !*cfg.TrustProxy {
		t.Error("TrustProxy must default to true")
	}
}

func TestStartHalfTLSFailFast(t *testing.T) {
	srv := New(nil, &Config{
		Address:   "127.0.0.1:0",
		TLSKeyPEM: "-----BEGIN PRIVATE KEY-----\n-----END PRIVATE KEY-----",
	})
	if srv == nil {
		t.Fatal("New returned nil")
	}

	err := srv.Start()
	if !errors.Is(err, ErrIncompleteTLSConfig) {
		t.Fatalf("expected ErrIncompleteTLSConfig, got %v", err)
	}

	// And the mirror case: cert only.
	srv2 := New(nil, &Config{
		Address:    "127.0.0.1:0",
		TLSCertPEM: "-----BEGIN CERTIFICATE-----\n-----END CERTIFICATE-----",
	})
	if srv2 == nil {
		t.Fatal("New returned nil")
	}

	err = srv2.Start()
	if !errors.Is(err, ErrIncompleteTLSConfig) {
		t.Fatalf("expected ErrIncompleteTLSConfig, got %v", err)
	}
}

func TestCheckOrigin(t *testing.T) {
	newCtx := func(origin, host string) fiber.Ctx {
		var fctx fasthttp.RequestCtx
		if origin != "" {
			fctx.Request.Header.Set("Origin", origin)
		}

		if host != "" {
			fctx.Request.Header.Set("Host", host)
		}

		return fiber.New().AcquireCtx(&fctx)
	}

	tests := []struct {
		name   string
		cfg    *Config
		origin string
		host   string
		want   bool
	}{
		{"no origin header (non-browser)", (&Config{}).Ensure(), "", "example.com", true},
		{"same origin http", (&Config{}).Ensure(), "http://example.com", "example.com", true},
		{"same origin https", (&Config{}).Ensure(), "https://example.com", "example.com", true},
		{"cross origin default", (&Config{}).Ensure(), "https://evil.com", "example.com", false},
		{"wildcard whitelist", &Config{Origins: []string{"*"}}, "https://evil.com", "example.com", true},
		{"explicit whitelist match", &Config{Origins: []string{"https://allowed.com"}}, "https://allowed.com", "example.com", true},
		{"explicit whitelist miss", &Config{Origins: []string{"https://allowed.com"}}, "https://evil.com", "example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := &WebsocketServer{config: tt.cfg}
			c := newCtx(tt.origin, tt.host)
			if got := srv.checkOrigin(c); got != tt.want {
				t.Errorf("checkOrigin() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestOriginPolicyIntegration verifies the origin gate over a real handshake.
func TestOriginPolicyIntegration(t *testing.T) {
	srv := New(nil, &Config{Address: "127.0.0.1:0"})
	if srv == nil {
		t.Fatal("New returned nil")
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	defer func() {
		if err := srv.Stop(); err != nil {
			t.Errorf("Stop failed: %v", err)
		}
	}()

	addr := srv.Addr().String()
	dial := func(origin string) (int, string) {
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err != nil {
			t.Fatalf("dial failed: %v", err)
		}

		defer conn.Close()
		_, _ = fmt.Fprintf(conn,
			"GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\nSec-WebSocket-Version: 13\r\nOrigin: %s\r\n\r\n",
			DefaultPath, addr, origin)
		resp, err := http.ReadResponse(bufio.NewReader(conn), nil) //nolint:bodyclose // test closes the conn below
		if err != nil {
			return 0, ""
		}

		return resp.StatusCode, resp.Status
	}

	if code, _ := dial("https://evil.com"); code != http.StatusForbidden {
		t.Errorf("cross-origin upgrade: expected 403, got %d", code)
	}

	if code, _ := dial("http://" + addr); code != http.StatusSwitchingProtocols {
		t.Errorf("same-origin upgrade: expected 101, got %d", code)
	}

	if code, _ := dial(""); code != http.StatusSwitchingProtocols {
		t.Errorf("no-origin upgrade: expected 101, got %d", code)
	}
}

// TestSafelyInvoke verifies panic isolation around business handlers.
func TestSafelyInvoke(t *testing.T) {
	srv := New(nil, &Config{Address: "127.0.0.1:0"})
	if srv == nil {
		t.Fatal("New returned nil")
	}

	sess := &Session{}

	if err := srv.safelyInvoke("data", sess, func() error {
		panic("boom")
	}); err == nil {
		t.Error("expected error from panicking handler")
	}

	if err := srv.safelyInvoke("data", sess, func() error {
		return errors.New("plain failure")
	}); err == nil {
		t.Error("expected error from failing handler")
	}

	if err := srv.safelyInvoke("data", sess, func() error {
		return nil
	}); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

// TestSessionSendMutualExclusion exercises the serialized writer without a
// live connection (write errors are expected, panics are not).
func TestSessionSendMutualExclusion(t *testing.T) {
	sess := &Session{}
	done := make(chan struct{})
	for range 8 {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Send panicked: %v", r)
				}

				done <- struct{}{}
			}()

			_ = sess.Send(0, []byte(strings.Repeat("x", 1024)))
		}()
	}

	for range 8 {
		<-done
	}
}
