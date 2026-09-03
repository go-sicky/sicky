package udp

import (
	"net"
	"testing"

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

	if !srv.allowPacket(addr) || !srv.allowPacket(addr) {
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
	for i := 0; i < 10; i++ {
		if !srv.allowPacket(addr) {
			t.Fatal("disabled limiter must pass")
		}
	}
}
