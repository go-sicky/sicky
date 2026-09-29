package udp

import (
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/go-sicky/sicky/server"
)

func TestSessionSetKey(t *testing.T) {
	p := NewPool(60)
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 41001}
	s := NewSession(nil, addr)
	p.Put(s)

	if got := p.GetByAddr(addr); got != s {
		t.Fatal("GetByAddr missed: pointer-keyed index would fail here")
	}

	// A fresh *UDPAddr with the same address must hit the same session
	// (ReadFromUDP allocates a new pointer per datagram).
	dup := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 41001}
	if got := p.GetByAddr(dup); got != s {
		t.Fatal("GetByAddr missed for equal address with different pointer")
	}

	s.SetKey("alpha")
	if got := p.GetByKey("alpha"); got != s {
		t.Fatal("GetByKey(alpha) missed after SetKey")
	}

	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if got := p.GetByAddr(addr); got != nil {
		t.Fatal("addr still indexed after Close")
	}

	if err := s.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

func TestAllowPacketRateLimit(t *testing.T) {
	srv := New(
		&server.Options{Name: "udp-ratelimit-test"},
		&Config{Network: "udp", Address: "127.0.0.1:0", BufferSize: 1024, MaxPacketsPerSecond: 2},
	)
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 41002}

	// Two calls: the first two packets in the window must pass.
	first, second := srv.allowPacket(addr), srv.allowPacket(addr)
	if !first || !second {
		t.Fatal("first two packets must pass")
	}

	if srv.allowPacket(addr) {
		t.Fatal("third packet in window must drop")
	}

	// Other sources have independent budgets.
	other := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 41003}
	if !srv.allowPacket(other) {
		t.Fatal("independent source must pass")
	}

	// Disabled limiter passes everything.
	srv.config.MaxPacketsPerSecond = 0
	for range 10 {
		if !srv.allowPacket(addr) {
			t.Fatal("disabled limiter must pass")
		}
	}
}

func TestAllowPacketSourceTableIsBounded(t *testing.T) {
	srv := New(
		&server.Options{Name: "udp-ratelimit-cap-test"},
		&Config{Network: "udp", Address: "127.0.0.1:0", BufferSize: 1024, MaxPacketsPerSecond: 2},
	)

	// Pre-fill the table to its cap directly rather than looping
	// MaxRateLimitSources times: the test is about the guard's decision
	// at the boundary, not about how fast the table fills.
	srv.rateMu.Lock()
	srv.rateWindow = time.Now()
	tracked := "10.0.0.1:1000"
	srv.rateCounts[tracked] = 1
	for i := range MaxRateLimitSources - 1 {
		srv.rateCounts[strconv.Itoa(i)] = 1
	}
	srv.rateMu.Unlock()

	if got := len(srv.rateCounts); got != MaxRateLimitSources {
		t.Fatalf("pre-filled table holds %d entries, want %d", got, MaxRateLimitSources)
	}

	// An unseen source must be dropped rather than inserted: the keys are
	// client-supplied and UDP lets a client spoof them, so an unbounded
	// table turns the flood guard into the memory-exhaustion vector.
	spoofed := &net.UDPAddr{IP: net.ParseIP("203.0.113.7"), Port: 9999}
	if srv.allowPacket(spoofed) {
		t.Fatal("an unseen source must be dropped once the table is full")
	}

	if got := len(srv.rateCounts); got != MaxRateLimitSources {
		t.Fatalf("table grew past the cap: %d entries, want %d", got, MaxRateLimitSources)
	}

	if _, known := srv.rateCounts[addrKey(spoofed)]; known {
		t.Fatal("a dropped spoofed source must not be recorded in the table")
	}

	// A source already in the table keeps its own budget: the cap must
	// not turn every packet from a known source into a drop, which would
	// let an attacker lock out every legitimate client once full.
	known := &net.UDPAddr{IP: net.ParseIP("10.0.0.1"), Port: 1000}
	if !srv.allowPacket(known) {
		t.Fatal("a known source must still be allowed while the table is full")
	}

	if got := srv.rateCounts[addrKey(known)]; got != 2 {
		t.Fatalf("known source count = %d, want 2 (its budget is still enforced)", got)
	}
}
