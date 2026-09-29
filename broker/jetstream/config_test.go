package jetstream

import "testing"

// DefaultStreamMaxConsumers (256) was only reachable through
// DefaultConfig, i.e. while c.Stream was nil. Any partial stream block
// — {"stream":{"name":"events"}} is the natural config — kept
// MaxConsumers at 0 and shipped AddStream(MaxConsumers: 0), which the
// NATS server reads as "no consumers permitted".
func TestConfigEnsureStreamMaxConsumersFilled(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
	}{
		{
			name: "partial stream block",
			cfg:  &Config{Stream: &StreamConfig{Name: "events"}},
		},
		{
			name: "subjects set, no max",
			cfg:  &Config{Stream: &StreamConfig{Subjects: []string{"orders.*"}}},
		},
		{
			name: "explicit zero still refilled",
			cfg:  &Config{Stream: &StreamConfig{Name: "events", MaxConsumers: 0}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.cfg.Ensure()

			if c.Stream.MaxConsumers != DefaultStreamMaxConsumers {
				t.Fatalf("max_consumers = %d, want %d (0 means no consumers permitted)",
					c.Stream.MaxConsumers, DefaultStreamMaxConsumers)
			}
		})
	}
}

func TestConfigEnsureStreamMaxConsumersRespectsExplicit(t *testing.T) {
	c := (&Config{Stream: &StreamConfig{MaxConsumers: 7}}).Ensure()

	if c.Stream.MaxConsumers != 7 {
		t.Fatalf("explicit max_consumers clobbered: %d", c.Stream.MaxConsumers)
	}
}

func TestConfigEnsureStreamNilUsesDefaults(t *testing.T) {
	c := (&Config{}).Ensure()

	if c.Stream == nil {
		t.Fatal("nil stream must be filled with defaults")
	}

	if c.Stream.MaxConsumers != DefaultStreamMaxConsumers {
		t.Fatalf("max_consumers = %d, want %d", c.Stream.MaxConsumers, DefaultStreamMaxConsumers)
	}
}

// The deprecated misspelled field is still honored and mirrored back.
func TestConfigEnsureStreamDeprecatedMaxConsumersMigrates(t *testing.T) {
	c := (&Config{Stream: &StreamConfig{MaxConsummers: 12}}).Ensure()

	if c.Stream.MaxConsumers != 12 {
		t.Fatalf("deprecated max_consummers not migrated: %d", c.Stream.MaxConsumers)
	}

	if c.Stream.MaxConsumers != 12 {
		t.Fatalf("deprecated field not mirrored: %d", c.Stream.MaxConsumers)
	}
}

func TestConfigValidateMaxConsumers(t *testing.T) {
	if err := (&Config{Stream: &StreamConfig{MaxConsumers: -1}}).Validate(); err == nil {
		t.Fatal("negative max_consumers must be rejected")
	}

	if err := (&Config{Stream: &StreamConfig{MaxConsumers: 5}}).Validate(); err != nil {
		t.Fatalf("positive max_consumers must validate: %v", err)
	}

	// A nil stream disables JetStream stream management entirely.
	if err := (&Config{}).Validate(); err != nil {
		t.Fatalf("nil stream must validate: %v", err)
	}
}
