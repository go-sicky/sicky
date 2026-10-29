/**
 * @file manager_infra_health_test.go
 * @package sicky
 * @author Dr.NP <np@herewe.tech>
 * @since 10/29/2026
 */

package sicky

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/go-sicky/sicky/infra"
)

// Named infra instances must appear on /health next to their component.
// Ristretto is used because it starts locally and needs no external server.
func TestHealthReportsNamedInfraInstances(t *testing.T) {
	infra.ShutdownRistretto()
	t.Cleanup(func() { infra.ShutdownRistretto() })

	if _, err := infra.InitRistretto(&infra.RistrettoConfig{
		NumCounters: 1000,
		MaxCost:     1000,
		Instances: map[string]*infra.RistrettoConfig{
			"hot": {NumCounters: 1000, MaxCost: 1000},
		},
	}); err != nil {
		t.Fatalf("InitRistretto: %v", err)
	}

	m := NewManager(nil, "app", "v1")

	rec := httptest.NewRecorder()
	m.health().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", http.NoBody))

	var body struct {
		Components []componentHealth `json:"components"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode /health: %v", err)
	}

	names := make([]string, 0, len(body.Components))
	for _, component := range body.Components {
		names = append(names, component.Name)
	}

	for _, want := range []string{"ristretto", "ristretto:hot"} {
		if !slices.Contains(names, want) {
			t.Errorf("/health components = %v, want %q", names, want)
		}
	}
}

// The named definitions are generic: cover the local Bun and Badger paths too,
// so a future edit to probeComponents cannot silently drop one component's
// instances while the Ristretto case keeps passing.
func TestHealthReportsLocalNamedInstances(t *testing.T) {
	infra.ShutdownBun()
	infra.ShutdownBadger()
	t.Cleanup(func() {
		infra.ShutdownBun()
		infra.ShutdownBadger()
	})

	if _, err := infra.InitBun(&infra.BunConfig{
		Driver: "sqlite",
		DSN:    ":memory:",
		Instances: map[string]*infra.BunConfig{
			"reporting": {DSN: ":memory:"},
		},
	}); err != nil {
		t.Fatalf("InitBun: %v", err)
	}

	if _, err := infra.InitBadger(&infra.BadgerConfig{
		Path: t.TempDir(),
		Instances: map[string]*infra.BadgerConfig{
			"archive": {Path: t.TempDir()},
		},
	}); err != nil {
		t.Fatalf("InitBadger: %v", err)
	}

	m := NewManager(nil, "app", "v1")

	rec := httptest.NewRecorder()
	m.health().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", http.NoBody))

	var body struct {
		Components []componentHealth `json:"components"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode /health: %v", err)
	}

	names := make([]string, 0, len(body.Components))
	for _, component := range body.Components {
		names = append(names, component.Name)
	}

	for _, want := range []string{"bun", "bun:reporting", "badger", "badger:archive"} {
		if !slices.Contains(names, want) {
			t.Errorf("/health components = %v, want %q", names, want)
		}
	}
}

// A remote named instance must reach /health with its component:name key too.
// Elastic is used because a fake cluster can answer the startup Info check and
// the health probe without a real backend.
func TestHealthReportsNamedElasticInstance(t *testing.T) {
	infra.ShutdownElastic()
	t.Cleanup(func() { infra.ShutdownElastic() })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"fake","version":{"number":"9.0.0"}}`))
	}))
	defer server.Close()

	if _, err := infra.InitElastic(&infra.ElasticConfig{
		Addresses: []string{server.URL},
		Instances: map[string]*infra.ElasticConfig{
			"search": {Addresses: []string{server.URL}},
		},
	}); err != nil {
		t.Fatalf("InitElastic: %v", err)
	}

	m := NewManager(nil, "app", "v1")

	rec := httptest.NewRecorder()
	m.health().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", http.NoBody))

	var body struct {
		Components []componentHealth `json:"components"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode /health: %v", err)
	}

	names := make([]string, 0, len(body.Components))
	for _, component := range body.Components {
		names = append(names, component.Name)
	}

	for _, want := range []string{"elastic", "elastic:search"} {
		if !slices.Contains(names, want) {
			t.Errorf("/health components = %v, want %q", names, want)
		}
	}
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
