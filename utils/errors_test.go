package utils

import (
	"errors"
	"testing"
)

func TestErrorCodeRegistry(t *testing.T) {
	if MessageOf(CodeNotFound) != "not found" {
		t.Fatalf("builtin message = %q", MessageOf(CodeNotFound))
	}

	if StatusOf(CodeNotFound) != StatusNotFound {
		t.Fatalf("builtin status = %d", StatusOf(CodeNotFound))
	}

	if StatusOf(999999) != StatusInternalServerError {
		t.Fatal("unknown code must map to 500")
	}

	if err := RegisterErrorCode(40001, "order paid", StatusConflict); err != nil {
		t.Fatalf("register: %v", err)
	}

	if err := RegisterErrorCode(40001, "dup", StatusConflict); err == nil {
		t.Fatal("duplicate registration must fail")
	}

	if MessageOf(40001) != "order paid" {
		t.Fatal("custom message not stored")
	}
}

func TestCodedErrorChain(t *testing.T) {
	base := errors.New("db down")
	err := NewCodedError(CodeInternal, "", base)
	if CodeOf(err) != CodeInternal {
		t.Fatalf("CodeOf = %d", CodeOf(err))
	}

	if !errors.Is(err, base) {
		t.Fatal("CodedError must unwrap")
	}

	wrapped := &CodedError{Code: CodeNotFound}
	if CodeOf(wrapped) != CodeNotFound {
		t.Fatal("empty message must still carry code")
	}

	if CodeOf(errors.New("plain")) != CodeInternal {
		t.Fatal("plain error must map to internal")
	}
}

func TestEnvelopeT(t *testing.T) {
	ok := OkT(map[string]string{"k": "v"})
	if ok.Code != CodeOK || ok.Status != StatusOK || ok.Data["k"] != "v" {
		t.Fatalf("ok envelope: %+v", ok)
	}

	fail := FailT[string](CodeNotFound, "")
	if fail.Message != "not found" || fail.Status != StatusNotFound {
		t.Fatalf("fail envelope: %+v", fail)
	}

	fail.WithRequestID("r1").WithPagination(&Pagination{Total: 1})
	if fail.RequestID != "r1" || fail.Pagination.Total != 1 {
		t.Fatalf("chained envelope: %+v", fail)
	}
}
