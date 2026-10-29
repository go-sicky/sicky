/**
 * @file configschema_resolve_test.go
 * @package sicky
 * @author Dr.NP <np@herewe.tech>
 * @since 10/29/2026
 */

package sicky

import (
	"reflect"
	"testing"
)

type schemaLeaf struct {
	Name   string `mapstructure:"name"`
	Nested struct {
		Value string `mapstructure:"value"`
	} `mapstructure:"nested"`
	Open map[string]string `mapstructure:"open"`
}

type schemaNestedMap struct {
	Items map[string]*schemaLeaf `mapstructure:"items"`
}

type SchemaSquashInner struct {
	Inner string `mapstructure:"inner"`
}

type SchemaSquashOuter struct {
	SchemaSquashInner `mapstructure:",squash"`
}

func TestResolveStructFieldsBranches(t *testing.T) {
	leaf := reflect.TypeFor[schemaLeaf]()

	cases := []struct {
		name     string
		t        reflect.Type
		segments []string
		known    bool
		bad      string
	}{
		{"empty", leaf, nil, true, ""},
		{"leaf", leaf, []string{"name"}, true, ""},
		{"unknown leaf", leaf, []string{"missing"}, false, "missing"},
		{"nested struct", leaf, []string{"nested", "value"}, true, ""},
		{"unknown nested", leaf, []string{"nested", "missing"}, false, "nested.missing"},
		{"open map", leaf, []string{"open", "anything"}, true, ""},
		{"extra segment", leaf, []string{"name", "extra"}, false, "name"},
		{"non-struct", reflect.TypeFor[string](), []string{"x"}, false, "x"},
		{"nested map value", reflect.TypeFor[schemaNestedMap](), []string{"items", "one", "name"}, true, ""},
		{"nested map key only", reflect.TypeFor[schemaNestedMap](), []string{"items", "one"}, true, ""},
		{"nested map unknown", reflect.TypeFor[schemaNestedMap](), []string{"items", "one", "missing"}, false, "items.one.missing"},
		{"squashed field", reflect.TypeFor[SchemaSquashOuter](), []string{"inner"}, true, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			known, bad := resolveStructFields(tc.t, tc.segments, "")
			if known != tc.known || bad != tc.bad {
				t.Fatalf("resolveStructFields = (%v, %q), want (%v, %q)", known, bad, tc.known, tc.bad)
			}
		})
	}
}

func TestResolveMapKeyBranches(t *testing.T) {
	schema := configSchema{
		mapValues: map[string]reflect.Type{
			"things": reflect.TypeFor[schemaLeaf](),
			"open":   nil,
		},
	}

	cases := []struct {
		key   string
		known bool
		bad   string
	}{
		{"things", true, ""},
		{"things.one", true, ""},
		{"things.one.name", true, ""},
		{"things.one.missing", false, "things.one.missing"},
		{"open.anything.goes", true, ""},
		{"other.one.name", false, ""},
	}

	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			known, bad := schema.resolveMapKey(tc.key)
			if known != tc.known || bad != tc.bad {
				t.Fatalf("resolveMapKey(%q) = (%v, %q), want (%v, %q)", tc.key, known, bad, tc.known, tc.bad)
			}
		})
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
