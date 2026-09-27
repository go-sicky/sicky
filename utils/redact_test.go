package utils

import "testing"

func TestSanitizeValueRedactsSecretSubtrees(t *testing.T) {
	in := map[string]any{
		"dsn":      "postgres://u:p@h/db",
		"username": "ops", // plain identifiers stay visible
		"manager": map[string]any{
			"auth_token": "s3cr3t",
			"address":    ":8888",
		},
		"tracer": map[string]any{
			"type": "otlp",
			// The CLI's own list missed this: the outer key is just
			// "headers".
			"headers": map[string]any{"Authorization": "Bearer abc123"},
		},
		"private_key": "-----BEGIN PRIVATE KEY-----",
		"access_key":  "AKIASECRET",
		"secrets":     map[string]any{"db": "from-parent-key"},
		"databases":   []any{map[string]any{"password": "in-array"}},
		"tls": map[string]any{
			"key_pem": "-----BEGIN KEY-----",
			"enabled": true,
		},
	}

	out, ok := SanitizeValue("", in).(map[string]any)
	if !ok {
		t.Fatal("SanitizeValue did not return a map")
	}

	redacted := []string{
		"dsn", "private_key", "access_key", "secrets", "private_key",
	}
	for _, k := range redacted {
		if out[k] != "***redacted***" {
			t.Errorf("out[%q] = %v, want redacted", k, out[k])
		}
	}

	manager, _ := out["manager"].(map[string]any)
	if manager["auth_token"] != "***redacted***" {
		t.Errorf("auth_token = %v, want redacted", manager["auth_token"])
	}

	if manager["address"] != ":8888" {
		t.Errorf("address = %v, want it kept", manager["address"])
	}

	tracer, _ := out["tracer"].(map[string]any)
	headers, _ := tracer["headers"].(map[string]any)
	if headers["Authorization"] != "***redacted***" {
		t.Errorf("Authorization = %v, want redacted", headers["Authorization"])
	}

	tlsCfg, _ := out["tls"].(map[string]any)
	if tlsCfg["key_pem"] != "***redacted***" {
		t.Errorf("key_pem = %v, want redacted", tlsCfg["key_pem"])
	}

	dbs, _ := out["databases"].([]any)
	if len(dbs) != 1 {
		t.Fatalf("databases = %v", out["databases"])
	}

	row, _ := dbs[0].(map[string]any)
	if row["password"] != "***redacted***" {
		t.Errorf("array entry password = %v, want redacted", row["password"])
	}

	if out["username"] != "ops" {
		t.Errorf("username = %v, want it kept", out["username"])
	}

	// The original must not be mutated: /config serves a deep copy.
	if in["dsn"] == "***redacted***" {
		t.Error("SanitizeValue mutated its input")
	}
}

func TestSanitizeValueKeepsNilAndEmptySecrets(t *testing.T) {
	if got := SanitizeValue("password", nil); got != nil {
		t.Errorf("nil secret = %v, want nil", got)
	}

	// An empty string is still a credential slot: the value is hidden
	// (matching the Manager's long-standing behavior) so an operator
	// cannot read a configured-but-empty password out of /config.
	if got := SanitizeValue("password", ""); got != "***redacted***" {
		t.Errorf("empty secret = %v, want redacted", got)
	}

	if got := SanitizeValue("unrelated", ""); got != "" {
		t.Errorf("unrelated empty value = %v, want it unchanged", got)
	}
}
