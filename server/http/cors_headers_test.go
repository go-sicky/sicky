package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/uptrace/bunrouter"
)

// callCORSWithHeaders drives the middleware with request headers, which a
// preflight advertises through Access-Control-Request-* fields.
func callCORSWithHeaders(t *testing.T, cfg *CORSConfig, method, origin string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, "http://x.test/", http.NoBody)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}

	for name, value := range headers {
		req.Header.Set(name, value)
	}

	h := NewCORSMiddleware(cfg)(func(w http.ResponseWriter, r bunrouter.Request) error {
		w.WriteHeader(http.StatusTeapot)

		return nil
	})
	if err := h(w, bunrouter.NewRequest(req)); err != nil {
		t.Fatalf("middleware error: %v", err)
	}

	return w
}

// TestCORSAllowedHeadersExplicit: a configured list is authoritative and
// must not be widened by whatever the client asks for.
func TestCORSAllowedHeadersExplicit(t *testing.T) {
	cfg := &CORSConfig{
		AllowedOrigins: []string{"https://app.test"},
		AllowedHeaders: []string{"X-Api-Key", " X-Tenant "},
	}

	w := callCORSWithHeaders(t, cfg, http.MethodOptions, "https://app.test", map[string]string{
		"Access-Control-Request-Method":  "GET",
		"Access-Control-Request-Headers": "X-Something-Else",
	})

	if got := w.Header().Get("Access-Control-Allow-Headers"); got != "X-Api-Key, X-Tenant" {
		t.Fatalf("allow-headers = %q, want the configured list", got)
	}
}

// TestCORSAllowedHeadersEchoRequested: with no configured list the
// preflight must reflect what the browser says the request needs. The old
// hardcoded list broke every client using a header the framework had not
// foreseen, which is the whole point of this feature.
func TestCORSAllowedHeadersEchoRequested(t *testing.T) {
	cfg := &CORSConfig{AllowedOrigins: []string{"https://app.test"}}

	w := callCORSWithHeaders(t, cfg, http.MethodOptions, "https://app.test", map[string]string{
		"Access-Control-Request-Method":  "PATCH",
		"Access-Control-Request-Headers": "X-Api-Key, X-Tenant",
	})

	if got := w.Header().Get("Access-Control-Allow-Headers"); got != "X-Api-Key, X-Tenant" {
		t.Fatalf("allow-headers = %q, want the requested headers echoed", got)
	}

	if !hasVary(w.Header().Values("Vary"), "Access-Control-Request-Headers") {
		t.Fatalf("echoed preflight must vary on Access-Control-Request-Headers: %v", w.Header().Values("Vary"))
	}

	if !hasVary(w.Header().Values("Vary"), "Access-Control-Request-Method") {
		t.Fatalf("preflight must vary on Access-Control-Request-Method: %v", w.Header().Values("Vary"))
	}
}

// TestCORSAllowedHeadersFallback: a preflight that names no headers keeps
// advertising the framework's own (tracing, request id) instead of an
// empty list, so tooling that inspects the response sees a usable set.
func TestCORSAllowedHeadersFallback(t *testing.T) {
	cfg := &CORSConfig{AllowedOrigins: []string{"https://app.test"}}

	w := callCORSWithHeaders(t, cfg, http.MethodOptions, "https://app.test", map[string]string{
		"Access-Control-Request-Method": "GET",
	})

	got := w.Header().Get("Access-Control-Allow-Headers")
	for _, want := range []string{"Authorization", "X-Request-ID", "X-B3-Traceid"} {
		if !strings.Contains(got, want) {
			t.Fatalf("fallback allow-headers missing %q: %q", want, got)
		}
	}
}

// TestCORSAllowedMethods: the default must cover the methods a browser
// actually preflights (PATCH and HEAD were missing from the old hardcoded
// list), and an explicit list is normalized to upper case.
func TestCORSAllowedMethods(t *testing.T) {
	if got, want := corsAllowMethods(&CORSConfig{}), "GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS, QUERY"; got != want {
		t.Fatalf("default methods = %q, want %q (keep server/fiber identical)", got, want)
	}

	cfg := &CORSConfig{
		AllowedOrigins: []string{"https://app.test"},
		AllowedMethods: []string{"patch", " get "},
	}

	w := callCORSWithHeaders(t, cfg, http.MethodOptions, "https://app.test", map[string]string{
		"Access-Control-Request-Method": "PATCH",
	})

	if got := w.Header().Get("Access-Control-Allow-Methods"); got != "PATCH, GET" {
		t.Fatalf("allow-methods = %q, want PATCH, GET", got)
	}
}

// TestCORSExposeHeaders: exposed headers belong to the actual response;
// a preflight carries no use for them.
func TestCORSExposeHeaders(t *testing.T) {
	cfg := &CORSConfig{
		AllowedOrigins: []string{"https://app.test"},
		ExposedHeaders: []string{"X-Request-ID", " X-Tenant "},
	}

	trusted := map[string]string{"Access-Control-Request-Method": "GET"}

	w := callCORSWithHeaders(t, cfg, http.MethodGet, "https://app.test", trusted)
	if got := w.Header().Get("Access-Control-Expose-Headers"); got != "X-Request-ID, X-Tenant" {
		t.Fatalf("expose-headers = %q", got)
	}

	w = callCORSWithHeaders(t, cfg, http.MethodOptions, "https://app.test", trusted)
	if got := w.Header().Get("Access-Control-Expose-Headers"); got != "" {
		t.Fatalf("preflight must not carry expose-headers, got %q", got)
	}
}

// TestCORSValidateLists: every list is written into a response header, so
// a value that cannot be one is a configuration error rather than a header
// that fails in the browser. "*" follows the same credentials rule as
// origins because browsers read it literally on credentialed requests.
func TestCORSValidateLists(t *testing.T) {
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

// TestCORSEnsureLeavesListsUnset: an unset list means "resolve at response
// time"; filling it in Ensure would erase that distinction.
func TestCORSEnsureLeavesListsUnset(t *testing.T) {
	cfg := (&CORSConfig{}).Ensure()

	if len(cfg.AllowedMethods) != 0 || len(cfg.AllowedHeaders) != 0 || len(cfg.ExposedHeaders) != 0 {
		t.Fatalf("Ensure filled lists: methods=%v headers=%v exposed=%v",
			cfg.AllowedMethods, cfg.AllowedHeaders, cfg.ExposedHeaders)
	}
}

func hasVary(values []string, want string) bool {
	for _, v := range values {
		for part := range strings.SplitSeq(v, ",") {
			if strings.EqualFold(strings.TrimSpace(part), want) {
				return true
			}
		}
	}

	return false
}

// TestCORSWildcardPreflightCarriesMethodHeaders: the wildcard branch used
// to answer a preflight with the origin grant only, so every preflighted
// request (anything but a simple one) failed while plain GETs worked.
func TestCORSWildcardPreflightCarriesMethodHeaders(t *testing.T) {
	cfg := &CORSConfig{AllowedOrigins: []string{"*"}}

	w := callCORS(t, cfg, http.MethodOptions, "https://anything.test")
	if w.Code != http.StatusNoContent {
		t.Fatalf("preflight code = %d, want 204", w.Code)
	}

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("ACAO = %q, want *", got)
	}

	if got := w.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Fatal("wildcard preflight carries no Allow-Methods: browsers reject the actual request")
	}

	if got := w.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Fatal("wildcard preflight carries no Allow-Headers")
	}

	if got := w.Header().Get("Access-Control-Max-Age"); got == "" {
		t.Fatal("wildcard preflight carries no Max-Age")
	}
}

// TestCORSDenyVaryOnEveryResponse: a shared cache must not serve one
// origin's response to another, which is what Vary: Origin on the deny
// path prevents - it used to be written only for preflights.
func TestCORSDenyVaryOnEveryResponse(t *testing.T) {
	cfg := &CORSConfig{AllowedOrigins: []string{"https://app.test"}}

	for _, method := range []string{http.MethodGet, http.MethodOptions} {
		w := callCORS(t, cfg, method, "https://evil.test")

		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Fatalf("%s: untrusted origin echoed: %q", method, got)
		}

		if !hasVary(w.Header().Values("Vary"), "Origin") {
			t.Fatalf("%s: deny path does not vary on Origin: %v", method, w.Header().Values("Vary"))
		}
	}
}

// TestCORSVaryAppends: Vary is built with Add so another layer's entry
// (Accept-Encoding, for instance) survives.
func TestCORSVaryAppends(t *testing.T) {
	cfg := &CORSConfig{AllowedOrigins: []string{"https://app.test"}}

	w := callCORS(t, cfg, http.MethodGet, "https://app.test")
	if !hasVary(w.Header().Values("Vary"), "Origin") {
		t.Fatalf("trusted path does not vary on Origin: %v", w.Header().Values("Vary"))
	}
}
