package fiber

import (
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/go-sicky/sicky/server"
)

func TestProxySettingsTrustsLoopbackOnly(t *testing.T) {
	trust, trusted := proxySettings((&Config{}).Ensure())

	if !trust {
		t.Fatal("trust proxy must stay enabled by default (a local reverse proxy is the common case)")
	}

	if !trusted.Loopback {
		t.Fatal("loopback must be trusted: that is the local reverse proxy")
	}

	if trusted.Private {
		t.Fatal("the whole private range must not be trusted: every Docker/K8s/LAN peer is private and could forge forwarded headers")
	}

	if trusted.LinkLocal {
		t.Fatal("link-local must not be trusted")
	}

	if len(trusted.Proxies) != 0 {
		t.Fatalf("proxies = %v, want none by default", trusted.Proxies)
	}

	off := false
	if trust, _ := proxySettings(&Config{TrustProxy: &off}); trust {
		t.Fatal("an explicit false must disable trust")
	}

	if _, trusted := proxySettings(&Config{TrustProxies: []string{"10.0.0.0/8"}}); len(trusted.Proxies) != 1 {
		t.Fatalf("explicit proxies = %v, want them propagated", trusted.Proxies)
	}
}

// forwardedScheme starts the server and reports what the handler saw as
// the request scheme while the client claims https via X-Forwarded-Proto.
func forwardedScheme(t *testing.T, addr string) string {
	t.Helper()

	srv := New(
		&server.Options{Name: "fiber-proxy-test"},
		&Config{Network: "tcp", Address: addr},
	)

	srv.App().Get("/scheme", func(c fiber.Ctx) error {
		return c.SendString(c.Scheme())
	})

	if err := srv.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	defer func() {
		if err := srv.Stop(); err != nil {
			t.Errorf("stop: %v", err)
		}
	}()

	req, err := http.NewRequest(http.MethodGet, "http://"+srv.Addr().String()+"/scheme", http.NoBody)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	req.Header.Set("X-Forwarded-Proto", "https")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	return string(body)
}

// TestLoopbackPeerMayForward: the local reverse proxy case must keep
// working - a loopback peer's forwarded scheme is honored.
func TestLoopbackPeerMayForward(t *testing.T) {
	if got := forwardedScheme(t, "127.0.0.1:0"); got != "https" {
		t.Fatalf("forwarded scheme from loopback = %q, want https", got)
	}
}

// TestPrivatePeerMayNotForward: on a private segment every peer is
// private, so trusting that range wholesale let any same-segment client
// claim the request arrived over TLS (or plain HTTP).
func TestPrivatePeerMayNotForward(t *testing.T) {
	addr := privateInterfaceAddr(t)
	if addr == "" {
		t.Skip("no private interface on this host")
	}

	got := forwardedScheme(t, net.JoinHostPort(addr, "0"))
	if got == "https" {
		t.Fatal("a private peer forged X-Forwarded-Proto: only loopback is trusted by default")
	}

	if got != "http" {
		t.Fatalf("scheme = %q, want http", got)
	}
}

// privateInterfaceAddr returns a non-loopback private IPv4 address.
func privateInterfaceAddr(t *testing.T) string {
	t.Helper()

	ifaces, err := net.Interfaces()
	if err != nil {
		t.Fatalf("interfaces: %v", err)
	}

	for _, iface := range ifaces {
		addrs, aerr := iface.Addrs()
		if aerr != nil {
			continue
		}

		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}

			ip4 := ipnet.IP.To4()
			if ip4 != nil && ip4.IsPrivate() && !ip4.IsLoopback() {
				return ip4.String()
			}
		}
	}

	return ""
}
