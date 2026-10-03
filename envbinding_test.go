/**
 * @file envbinding_test.go
 * @package sicky
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package sicky

import (
	"slices"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// nestedAppConfig is the shape every scaffold generates: the framework
// Config is nested under a "sicky" key alongside the application's own
// server and service blocks. See cli/template/project/*/config.go.gotmpl.
type nestedAppConfig struct {
	Server struct {
		HTTP *struct {
			Address string `mapstructure:"address"`
		} `mapstructure:"http"`
	} `mapstructure:"server"`
	Sicky *Config `mapstructure:"sicky"`
}

// Every sensitiveEnvKeys entry was bound at the flat path only, so for this
// shape — the one every generated project uses — none of them was ever read.
// The operator set SICKY_MANAGER_AUTH_TOKEN to protect /config and /services,
// or SICKY_INFRA_BUN_DSN to inject a credential in a container, and the
// variable was accepted, reported as known by the ignored-variable check, and
// dropped on the floor.
//
// This is the regression test for all of them at once: it walks the real list
// rather than one hand-picked key, because the failure was never specific to
// a key.
func TestSensitiveEnvReachesNestedApplicationConfig(t *testing.T) {
	t.Setenv("SICKY_MANAGER_AUTH_TOKEN", "env-token")
	t.Setenv("SICKY_INFRA_BUN_DSN", "postgres://env-host/db")
	t.Setenv("SICKY_TRACER_DSN", "env-dsn")

	v := viper.New()
	v.SetEnvPrefix(DefaultEnvPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	cfg := &nestedAppConfig{Sicky: &Config{}}

	bound := bindSensitiveEnvFor(v, cfg, DefaultEnvPrefix)

	if err := v.Unmarshal(cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if cfg.Sicky.Manager == nil {
		t.Fatal("manager block missing entirely")
	}

	if cfg.Sicky.Manager.AuthToken != "env-token" {
		t.Errorf("SICKY_MANAGER_AUTH_TOKEN: got %q, want %q — the binding used the "+
			"flat key but the value lives at sicky.manager.auth_token",
			cfg.Sicky.Manager.AuthToken, "env-token")
	}

	if cfg.Sicky.Infra == nil || cfg.Sicky.Infra.Bun == nil {
		t.Fatal("infra.bun block missing entirely")
	}

	if cfg.Sicky.Infra.Bun.DSN != "postgres://env-host/db" {
		t.Errorf("SICKY_INFRA_BUN_DSN: got %q, want the env value",
			cfg.Sicky.Infra.Bun.DSN)
	}

	if cfg.Sicky.Tracer == nil || cfg.Sicky.Tracer.DSN != "env-dsn" {
		got := ""
		if cfg.Sicky.Tracer != nil {
			got = cfg.Sicky.Tracer.DSN
		}

		t.Errorf("SICKY_TRACER_DSN: got %q, want %q", got, "env-dsn")
	}

	// Every bound variable must be reported as known, or the operator is
	// told the override "will be ignored" while it is in fact applied.
	for _, key := range sensitiveEnvKeys {
		name := envName(DefaultEnvPrefix, key)
		if slices.Contains(bound, name) {
			continue
		}

		t.Errorf("%s was never bound; a sensitive key that is not bound is unreadable", name)
	}
}

// The binding must not disturb what the config file supplies, and must not
// reach into the application's own blocks.
func TestSensitiveEnvBindingRespectsFileAndSiblingKeys(t *testing.T) {
	t.Setenv("SICKY_MANAGER_AUTH_TOKEN", "env-token")

	v := viper.New()
	v.SetEnvPrefix(DefaultEnvPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	v.SetConfigType("json")

	// The file supplies the address and no auth token at all, which is
	// exactly the situation the binding exists for.
	if err := v.ReadConfig(strings.NewReader(
		`{"sicky":{"manager":{"address":"127.0.0.1:9999"},"log_level":"warn"},` +
			`"server":{"http":{"address":":8080"}}}`)); err != nil {
		t.Fatalf("read config: %v", err)
	}

	cfg := &nestedAppConfig{Sicky: &Config{}}
	bindSensitiveEnvFor(v, cfg, DefaultEnvPrefix)

	if err := v.Unmarshal(cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if cfg.Sicky.Manager.AuthToken != "env-token" {
		t.Errorf("auth token = %q, want the env value", cfg.Sicky.Manager.AuthToken)
	}

	if cfg.Sicky.Manager.Address != "127.0.0.1:9999" {
		t.Errorf("address = %q, want the file value: the binding must not clobber "+
			"keys it has nothing to say about", cfg.Sicky.Manager.Address)
	}

	if cfg.Sicky.LogLevel != "warn" {
		t.Errorf("log level = %q, want %q", cfg.Sicky.LogLevel, "warn")
	}

	if cfg.Server.HTTP == nil || cfg.Server.HTTP.Address != ":8080" {
		t.Error("the application's own server block was disturbed by a framework binding")
	}
}

// A caller that unmarshals a Config directly has no nesting, and must keep
// the flat behavior it has always had.
func TestSensitiveEnvStillWorksForFlatTarget(t *testing.T) {
	t.Setenv("SICKY_MANAGER_AUTH_TOKEN", "flat-token")

	v := viper.New()
	v.SetEnvPrefix(DefaultEnvPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	cfg := &Config{}
	bindSensitiveEnvFor(v, cfg, DefaultEnvPrefix)

	if err := v.Unmarshal(cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if cfg.Manager == nil || cfg.Manager.AuthToken != "flat-token" {
		t.Errorf("flat target: the binding stopped working")
	}
}

// configPrefixes is what makes the fix depth-independent, so it is tested
// directly rather than only through its effect.
func TestConfigPrefixesFindsEveryNesting(t *testing.T) {
	type deepConfig struct {
		App struct {
			Framework *Config `mapstructure:"framework"`
		} `mapstructure:"app"`
	}

	for name, tc := range map[string]struct {
		target any
		want   []string
	}{
		"config itself": {
			target: &Config{},
			want:   []string{""},
		},
		"nested under one key": {
			target: &nestedAppConfig{},
			want:   []string{"sicky", ""},
		},
		"nested two levels down": {
			target: &deepConfig{},
			want:   []string{"app.framework", ""},
		},
		"no config at all": {
			target: &struct {
				Whatever string `mapstructure:"whatever"`
			}{},
			want: []string{""},
		},
	} {
		t.Run(name, func(t *testing.T) {
			got := configPrefixes(tc.target)
			if len(got) != len(tc.want) {
				t.Fatalf("prefixes = %v, want %v", got, tc.want)
			}

			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("prefix[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// An unexported or mapstructure:"-" field must not be walked: binding a path
// that cannot be unmarshaled is how the phantom-entry problem started.
func TestConfigPrefixesSkipsUnbindableFields(t *testing.T) {
	type hidden struct {
		Visible   *Config `mapstructure:"visible"`
		Skipped   *Config `mapstructure:"-"`
		unexposed *Config //nolint:unused // present to prove it is skipped
	}

	got := configPrefixes(&hidden{})

	for _, prefix := range got {
		if prefix != "visible" && prefix != "" {
			t.Errorf("prefix %q must not be produced: it cannot be unmarshaled into", prefix)
		}
	}
}

// A squashed struct contributes no key of its own; its fields sit at the
// parent's path.
func TestConfigPrefixesRespectsSquash(t *testing.T) {
	type squashed struct {
		Inline struct {
			Inner *Config `mapstructure:"inner"`
		} `mapstructure:",squash"`
	}

	got := configPrefixes(&squashed{})

	for _, prefix := range got {
		if prefix != "inner" && prefix != "" {
			t.Errorf("prefix %q: a squashed field must not add a path segment", prefix)
		}
	}
}
