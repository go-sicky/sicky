package sicky

import (
	"slices"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func newTestViper() *viper.Viper {
	v := viper.New()
	v.SetEnvPrefix("SICKY")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	return v
}

type managerEnvProbe struct {
	Manager struct {
		AuthToken string `mapstructure:"auth_token"`
	} `mapstructure:"manager"`
}

// TestSensitiveEnvReachesUnmarshal guards the silent-ignore trap:
// AutomaticEnv answers Get() for any key, but Unmarshal walks AllKeys,
// which does not include the automatic env namespace - so a variable for
// a key missing from the config file used to vanish without a trace.
func TestSensitiveEnvReachesUnmarshal(t *testing.T) {
	t.Setenv("SICKY_MANAGER_AUTH_TOKEN", "secret-token")

	// Without the explicit binding the override is dropped.
	unbound := newTestViper()
	var before managerEnvProbe
	if err := unbound.Unmarshal(&before); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if before.Manager.AuthToken != "" {
		t.Fatalf("unbound probe = %q, want the pre-binding behavior (ignored)", before.Manager.AuthToken)
	}

	// With the binding the same variable must arrive.
	bound := newTestViper()
	bindSensitiveEnv(bound)

	var after managerEnvProbe
	if err := bound.Unmarshal(&after); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if after.Manager.AuthToken != "secret-token" {
		t.Fatalf("auth_token = %q, want secret-token from the environment", after.Manager.AuthToken)
	}

	if !slices.Contains(bound.AllKeys(), "manager.auth_token") {
		t.Fatal("bound key missing from AllKeys: Unmarshal will skip it")
	}
}

// TestSensitiveEnvKeysExist: a typo in the binding list would silently
// restore the trap this list exists to close.
func TestSensitiveEnvKeysExist(t *testing.T) {
	v := newTestViper()
	bindSensitiveEnv(v)

	known := make(map[string]bool)
	for _, key := range v.AllKeys() {
		known[key] = true
	}

	for _, key := range sensitiveEnvKeys {
		if !known[key] {
			t.Errorf("sensitiveEnvKeys entry %q was not bound", key)
		}
	}
}

// TestIgnoredEnvReportsUnknownVariables: a SICKY_* variable that maps to
// no known key cannot be read at all and must not pass unnoticed.
func TestIgnoredEnvReportsUnknownVariables(t *testing.T) {
	t.Setenv("SICKY_TYPO_SETTING", "1")
	t.Setenv("SICKY_MANAGER_AUTH_TOKEN", "secret-token")

	v := newTestViper()
	bindSensitiveEnv(v)

	got := ignoredEnv(v, "SICKY")
	if !slices.Contains(got, "SICKY_TYPO_SETTING") {
		t.Fatalf("ignored = %v, want SICKY_TYPO_SETTING reported", got)
	}

	if slices.Contains(got, "SICKY_MANAGER_AUTH_TOKEN") {
		t.Fatalf("ignored = %v, a bound key must not be reported", got)
	}
}

// TestEnvNameMatchesViperDerivation: the warning compares against the
// name viper itself derives, so both must agree.
func TestEnvNameMatchesViperDerivation(t *testing.T) {
	if got := envName("SICKY", "manager.auth_token"); got != "SICKY_MANAGER_AUTH_TOKEN" {
		t.Fatalf("envName = %q", got)
	}

	if got := envName("SICKY", "infra.s3.secret_key"); got != "SICKY_INFRA_S3_SECRET_KEY" {
		t.Fatalf("envName = %q", got)
	}
}
