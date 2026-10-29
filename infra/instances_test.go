/**
 * @file instances_test.go
 * @package infra
 * @author Dr.NP <np@herewe.tech>
 * @since 10/29/2026
 */

package infra

import (
	"errors"
	"testing"
)

func TestNormalizeAndValidateInstanceName(t *testing.T) {
	if got := normalizeInstanceName("  Gallery-1  "); got != "gallery-1" {
		t.Fatalf("normalizeInstanceName = %q, want gallery-1", got)
	}

	valid := []string{"a", "cache", "cache-1", "tenant_01", "0"}
	for _, name := range valid {
		if err := validateInstanceName(name); err != nil {
			t.Errorf("validateInstanceName(%q) = %v, want nil", name, err)
		}
	}

	invalid := []string{"", "Gallery", "a.b", "a:b", "-cache", "_cache", string(make([]byte, 65))}
	for _, name := range invalid {
		if err := validateInstanceName(name); err == nil {
			t.Errorf("validateInstanceName(%q) = nil, want an error", name)
		}
	}
}

func TestFirstInstanceName(t *testing.T) {
	if got := firstInstanceName(nil); got != "" {
		t.Fatalf("firstInstanceName(nil) = %q, want empty", got)
	}

	if got := firstInstanceName([]string{"  Gallery  ", "other"}); got != "gallery" {
		t.Fatalf("firstInstanceName = %q, want gallery", got)
	}
}

func TestInstanceSetFirstWins(t *testing.T) {
	set := newInstanceSet[string]()

	first, stored := set.putFirst("cache", "first")
	if !stored || first != "first" {
		t.Fatalf("putFirst first = (%q, %v), want (first, true)", first, stored)
	}

	second, stored := set.putFirst("cache", "second")
	if stored || second != "first" {
		t.Fatalf("putFirst duplicate = (%q, %v), want (first, false)", second, stored)
	}

	if got, ok := set.get("cache"); !ok || got != "first" {
		t.Fatalf("get(cache) = (%q, %v), want (first, true)", got, ok)
	}
}

func TestInstanceSetSnapshotIsCopy(t *testing.T) {
	set := newInstanceSet[string]()
	set.putFirst("a", "one")

	snapshot := set.snapshot()
	snapshot["a"] = "changed"
	snapshot["b"] = "new"

	if got, _ := set.get("a"); got != "one" {
		t.Fatalf("snapshot mutation changed the registry: %q", got)
	}

	if _, ok := set.get("b"); ok {
		t.Fatal("snapshot mutation added a registry entry")
	}
}

func TestInstanceSetClearAndRemove(t *testing.T) {
	set := newInstanceSet[string]()
	set.putFirst("a", "one")
	set.putFirst("b", "two")

	removed := set.clear()
	if len(removed) != 2 {
		t.Fatalf("clear returned %d entries, want 2", len(removed))
	}

	if got := set.names(); len(got) != 0 {
		t.Fatalf("names after clear = %v, want empty", got)
	}

	set.putFirst("c", "three")
	if value, ok := set.remove("c"); !ok || value != "three" {
		t.Fatalf("remove(c) = (%q, %v), want (three, true)", value, ok)
	}
}

func TestNormalizeInstanceNamesDetectsDuplicates(t *testing.T) {
	raw := map[string]*struct{ Name string }{
		"Gallery": {Name: "one"},
		"gallery": {Name: "two"},
	}

	_, _, err := normalizeInstanceNames(raw)
	if !errors.Is(err, ErrInstanceNameDuplicate) {
		t.Fatalf("normalizeInstanceNames = %v, want ErrInstanceNameDuplicate", err)
	}
}

func TestNormalizeInstanceNamesRejectsNilAndInvalid(t *testing.T) {
	if _, _, err := normalizeInstanceNames(map[string]*struct{ Name string }{"cache": nil}); !errors.Is(err, ErrInstanceDecode) {
		t.Fatalf("nil instance = %v, want ErrInstanceDecode", err)
	}

	if _, _, err := normalizeInstanceNames(map[string]*struct{ Name string }{"Bad.Name": {}}); !errors.Is(err, ErrInstanceNameInvalid) {
		t.Fatalf("invalid instance name = %v, want ErrInstanceNameInvalid", err)
	}
}

func TestInstanceLabel(t *testing.T) {
	top := defaultSelection{UseTopLevel: true}
	alias := defaultSelection{Instance: "cache"}
	other := defaultSelection{Instance: "other"}
	none := defaultSelection{}

	cases := []struct {
		name      string
		selection defaultSelection
		want      string
	}{
		{"top-level default", top, "redis:cache"},
		{"named default alias", alias, "redis"},
		{"another named default", other, "redis:cache"},
		{"no default", none, "redis:cache"},
	}

	for _, tc := range cases {
		if got := instanceLabel("redis", "cache", tc.selection); got != tc.want {
			t.Errorf("%s: instanceLabel = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestResolveDefaultSelectionIdentityError(t *testing.T) {
	identityErr := errors.New("missing identity")

	if _, err := resolveDefaultSelection(false, "", nil, false, identityErr); !errors.Is(err, identityErr) {
		t.Fatalf("resolveDefaultSelection = %v, want the identity error", err)
	}
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
