/**
 * @file generate_model_test.go
 * @package cli
 * @author Dr.NP <np@herewe.tech>
 * @since 10/19/2026
 */

package cli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runGenerate runs one `sicky generate` command inside dir and fails the test
// when the command rejects work the scaffold should accept.
func runGenerate(t *testing.T, dir string, args ...string) string {
	t.Helper()

	out, code := runGenerateRaw(t, dir, args...)
	if code != 0 {
		t.Fatalf("sicky generate %s: exit %d\n%s", strings.Join(args, " "), code, out)
	}

	return out
}

// runGenerateRaw runs one `sicky generate` command inside dir and returns its
// combined output and exit code. It is the form the rejection tests need: they
// assert on the message and the status, not on a Go error.
func runGenerateRaw(t *testing.T, dir string, args ...string) (out string, code int) {
	t.Helper()

	cmd := exec.Command(scaffoldBinary(t), append([]string{"generate"}, args...)...)
	cmd.Dir = dir

	combined, err := cmd.CombinedOutput()
	if err == nil {
		return string(combined), 0
	}

	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return string(combined), exitErr.ExitCode()
	}

	t.Fatalf("run sicky generate %s: %v\n%s", strings.Join(args, " "), err, combined)

	return "", 0
}

func writeTempIn(t *testing.T, dir, name, body string) {
	t.Helper()

	writeFile(t, filepath.Join(dir, name), body)
}

// TestGenerateModelWiresProject drills the whole command: the three resource
// files, the bun config block, and every main.go change. The second run with
// --force pins the repeat semantics: resource files are rewritten, aggregated
// DTO declarations are not duplicated, and the wiring stays deduplicated.
func TestGenerateModelWiresProject(t *testing.T) {
	dir := scaffoldProjectDir(t, "genmodel", "--http", "--fiber=false", "--no-grpc")
	runGenerate(t, dir, "model", "train")

	for _, name := range []string{"model/train.go", "service/train.go", "handler/train.http.go"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}

	cfg := readJSON(t, filepath.Join(dir, "config.json"))
	if _, ok := cfg["sicky"].(map[string]any)["infra"].(map[string]any)["bun"]; !ok {
		t.Error("generate model did not add sicky.infra.bun")
	}

	mainSrc := readFile(t, filepath.Join(dir, "main.go"))
	for _, want := range []string{"(model.Train{}).InitDB(ctx, false)", "handler.NewHTTPTrain()", `MustInfra: []string{"bun"}`} {
		if !strings.Contains(mainSrc, want) {
			t.Errorf("main.go missing %q", want)
		}
	}

	if !strings.Contains(readFile(t, filepath.Join(dir, "handler", "train.http.go")), ".Use(validateSession)") {
		t.Error("a model handler in an auth-enabled project must mount validateSession")
	}

	// --force rerun must not duplicate DTO declarations.
	runGenerate(t, dir, "model", "train", "--force")
	if got := strings.Count(readFile(t, filepath.Join(dir, "handler", "response", "http.go")), "type HttpTrain struct"); got != 1 {
		t.Errorf("HttpTrain declared %d times after --force, want 1", got)
	}

	if got := strings.Count(readFile(t, filepath.Join(dir, "main.go")), "handler.NewHTTPTrain()"); got != 1 {
		t.Errorf("handler registration appears %d times after --force, want 1", got)
	}
}

// TestGenerateModelNoWire: the files are the whole command. config.json and
// main.go stay byte-identical, and the manual steps are printed instead.
func TestGenerateModelNoWire(t *testing.T) {
	dir := scaffoldProjectDir(t, "nowire", "--http", "--fiber=false", "--no-grpc")
	cfgBefore := readFile(t, filepath.Join(dir, "config.json"))
	mainBefore := readFile(t, filepath.Join(dir, "main.go"))

	out := runGenerate(t, dir, "model", "train", "--no-wire")

	if readFile(t, filepath.Join(dir, "config.json")) != cfgBefore {
		t.Error("--no-wire changed config.json")
	}

	if readFile(t, filepath.Join(dir, "main.go")) != mainBefore {
		t.Error("--no-wire changed main.go")
	}

	for _, want := range []string{"infra.bun", "MustInfra", "initdb", "handler.NewHTTPTrain()"} {
		if !strings.Contains(out, want) {
			t.Errorf("--no-wire output does not mention %q:\n%s", want, out)
		}
	}
}

// TestGenerateModelOutsideLayering: a project without the HTTP request and
// response aggregation files cannot receive a model DTO. The command must
// refuse before writing anything, and the message must name the file that is
// missing so the user knows what to create.
func TestGenerateModelOutsideLayering(t *testing.T) {
	dir := t.TempDir()
	writeTempIn(t, dir, "go.mod", "module example.com/old\n\ngo 1.27.0\n")
	writeTempIn(t, dir, "main.go", "package main\n\nfunc main() {}\n")

	out, code := runGenerateRaw(t, dir, "model", "key")
	if code == 0 {
		t.Fatal("generate model must reject a project without the HTTP layering")
	}

	if !strings.Contains(out, "handler/request/http.go") {
		t.Errorf("error does not name the missing file:\n%s", out)
	}

	if _, err := os.Stat(filepath.Join(dir, "model", "key.go")); err == nil {
		t.Error("a rejected generate model must not write files")
	}
}

// TestGenerateModelRejectsAuthName: the scaffold already generates an auth
// handler and service, so a model with that name would redeclare them.
func TestGenerateModelRejectsAuthName(t *testing.T) {
	dir := scaffoldProjectDir(t, "genauth", "--http", "--fiber=false", "--no-grpc")
	out, code := runGenerateRaw(t, dir, "model", "auth")
	if code == 0 {
		t.Fatal("model name auth must be rejected")
	}

	if !strings.Contains(out, "reserved") {
		t.Errorf("error does not explain the collision:\n%s", out)
	}
}

// TestGenerateModelMissingAnchors: a main.go the wiring cannot understand is a
// preflight failure. The scaffold's own main.go is edited in place to remove
// the sicky.Init call, and the command must name the missing anchor without
// leaving model/train.go behind.
func TestGenerateModelMissingAnchors(t *testing.T) {
	dir := scaffoldProjectDir(t, "genanchors", "--http", "--fiber=false", "--no-grpc")

	mainPath := filepath.Join(dir, "main.go")
	mainSrc := strings.ReplaceAll(readFile(t, mainPath), "sicky.Init(", "sicky.InitBogus(")
	writeFile(t, mainPath, mainSrc)

	out, code := runGenerateRaw(t, dir, "model", "train")
	if code == 0 {
		t.Fatal("generate model must reject a main.go without sicky.Init")
	}

	if !strings.Contains(out, "sicky.Init") || !strings.Contains(out, "does not look like a sicky scaffold") {
		t.Errorf("error does not name the missing anchor:\n%s", out)
	}

	if _, err := os.Stat(filepath.Join(dir, "model", "train.go")); err == nil {
		t.Error("a rejected generate model must not write files")
	}
}

// TestGenerateModelNoAuth: --no-auth projects have no validateSession, so the
// resource handler must not name it. The wiring itself is unaffected.
func TestGenerateModelNoAuth(t *testing.T) {
	dir := scaffoldProjectDir(t, "gennoauth", "--http", "--fiber=false", "--no-grpc", "--no-auth")
	runGenerate(t, dir, "model", "train")

	if strings.Contains(readFile(t, filepath.Join(dir, "handler", "train.http.go")), ".Use(validateSession)") {
		t.Error("a --no-auth project must not mount a middleware it does not define")
	}

	if !strings.Contains(readFile(t, filepath.Join(dir, "main.go")), "handler.NewHTTPTrain()") {
		t.Error("a --no-auth project must still register the resource handler")
	}
}

// TestGenerateModelRefusesExistingWithoutForce: the repeat semantics without
// --force are all-or-nothing. Nothing is written and nothing is wired, which
// is why the failure message has to say how to proceed.
func TestGenerateModelRefusesExistingWithoutForce(t *testing.T) {
	dir := scaffoldProjectDir(t, "genexists", "--http", "--fiber=false", "--no-grpc")
	runGenerate(t, dir, "model", "train")

	mainBefore := readFile(t, filepath.Join(dir, "main.go"))
	cfgBefore := readFile(t, filepath.Join(dir, "config.json"))

	out, code := runGenerateRaw(t, dir, "model", "train")
	if code == 0 {
		t.Fatal("a second generate model without --force must fail")
	}

	if !strings.Contains(out, "exists (use --force)") {
		t.Errorf("error does not explain the overwrite flag:\n%s", out)
	}

	if readFile(t, filepath.Join(dir, "main.go")) != mainBefore {
		t.Error("a refused run must not rewire main.go")
	}

	if readFile(t, filepath.Join(dir, "config.json")) != cfgBefore {
		t.Error("a refused run must not rewire config.json")
	}
}

// TestGenerateModelMalformedConfigWritesNothing is the preflight contract: a
// config.json the wiring cannot read must fail before the resource files and
// DTO declarations are written. The scaffolding order used to write first, so
// this test fails if the plan step ever moves back after an apply.
func TestGenerateModelMalformedConfigWritesNothing(t *testing.T) {
	dir := scaffoldProjectDir(t, "genbadconfig", "--http", "--fiber=false", "--no-grpc")
	writeFile(t, filepath.Join(dir, "config.json"), "{ not json\n")

	mainBefore := readFile(t, filepath.Join(dir, "main.go"))

	out, code := runGenerateRaw(t, dir, "model", "train")
	if code == 0 {
		t.Fatal("generate model must reject a malformed config.json")
	}

	if !strings.Contains(out, "config.json") {
		t.Errorf("error does not name config.json:\n%s", out)
	}

	for _, name := range []string{"model/train.go", "service/train.go", "handler/train.http.go"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name))); err == nil {
			t.Errorf("a rejected wiring must not write %s", name)
		}
	}

	if readFile(t, filepath.Join(dir, "main.go")) != mainBefore {
		t.Error("a rejected wiring must not touch main.go")
	}
}

// TestGenerateModelAliasedImportWritesNothing covers the main.go checks that
// only wireMainGo can see: a foreign alias on a generated import cannot be
// safely resolved, so the command must reject it before writing any file.
func TestGenerateModelAliasedImportWritesNothing(t *testing.T) {
	dir := scaffoldProjectDir(t, "genalias", "--http", "--fiber=false", "--no-grpc")

	mainPath := filepath.Join(dir, "main.go")
	mainSrc := strings.Replace(
		readFile(t, mainPath),
		`"example.com/genalias/handler"`,
		`h "example.com/genalias/handler"`,
		1,
	)
	writeFile(t, mainPath, mainSrc)

	out, code := runGenerateRaw(t, dir, "model", "train")
	if code == 0 {
		t.Fatal("generate model must reject a foreign-aliased handler import")
	}

	if !strings.Contains(out, "imported as h") {
		t.Errorf("error does not name the alias:\n%s", out)
	}

	if _, err := os.Stat(filepath.Join(dir, "model", "train.go")); err == nil {
		t.Error("a rejected wiring must not write resource files")
	}
}

// TestGenerateModelRejectsOutputTraversal: -o ../x must be rejected by the
// same path guard as every generate schematic, before any directory is
// created.
func TestGenerateModelRejectsOutputTraversal(t *testing.T) {
	dir := scaffoldProjectDir(t, "gentraversal", "--http", "--fiber=false", "--no-grpc")

	out, code := runGenerateRaw(t, dir, "model", "train", "--output", "../escaped")
	if code == 0 {
		t.Fatal("generate model must reject an output root containing ..")
	}

	if !strings.Contains(out, "..") {
		t.Errorf("error does not explain the rejected path:\n%s", out)
	}

	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "escaped")); err == nil {
		t.Error("a rejected output root must not be created")
	}

	if _, err := os.Stat(filepath.Join(dir, "model", "train.go")); err == nil {
		t.Error("a rejected output root must not write into the project")
	}
}

// TestGenerateModelUsesTargetProjectIdentity: with --output the generated
// headers must describe the target project, not the directory the command was
// run from. Without the root-aware identity lookup, the caller's license and
// git identity leak into the target's files.
func TestGenerateModelUsesTargetProjectIdentity(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed; identity detection cannot be exercised")
	}

	target := scaffoldProjectDir(t, "genidentity", "--http", "--fiber=false", "--no-grpc", "--license", "Apache-2.0")

	for _, kv := range [][2]string{{"user.name", "Target Holder"}, {"user.email", "target@example.com"}} {
		cmd := exec.Command("git", "-C", target, "config", kv[0], kv[1])
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("set target git config: %v\n%s", err, out)
		}
	}

	// A caller directory outside the target repository: its own identity must
	// not be used for the generated files.
	caller := t.TempDir()
	runGenerate(t, caller, "model", "train", "--output", target)

	src := readFile(t, filepath.Join(target, "model", "train.go"))
	for _, want := range []string{
		"SPDX-License-Identifier: Apache-2.0",
		"Copyright (c) ",
		"Target Holder",
		"@author Target Holder <target@example.com>",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("generated model does not carry the target identity %q:\n%s", want, src[:400])
		}
	}
}
