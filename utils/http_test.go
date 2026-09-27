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
 * @file http_test.go
 * @package utils
 * @author Dr.NP <np@herewe.tech>
 * @since 07/07/2026
 */

package utils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWrapHTTPResponse(t *testing.T) {
	data := map[string]string{"key": "value"}
	e := WrapHTTPResponse(data)

	if e == nil {
		t.Fatal("WrapHTTPResponse returned nil")
	}

	if e.Code != CodeOK {
		t.Errorf("expected code %d, got %d", CodeOK, e.Code)
	}

	if e.Message != MsgOK {
		t.Errorf("expected message %q, got %q", MsgOK, e.Message)
	}

	if val, ok := e.Data.(map[string]string); !ok || val["key"] != "value" {
		t.Errorf("expected data map with key=value, got %v", e.Data)
	}

	if e.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestWrapHTTPResponseNilData(t *testing.T) {
	e := WrapHTTPResponse(nil)
	if e == nil {
		t.Fatal("WrapHTTPResponse returned nil for nil data")
	}

	if e.Data != nil {
		t.Errorf("expected nil data, got %v", e.Data)
	}
}

func TestEnvelopeSetCode(t *testing.T) {
	e := WrapHTTPResponse(nil)
	customCode := 1001
	result := e.SetCode(customCode)

	if result != e {
		t.Error("SetCode should return same envelope for chaining")
	}

	if e.Code != customCode {
		t.Errorf("expected code %d, got %d", customCode, e.Code)
	}
}

func TestEnvelopeSetStatus(t *testing.T) {
	e := WrapHTTPResponse(nil)
	customStatus := 404
	result := e.SetStatus(customStatus)

	if result != e {
		t.Error("SetStatus should return same envelope for chaining")
	}

	if e.Status != customStatus {
		t.Errorf("expected status %d, got %d", customStatus, e.Status)
	}
}

func TestEnvelopeSetMessage(t *testing.T) {
	e := WrapHTTPResponse(nil)
	customMsg := "Custom error message"
	result := e.SetMessage(customMsg)

	if result != e {
		t.Error("SetMessage should return same envelope for chaining")
	}

	if e.Message != customMsg {
		t.Errorf("expected message %q, got %q", customMsg, e.Message)
	}
}

func TestEnvelopeSetRequestID(t *testing.T) {
	e := WrapHTTPResponse(nil)
	rid := "req-12345"
	result := e.SetRequestID(rid)

	if result != e {
		t.Error("SetRequestID should return same envelope for chaining")
	}

	if e.RequestID != rid {
		t.Errorf("expected request_id %q, got %q", rid, e.RequestID)
	}
}

func TestEnvelopeSetData(t *testing.T) {
	e := WrapHTTPResponse("old")
	newData := 42
	result := e.SetData(newData)

	if result != e {
		t.Error("SetData should return same envelope for chaining")
	}

	if e.Data != 42 {
		t.Errorf("expected data 42, got %v", e.Data)
	}
}

func TestEnvelopeChainedBuild(t *testing.T) {
	data := struct{ Name string }{"test"}
	e := WrapHTTPResponse(data).
		SetCode(2000).
		SetStatus(400).
		SetMessage("Bad Request").
		SetRequestID("abc-123")

	if e.Code != 2000 {
		t.Errorf("expected code 2000, got %d", e.Code)
	}

	if e.Status != 400 {
		t.Errorf("expected status 400, got %d", e.Status)
	}

	if e.Message != "Bad Request" {
		t.Errorf("expected message 'Bad Request', got %q", e.Message)
	}

	if e.RequestID != "abc-123" {
		t.Errorf("expected request_id 'abc-123', got %q", e.RequestID)
	}

	if e.Data != data {
		t.Error("data should be preserved through chaining")
	}
}

func TestPagination(t *testing.T) {
	p := &Pagination{
		Total:   100,
		Limit:   10,
		Offset:  20,
		Current: 3,
		Pages:   10,
	}

	if p.Total != 100 {
		t.Errorf("expected Total=100, got %d", p.Total)
	}

	if p.Limit != 10 {
		t.Errorf("expected Limit=10, got %d", p.Limit)
	}

	if p.Offset != 20 {
		t.Errorf("expected Offset=20, got %d", p.Offset)
	}

	if p.Current != 3 {
		t.Errorf("expected Current=3, got %d", p.Current)
	}

	if p.Pages != 10 {
		t.Errorf("expected Pages=10, got %d", p.Pages)
	}
}

// TestAutoFormatDataNegotiation: ordinary Accept headers (parameters,
// quality values, wildcards) must match instead of falling through to
// the plain-text branch - and no branch may print unexported fields,
// which fmt.Sprintf("%v") used to do.
func TestAutoFormatDataNegotiation(t *testing.T) {
	type payload struct {
		Name     string `json:"name"`
		password string
	}

	data := &payload{Name: "app", password: "hunter2"}

	newReq := func(accept string) *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		if accept != "" {
			req.Header.Set("Accept", accept)
		}

		return req
	}

	for _, accept := range []string{
		"application/json",
		"application/json; charset=utf-8",
		"application/json;q=0.9, application/xml;q=0.8",
		"",
		"*/*",
		"text/html,application/xhtml+xml,*/*;q=0.8",
	} {
		ct, body, err := AutoFormatData(newReq(accept), data)
		if err != nil {
			t.Fatalf("Accept %q: %v", accept, err)
		}

		if ct != "application/json" {
			t.Fatalf("Accept %q -> content type %q, want application/json", accept, ct)
		}

		if !json.Valid(body) {
			t.Fatalf("Accept %q -> body is not JSON: %q", accept, body)
		}

		if strings.Contains(string(body), "hunter2") {
			t.Fatalf("Accept %q leaked an unexported field: %s", accept, body)
		}
	}

	if ct, _, err := AutoFormatData(newReq("application/xml"), data); err != nil || ct != "application/xml" {
		t.Fatalf("xml: ct = %q err = %v", ct, err)
	}

	if ct, _, err := AutoFormatData(newReq("application/x-yaml"), data); err != nil || ct != "application/x-yaml" {
		t.Fatalf("yaml: ct = %q err = %v", ct, err)
	}

	// Explicit plain text keeps scalars verbatim...
	if ct, body, err := AutoFormatData(newReq("text/plain"), "hello"); err != nil || ct != "text/plain; charset=utf-8" || string(body) != "hello" {
		t.Fatalf("plain string: ct = %q body = %q err = %v", ct, body, err)
	}

	// ...but structured payloads go through JSON, never %v.
	if ct, body, err := AutoFormatData(newReq("text/plain"), data); err != nil || ct != "application/json" {
		t.Fatalf("plain struct: ct = %q err = %v", ct, err)
	} else if strings.Contains(string(body), "hunter2") {
		t.Fatalf("text/plain leaked an unexported field: %s", body)
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
