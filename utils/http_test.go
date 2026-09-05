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

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
