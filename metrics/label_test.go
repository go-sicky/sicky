package metrics

import "testing"

func TestNormalizeHTTPMethod(t *testing.T) {
	for _, tt := range []struct {
		in   string
		want string
	}{
		{"GET", "GET"},
		{"get", "GET"},
		{"Post", "POST"},
		{"HEAD", "HEAD"},
		{"OPTIONS", "OPTIONS"},
		{"", UnknownMethod},
		{"GET1", UnknownMethod},
		{"FOO", UnknownMethod},
		{"GET / HTTP/1.1", UnknownMethod},
		{"get\r\nX-Injected: 1", UnknownMethod},
	} {
		if got := NormalizeHTTPMethod(tt.in); got != tt.want {
			t.Fatalf("NormalizeHTTPMethod(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestNormalizeRouteLabel(t *testing.T) {
	if got := NormalizeRouteLabel("/user/:id"); got != "/user/:id" {
		t.Fatalf("registered route must stay verbatim, got %q", got)
	}

	// The unmatched path is raw client input and must never become a label.
	if got := NormalizeRouteLabel(""); got != UnmatchedRoute {
		t.Fatalf("empty route = %q, want %q", got, UnmatchedRoute)
	}
}

// TestNormalizeMethodBoundsSeries is the property the fix exists for:
// any input maps into a fixed set.
func TestNormalizeMethodBoundsSeries(t *testing.T) {
	seen := map[string]bool{}
	for _, m := range []string{"GET", "get", "GeT", "DELETE1", "X", "", "POST", "whatever", "OPTIONS", "pOsT"} {
		seen[NormalizeHTTPMethod(m)] = true
	}

	if len(seen) > len(knownHTTPMethods)+1 {
		t.Fatalf("unbounded method labels: %v", seen)
	}
}
