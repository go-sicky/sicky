package registry

import "testing"

// pool_purge_interval: 0 previously became 60, which made sicky.Run's
// `> 0` guard dead and silently overrode an explicit opt-out.
func TestConfigEnsurePoolPurgeInterval(t *testing.T) {
	tests := []struct {
		name string
		in   int64
		want int64
	}{
		// Zero is also what an absent key decodes to, so it must fill
		// the default rather than disable discovery.
		{"absent or zero fills default", 0, DefaultPoolPurgeInterval},
		{"negative disables", -1, 0},
		{"explicit kept", 15, 15},
		{"above default kept", 300, 300},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := (&Config{PoolPurgeInterval: tt.in}).Ensure()

			if c.PoolPurgeInterval != tt.want {
				t.Fatalf("pool_purge_interval(%d) = %d, want %d",
					tt.in, c.PoolPurgeInterval, tt.want)
			}
		})
	}
}

// The disabled state must survive Ensure, otherwise sicky.Run can
// never see a non-positive interval and its `> 0` guard stays dead.
func TestConfigEnsurePoolPurgeIntervalDisabledStaysZero(t *testing.T) {
	c := (&Config{PoolPurgeInterval: -1}).Ensure()

	if c.PoolPurgeInterval > 0 {
		t.Fatalf("disabled interval refilled to %d, Run() guard stays dead",
			c.PoolPurgeInterval)
	}
}

func TestConfigEnsureNilSafe(t *testing.T) {
	c := (*Config)(nil).Ensure()

	if c == nil {
		t.Fatal("nil Ensure returned nil")
	}

	if c.PoolPurgeInterval != DefaultPoolPurgeInterval {
		t.Fatalf("nil Config pool_purge_interval = %d, want %d",
			c.PoolPurgeInterval, DefaultPoolPurgeInterval)
	}
}

func TestDefaultConfigPoolPurgeInterval(t *testing.T) {
	if got := DefaultConfig().PoolPurgeInterval; got != DefaultPoolPurgeInterval {
		t.Fatalf("DefaultConfig pool_purge_interval = %d, want %d", got, DefaultPoolPurgeInterval)
	}
}
