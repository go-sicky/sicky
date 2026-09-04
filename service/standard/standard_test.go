package standard

import (
	"testing"

	"github.com/go-sicky/sicky/service"
	"github.com/google/uuid"
)

func TestStandardEmptyStartStop(t *testing.T) {
	service.Clear()
	svc := New(&service.Options{ID: uuid.New(), Name: "std-test"}, &Config{})
	if svc == nil {
		t.Fatal("New must succeed")
	}
	if errs := svc.Start(); len(errs) != 0 {
		t.Fatalf("empty Start: %v", errs)
	}
	if errs := svc.Stop(); len(errs) != 0 {
		t.Fatalf("empty Stop: %v", errs)
	}
	if service.Default() != svc {
		t.Fatal("first service must be Default")
	}
	service.Clear()
}
