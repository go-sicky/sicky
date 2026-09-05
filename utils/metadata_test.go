/*
 * The MIT License (MIT)
 *
 * Copyright (c) 2024 HereweTech Co.LTD
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy of
 * this software and associated documentation files (the "Software"), to deal in
 * the Software without restriction, including without limitation the rights to
 * use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
 * the Software, and to permit persons to whom the Software is furnished to do so,
 * subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
 * FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
 * COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
 * IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
 * CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 */

/**
 * @file metadata_test.go
 * @package utils
 * @author Dr.NP <np@herewe.tech>
 * @since 07/07/2026
 */

package utils

import (
	"slices"
	"testing"
)

func TestNewMetadata(t *testing.T) {
	md := NewMetadata()
	if md == nil {
		t.Fatal("NewMetadata returned nil")
	}

	if len(md) != 0 {
		t.Fatalf("expected empty map, got %d entries", len(md))
	}
}

func TestMetadataGet(t *testing.T) {
	md := Metadata{"key1": "val1", "key2": "val2"}

	tests := []struct {
		name  string
		key   string
		val   string
		found bool
	}{
		{"existing key", "key1", "val1", true},
		{"another existing key", "key2", "val2", true},
		{"missing key", "key3", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, ok := md.Get(tt.key)
			if ok != tt.found {
				t.Errorf("expected found=%v, got %v", tt.found, ok)
			}

			if val != tt.val {
				t.Errorf("expected value=%q, got %q", tt.val, val)
			}
		})
	}

	empty := Metadata{}
	v, ok := empty.Get("any")
	if ok {
		t.Error("expected not found on empty metadata")
	}

	if v != "" {
		t.Errorf("expected empty string, got %q", v)
	}
}

func TestMetadataValue(t *testing.T) {
	md := Metadata{"key1": "val1"}

	tests := []struct {
		name string
		key  string
		def  string
		want string
	}{
		{"existing key ignores default", "key1", "default", "val1"},
		{"missing key returns default", "key2", "default", "default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := md.Value(tt.key, tt.def)
			if got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestMetadataSet(t *testing.T) {
	md := NewMetadata()

	md.Set("key1", "val1")
	v, ok := md["key1"]
	if !ok || v != "val1" {
		t.Errorf("expected key1=val1, got %q", v)
	}

	md.Set("key1", "overwritten")
	v, ok = md["key1"]
	if !ok || v != "overwritten" {
		t.Errorf("expected key1=overwritten after overwrite, got %q", v)
	}
}

func TestMetadataDelete(t *testing.T) {
	md := Metadata{"key1": "val1", "key2": "val2"}

	md.Delete("key1")
	if _, ok := md["key1"]; ok {
		t.Error("key1 should have been deleted")
	}

	if v, ok := md["key2"]; !ok || v != "val2" {
		t.Error("key2 should still exist")
	}

	md.Delete("nonexistent")
	if len(md) != 1 {
		t.Errorf("expected 1 entry after deleting nonexistent key, got %d", len(md))
	}
}

func TestMetadataMerge(t *testing.T) {
	md := Metadata{"a": "1", "b": "2"}

	md.Merge(Metadata{"c": "3"})
	if len(md) != 3 {
		t.Errorf("expected 3 entries, got %d", len(md))
	}

	if md["c"] != "3" {
		t.Errorf("expected c=3, got %q", md["c"])
	}

	md.Merge(Metadata{"a": "overwritten"})
	if md["a"] != "overwritten" {
		t.Errorf("expected a=overwritten after merge, got %q", md["a"])
	}

	md.Merge(Metadata{})
	if len(md) != 3 {
		t.Errorf("expected 3 entries after empty merge, got %d", len(md))
	}
}

func TestMetadataCopy(t *testing.T) {
	md := Metadata{"a": "1", "b": "2"}
	cp := md.Copy()

	if len(cp) != len(md) {
		t.Errorf("expected %d entries in copy, got %d", len(md), len(cp))
	}

	for k, v := range md {
		if cp[k] != v {
			t.Errorf("copy[%q] = %q, want %q", k, cp[k], v)
		}
	}

	cp["a"] = "modified"
	if md["a"] != "1" {
		t.Error("copy mutation affected original")
	}
}

func TestMetadataClone(t *testing.T) {
	md := Metadata{"a": "1"}
	cl := md.Clone()

	cl["a"] = "modified"
	if md["a"] != "1" {
		t.Error("clone mutation affected original")
	}
}

func TestMetadataStrings(t *testing.T) {
	tests := []struct {
		name string
		md   Metadata
		want []string
	}{
		{"empty", Metadata{}, []string{}},
		{"single", Metadata{"key": "val"}, []string{"key=val"}},
		{"multiple", Metadata{"a": "1", "b": "2"}, []string{"a=1", "b=2"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.md.Strings()
			if len(got) != len(tt.want) {
				t.Fatalf("expected %d strings, got %d", len(tt.want), len(got))
			}

			for _, want := range tt.want {
				if !slices.Contains(got, want) {
					t.Errorf("expected %q in output, got %v", want, got)
				}
			}
		})
	}
}

func TestMetadataFromStrings(t *testing.T) {
	tests := []struct {
		name   string
		input  []string
		expect Metadata
	}{
		{"empty", []string{}, Metadata{}},
		{"single pair", []string{"key=val"}, Metadata{"key": "val"}},
		{"multiple pairs", []string{"a=1", "b=2"}, Metadata{"a": "1", "b": "2"}},
		{"whitespace trimmed", []string{" key = val "}, Metadata{"key": "val"}},
		{"malformed ignored", []string{"noequals", "a=1"}, Metadata{"a": "1"}},
		{"multiple equals", []string{"key=val=ue"}, Metadata{"key": "val=ue"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MetadataFromStrings(tt.input)
			if len(got) != len(tt.expect) {
				t.Errorf("expected %d entries, got %d", len(tt.expect), len(got))
			}

			for k, v := range tt.expect {
				if got[k] != v {
					t.Errorf("expected %s=%s, got %q", k, v, got[k])
				}
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
