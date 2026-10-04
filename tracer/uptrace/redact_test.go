package uptrace

import (
	"strings"
	"testing"
)

const canary = "SUPERSECRETUPTRACETOKEN"

// These are the exact strings uptrace-go builds when it rejects a DSN
// (uptrace/dsn.go embeds dsnStr in every error; uptrace/uptrace.go prints it
// through a logger wired to os.Stderr at init). A typo in the host or the
// scheme is an ordinary operator mistake, and each of these used to put the
// token on fd 2 in clear — outside sicky's logger, so none of the redaction
// this package does on its own log lines applied.
func TestRedactUserinfoStripsTheTokenFromEveryDSNRejection(t *testing.T) {
	for _, msg := range []string{
		`invalid Uptrace DSN: DSN="https://` + canary + `@uptrace.acme.com/1" does not have a scheme (Uptrace is disabled)`,
		`invalid Uptrace DSN: DSN="` + canary + `@uptrace.acme.com/1" does not have a scheme (Uptrace is disabled)`,
		`invalid Uptrace DSN: DSN="https://` + canary + `@" does not have a host (Uptrace is disabled)`,
		`invalid Uptrace DSN: can't parse DSN="https://` + canary + `@[::1": missing ']' in host (Uptrace is disabled)`,
		`invalid Uptrace DSN: DSN="https://uptrace.acme.com/1" does not have a token (Uptrace is disabled)`,
		`dummy Uptrace DSN detected: "https://` + canary + `@uptrace.acme.com/1" (Uptrace is disabled)`,
	} {
		t.Run(msg[:min(len(msg), 48)], func(t *testing.T) {
			got := redactUserinfo(msg)

			if strings.Contains(got, canary) {
				t.Errorf("token survived redaction:\n  in : %s\n  out: %s", msg, got)
			}
		})
	}
}

// Redaction must not be a sledgehammer. The whole value of the message is
// *why* the DSN was rejected, and utils.RedactDSN would have replaced all of
// it with the single word "REDACTED".
func TestRedactUserinfoKeepsTheDiagnosis(t *testing.T) {
	msg := `invalid Uptrace DSN: DSN="https://` + canary + `@" does not have a host (Uptrace is disabled)`

	got := redactUserinfo(msg)

	if !strings.Contains(got, "does not have a host") {
		t.Errorf("the reason was discarded: %s", got)
	}

	if !strings.Contains(got, "REDACTED@") {
		t.Errorf("the credential slot should stay visible as a placeholder: %s", got)
	}
}

// A message with nothing credential-shaped must pass through untouched — an
// over-eager pattern would redact ordinary diagnostics.
func TestRedactUserinfoLeavesOtherMessagesAlone(t *testing.T) {
	for _, msg := range []string{
		"uptrace-go uses OTLP/gRPC exporter, but got host \"localhost\"",
		"no credentials here at all",
		"",
	} {
		if got := redactUserinfo(msg); got != msg {
			t.Errorf("redactUserinfo(%q) = %q, want it unchanged", msg, got)
		}
	}
}

// The bare pattern cannot tell an email address from a scheme-less DSN token,
// and does not try. Redacting an address in a diagnostic costs nothing;
// leaking the token costs a credential, and "TOKEN@host/1" is exactly what an
// operator produces by forgetting https:// — the very mistake that produced
// the leak this exists to close.
func TestRedactUserinfoOverRedactsAnAddressOnPurpose(t *testing.T) {
	got := redactUserinfo("contact ops@example.com about the outage")

	if strings.Contains(got, "ops@example.com") {
		t.Errorf("an address was left intact: %s", got)
	}

	if !strings.Contains(got, "contact") || !strings.Contains(got, "about the outage") {
		t.Errorf("the surrounding sentence must survive: %s", got)
	}
}

// Every URL in a message is covered, not just the first.
func TestRedactUserinfoHandlesEveryURLInAMessage(t *testing.T) {
	msg := "primary https://" + canary + "@a.example/1 and fallback https://" + canary + "@b.example/2"

	got := redactUserinfo(msg)
	if strings.Contains(got, canary) {
		t.Errorf("a token survived in a multi-URL message: %s", got)
	}

	if strings.Count(got, "REDACTED@") != 2 {
		t.Errorf("want both userinfo slots redacted, got: %s", got)
	}
}
