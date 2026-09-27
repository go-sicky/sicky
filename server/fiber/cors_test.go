package fiber

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-sicky/sicky/server"
)

// TestCORSValidateRejectsUnparseableOrigins: fiber's cors.New panics on
// anything it cannot parse, so the check has to happen in Validate()
// where the constructor can turn it into a fail-closed configuration.
func TestCORSValidateRejectsUnparseableOrigins(t *testing.T) {
	for _, origin := range []string{
		"http://",            // no host
		"foo",                // not a URL
		"https://*",          // wildcard is only valid alone
		"https://app.test/x", // path is not part of an origin
		"https://app.test?a=1",
		"https://user:pass@app.test",
		"mailto:x@example.com",
	} {
		cfg := &CORSConfig{AllowedOrigins: []string{origin}}
		if err := cfg.Validate(); err == nil {
			t.Errorf("Validate(%q) = nil, want an error (cors.New would panic)", origin)
		}
	}

	for _, origin := range []string{
		"https://app.test",
		"https://app.test/",
		"http://*.example.com",
		"*",
		"  *  ",
	} {
		cfg := &CORSConfig{AllowedOrigins: []string{origin}}
		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate(%q) = %v, want nil", origin, err)
		}
	}
}

// TestCORSValidateTrimmedWildcardWithCredentials: fiber trims origins
// before its wildcard check, so the credentials guard has to trim too -
// otherwise "* " slips past Validate and panics inside cors.New.
func TestCORSValidateTrimmedWildcardWithCredentials(t *testing.T) {
	cfg := &CORSConfig{AllowedOrigins: []string{" * "}, AllowCredentials: true}
	if err := cfg.Validate(); err == nil {
		t.Fatal(`" * " with credentials must be rejected`)
	}
}

// TestInvalidOriginDoesNotPanicConstruction: the configuration error has
// to surface as a fail-closed CORS policy, not as a process crash.
func TestInvalidOriginDoesNotPanicConstruction(t *testing.T) {
	srv := New(
		&server.Options{Name: "fiber-cors"},
		&Config{
			Network: "tcp",
			Address: "127.0.0.1:0",
			CORS:    &CORSConfig{AllowedOrigins: []string{"http://"}},
		},
	)
	if srv == nil {
		t.Fatal("New must not fail on an invalid origin: it falls back to deny-all")
	}

	req := httptest.NewRequest(http.MethodGet, "http://x.test/", http.NoBody)
	req.Header.Set("Origin", "http://legit.test")

	resp, err := srv.App().Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("an origin that failed validation must fall back to deny-all, got ACAO=%q", got)
	}
}
