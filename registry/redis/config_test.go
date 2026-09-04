package redis

import "testing"

func TestConfigEnsureDefaults(t *testing.T) {
	c := (&Config{}).Ensure()
	if c.Addr != DefaultAddr || c.PoolSize != DefaultPoolSize {
		t.Fatalf("defaults: %+v", c)
	}
	if c.NotifyKey != DefaultNotifyKey || c.InstanceKey != DefaultInstanceKey {
		t.Fatalf("key defaults: %+v", c)
	}
	var nilCfg *Config
	if nilCfg.Ensure() == nil {
		t.Fatal("nil Ensure must return non-nil")
	}
}
