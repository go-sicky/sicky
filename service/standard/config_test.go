package standard

import (
	"encoding/json"
	"testing"
)

// TestConfigIgnoresRemovedKeys pins the C6 removal. disable_trace sat next
// to disable_tracing, so it read like the tracing switch while doing nothing;
// a file that still sets it must not disturb the switch that does work.
func TestConfigIgnoresRemovedKeys(t *testing.T) {
	const payload = `{
		"disable_trace": true,
		"disable_tracing": false,
		"disable_jobs": true
	}`

	var cfg Config
	if err := json.Unmarshal([]byte(payload), &cfg); err != nil {
		t.Fatalf("a config file that still carries disable_trace must still decode: %v", err)
	}

	if cfg.DisableTracing {
		t.Fatal("disable_trace must not turn tracing off: disable_tracing was explicitly false")
	}

	if !cfg.DisableJobs {
		t.Fatal("disable_jobs = false, want true: the decode ignored a key it should honor")
	}
}

// TestDefaultConfigLeavesSwitchesOff pins the remaining default shape.
func TestDefaultConfigLeavesSwitchesOff(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig must return non-nil")
	}

	if cfg.DisableServerRegister || cfg.DisableJobs || cfg.DisableTracing {
		t.Fatalf("DefaultConfig must leave every switch false: %+v", cfg)
	}

	var nilCfg *Config
	if nilCfg.Ensure() == nil {
		t.Fatal("nil Ensure must return non-nil")
	}
}
