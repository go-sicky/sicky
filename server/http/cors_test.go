package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/uptrace/bunrouter"
)

func TestCORSValidateRejectsWildcardWithCredentials(t *testing.T) {
	cfg := &CORSConfig{AllowedOrigins: []string{"*"}, AllowCredentials: true}
	if err := cfg.Ensure().Validate(); err == nil {
		t.Fatal("wildcard + credentials must fail validation")
	}

	ok := &CORSConfig{AllowedOrigins: []string{"https://app.test"}, AllowCredentials: true}
	if err := ok.Ensure().Validate(); err != nil {
		t.Fatalf("explicit origin + credentials must validate: %v", err)
	}

	public := &CORSConfig{AllowedOrigins: []string{"*"}}
	if err := public.Ensure().Validate(); err != nil {
		t.Fatalf("wildcard without credentials must validate: %v", err)
	}
}

func TestCORSMiddlewareFailsClosedOnWildcardWithCredentials(t *testing.T) {
	cfg := &CORSConfig{AllowedOrigins: []string{"*"}, AllowCredentials: true}
	w := callCORS(t, cfg, http.MethodGet, "https://any.test")
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("illegal combo must not emit ACAO, got %q", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("illegal combo must not emit credentials, got %q", got)
	}
}

// The deprecated middleware must never reflect an arbitrary Origin with
// credentials; it now denies all cross-origin requests.
func TestDeprecatedCORSMiddlewareDenies(t *testing.T) {
	h := CORSMiddleware(func(w http.ResponseWriter, r bunrouter.Request) error {
		w.WriteHeader(http.StatusTeapot)
		return nil
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://x.test/", nil)
	req.Header.Set("Origin", "https://evil.test")
	if err := h(w, bunrouter.NewRequest(req)); err != nil {
		t.Fatalf("middleware error: %v", err)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("deprecated middleware reflected origin: %q", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("deprecated middleware set credentials: %q", got)
	}
}
