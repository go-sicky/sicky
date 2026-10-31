package fiber

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/go-sicky/sicky/server"
)

func newCORSTestServer(t *testing.T, cfg *CORSConfig) *FiberServer {
	t.Helper()

	srv := New(&server.Options{Name: "fiber-cors-preflight"}, &Config{
		Network: "tcp",
		Address: "127.0.0.1:0",
		CORS:    cfg,
	})
	if srv == nil {
		t.Fatal("New returned nil")
	}

	return srv
}

func doCORSPreflight(t *testing.T, srv *FiberServer, origin string, headers map[string]string) http.Header {
	t.Helper()

	req := httptest.NewRequest(http.MethodOptions, "http://x.test/", http.NoBody)
	req.Header.Set("Origin", origin)

	for name, value := range headers {
		req.Header.Set(name, value)
	}

	resp, err := srv.App().Test(req)
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", resp.StatusCode)
	}

	return resp.Header
}

// TestCORSFiberEchoesRequestedHeaders: fiber already echoed the browser's
// Access-Control-Request-Headers; the config must not replace that with a
// fixed list, and the methods default must cover PATCH.
func TestCORSFiberEchoesRequestedHeaders(t *testing.T) {
	srv := newCORSTestServer(t, &CORSConfig{AllowedOrigins: []string{"https://app.test"}})

	header := doCORSPreflight(t, srv, "https://app.test", map[string]string{
		"Access-Control-Request-Method":  "PATCH",
		"Access-Control-Request-Headers": "X-Api-Key, X-Tenant",
	})

	if got := header.Get("Access-Control-Allow-Headers"); got != "X-Api-Key, X-Tenant" {
		t.Fatalf("allow-headers = %q, want the requested headers echoed", got)
	}

	if got := header.Get("Access-Control-Allow-Methods"); !strings.Contains(got, "PATCH") {
		t.Fatalf("allow-methods missing PATCH: %q", got)
	}
}

// TestCORSFiberExplicitHeaders: a configured list wins over the request.
func TestCORSFiberExplicitHeaders(t *testing.T) {
	srv := newCORSTestServer(t, &CORSConfig{
		AllowedOrigins: []string{"https://app.test"},
		AllowedHeaders: []string{"X-Api-Key", " X-Tenant "},
	})

	header := doCORSPreflight(t, srv, "https://app.test", map[string]string{
		"Access-Control-Request-Method":  "GET",
		"Access-Control-Request-Headers": "X-Something-Else",
	})

	if got := header.Get("Access-Control-Allow-Headers"); got != "X-Api-Key, X-Tenant" {
		t.Fatalf("allow-headers = %q, want the configured list", got)
	}
}

// TestCORSFiberHeaderFallback: fiber emits nothing when a preflight names
// no headers; the wrapper fills in the framework default so both stacks
// answer the same preflight alike.
func TestCORSFiberHeaderFallback(t *testing.T) {
	srv := newCORSTestServer(t, &CORSConfig{AllowedOrigins: []string{"https://app.test"}})

	header := doCORSPreflight(t, srv, "https://app.test", map[string]string{
		"Access-Control-Request-Method": "GET",
	})

	if got, want := header.Get("Access-Control-Allow-Headers"), strings.Join(defaultCORSAllowedHeaders, ", "); got != want {
		t.Fatalf("fallback allow-headers = %q, want %q", got, want)
	}
}

// TestCORSFiberExposeHeaders: browser scripts can only read a response
// header that appears here.
func TestCORSFiberExposeHeaders(t *testing.T) {
	srv := newCORSTestServer(t, &CORSConfig{
		AllowedOrigins: []string{"https://app.test"},
		ExposedHeaders: []string{"X-Request-ID", " X-Tenant "},
	})
	srv.App().Get("/", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "http://x.test/", http.NoBody)
	req.Header.Set("Origin", "https://app.test")

	resp, err := srv.App().Test(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if got := resp.Header.Get("Access-Control-Expose-Headers"); got != "X-Request-ID, X-Tenant" {
		t.Fatalf("expose-headers = %q", got)
	}
}

// TestCORSFiberDefaultMethods: pinned to the same literal as server/http
// so the two stacks cannot drift apart.
func TestCORSFiberDefaultMethods(t *testing.T) {
	if got, want := strings.Join(corsMethods(&CORSConfig{}), ", "), "GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS, QUERY"; got != want {
		t.Fatalf("default methods = %q, want %q (keep server/http identical)", got, want)
	}
}

// TestCORSFiberValidateLists: same rules as server/http, since the values
// end up in response headers either way.
func TestCORSFiberValidateLists(t *testing.T) {
	cases := []struct {
		name    string
		cfg     *CORSConfig
		wantErr bool
	}{
		{"wildcard header with credentials", &CORSConfig{AllowedHeaders: []string{"*"}, AllowCredentials: true}, true},
		{"wildcard exposed with credentials", &CORSConfig{ExposedHeaders: []string{"*"}, AllowCredentials: true}, true},
		{"wildcard method with credentials", &CORSConfig{AllowedMethods: []string{"*"}, AllowCredentials: true}, true},
		{"explicit lists with credentials", &CORSConfig{
			AllowedOrigins:   []string{"https://app.test"},
			AllowedMethods:   []string{"GET"},
			AllowedHeaders:   []string{"X-Api-Key"},
			ExposedHeaders:   []string{"X-Request-ID"},
			AllowCredentials: true,
		}, false},
		{"empty entry", &CORSConfig{AllowedHeaders: []string{"X-Ok", " "}}, true},
		{"crlf injection", &CORSConfig{AllowedHeaders: []string{"X-Bad\r\nX-Injected: 1"}}, true},
		{"token with space", &CORSConfig{AllowedMethods: []string{"GET POST"}}, true},
		{"lowercase method", &CORSConfig{AllowedMethods: []string{"patch"}}, false},
		{"wildcard mixed with explicit", &CORSConfig{AllowedHeaders: []string{"*", "X-Ok"}}, true},
		{"wildcard alone without credentials", &CORSConfig{AllowedHeaders: []string{"*"}}, false},
		{"blank exposed", &CORSConfig{ExposedHeaders: []string{""}}, true},
	}

	for _, tc := range cases {
		if err := tc.cfg.Validate(); (err != nil) != tc.wantErr {
			t.Errorf("%s: Validate() = %v, wantErr %v", tc.name, err, tc.wantErr)
		}
	}
}

// TestCORSFiberEnsureLeavesListsUnset: "unset" must stay distinguishable
// from "explicitly empty"; the list is resolved when the response is built.
func TestCORSFiberEnsureLeavesListsUnset(t *testing.T) {
	cfg := (&CORSConfig{}).Ensure()

	if len(cfg.AllowedMethods) != 0 || len(cfg.AllowedHeaders) != 0 || len(cfg.ExposedHeaders) != 0 {
		t.Fatalf("Ensure filled lists: methods=%v headers=%v exposed=%v",
			cfg.AllowedMethods, cfg.AllowedHeaders, cfg.ExposedHeaders)
	}
}
