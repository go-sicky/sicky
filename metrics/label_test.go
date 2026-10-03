/**
 * @file label_test.go
 * @package metrics
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package metrics

import (
	"strings"
	"testing"
)

// NormalizeClientHost is the only bound standing between a caller-supplied
// destination and a permanent Prometheus series. client/http.Do passes
// req.URL.Host straight through, so an application that fetches a
// caller-provided URL (gateway, webhook relay, fetch-a-URL feature) would
// otherwise mint one counter, one histogram child, one cache entry and one
// /metrics line per request.
func TestNormalizeClientHostBoundsEveryInput(t *testing.T) {
	hosts := []string{
		"a.example.com",
		"evil.example.com",
		"10.0.0.1:8080",
		"attacker-8f3a9c2b1d.example:443",
		strings.Repeat("x", 4096) + ".example.com",
	}

	seen := make(map[string]struct{}, len(hosts))

	for _, h := range hosts {
		got := NormalizeClientHost(h)
		if got != OtherHost {
			t.Errorf("NormalizeClientHost(%q) = %q, want %q", truncate(h), got, OtherHost)
		}

		seen[got] = struct{}{}
	}

	if len(seen) != 1 {
		t.Fatalf("distinct inputs produced %d label values, want 1: the bound is not working", len(seen))
	}
}

// An empty host is a caller bug, not an attack, so it is kept out of the
// OtherHost bucket: otherwise every misconfigured client shares a series and
// the signal that something is broken disappears into the aggregate.
func TestNormalizeClientHostSeparatesEmptyFromOther(t *testing.T) {
	if got := NormalizeClientHost(""); got != UnknownHost {
		t.Errorf("NormalizeClientHost(\"\") = %q, want %q", got, UnknownHost)
	}

	if UnknownHost == OtherHost {
		t.Fatal("the two sentinels must differ, or the empty case is indistinguishable")
	}
}

func truncate(s string) string {
	if len(s) <= 40 {
		return s
	}

	return s[:40] + "..."
}
