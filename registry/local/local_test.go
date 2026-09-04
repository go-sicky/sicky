package local

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-sicky/sicky/registry"
	"github.com/google/uuid"
)

func TestValidateRejectsRelativePath(t *testing.T) {
	cfg := (&Config{RegistryFilePath: "relative/dir"}).Ensure()
	if err := cfg.Validate(); !errors.Is(err, ErrLocalPathNotAbsolute) {
		t.Fatalf("relative path must fail validation, got %v", err)
	}
	if got := New(&registry.Options{}, cfg); got != nil {
		t.Fatal("New with relative path must return nil")
	}
}

func TestCleanupKeepsNonUUIDFiles(t *testing.T) {
	dir := t.TempDir()
	keep := filepath.Join(dir, "notes.json")
	if err := os.WriteFile(keep, []byte("{}"), 0o600); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	stale := filepath.Join(dir, uuid.New().String()+".json")
	if err := os.WriteFile(stale, []byte("{}"), 0o600); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	rg := New(&registry.Options{}, &Config{RegistryFilePath: dir, CleanupOnStart: true})
	if rg == nil {
		t.Fatal("New must succeed on absolute tmp path")
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("uuid-named stale file must be removed")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatal("non-uuid file must be preserved")
	}
}

func TestRegisterLoadDeregisterRoundTrip(t *testing.T) {
	dir := t.TempDir()
	rg := New(&registry.Options{}, &Config{RegistryFilePath: dir})
	if rg == nil {
		t.Fatal("New must succeed")
	}

	ins := &registry.Instance{ID: uuid.New(), ServiceName: "svc"}
	if err := rg.Register(ins); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if !rg.CheckInstance(ins.ID) {
		t.Fatal("CheckInstance must find registered instance")
	}

	loaded, err := rg.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	found := false
	for _, in := range loaded {
		if in != nil && in.ID == ins.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("Load must return registered instance, got %d", len(loaded))
	}

	if err := rg.Deregister(ins.ID); err != nil {
		t.Fatalf("Deregister: %v", err)
	}
	if rg.CheckInstance(ins.ID) {
		t.Fatal("CheckInstance must miss deregistered instance")
	}
}
