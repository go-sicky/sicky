package utils

import (
	"strings"
	"testing"
)

// RedactDSN documents itself as failing closed — "when the input cannot be
// parsed or still looks like it carries a secret" it returns the single word
// REDACTED. It did not, for a DSN missing one slash.
//
// url.Parse puts "https//tok@host:4317/1" entirely in Path with User nil, so
// hadUser is false; and net.SplitHostPort does not validate that the port is
// numeric, so "4317/1" came back as a port. Both guards missed and the raw
// DSN — credential included — was returned verbatim.
//
// That matters because the token in a DSN IS its userinfo, and the uptrace
// tracer logs the DSN at seven lifecycle sites.
func TestRedactDSNFailsClosedOnAMalformedScheme(t *testing.T) {
	const token = "tok3n"

	for _, in := range []string{
		"https//" + token + "@127.0.0.1:14317/1",
		"http//" + token + "@collector:4317",
		"//" + token + "@collector:4317/1",
		"x" + token + "y@collector:4317/1",
	} {
		t.Run(in, func(t *testing.T) {
			got := RedactDSN(in)

			if strings.Contains(got, token) {
				t.Errorf("RedactDSN(%q) = %q: the credential survived", in, got)
			}
		})
	}
}

// The bare-address pass-through is deliberate — "127.0.0.1:6379" is not a
// DSN and redacting it would be noise. It must still work, or the fix above
// has broken every non-DSN address that reaches this function.
func TestRedactDSNStillPassesThroughABareAddress(t *testing.T) {
	for _, in := range []string{
		"127.0.0.1:6379",
		"localhost:5432",
		"[::1]:4317",
	} {
		if got := RedactDSN(in); got != in {
			t.Errorf("RedactDSN(%q) = %q, want it unchanged: a bare host:port "+
				"carries no credential", in, got)
		}
	}
}

// And a well-formed DSN still round-trips to a readable redacted form rather
// than collapsing to the single word.
func TestRedactDSNStillStripsAWellFormedUserinfo(t *testing.T) {
	got := RedactDSN("https://tok3n@127.0.0.1:14317/1")

	if strings.Contains(got, "tok3n") {
		t.Fatalf("credential survived: %q", got)
	}

	if got == Redacted {
		t.Errorf("a well-formed DSN collapsed to %q; the host is useful and safe "+
			"to show", Redacted)
	}

	if !strings.Contains(got, "127.0.0.1:14317") {
		t.Errorf("= %q, want the endpoint preserved", got)
	}
}
