package local

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/go-sicky/sicky/registry"
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

// TestDefaultRegistryDirHonorsXDG: the shared temporary root is
// world-writable, so the per-user runtime directory is preferred when
// available.
func TestDefaultRegistryDirHonorsXDG(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/4242")
	if got := defaultRegistryDir(); got != "/run/user/4242/sicky/registry" {
		t.Fatalf("defaultRegistryDir() = %q", got)
	}

	t.Setenv("XDG_RUNTIME_DIR", "")
	if got := defaultRegistryDir(); got != DefaultRegistryFilePath {
		t.Fatalf("fallback = %q, want %q", got, DefaultRegistryFilePath)
	}

	t.Setenv("XDG_RUNTIME_DIR", "relative/dir")
	if got := defaultRegistryDir(); got != DefaultRegistryFilePath {
		t.Fatalf("non-absolute XDG_RUNTIME_DIR must not be used: %q", got)
	}
}

// TestValidateRejectsWorldWritableDir: any local user could replace
// <uuid>.json in such a directory and redirect service discovery.
func TestValidateRejectsWorldWritableDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "registry")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	// Mkdir applies the umask, so chmod the world-writable mode in.
	if err := os.Chmod(dir, 0o777); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	cfg := (&Config{RegistryFilePath: dir}).Ensure()
	if err := cfg.Validate(); !errors.Is(err, ErrLocalPathWorldWritable) {
		t.Fatalf("Validate() = %v, want ErrLocalPathWorldWritable", err)
	}

	if rg := New(&registry.Options{}, cfg); rg != nil {
		t.Fatal("New must return nil for a world-writable directory")
	}
}

// TestValidateRejectsSymlinkedDir: writing through a symlink hands the
// registry to whoever owns the target.
func TestValidateRejectsSymlinkedDir(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "target-dir")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	link := filepath.Join(parent, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	cfg := (&Config{RegistryFilePath: link}).Ensure()
	if err := cfg.Validate(); !errors.Is(err, ErrLocalPathSymlink) {
		t.Fatalf("Validate() = %v, want ErrLocalPathSymlink", err)
	}
}

// TestRegisterRefusesSymlinkedFile: os.WriteFile follows symlinks, so a
// planted <uuid>.json would be overwritten in place.
func TestRegisterRefusesSymlinkedFile(t *testing.T) {
	dir := t.TempDir()
	rg := New(&registry.Options{}, &Config{RegistryFilePath: dir})
	if rg == nil {
		t.Fatal("New must succeed")
	}

	ins := &registry.Instance{ID: uuid.New(), ServiceName: "svc"}
	target := filepath.Join(dir, "target.json")
	if err := os.WriteFile(target, []byte("{}"), 0o600); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	if err := os.Symlink(target, filepath.Join(dir, ins.ID.String()+".json")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if err := rg.Register(ins); !errors.Is(err, ErrLocalPathSymlink) {
		t.Fatalf("Register() = %v, want ErrLocalPathSymlink", err)
	}

	// The planted target must be untouched.
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "{}" {
		t.Fatalf("symlink target was written through: %q (%v)", data, err)
	}
}

// TestLoadSkipsSymlinkedFile: a link to an arbitrary host file must not
// be read into the discovery pool.
func TestLoadSkipsSymlinkedFile(t *testing.T) {
	dir := t.TempDir()
	rg := New(&registry.Options{}, &Config{RegistryFilePath: dir})
	if rg == nil {
		t.Fatal("New must succeed")
	}

	outside := filepath.Join(t.TempDir(), "outside.json")
	payload := `{"id":"00000000-0000-0000-0000-000000000000","service_name":"evil"}`
	if err := os.WriteFile(outside, []byte(payload), 0o600); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	if err := os.Symlink(outside, filepath.Join(dir, uuid.New().String()+".json")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	loaded, err := rg.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(loaded) != 0 {
		t.Fatalf("symlinked file was loaded: %d instances", len(loaded))
	}
}

// TestLoadSkipsOversizedFile: Load reads every *.json in the directory,
// so one planted giant file must not be pulled into memory whole.
func TestLoadSkipsOversizedFile(t *testing.T) {
	dir := t.TempDir()
	rg := New(&registry.Options{}, &Config{RegistryFilePath: dir})
	if rg == nil {
		t.Fatal("New must succeed")
	}

	oversized := filepath.Join(dir, uuid.New().String()+".json")
	if err := os.WriteFile(oversized, []byte(strings.Repeat("a", maxInstanceBytes+1024)), 0o600); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	loaded, err := rg.Load()
	if err != nil {
		t.Fatalf("Load must survive an oversized file: %v", err)
	}

	if len(loaded) != 0 {
		t.Fatalf("oversized file was loaded: %d instances", len(loaded))
	}

	if err := os.Remove(oversized); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
}

// TestReadInstanceFileCapsSize exercises the reader directly.
func TestReadInstanceFileCapsSize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "small.json")
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	data, err := readInstanceFile(path)
	if err != nil || string(data) != "{}" {
		t.Fatalf("readInstanceFile = %q (%v)", data, err)
	}

	big := filepath.Join(dir, "big.json")
	if err := os.WriteFile(big, make([]byte, maxInstanceBytes+1), 0o600); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	if _, err := readInstanceFile(big); !errors.Is(err, ErrLocalInstanceTooLarge) {
		t.Fatalf("oversized read = %v, want ErrLocalInstanceTooLarge", err)
	}

	if _, err := readInstanceFile(filepath.Join(dir, "missing.json")); err == nil || errors.Is(err, io.EOF) {
		t.Fatalf("missing file error = %v, want a not-exist error", err)
	}
}

// TestWatchIsIdempotent: a second Watch built another fsnotify watcher
// and orphaned the first one together with its goroutine.
func TestWatchIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	rg := New(&registry.Options{}, &Config{RegistryFilePath: dir})
	if rg == nil {
		t.Fatal("New must succeed")
	}

	if err := rg.Watch(); err != nil {
		t.Fatalf("first watch: %v", err)
	}

	rg.watchMu.Lock()
	first := rg.watcher
	rg.watchMu.Unlock()

	if first == nil {
		t.Fatal("watcher not created")
	}

	if err := rg.Watch(); err != nil {
		t.Fatalf("second watch: %v", err)
	}

	rg.watchMu.Lock()
	second := rg.watcher
	rg.watchMu.Unlock()

	if second != first {
		t.Fatal("second Watch replaced the watcher (the first one leaks)")
	}

	if err := rg.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}

	rg.watchMu.Lock()
	after := rg.watcher
	rg.watchMu.Unlock()

	if after != nil {
		t.Fatal("Stop must release the watcher")
	}

	// Stopping twice is a no-op, not a double close.
	if err := rg.Stop(); err != nil {
		t.Fatalf("second stop: %v", err)
	}
}
