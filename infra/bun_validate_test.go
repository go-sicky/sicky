package infra

import (
	"errors"
	"testing"
)

// Every int on BunConfig is a threshold: zero means "driver default / off",
// negative is an operator mistake. Four of the five enforced that; the fifth
// did not, so a negative slow_duration silently disabled the query hook
// instead of failing startup — indistinguishable, from the outside, from
// omitting the key.
func TestBunValidateRejectsEveryNegativeInt(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*BunConfig)
		wantErr error
	}{
		{
			name:    "slow_duration",
			mutate:  func(c *BunConfig) { c.SlowDuration = -1 },
			wantErr: ErrBunSlowInvalid,
		},
		{
			name:    "max_open_conns",
			mutate:  func(c *BunConfig) { c.MaxOpenConns = -1 },
			wantErr: ErrBunPoolInvalid,
		},
		{
			name:    "max_idle_conns",
			mutate:  func(c *BunConfig) { c.MaxIdleConns = -1 },
			wantErr: ErrBunPoolInvalid,
		},
		{
			name:    "conn_max_lifetime_sec",
			mutate:  func(c *BunConfig) { c.ConnMaxLifetimeSec = -1 },
			wantErr: ErrBunPoolInvalid,
		},
		{
			name:    "conn_max_idle_time_sec",
			mutate:  func(c *BunConfig) { c.ConnMaxIdleTimeSec = -1 },
			wantErr: ErrBunPoolInvalid,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &BunConfig{Driver: "sqlite", DSN: ":memory:"}
			tc.mutate(cfg)

			if err := cfg.Validate(); !errors.Is(err, tc.wantErr) {
				t.Errorf("Validate() = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

// Zero must stay legal everywhere: it is the documented "use the default" for
// the pool knobs and "leave the hook off" for slow_duration.
func TestBunValidateAcceptsZeroForEveryInt(t *testing.T) {
	cfg := &BunConfig{Driver: "sqlite", DSN: ":memory:"}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil for an all-zero config", err)
	}
}

// The driver allow-list and the DSN check still take precedence in the same
// order: adding the slow_duration check must not have moved them.
func TestBunValidateStillRejectsDriverAndDSNFirst(t *testing.T) {
	if err := (&BunConfig{Driver: "sqlite", DSN: "  "}).Validate(); !errors.Is(err, ErrBunDSNEmpty) {
		t.Errorf("blank DSN = %v, want ErrBunDSNEmpty", err)
	}

	if err := (&BunConfig{Driver: "postgres2", DSN: "dsn"}).Validate(); !errors.Is(err, ErrBunUnsupportedDriver) {
		t.Errorf("bad driver = %v, want ErrBunUnsupportedDriver", err)
	}

	// A negative slow_duration alongside a bad driver: whichever fires first,
	// the operator gets a sentinel rather than a silently ignored value.
	if err := (&BunConfig{Driver: "postgres2", DSN: "dsn", SlowDuration: -1}).Validate(); err == nil {
		t.Error("a negative slow_duration must not be silently accepted")
	}
}
