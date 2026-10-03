/**
 * @file encode_test.go
 * @package consul
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package consul

import (
	"slices"
	"testing"
)

// Consul's Meta is a map, so tags spread across one key per element came back
// in Go's randomized iteration order. Load runs on every pool purge tick, so
// the same service produced a different tag order on every read — and a
// consumer indexing the slice saw flapping values.
//
// The other two backends do not have this problem: they JSON-marshal the whole
// Instance and the order rides along. This pins that consul now matches them.
func TestTagRoundTripPreservesOrder(t *testing.T) {
	want := []string{"v3", "canary", "zone-a", "grpc"}

	value, present := encodeTags(want)
	if !present {
		t.Fatal("a non-empty tag slice must be written")
	}

	meta := map[string]string{metaTagsKey: value}

	for range 200 {
		got := instanceTags(meta)
		if !slices.Equal(got, want) {
			t.Fatalf("tags = %v, want %v: order must not depend on map iteration", got, want)
		}
	}
}

// A service already registered by an older version of this backend carries
// per-index Meta keys. Reading those must still work, or an upgrade silently
// drops every tag in a mixed-version cluster.
func TestLegacyPerIndexTagsAreStillRead(t *testing.T) {
	meta := map[string]string{
		"tag-0":  "first",
		"tag-1":  "second",
		"tag-2":  "third",
		"tag-10": "eleventh",
		"tag-3":  "fourth",
	}

	want := []string{"first", "second", "third", "fourth", "eleventh"}

	for range 200 {
		got := instanceTags(meta)
		if !slices.Equal(got, want) {
			t.Fatalf("legacy tags = %v, want %v: they must come back in index order", got, want)
		}
	}
}

// The modern key wins, so a service registered by a newer version is not
// doubled up by a leftover legacy key.
func TestModernKeyTakesPrecedenceOverLegacy(t *testing.T) {
	meta := map[string]string{
		metaTagsKey: `["new"]`,
		"tag-0":     "old",
		"tag-1":     "older",
	}

	got := instanceTags(meta)
	if !slices.Equal(got, []string{"new"}) {
		t.Errorf("tags = %v, want [new]", got)
	}
}

// A malformed modern value must not lose the tags silently: the legacy keys
// may still hold them, and reporting nothing is the one outcome an operator
// cannot diagnose.
func TestMalformedModernValueFallsBackToLegacy(t *testing.T) {
	meta := map[string]string{
		metaTagsKey: "{not json",
		"tag-0":     "recoverable",
	}

	got := instanceTags(meta)
	if !slices.Equal(got, []string{"recoverable"}) {
		t.Errorf("tags = %v, want the legacy value", got)
	}
}

// Nothing to write means no key, not an empty array: an empty key would make
// every registration carry a value that says nothing.
func TestEncodeTagsWritesNothingForEmpty(t *testing.T) {
	for _, tags := range [][]string{nil, {}} {
		if _, present := encodeTags(tags); present {
			t.Errorf("encodeTags(%v) wrote a key for an empty slice", tags)
		}
	}

	if got := instanceTags(nil); got != nil {
		t.Errorf("decodeTags(nil) = %v, want nil", got)
	}
}

// A tag whose name looks like an index must not be misread as one. The legacy
// reader skips unparseable suffixes, which is what keeps this honest.
func TestNonIndexTagKeysAreIgnored(t *testing.T) {
	meta := map[string]string{
		"tag-":   "empty index",
		"tag-x":  "not a number",
		"tag--1": "negative",
	}

	if got := instanceTags(meta); len(got) != 0 {
		t.Errorf("tags = %v, want none", got)
	}
}
