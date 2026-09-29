package service

import "testing"

// TestConfigEnsureIsNilSafe covers the placeholder shape this Config has
// after the dead enable_manager key was removed: an empty struct whose only
// job is to hand back a non-nil receiver.
func TestConfigEnsureIsNilSafe(t *testing.T) {
	var nilCfg *Config
	if nilCfg.Ensure() == nil {
		t.Fatal("nil Ensure must return non-nil")
	}

	if cfg := (&Config{}).Ensure(); cfg == nil {
		t.Fatal("non-nil Ensure must return the receiver")
	}

	if cfg := DefaultConfig(); cfg == nil {
		t.Fatal("DefaultConfig must return non-nil")
	}
}
