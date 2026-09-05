package runner

import "testing"

func TestConfigEnsureNilSafe(t *testing.T) {
	var c *Config
	if c.Ensure() == nil {
		t.Fatal("Ensure on nil must return non-nil config")
	}

	if (&Config{}).Ensure() == nil {
		t.Fatal("Ensure must return non-nil config")
	}
}
