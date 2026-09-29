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
 * @file guards_test.go
 * @package websocket
 * @author Dr.NP <np@herewe.tech>
 * @since 09/29/2026
 */

package websocket

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"
)

// A2 regression: an absent (or negative) max_message_bytes must never
// survive Ensure() as 0, because gorilla reads a zero read limit as
// "unbounded" and buffers the whole frame in RAM.
func TestEnsureAlwaysAppliesMessageReadLimit(t *testing.T) {
	cases := []struct {
		name string
		in   int
		want int
	}{
		{"absent key reads as zero", 0, DefaultMaxMessageBytes},
		{"negative must not disable the guard", -1, DefaultMaxMessageBytes},
		{"explicit value is preserved", 1 << 10, 1 << 10},
		{"raised cap is preserved", 8 << 20, 8 << 20},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := (&Config{MaxMessageBytes: tc.in}).Ensure().MaxMessageBytes
			if got != tc.want {
				t.Fatalf("MaxMessageBytes = %d, want %d", got, tc.want)
			}

			if got <= 0 {
				t.Fatalf("MaxMessageBytes = %d: a zero read limit means unlimited in gorilla", got)
			}
		})
	}
}

// A6 regression: MaxSessions is opt-in, so Ensure() must preserve both an
// explicit cap and an explicit "off" rather than inventing a default.
func TestEnsurePreservesSessionCap(t *testing.T) {
	if got := (&Config{}).Ensure().MaxSessions; got != 0 {
		t.Fatalf("absent MaxSessions = %d, want 0 (opt-in, warned about in New)", got)
	}

	if got := (&Config{MaxSessions: 250}).Ensure().MaxSessions; got != 250 {
		t.Fatalf("MaxSessions = %d, want 250", got)
	}
}

// A3 regression: only loopback plus operator-named proxies may set
// X-Forwarded-*. The old default also trusted link-local and private
// ranges, which let any same-segment peer forge the client IP.
func TestNewWiresNarrowedTrustProxyConfig(t *testing.T) {
	proxies := []string{"10.0.0.7", "192.168.1.0/24"}

	srv := New(nil, &Config{Address: "127.0.0.1:0", TrustProxies: proxies})
	if srv == nil {
		t.Fatal("New returned nil")
	}

	cfg := srv.app.Config()

	if !cfg.TrustProxy {
		t.Error("trusted-proxy handling must stay enabled by default")
	}

	if !cfg.TrustProxyConfig.Loopback {
		t.Error("loopback must remain trusted")
	}

	// These two are the regression: fiber defaults them to true.
	if cfg.TrustProxyConfig.LinkLocal {
		t.Error("link-local range must not be trusted: a link-local peer can forge X-Forwarded-For")
	}

	if cfg.TrustProxyConfig.Private {
		t.Error("private range must not be trusted: any pod-network/VPC/LAN peer can forge X-Forwarded-For")
	}

	if len(cfg.TrustProxyConfig.Proxies) != len(proxies) {
		t.Fatalf("proxies = %v, want %v", cfg.TrustProxyConfig.Proxies, proxies)
	}

	for i, want := range proxies {
		if cfg.TrustProxyConfig.Proxies[i] != want {
			t.Fatalf("proxies[%d] = %q, want %q", i, cfg.TrustProxyConfig.Proxies[i], want)
		}
	}
}

// A6 regression: the cap must reject an upgrade BEFORE a session is
// allocated, so a flood of simultaneous connections cannot pin the
// process with one goroutine each.
func TestSessionCapRejectsUpgrades(t *testing.T) {
	srv := New(nil, &Config{Address: "127.0.0.1:0", MaxSessions: 1})
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

	// dial performs a raw upgrade handshake and keeps the socket open, so
	// the server's operator goroutine stays parked holding its session.
	dial := func() (net.Conn, int) {
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err != nil {
			t.Fatalf("dial failed: %v", err)
		}

		if _, err = fmt.Fprintf(conn,
			"GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\nSec-WebSocket-Version: 13\r\n\r\n",
			DefaultPath, addr); err != nil {
			_ = conn.Close()
			t.Fatalf("write handshake failed: %v", err)
		}

		resp, err := http.ReadResponse(bufio.NewReader(conn), nil) //nolint:bodyclose // the test owns the conn
		if err != nil {
			_ = conn.Close()
			return nil, 0
		}

		return conn, resp.StatusCode
	}

	first, code := dial()
	if code != http.StatusSwitchingProtocols {
		t.Fatalf("first upgrade: expected 101, got %d", code)
	}

	defer first.Close()

	// The first connection only reaches the pool once operator has run;
	// wait for it so the second dial is not racing the first.
	waitForPool(t, 1)

	// The handshake response itself cannot be the signal: gorilla writes
	// the 101 before operator runs, so a rejected client still sees 101
	// and is dropped a moment later. What must hold is that the cap
	// allocates no second session and hangs up on the peer.
	second, _ := dial()
	if second == nil {
		t.Fatal("second dial produced no connection to inspect")
	}

	defer second.Close()

	if n := SessionPool.Length(); n > 1 {
		t.Fatalf("session pool holds %d sessions under a cap of 1: "+
			"the cap must reject before allocating a session", n)
	}

	if err := second.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}

	buf := make([]byte, 1)

	if _, err := second.Read(buf); err == nil {
		t.Fatalf("rejected connection stayed open and delivered a byte: " +
			"the cap must hang up on the peer")
	}
}

// waitForPool polls until the session pool reaches want, or fails.
func waitForPool(t *testing.T, want int) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if SessionPool != nil && SessionPool.Length() >= want {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("session pool never reached %d (got %d)", want, SessionPool.Length())
}
