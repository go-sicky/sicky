package consul

import "testing"

func TestConfigEnsureDefaults(t *testing.T) {
	c := (&Config{}).Ensure()
	if c.Endpoint != DefaultConsulEndpoint {
		t.Fatalf("endpoint = %q, want %q", c.Endpoint, DefaultConsulEndpoint)
	}
	var nilCfg *Config
	if nilCfg.Ensure() == nil {
		t.Fatal("nil Ensure must return non-nil")
	}
}
