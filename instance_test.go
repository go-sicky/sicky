package sicky

import (
	"net"
	"testing"

	"github.com/go-sicky/sicky/server"
	httpServer "github.com/go-sicky/sicky/server/http"
	"github.com/go-sicky/sicky/utils"
)

// TestRegistryAdvertiseAddrHonorsConfiguredAddress: registration used to
// publish the bind address, silently ignoring advertise_address - a
// service behind NAT or a load balancer registered an address no peer
// could reach.
func TestRegistryAdvertiseAddrHonorsConfiguredAddress(t *testing.T) {
	srv := httpServer.New(
		&server.Options{Name: "http-test"},
		&httpServer.Config{
			Network:          "tcp",
			Address:          "127.0.0.1:0",
			AdvertiseAddress: "203.0.113.7:9999",
		},
	)

	addr := registryAdvertiseAddr(srv)
	if addr == nil {
		t.Fatal("advertise address not resolved")
	}
	if addr.String() != "203.0.113.7:9999" {
		t.Fatalf("advertise address = %q, want 203.0.113.7:9999", addr.String())
	}
}

// TestRegistryAdvertiseAddrResolvesWildcardBind: `:3000` with no
// advertise_address binds 0.0.0.0/[::]. Publishing that would make every
// remote client dial 0.0.0.0 and reach itself.
func TestRegistryAdvertiseAddrResolvesWildcardBind(t *testing.T) {
	srv := httpServer.New(
		&server.Options{Name: "http-test"},
		&httpServer.Config{Network: "tcp", Address: ":0"},
	)
	if err := srv.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	defer func() {
		if err := srv.Stop(); err != nil {
			t.Errorf("stop: %v", err)
		}
	}()

	addr := registryAdvertiseAddr(srv)
	if addr == nil {
		t.Fatal("advertise address not resolved for wildcard bind")
	}

	ip := utils.AddrToIP(addr)
	if ip == nil || ip.IsUnspecified() {
		t.Fatalf("published a wildcard address: %q", addr.String())
	}

	if port := utils.AddrToPort(addr); port != srv.Port() {
		t.Fatalf("advertise port = %d, want %d", port, srv.Port())
	}
}

// TestRegistryAdvertiseAddrKeepsLoopback: a loopback bind is routable on
// the host and must be published as-is, not replaced.
func TestRegistryAdvertiseAddrKeepsLoopback(t *testing.T) {
	want := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}
	srv := &staticServer{addr: want, advertiseAddr: want}

	if got := registryAdvertiseAddr(srv); got == nil || got.String() != want.String() {
		t.Fatalf("advertise address = %v, want %s", got, want.String())
	}
}

// TestRegistryAdvertiseAddrSkipsUnstarted: a wildcard bind that has not
// been started yet still has port 0 - publishing it would be undialable.
func TestRegistryAdvertiseAddrSkipsUnstarted(t *testing.T) {
	wildcard := &net.TCPAddr{Port: 0}
	srv := &staticServer{addr: wildcard, advertiseAddr: wildcard}

	if got := registryAdvertiseAddr(srv); got != nil {
		t.Fatalf("advertise address = %v, want nil for a zero port", got)
	}
}

// TestRegistryAdvertiseAddrNilWhenUnresolvable: nothing usable is better
// than an address peers cannot dial.
func TestRegistryAdvertiseAddrNilWhenUnresolvable(t *testing.T) {
	srv := &staticServer{}
	if got := registryAdvertiseAddr(srv); got != nil {
		t.Fatalf("advertise address = %v, want nil", got)
	}
}

// staticServer is a minimal server.Server for address handling tests.
type staticServer struct {
	server.Server
	addr          net.Addr
	advertiseAddr net.Addr
	advertiseIP   net.IP
}

func (s *staticServer) Addr() net.Addr { return s.addr }

func (s *staticServer) AdvertiseAddr() net.Addr { return s.advertiseAddr }

func (s *staticServer) AdvertiseIP() net.IP {
	if s.advertiseIP != nil {
		return s.advertiseIP
	}

	return utils.AddrToIP(s.advertiseAddr)
}
