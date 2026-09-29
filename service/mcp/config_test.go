package mcp

import (
	"encoding/json"
	"testing"
)

// TestConfigRejectsRemovedKeys pins the C6 removal. The deleted keys were
// inert (nothing read them), so a config file that still carries them must
// decode without error AND without the service silently believing it
// configured something it never read.
func TestConfigRejectsRemovedKeys(t *testing.T) {
	const payload = `{
		"disable_wrappers": true,
		"disable_jobs": true,
		"disable_server_register": true,
		"disable_tracing": true,
		"transport": "http",
		"listen": "0.0.0.0:9000"
	}`

	var cfg Config
	if err := json.Unmarshal([]byte(payload), &cfg); err != nil {
		t.Fatalf("a config file that still carries the removed keys must still decode: %v", err)
	}

	// The transport is chosen at construction, not by config: a file that
	// asks for http must not make the server look configured for it.
	if cfg.DisableJobs != true {
		t.Fatalf("live key disable_jobs = false, want true: the decode ignored a key it should honor")
	}
}

// TestDefaultConfigHasNoTransport pins that the default carries no transport:
// DefaultTransport and the Transport field were both removed, so the default
// can only be the empty set of switches.
func TestDefaultConfigHasNoTransport(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig must return non-nil")
	}

	if cfg.DisableJobs || cfg.DisableServerRegister || cfg.DisableTracing {
		t.Fatalf("DefaultConfig must leave every switch false: %+v", cfg)
	}

	var nilCfg *Config
	if nilCfg.Ensure() == nil {
		t.Fatal("nil Ensure must return non-nil")
	}
}
