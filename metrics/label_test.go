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

// TestNormalizeRedisCommandKeepsKnownVerbs verifies the typed go-redis
// methods keep their byte-identical label. Their command name is a
// compile-time constant, so normalization must be a no-op for them: a
// dashboard or alert built on sicky_infra_ops_total{op="HGETALL"} keeps
// working.
func TestNormalizeRedisCommandKeepsKnownVerbs(t *testing.T) {
	for _, cmd := range []string{"GET", "SET", "HGETALL", "EVAL", "PING", "ZADD", "SCAN", "PUBLISH"} {
		if got := NormalizeRedisCommand(cmd); got != cmd {
			t.Fatalf("NormalizeRedisCommand(%q) = %q, want %q: a typed method's label must not change", cmd, got, cmd)
		}
	}
}

// TestNormalizeRedisCommandBoundsCallerSuppliedNames is the regression.
// go-redis takes cmd.Name() from the first argument, so a Do or NewCmd
// caller supplying a variable command name minted one permanent series
// per distinct value.
func TestNormalizeRedisCommandBoundsCallerSuppliedNames(t *testing.T) {
	for _, cmd := range []string{"tenant:42:shard-1", "shard-7", "CUSTOM.LOAD", "get ", "get\n", "GET2"} {
		if got := NormalizeRedisCommand(cmd); got != UnknownCommand {
			t.Fatalf("NormalizeRedisCommand(%q) = %q, want %q: a caller-supplied name must not become a label", cmd, got, UnknownCommand)
		}
	}

	if got := NormalizeRedisCommand(""); got != UnknownCommand {
		t.Fatalf("NormalizeRedisCommand(\"\") = %q, want %q", got, UnknownCommand)
	}
}

// TestNormalizeRedisCommandIsCaseInsensitive matches go-redis, which
// lower-cases the name it derives; an upper-cased known verb must not
// fall through to OTHER.
func TestNormalizeRedisCommandIsCaseInsensitive(t *testing.T) {
	for _, cmd := range []string{"get", "Get", "hgetall", "HgetAll"} {
		if got := NormalizeRedisCommand(cmd); got == UnknownCommand {
			t.Fatalf("NormalizeRedisCommand(%q) collapsed a known verb to %q", cmd, UnknownCommand)
		}
	}
}

// TestNormalizeRedisCommandBoundsSeries is the property the fix exists
// for: any input maps into a fixed set, whatever the caller passes.
func TestNormalizeRedisCommandBoundsSeries(t *testing.T) {
	seen := map[string]bool{}
	for _, cmd := range []string{
		"GET", "get", "GET1", "X", "", "SET", "whatever", "PING", "pInG",
		"tenant:1", "tenant:2", "tenant:3", "shard-1", "shard-2",
	} {
		seen[NormalizeRedisCommand(cmd)] = true
	}

	if len(seen) > len(knownRedisCommands)+1 {
		t.Fatalf("unbounded redis command labels: %v", seen)
	}
}
