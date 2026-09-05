package registry

import (
	"testing"

	"github.com/google/uuid"
)

// Helpers without a default registry must be safe no-ops so the
// orchestrator can call them unconditionally.
func TestHelpersNilSafeWithoutDefault(t *testing.T) {
	Clear()
	if Default() != nil {
		t.Fatal("Default must be nil after Clear")
	}

	if err := Register(&Instance{ID: uuid.New(), ServiceName: "svc"}); err != nil {
		t.Fatalf("Register without default: %v", err)
	}

	if err := Deregister(uuid.New()); err != nil {
		t.Fatalf("Deregister without default: %v", err)
	}

	if Watch() != nil {
		t.Fatal("Watch without default must return nil")
	}

	if Stop() != nil {
		t.Fatal("Stop without default must return nil")
	}
}
