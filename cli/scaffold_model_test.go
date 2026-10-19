/**
 * @file scaffold_model_test.go
 * @package cli
 * @author Dr.NP <np@herewe.tech>
 * @since 10/19/2026
 */

package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

// TestNewWithModels drills the whole model path: the three per-resource files,
// the merged request/response declarations, the bun config block and the
// main.go wiring that only exists when a model was asked for.
func TestNewWithModels(t *testing.T) {
	dir := scaffoldProjectDir(t, "withmodels", "--http", "--fiber=false", "--no-grpc", "--model", "key,cabinet")

	for _, name := range []string{"model/key.go", "service/key.go", "handler/key.http.go", "model/cabinet.go"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}

	cfg := readJSON(t, filepath.Join(dir, "config.json"))
	sickyBlock := cfg["sicky"].(map[string]any)
	infra := sickyBlock["infra"].(map[string]any)
	if _, ok := infra["bun"].(map[string]any); !ok {
		t.Error("config.json has no sicky.infra.bun block")
	}

	mainSrc := readFile(t, filepath.Join(dir, "main.go"))
	for _, want := range []string{
		`MustInfra: []string{"bun"}`,
		`Flag:     "initdb"`,
		"CREATE EXTENSION IF NOT EXISTS",
		"(model.Key{}).InitDB(ctx, false)",
		"(model.Cabinet{}).InitDB(ctx, false)",
		"handler.NewHTTPKey()",
		"handler.NewHTTPCabinet()",
	} {
		if !strings.Contains(mainSrc, want) {
			t.Errorf("main.go missing %q", want)
		}
	}

	if !strings.Contains(readFile(t, filepath.Join(dir, "handler", "key.http.go")), ".Use(validateSession)") {
		t.Error("a model handler in an auth-enabled project must mount validateSession")
	}

	reqSrc := readFile(t, filepath.Join(dir, "handler", "request", "http.go"))
	for _, want := range []string{"HttpKeyPostCreate", "HttpKeyPutUpdate", "HttpCabinetPostCreate", "HttpCabinetPutUpdate"} {
		if !strings.Contains(reqSrc, want) {
			t.Errorf("handler/request/http.go missing merged declaration %q", want)
		}
	}

	respSrc := readFile(t, filepath.Join(dir, "handler", "response", "http.go"))
	for _, want := range []string{"type HttpKey struct", "type HttpCabinet struct"} {
		if !strings.Contains(respSrc, want) {
			t.Errorf("handler/response/http.go missing merged declaration %q", want)
		}
	}

	// Generated code must check every error: discarding RowsAffected's
	// error reports a successful write as "no rows affected".
	if strings.Contains(readFile(t, filepath.Join(dir, "model", "key.go")), "n, _ := r.RowsAffected()") {
		t.Error("generated model discards the RowsAffected error")
	}
}

// A project built with --no-auth has no validateSession to mount, so the
// model handler must not name it; naming it would not compile.
func TestNewModelNoAuthNoMiddleware(t *testing.T) {
	dir := scaffoldProjectDir(t, "modelnoauth", "--http", "--fiber=false", "--no-grpc", "--no-auth", "--model", "key")

	if strings.Contains(readFile(t, filepath.Join(dir, "handler", "key.http.go")), ".Use(validateSession)") {
		t.Error("--no-auth must leave model handlers without the middleware")
	}

	if strings.Contains(readFile(t, filepath.Join(dir, "handler", "http.go")), "func validateSession") {
		t.Error("--no-auth must not define validateSession")
	}
}

// Models are HTTP resources. Rejecting the combination before anything is
// written keeps a rejected run from leaving a half-built project behind.
func TestNewModelRequiresHTTP(t *testing.T) {
	dir := t.TempDir()
	out, err := exec.Command(scaffoldBinary(t), "new", "nodb", "--module", "example.com/nodb",
		"--type", "standard", "--fiber", "--no-grpc", "--model", "key", "-o", dir).CombinedOutput()
	if err == nil {
		t.Fatal("--model without HTTP must fail")
	}

	if !strings.Contains(string(out), "--model requires an HTTP server") {
		t.Errorf("error does not explain the requirement:\n%s", out)
	}

	if _, statErr := os.Stat(filepath.Join(dir, "nodb")); statErr == nil {
		t.Error("a rejected scaffold must not leave a project behind")
	}
}

// auth is generated as its own handler/service pair; a model of the same
// name would redeclare them.
func TestNewRejectsAuthModel(t *testing.T) {
	dir := t.TempDir()
	out, err := exec.Command(scaffoldBinary(t), "new", "badmodel", "--module", "example.com/badmodel",
		"--type", "standard", "--http", "--fiber=false", "--no-grpc", "--model", "auth", "-o", dir).CombinedOutput()
	if err == nil {
		t.Fatal("model name auth must be rejected")
	}

	if !strings.Contains(string(out), "reserved") {
		t.Errorf("error does not explain the collision:\n%s", out)
	}
}

// Models are generated from the standard layering only: mcp and interactive
// projects have no handler/request|response files and no validateSession, so
// the generated resource handler would not compile.
func TestNewModelRequiresStandardType(t *testing.T) {
	dir := t.TempDir()
	out, err := exec.Command(scaffoldBinary(t), "new", "nodemcp", "--module", "example.com/nodemcp",
		"--type", "mcp", "--http", "--fiber=false", "--no-grpc", "--model", "key", "-o", dir).CombinedOutput()
	if err == nil {
		t.Fatal("--model with a non-standard type must fail")
	}

	if !strings.Contains(string(out), "--model requires --type standard") {
		t.Errorf("error does not explain the requirement:\n%s", out)
	}

	if _, statErr := os.Stat(filepath.Join(dir, "nodemcp")); statErr == nil {
		t.Error("a rejected scaffold must not leave a project behind")
	}
}

// http is the base handler's name: a model called http would generate
// HTTPHttp/NewHTTPHttp and redeclare it.
func TestNewRejectsHTTPModel(t *testing.T) {
	dir := t.TempDir()
	out, err := exec.Command(scaffoldBinary(t), "new", "badmodelhttp", "--module", "example.com/badmodelhttp",
		"--type", "standard", "--http", "--fiber=false", "--no-grpc", "--model", "http", "-o", dir).CombinedOutput()
	if err == nil {
		t.Fatal("model name http must be rejected")
	}

	if !strings.Contains(string(out), "reserved") {
		t.Errorf("error does not explain the collision:\n%s", out)
	}
}

// Two names that normalize to the same Go identifier or table would write the
// same declarations twice, which cannot compile.
func TestNewRejectsDuplicateModels(t *testing.T) {
	for _, models := range []string{"key,Key", "my-key,my_key", "my.key,my_key", "AB,Ab"} {
		t.Run(models, func(t *testing.T) {
			dir := t.TempDir()
			out, err := exec.Command(scaffoldBinary(t), "new", "dupmodel", "--module", "example.com/dupmodel",
				"--type", "standard", "--http", "--fiber=false", "--no-grpc", "--model", models, "-o", dir).CombinedOutput()
			if err == nil {
				t.Fatalf("duplicate models %q must fail", models)
			}

			if !strings.Contains(string(out), "collides") {
				t.Errorf("error does not explain the collision:\n%s", out)
			}

			if _, statErr := os.Stat(filepath.Join(dir, "dupmodel")); statErr == nil {
				t.Error("a rejected scaffold must not leave a project behind")
			}
		})
	}
}

// The generated project's go.mod carries no require lines, and `go build`
// runs in -mod=readonly: without a tidy step `make build` fails on a fresh
// project for the one reason the scaffold can prevent.
func TestScaffoldMakefileTidies(t *testing.T) {
	dir := scaffoldProjectDir(t, "tidy", "--http", "--fiber=false", "--no-grpc")
	makefile := readFile(t, filepath.Join(dir, "Makefile"))

	for _, want := range []string{"deps:", "$(GO) mod tidy", "all: summary proto deps fmt build"} {
		if !strings.Contains(makefile, want) {
			t.Errorf("Makefile missing %q", want)
		}
	}
}

// The standard-library uuid package arrived in Go 1.27, which is why the
// generated models import "uuid" rather than a third-party module.
func TestScaffoldGoDirective(t *testing.T) {
	dir := scaffoldProjectDir(t, "goversion", "--http", "--fiber=false", "--no-grpc")
	if !strings.Contains(readFile(t, filepath.Join(dir, "go.mod")), "go 1.27.0") {
		t.Error("generated go.mod must target Go 1.27")
	}
}

// Generated model methods log a constant lowercase message and carry the
// error in a structured field. Passing err.Error() as the message breaks the
// framework's lowercase-message rule and makes the log text depend on the
// driver's wording.
func TestGeneratedModelLogsConstantMessages(t *testing.T) {
	dir := scaffoldProjectDir(t, "modellogs", "--http", "--fiber=false", "--no-grpc", "--model", "key")

	src := readFile(t, filepath.Join(dir, "model", "key.go"))
	if strings.Contains(src, "ErrorContext(ctx, err.Error())") {
		t.Error("generated model passes err.Error() as the log message")
	}

	if !strings.Contains(src, `ErrorContext(ctx, "create model failed", "error", err)`) {
		t.Error("generated model does not log a constant message with a structured error")
	}

	const call = `ErrorContext(ctx, "`

	for line := range strings.SplitSeq(src, "\n") {
		_, rest, ok := strings.Cut(line, call)
		if !ok {
			continue
		}

		end := strings.Index(rest, `"`)
		if end <= 0 {
			t.Errorf("error log message is not a string literal: %s", strings.TrimSpace(line))

			continue
		}

		message := rest[:end]

		if first, _ := utf8.DecodeRuneInString(message); unicode.IsUpper(first) {
			t.Errorf("error log message %q does not start lowercase", message)
		}
	}
}
