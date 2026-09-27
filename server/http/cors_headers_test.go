package http

import (
	"net/http"
	"strings"
	"testing"
)

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
