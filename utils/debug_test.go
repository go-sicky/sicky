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
 * @file debug_test.go
 * @package utils
 * @author Dr.NP <np@herewe.tech>
 * @since 07/07/2026
 */

package utils

import (
	"encoding/json"
	"encoding/xml"
	"testing"
)

type testStruct struct {
	Name  string `json:"name"  xml:"name"`
	Value int    `json:"value" xml:"value"`
}

func TestJSONAnyString(t *testing.T) {
	t.Run("valid struct", func(t *testing.T) {
		d := testStruct{Name: "test", Value: 42}
		got := JSONAnyString(d)
		if got == "" {
			t.Error("expected non-empty JSON string")
		}

		var parsed testStruct
		if err := json.Unmarshal([]byte(got), &parsed); err != nil {
			t.Errorf("failed to parse JSON output: %v", err)
		}

		if parsed.Name != "test" || parsed.Value != 42 {
			t.Errorf("unexpected parsed value: %+v", parsed)
		}
	})

	t.Run("nil input", func(t *testing.T) {
		got := JSONAnyString(nil)
		if got != "null" {
			t.Errorf("expected 'null' for nil input, got %q", got)
		}
	})
}

func TestJSONAnyBytes(t *testing.T) {
	t.Run("valid struct", func(t *testing.T) {
		d := testStruct{Name: "test", Value: 42}
		got := JSONAnyBytes(d)
		if got == nil {
			t.Fatal("expected non-nil JSON bytes")
		}

		var parsed testStruct
		if err := json.Unmarshal(got, &parsed); err != nil {
			t.Errorf("failed to parse JSON output: %v", err)
		}

		if parsed.Name != "test" || parsed.Value != 42 {
			t.Errorf("unexpected parsed value: %+v", parsed)
		}
	})

	t.Run("nil input", func(t *testing.T) {
		got := JSONAnyBytes(nil)
		if string(got) != "null" {
			t.Errorf("expected 'null' bytes for nil input, got %v", got)
		}
	})
}

func TestXMLAnyString(t *testing.T) {
	t.Run("valid struct", func(t *testing.T) {
		d := testStruct{Name: "test", Value: 42}
		got := XMLAnyString(d)
		if got == "" {
			t.Error("expected non-empty XML string")
		}

		var parsed testStruct
		if err := xml.Unmarshal([]byte(got), &parsed); err != nil {
			t.Errorf("failed to parse XML output: %v", err)
		}

		if parsed.Name != "test" || parsed.Value != 42 {
			t.Errorf("unexpected parsed value: %+v", parsed)
		}
	})

	t.Run("nil input", func(t *testing.T) {
		got := XMLAnyString(nil)
		if got != "" {
			t.Errorf("expected empty string for nil input, got %q", got)
		}
	})
}

func TestXMLAnyBytes(t *testing.T) {
	t.Run("valid struct", func(t *testing.T) {
		d := testStruct{Name: "test", Value: 42}
		got := XMLAnyBytes(d)
		if got == nil {
			t.Fatal("expected non-nil XML bytes")
		}

		var parsed testStruct
		if err := xml.Unmarshal(got, &parsed); err != nil {
			t.Errorf("failed to parse XML output: %v", err)
		}

		if parsed.Name != "test" || parsed.Value != 42 {
			t.Errorf("unexpected parsed value: %+v", parsed)
		}
	})

	t.Run("nil input", func(t *testing.T) {
		got := XMLAnyBytes(nil)
		if got != nil {
			t.Errorf("expected nil bytes for nil input, got %v", got)
		}
	})
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
