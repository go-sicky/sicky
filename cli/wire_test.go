/**
 * @file wire_test.go
 * @package cli
 * @author Dr.NP <np@herewe.tech>
 * @since 10/19/2026
 */

package cli

import (
	"encoding/json"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const wireMainFixture = `package main

import (
	"context"
	"log"

	"github.com/go-sicky/sicky"
	srvHTTP "github.com/go-sicky/sicky/server/http"
	"github.com/go-sicky/sicky/service/standard"
	"example.com/demo/handler"
)

func main() {
	ctx := context.Background()

	if err := sicky.Init(&sicky.Options{AppName: "demo", Context: ctx}); err != nil {
		log.Fatalf("init: %v", err)
	}

	httpSrv := srvHTTP.New(nil, nil)
	httpSrv.Handle(handler.NewHTTPHttp())
}

func run() {}
`

// wireMainMultilineFixture mirrors the real scaffold template's call shapes:
// a multiline sicky.Init with a multiline Options literal, a multiline
// srvHTTP.New assignment, and a Run call. The one-line fixture above never
// reaches the multiline branches of the edit helpers.
const wireMainMultilineFixture = `package main

import (
	"context"
	"errors"
	"log"

	"example.com/demo/handler"
	"github.com/go-sicky/sicky"
	"github.com/go-sicky/sicky/server"
	srvHTTP "github.com/go-sicky/sicky/server/http"
	"github.com/go-sicky/sicky/service"
	"github.com/go-sicky/sicky/service/standard"
)

func main() {
	ctx := context.Background()

	// Sicky
	if err := sicky.Init(
		&sicky.Options{
			AppName: "demo",
			Context: ctx,
		},
	); err != nil {
		if errors.Is(err, sicky.ErrVersionShown) {
			return
		}

		log.Fatalf("sicky: init: %v", err)
	}

	// HTTP server
	httpSrv := srvHTTP.New(
		&server.Options{
			Name:    "demo@http",
			Context: ctx,
		},
		nil,
	)
	httpSrv.Handle(handler.NewHTTPHttp())

	// Service
	svc := standard.New(
		&service.Options{
			Name:    "demo",
			Context: ctx,
		},
		nil,
	)
	svc.Servers(
		httpSrv,
	)

	if err := sicky.Run(nil); err != nil {
		log.Fatalf("sicky: run: %v", err)
	}
}
`

func TestWireMainGoMultilineScaffoldShape(t *testing.T) {
	path := writeTemp(t, "main.go", wireMainMultilineFixture)

	if err := wireMainGo(path, "example.com/demo", "key", "Key"); err != nil {
		t.Fatalf("wireMainGo: %v", err)
	}

	src := readFile(t, path)

	if _, err := parser.ParseFile(token.NewFileSet(), path, []byte(src), 0); err != nil {
		t.Fatalf("wired main.go does not parse: %v\n%s", err, src)
	}

	formatted, err := format.Source([]byte(src))
	if err != nil {
		t.Fatalf("gofmt: %v", err)
	}

	if string(formatted) != src {
		t.Errorf("wired main.go is not gofmt-stable:\n%s", src)
	}

	for _, once := range []string{
		`"example.com/demo/model"`,
		`MustInfra: []string{"bun"}`,
		`Flag:     "initdb"`,
		"func initdb() error",
		"CREATE EXTENSION IF NOT EXISTS",
		"(model.Key{}).InitDB(ctx, false)",
		"handler.NewHTTPKey()",
	} {
		if got := strings.Count(src, once); got != 1 {
			t.Errorf("%q appears %d times, want 1:\n%s", once, got, src)
		}
	}

	if err := wireMainGo(path, "example.com/demo", "key", "Key"); err != nil {
		t.Fatalf("second wireMainGo: %v", err)
	}

	if readFile(t, path) != src {
		t.Error("wireMainGo is not idempotent on the multiline scaffold shape")
	}
}

func TestWireMainGoAddsAllAnchors(t *testing.T) {
	path := writeTemp(t, "main.go", wireMainFixture)

	if err := wireMainGo(path, "example.com/demo", "key", "Key"); err != nil {
		t.Fatalf("wireMainGo: %v", err)
	}

	src := readFile(t, path)
	for _, want := range []string{
		`"example.com/demo/model"`,
		`MustInfra: []string{"bun"}`,
		`Flag:     "initdb"`,
		"func initdb() error",
		"CREATE EXTENSION IF NOT EXISTS",
		"(model.Key{}).InitDB(ctx, false)",
		"handler.NewHTTPKey()",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("wired main.go missing %q:\n%s", want, src)
		}
	}

	// Idempotent: a second call changes nothing.
	first := src
	if err := wireMainGo(path, "example.com/demo", "key", "Key"); err != nil {
		t.Fatalf("second wireMainGo: %v", err)
	}

	if readFile(t, path) != first {
		t.Error("wireMainGo is not idempotent")
	}
}

// The import must come from the project's own go.mod, not from the model
// name: two projects generate the same Key under different module paths.
func TestWireMainGoUsesTheProjectModule(t *testing.T) {
	path := writeTemp(t, "main.go", wireMainFixture)

	if err := wireMainGo(path, "example.com/other", "key", "Key"); err != nil {
		t.Fatalf("wireMainGo: %v", err)
	}

	if !strings.Contains(readFile(t, path), `"example.com/other/model"`) {
		t.Errorf("the model import does not use the project module:\n%s", readFile(t, path))
	}
}

func TestWireMainGoMissingAnchorWritesNothing(t *testing.T) {
	path := writeTemp(t, "main.go", "package main\n\nfunc main() {}\n")
	before := readFile(t, path)

	err := wireMainGo(path, "example.com/demo", "key", "Key")
	if err == nil {
		t.Fatal("a main.go without sicky.Init must be rejected")
	}

	if !strings.Contains(err.Error(), "sicky.Init") {
		t.Errorf("error does not name the missing anchor: %v", err)
	}

	if readFile(t, path) != before {
		t.Error("a failed preflight must not modify main.go")
	}
}

// The generated code names the model package `model.`, so an alias on the
// import would be an undefined identifier. Rejecting it beats emitting code
// that cannot compile.
func TestWireMainGoRejectsAliasedModelImport(t *testing.T) {
	body := `package main

import (
	"context"

	m "example.com/demo/model"
	"github.com/go-sicky/sicky"
)

func main() {
	ctx := context.Background()

	if err := sicky.Init(&sicky.Options{Context: ctx}); err != nil {
		return
	}
}
`
	path := writeTemp(t, "main.go", body)

	err := wireMainGo(path, "example.com/demo", "key", "Key")
	if err == nil {
		t.Fatal("an aliased model import must be rejected")
	}

	if !strings.Contains(err.Error(), "example.com/demo/model is imported as m") {
		t.Errorf("error does not name the alias: %v", err)
	}

	if readFile(t, path) != body {
		t.Error("a rejected main.go must stay byte-identical")
	}
}

// The same alias problem applies to the handler package, and the dedupe check
// cannot see through the alias, so every re-run would stack another broken
// registration.
func TestWireMainGoRejectsAliasedHandlerImport(t *testing.T) {
	body := `package main

import (
	"context"

	hnd "example.com/demo/handler"
	"github.com/go-sicky/sicky"
	srvHTTP "github.com/go-sicky/sicky/server/http"
)

func main() {
	ctx := context.Background()

	if err := sicky.Init(&sicky.Options{Context: ctx}); err != nil {
		return
	}

	httpSrv := srvHTTP.New(nil, nil)
}
`
	path := writeTemp(t, "main.go", body)

	err := wireMainGo(path, "example.com/demo", "key", "Key")
	if err == nil {
		t.Fatal("an aliased handler import must be rejected when a handler is registered")
	}

	if !strings.Contains(err.Error(), "example.com/demo/handler is imported as hnd") {
		t.Errorf("error does not name the alias: %v", err)
	}

	if readFile(t, path) != body {
		t.Error("a rejected main.go must stay byte-identical")
	}
}

// The appended InitDB call passes ctx. A hand-written initdb without one
// cannot receive that statement without breaking the build.
func TestWireMainGoRejectsInitDBWithoutCtx(t *testing.T) {
	body := `package main

import (
	"context"

	"example.com/demo/handler"
	"github.com/go-sicky/sicky"
	srvHTTP "github.com/go-sicky/sicky/server/http"
)

func main() {
	ctx := context.Background()

	if err := sicky.Init(&sicky.Options{Context: ctx}); err != nil {
		return
	}

	httpSrv := srvHTTP.New(nil, nil)
	httpSrv.Handle(handler.NewHTTPHttp())
}

func initdb() error {
	return nil
}
`
	path := writeTemp(t, "main.go", body)

	err := wireMainGo(path, "example.com/demo", "key", "Key")
	if err == nil {
		t.Fatal("an initdb without ctx must be rejected")
	}

	if !strings.Contains(err.Error(), "initdb does not declare ctx") {
		t.Errorf("error does not name the missing ctx: %v", err)
	}

	if readFile(t, path) != body {
		t.Error("a rejected main.go must stay byte-identical")
	}
}

// The rejections above must not fire for the shape the scaffold generates.
func TestWireMainGoAppendsToExistingInitDB(t *testing.T) {
	body := `package main

import (
	"context"

	"example.com/demo/handler"
	"github.com/go-sicky/sicky"
	srvHTTP "github.com/go-sicky/sicky/server/http"
)

func main() {
	ctx := context.Background()

	if err := sicky.Init(&sicky.Options{Context: ctx}); err != nil {
		return
	}

	httpSrv := srvHTTP.New(nil, nil)
	httpSrv.Handle(handler.NewHTTPHttp())
}

func initdb() error {
	ctx := context.Background()

	return nil
}
`
	path := writeTemp(t, "main.go", body)

	if err := wireMainGo(path, "example.com/demo", "key", "Key"); err != nil {
		t.Fatalf("wireMainGo: %v", err)
	}

	if !strings.Contains(readFile(t, path), "(model.Key{}).InitDB(ctx, false)") {
		t.Errorf("the InitDB call was not appended:\n%s", readFile(t, path))
	}
}

// A write performed before the preflight would normalize formatting, so an
// unformatted file is the input that can tell "no write" from "wrote the file
// back". The gofmt'd fixture above cannot: rewriting it would be invisible.
func TestWireMainGoMissingAnchorKeepsFormatting(t *testing.T) {
	body := "package main\n\n\n\nfunc main()    {}\n"
	path := writeTemp(t, "main.go", body)

	if err := wireMainGo(path, "example.com/demo", "key", "Key"); err == nil {
		t.Fatal("a main.go without sicky.Init must be rejected")
	}

	if readFile(t, path) != body {
		t.Error("a failed preflight must not rewrite main.go")
	}
}

func TestWireMainGoMissingOptionsWritesNothing(t *testing.T) {
	body := "package main\n\nfunc main() {\n\t_ = sicky.Init(nil)\n}\n"
	path := writeTemp(t, "main.go", body)

	err := wireMainGo(path, "example.com/demo", "key", "Key")
	if err == nil {
		t.Fatal("a sicky.Init call without &sicky.Options must be rejected")
	}

	if !strings.Contains(err.Error(), "sicky.Options") {
		t.Errorf("error does not name the missing anchor: %v", err)
	}

	if readFile(t, path) != body {
		t.Error("a failed preflight must not modify main.go")
	}
}

// Wiring a second model must extend the existing wiring rather than duplicate
// it: a second import, flag switch or MustInfra line does not compile.
func TestWireMainGoAddsSecondModel(t *testing.T) {
	path := writeTemp(t, "main.go", wireMainFixture)

	if err := wireMainGo(path, "example.com/demo", "key", "Key"); err != nil {
		t.Fatalf("wireMainGo(key): %v", err)
	}

	if err := wireMainGo(path, "example.com/demo", "cabinet", "Cabinet"); err != nil {
		t.Fatalf("wireMainGo(cabinet): %v", err)
	}

	src := readFile(t, path)
	for _, want := range []string{
		"(model.Key{}).InitDB(ctx, false)",
		"(model.Cabinet{}).InitDB(ctx, false)",
		"handler.NewHTTPKey()",
		"handler.NewHTTPCabinet()",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("wired main.go missing %q:\n%s", want, src)
		}
	}

	for _, once := range []string{
		`"example.com/demo/model"`,
		`MustInfra: []string{"bun"}`,
		`Flag:     "initdb"`,
		"func initdb() error",
		"CREATE EXTENSION IF NOT EXISTS",
		"(model.Key{}).InitDB(ctx, false)",
	} {
		if got := strings.Count(src, once); got != 1 {
			t.Errorf("%q appears %d times, want 1:\n%s", once, got, src)
		}
	}
}

// An existing MustInfra list must gain bun exactly once: duplicate entries
// would initialize the same infrastructure twice.
func TestWireMainGoAppendsBunToExistingMustInfra(t *testing.T) {
	body := strings.Replace(
		wireMainFixture,
		`&sicky.Options{AppName: "demo", Context: ctx}`,
		`&sicky.Options{AppName: "demo", Context: ctx, MustInfra: []string{"redis"}}`,
		1,
	)
	path := writeTemp(t, "main.go", body)

	for range 2 {
		if err := wireMainGo(path, "example.com/demo", "key", "Key"); err != nil {
			t.Fatalf("wireMainGo: %v", err)
		}
	}

	src := readFile(t, path)
	if got := strings.Count(src, `"bun"`); got != 1 {
		t.Errorf("bun appears %d times, want 1:\n%s", got, src)
	}

	if !strings.Contains(src, `"redis"`) {
		t.Errorf("the existing MustInfra entry was dropped:\n%s", src)
	}
}

func TestWireMainGoNoHTTPServerWarnsAndSkipsHandler(t *testing.T) {
	body := `package main

import (
	"context"

	"github.com/go-sicky/sicky"
)

func main() {
	ctx := context.Background()

	if err := sicky.Init(&sicky.Options{Context: ctx}); err != nil {
		return
	}
}
`
	path := writeTemp(t, "main.go", body)

	stderr := captureStderr(t, func() {
		if err := wireMainGo(path, "example.com/demo", "key", "Key"); err != nil {
			t.Errorf("wireMainGo: %v", err)
		}
	})

	if !strings.Contains(stderr, "handler.NewHTTPKey()") {
		t.Errorf("warning must name the manual registration, got %q", stderr)
	}

	src := readFile(t, path)
	if strings.Contains(src, "handler.NewHTTPKey()") {
		t.Errorf("main.go without an httpSrv anchor must not gain a handler registration:\n%s", src)
	}

	for _, want := range []string{`MustInfra: []string{"bun"}`, `Flag:     "initdb"`, "(model.Key{}).InitDB(ctx, false)"} {
		if !strings.Contains(src, want) {
			t.Errorf("wiring must continue without an httpSrv anchor, missing %q:\n%s", want, src)
		}
	}
}

func TestWireConfigBunAddsAndIsIdempotent(t *testing.T) {
	path := writeTemp(t, "config.json", `{"sicky":{"log_level":"debug"},"service":{"disable_jobs":true}}`)

	if err := wireConfigBun(path); err != nil {
		t.Fatalf("wireConfigBun: %v", err)
	}

	var cfg map[string]any
	raw := readFile(t, path)
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("config.json no longer parses: %v\n%s", err, raw)
	}

	bun := cfg["sicky"].(map[string]any)["infra"].(map[string]any)["bun"].(map[string]any)
	if bun["driver"] != "postgres" || !strings.Contains(bun["dsn"].(string), "postgres://") {
		t.Errorf("infra.bun wrong: %#v", bun)
	}

	first := readFile(t, path)
	if err := wireConfigBun(path); err != nil {
		t.Fatalf("second wireConfigBun: %v", err)
	}

	if readFile(t, path) != first {
		t.Error("wireConfigBun is not idempotent")
	}
}

// An operator who already chose a database must keep it: the placeholder DSN
// points every fresh project at the same local PostgreSQL.
func TestWireConfigBunPreservesExistingValues(t *testing.T) {
	path := writeTemp(t, "config.json", `{"sicky":{"infra":{"bun":{"driver":"sqlite","dsn":"file:demo.db"}}}}`)

	if err := wireConfigBun(path); err != nil {
		t.Fatalf("wireConfigBun: %v", err)
	}

	var cfg map[string]any
	raw := readFile(t, path)
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("config.json no longer parses: %v\n%s", err, raw)
	}

	bun := cfg["sicky"].(map[string]any)["infra"].(map[string]any)["bun"].(map[string]any)
	if bun["driver"] != "sqlite" || bun["dsn"] != "file:demo.db" {
		t.Errorf("wireConfigBun overwrote existing values: %#v", bun)
	}
}

func TestWireConfigBunRejectsNonObjectRoot(t *testing.T) {
	for _, body := range []string{`[]`, `"text"`, `null`} {
		path := writeTemp(t, "config.json", body)

		if err := wireConfigBun(path); err == nil {
			t.Fatalf("wireConfigBun accepted a %s root", body)
		}

		if readFile(t, path) != body {
			t.Errorf("a rejected config must stay byte-identical: %s", body)
		}
	}
}

func TestAppendDeclsDedupesByTypeName(t *testing.T) {
	path := writeTemp(t, "http.go", "package request\n\ntype HttpAuthPostLogin struct{}\n")

	decls := "type HttpKey struct {\n\tName string `json:\"name\"`\n}\n"
	for range 2 {
		if err := appendDecls(path, decls); err != nil {
			t.Fatalf("appendDecls: %v", err)
		}
	}

	if got := strings.Count(readFile(t, path), "type HttpKey struct"); got != 1 {
		t.Errorf("HttpKey declared %d times, want 1", got)
	}
}

// The vim modeline is an editor directive and has to stay at the end of the
// file; appending after it would silently disable it for every later edit.
func TestAppendDeclsKeepsModelineLast(t *testing.T) {
	body := "package request\n\ntype HttpAuthPostLogin struct{}\n\n/*\n * Local variables:\n * vim600: sw=4 ts=4 fdm=marker\n */\n"
	path := writeTemp(t, "http.go", body)

	if err := appendDecls(path, "type HttpKey struct{}\n"); err != nil {
		t.Fatalf("appendDecls: %v", err)
	}

	src := readFile(t, path)
	decl := strings.Index(src, "type HttpKey struct")
	modeline := strings.Index(src, "vim600:")
	if decl < 0 {
		t.Fatalf("HttpKey was not appended:\n%s", src)
	}

	if modeline < decl {
		t.Errorf("the vim modeline is no longer last:\n%s", src)
	}
}

func TestMainGoHasValidateSession(t *testing.T) {
	with := writeTemp(t, "http.go", "package handler\n\nfunc validateSession() {}\n")
	if !handlerHasValidateSession(with) {
		t.Error("validateSession was not detected")
	}

	without := writeTemp(t, "http2.go", "package handler\n\nfunc other() {}\n")
	if handlerHasValidateSession(without) {
		t.Error("a handler without validateSession reported true")
	}

	if handlerHasValidateSession(filepath.Join(t.TempDir(), "absent.go")) {
		t.Error("a missing handler file reported true")
	}
}

func writeTemp(t *testing.T, name, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	writeFile(t, path, body)

	return path
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}

	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// captureStderr runs fn with os.Stderr redirected. The tests never run in
// parallel, so the global swap cannot race another test's writes.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}

	old := os.Stderr
	os.Stderr = writer

	defer func() {
		os.Stderr = old
	}()

	fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("close stderr writer: %v", err)
	}

	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}

	if err := reader.Close(); err != nil {
		t.Fatalf("close stderr reader: %v", err)
	}

	return string(out)
}
