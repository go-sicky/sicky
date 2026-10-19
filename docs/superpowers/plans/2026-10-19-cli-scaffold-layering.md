# CLI Scaffold Layering Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `sicky new` generate the same `app/` + `model/` + `service/` + `handler/request|response` layering as `svc.key-cabinet`, and add `sicky generate model` as the single command that adds a resource and wires bun.

**Architecture:** Project templates move to a tree that mirrors the generated output and are embedded recursively. Standard projects get an auth/session example by default; models are the only trigger for bun. `sicky generate model` renders the same resource templates, then edits `config.json` and `main.go` through a preflighted, atomic wiring helper (`cli/wire.go`).

**Tech Stack:** Go 1.27, `text/template`, `go:embed`, `go/ast`, `go/format`, `encoding/json`, `pflag`, `bunrouter`, `bun`, sicky `utils` (`WrapHTTPResponse`, `BunAutoBind`, `BunAutoFormat[WithStatus]`, `ListFilter`, `RandomString`).

**Spec:** `docs/superpowers/specs/2026-10-19-cli-scaffold-layering-design.md`

## Global Constraints

- Generated `go.mod` uses `go 1.27.0`; models import the standard library `uuid`.
- The scaffolder itself never runs `go get`, `go mod tidy`, or any network command.
- No generated credentials: `admin_username`, `admin_password`, `general_token` default to empty strings.
- Every `.go` file written by `sicky new`, `sicky generate model`, and `sicky generate handler` is `gofmt`-clean, carries the SPDX header with `@file/@package/@author/@since/@brief`, and ends with the vim modeline. The older `sicky generate server|client|service|broker|job|middleware` stubs are out of scope for this plan (pre-existing, headerless).
- Structured log messages in generated code start lowercase (framework §3.8).
- All error returns are checked; no bare `_ =` in generated code.
- `--model` requires HTTP. `sicky generate model` requires an existing `handler/http.go`, `handler/request/http.go`, `handler/response/http.go`.
- Model names must not be `auth` (it would overwrite the auth example).
- Verification gate: `make verify` (fmt-check, vet, lint, build, race). Run `go test ./cli -run <name> -v` inside each task.

## Review Focus

1. `--no-auth` leaves unused imports in `handler/http.go` (`context`, `strings`, `app`, `response`) → generated project does not compile. Every template with conditionals must gate its imports too; `TestScaffoldBuilds` runs the `--no-auth` matrix.
2. `--model auth` (or `generate model auth`) overwrites `service/auth.go` / `handler/auth.http.go` and silently destroys the auth example. The name is rejected in both entry points; `TestNewRejectsAuthModel` and `TestGenerateModelRejectsAuthName` pin it.
3. `generate model` edits `config.json`/`main.go` before preflight succeeds, leaving a half-wired project. Preflight runs first and the test compares file bytes before/after a failed run.
4. Re-running `generate model --force` appends the same DTO declarations again → duplicate type build error. The append helper dedupes by declared type name; the test asserts exactly one `HttpKey` declaration.
5. `--model` without `--http` emits handlers and imports that do not exist → build failure. `new` rejects the combination before writing anything.

---

### Task 1: `@brief` in generated file headers

**Files:**
- Modify: `cli/header.go` (`fileMeta`, `goHeader`, `newFileMeta`)
- Modify: `cli/new.go` (file list brief strings)
- Modify: `cli/generate.go` (`fillGenerateIdentity` / `renderGenerateTemplate` brief)
- Test: `cli/header_test.go`

**Interfaces:**
- Consumes: existing `fileMeta`, `goHeader`, `injectFileHeader`.
- Produces: `fileMeta.Brief string`; `goHeader` emits `* @brief <text>` when `Brief != ""`. All later tasks set `Brief` per template/file.

- [ ] **Step 1: Write the failing test**

```go
func TestGoHeaderCarriesBrief(t *testing.T) {
	fm := fileMeta{
		SPDX: "MIT", Holder: "Example Corp", Year: "2026",
		Author: "Ada <ada@example.com>", File: "handler/http.go", Package: "handler",
		Since: "10/19/2026", Brief: "This file defines the HTTP handlers.",
	}

	got := goHeader(&fm)
	if !strings.Contains(got, "* @brief This file defines the HTTP handlers.\n") {
		t.Errorf("go header missing @brief:\n%s", got)
	}

	without := goHeader(&fileMeta{SPDX: "MIT", Holder: "H", Year: "2026", Author: "A", File: "x.go", Package: "x", Since: "10/19/2026"})
	if strings.Contains(without, "@brief") {
		t.Errorf("an empty brief must omit the line:\n%s", without)
	}
}
```

Also add `"* @brief This file defines the HTTP handlers."` to the expected list in `TestInjectionClassifiesFiles` (set `Brief` on that fixture).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cli -run 'TestGoHeaderCarriesBrief|TestInjectionClassifiesFiles' -v`
Expected: FAIL — `fileMeta` has no field `Brief`.

- [ ] **Step 3: Implement `Brief`**

Add `Brief string` to `fileMeta`. In `goHeader`, after the `@since` line, write `fmt.Fprintf(&b, " * @brief %s\n", fm.Brief)` when `fm.Brief != ""`.

Set briefs at every call site:
- `cli/new.go` file list entries gain a `brief string` column; e.g. `main.go` → `"This file contains the main entry point for the service."`, `app/config.go` → `"This file contains the application configuration."`, `handler/<resource>.http.go` → `"This file defines the HTTP handlers for <resource>."`.
- `cli/generate.go` sets `gc.Brief = "Generated " + schematic + " " + gc.Name + "."`, and `renderGenerateTemplate` assigns `fm.Brief = gc.Brief` after `newFileMeta` (the constructor signature stays unchanged).
- `cli/new.go` assigns `fm.Brief = f.brief` for each file-list entry after `newFileMeta`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cli -run 'TestGoHeaderCarriesBrief|TestInjectionClassifiesFiles|TestInjectionIsIdempotent|TestGeneratedGoIsGofmtClean' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cli/header.go cli/new.go cli/generate.go cli/header_test.go
git commit -m "feat : add @brief to generated file headers"
```

---

### Task 2: Mirror template tree + recursive embed (no behavior change)

**Files:**
- Modify: `cli/new.go` (`//go:embed`, file list template paths)
- Move: `cli/template/project/standard/{main.go,config.go,config.json}.gotmpl` stay; `fiber.go.gotmpl` → `handler/fiber.go.gotmpl`, `http.go.gotmpl` → `handler/http.go.gotmpl`, `grpc.go.gotmpl` → `handler/grpc.go.gotmpl`
- Move: `cli/template/project/mcp/{fiber.go,http.go}.gotmpl` → `handler/fiber.go.gotmpl`, `handler/http.go.gotmpl`; `handler.go.gotmpl` → `handler/handler.go.gotmpl`
- Move: `cli/template/project/interactive/handler.go.gotmpl` → `handler/handler.go.gotmpl`
- Modify: `cli/run_watch.go` (baseline gate repair: errcheck on `<-done` ×2)
- Modify: `server/fiber/fiber.go` (baseline gate repair: alias the `recover` import)
- Test: `cli/scaffold_layering_test.go` (new file, one embed test)

**Interfaces:**
- Consumes: Task 1's `Brief`.
- Produces: `projectTemplates` embeds `template/project` recursively; template paths mirror output paths; `scaffoldProject`'s file entries use the new paths. No API change.

Baseline gate repair (do this first, commit it separately): `make verify` is red at the base commit on three lint findings in files this plan did not touch. Fix them before the template move:
- `cli/run_watch.go:110` and `:118`: `done` is a `chan error`; `case <-done:` trips errcheck. Use `case err := <-done: _ = err` in both selects (the error is reaped by the goroutine that owns `exitErr`).
- `server/fiber/fiber.go:43`: the `recover` middleware import shadows the predeclared builtin. Alias it `fiberrecover` and update the two `recover.New`/`recover.Config` uses.

- [ ] **Step 1: Write the failing test**

```go
func TestProjectTemplatesEmbedNestedFiles(t *testing.T) {
	for _, name := range []string{
		"template/project/standard/handler/fiber.go.gotmpl",
		"template/project/standard/handler/http.go.gotmpl",
		"template/project/standard/handler/grpc.go.gotmpl",
		"template/project/mcp/handler/handler.go.gotmpl",
		"template/project/interactive/handler/handler.go.gotmpl",
	} {
		if _, err := projectTemplates.ReadFile(name); err != nil {
			t.Errorf("%s is not embedded: %v", name, err)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cli -run TestProjectTemplatesEmbedNestedFiles -v`
Expected: FAIL — old templates are one level deep.

- [ ] **Step 3: Move templates and change the embed**

Change the directive to:

```go
//go:embed template/project template/license/*.gotmpl
var projectTemplates embed.FS
```

Move the files with `mkdir -p` + `git mv` (one level deeper). Update `scaffoldProject`'s file list: `template/project/<type>/handler/fiber.go.gotmpl`, etc.; the fallback `handler.go.gotmpl` path becomes `handler/handler.go.gotmpl`. Keep output paths unchanged.

- [ ] **Step 4: Run the full cli suite**

Run: `go test ./cli -v`
Expected: PASS — every existing scaffold test still renders the same output.

- [ ] **Step 5: Commit**

```bash
git add cli/template cli/new.go cli/scaffold_layering_test.go
git commit -m "chore : mirror project templates to output tree"
```

---

### Task 3: Standard layering — app, auth/session, request/response, service

**Files:**
- Create: `cli/template/project/standard/app/config.go.gotmpl`
- Create: `cli/template/project/standard/app/session.go.gotmpl`
- Create: `cli/template/project/standard/handler/http.go.gotmpl` (replaces the current ping-only file)
- Create: `cli/template/project/standard/handler/auth.http.go.gotmpl`
- Create: `cli/template/project/standard/handler/request/http.go.gotmpl`
- Create: `cli/template/project/standard/handler/response/http.go.gotmpl`
- Create: `cli/template/project/standard/service/auth.go.gotmpl`
- Modify: `cli/template/project/standard/main.go.gotmpl`
- Modify: `cli/template/project/standard/config.go.gotmpl`
- Modify: `cli/template/project/standard/config.json.gotmpl`
- Modify: `cli/new.go` (`WithAuth`, `--no-auth`, file list)
- Modify: `cli/scaffold_config_test.go` (add the `app` schema; otherwise this task breaks the existing config test)
- Test: `cli/scaffold_layering_test.go`

**Interfaces:**
- Consumes: Task 2's template tree.
- Produces: `newContext.WithAuth bool`; `--no-auth` flag; generated `app.Config` (`*app.ConfigDef`), `app.AppendSession/DeleteSession/CheckSession`, `handler.NewHTTPHttp()`, `handler.NewHTTPAuth()`, `response.Code*`/`Msg*`.

- [ ] **Step 1: Write the failing tests**

```go
func TestNewStandardLayering(t *testing.T) {
	dir := scaffoldProjectDir(t, "layered", "--http", "--fiber=false", "--no-grpc")

	for _, name := range []string{
		"app/config.go", "app/session.go",
		"handler/http.go", "handler/auth.http.go",
		"handler/request/http.go", "handler/response/http.go",
		"service/auth.go",
	} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}

	httpSrc := readFile(t, filepath.Join(dir, "handler/http.go"))
	for _, want := range []string{"type HTTPHttp struct{}", "func NewHTTPHttp()", "func validateSession(", "func getToken("} {
		if !strings.Contains(httpSrc, want) {
			t.Errorf("handler/http.go missing %q", want)
		}
	}

	mainSrc := readFile(t, filepath.Join(dir, "main.go"))
	for _, want := range []string{"handler.NewHTTPHttp()", "handler.NewHTTPAuth()", "app.Config = config.App.Ensure()"} {
		if !strings.Contains(mainSrc, want) {
			t.Errorf("main.go missing %q", want)
		}
	}
}

func TestNewWithoutAuth(t *testing.T) {
	dir := scaffoldProjectDir(t, "noauth", "--http", "--fiber=false", "--no-grpc", "--no-auth")

	for _, name := range []string{"app/session.go", "handler/auth.http.go", "service/auth.go"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			t.Errorf("%s must not be generated with --no-auth", name)
		}
	}

	httpSrc := readFile(t, filepath.Join(dir, "handler/http.go"))
	if strings.Contains(httpSrc, "validateSession") || strings.Contains(httpSrc, "ctxKeyToken") {
		t.Error("--no-auth must remove the middleware block")
	}

	if strings.Contains(readFile(t, filepath.Join(dir, "main.go")), "NewHTTPAuth") {
		t.Error("--no-auth must not register the auth handler")
	}
}

func TestNewAuthDefaultsEmpty(t *testing.T) {
	dir := scaffoldProjectDir(t, "emptycreds", "--http", "--fiber=false", "--no-grpc")
	cfg := readJSON(t, filepath.Join(dir, "config.json"))

	app := cfg["app"].(map[string]any)
	for _, key := range []string{"admin_username", "admin_password", "general_token"} {
		if app[key] != "" {
			t.Errorf("app.%s = %v; the scaffold must not ship a credential", key, app[key])
		}
	}
}
```

Helper (shared by later tasks, put in `scaffold_layering_test.go`):

```go
func scaffoldProjectDir(t *testing.T, name string, args ...string) string {
	t.Helper()

	dir := t.TempDir()
	full := append([]string{"new", name, "--module", "example.com/" + name, "--type", "standard", "-o", dir}, args...)

	if out, err := exec.Command(scaffoldBinary(t), full...).CombinedOutput(); err != nil {
		t.Fatalf("scaffold %s: %v\n%s", name, err, out)
	}

	return filepath.Join(dir, name)
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()

	var doc map[string]any
	if err := json.Unmarshal([]byte(readFile(t, path)), &doc); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	return doc
}
```

Add to `scaffold_config_test.go` now (not Task 8):

```go
type appFields struct {
	AdminUsername string `mapstructure:"admin_username"`
	AdminPassword string `mapstructure:"admin_password"`
	GeneralToken  string `mapstructure:"general_token"`
}
```

and `App *appFields` in the standard case of `scaffoldConfigStruct`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cli -run 'TestNewStandardLayering|TestNewWithoutAuth|TestNewAuthDefaultsEmpty' -v`
Expected: FAIL — files and flags do not exist.

- [ ] **Step 3: Add the flag and context field**

In `newRun`, add:

```go
noAuthFlag := fs.Bool("no-auth", false, "Skip the auth/session example")
```

and set `nc.WithAuth = !*noAuthFlag` after parsing. Add `WithAuth bool` to `newContext`. In `interactivePrompt`, do not ask about auth; the flag is enough.

- [ ] **Step 4: Write the templates**

Follow the spec §7.3–§7.8 exactly.

`app/config.go`:

```go
type ConfigDef struct {
	AdminUsername string `json:"admin_username" mapstructure:"admin_username" yaml:"admin_username"`
	AdminPassword string `json:"admin_password" mapstructure:"admin_password" yaml:"admin_password"`
	GeneralToken  string `json:"general_token"  mapstructure:"general_token"  yaml:"general_token"`
}

func (c *ConfigDef) Ensure() *ConfigDef {
	if c == nil {
		c = &ConfigDef{}
	}

	return c
}

var Config *ConfigDef
```

`app/session.go` mirrors the reference (`sync.Map`, `AccessToken`, `AppendSession` with `sess_` + `utils.RandomString(32)`, `DeleteSession`, `CheckSession`).

`handler/http.go`:

```go
type HTTPHttp struct{}

func NewHTTPHttp() *HTTPHttp
func (h *HTTPHttp) Name() string { return "http.http" }
func (h *HTTPHttp) Type() string { return "http" }
func (h *HTTPHttp) Register(router *bunrouter.Router) { router.GET("/ping", h.ping) }
func (h *HTTPHttp) ping(...) error
```

With auth: `validateSession` uses `subtle.ConstantTimeCompare` for `app.Config.GeneralToken` (skip when the token is empty), falls back to `app.CheckSession`, and stores the token in context. Gate the imports (`context`, `crypto/subtle`, `strings`, `app`, `response`, `utils`) with `{{if .WithAuth}}`.

`handler/auth.http.go` implements login/logout per spec §7.6. `service/auth.go` rejects when either configured credential is empty, then compares with `subtle.ConstantTimeCompare`.

`handler/request/http.go` has the auth DTOs; `handler/response/http.go` has `CodeOK/CodeInvalidRequest/CodeInvalidCertificate/CodeInvalidAuthorization/CodeGeneralError`, the matching `Msg*` strings (`MsgOK = "OK"`), and the auth response DTOs.

`main.go`: after `ConfigUnmarshal`, set `app.Config = config.App.Ensure()`; register `NewHTTPHttp()` always (when HTTP is on) and `NewHTTPAuth()` only with auth. Add `app` import.

`config.go` / `config.json`: add `App *app.ConfigDef` and the empty `app` block.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./cli -run 'TestNewStandardLayering|TestNewWithoutAuth|TestNewAuthDefaultsEmpty|TestScaffoldChecksEverySickyError' -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add cli/new.go cli/template/project/standard cli/scaffold_layering_test.go
git commit -m "feat : generate app, auth and HTTP layering in standard projects"
```

---

### Task 4: `--model` — resource templates, bun config, Makefile deps, Go 1.27

**Files:**
- Create: `cli/model.go` (`modelSpec`, `modelSpecs`, `tableName`)
- Create: `cli/template/entity/model.go.gotmpl`
- Create: `cli/template/entity/service.go.gotmpl`
- Create: `cli/template/entity/handler.http.go.gotmpl`
- Create: `cli/template/entity/request.decls.go.gotmpl`
- Create: `cli/template/entity/response.decls.go.gotmpl`
- Modify: `cli/new.go` (`--model`, model loop, merged request/response bodies)
- Modify: `cli/template/project/standard/main.go.gotmpl` (models + initdb)
- Modify: `cli/template/project/standard/config.json.gotmpl` (`infra.bun`)
- Modify: `cli/template/project/shared/Makefile.gotmpl` (`deps`)
- Modify: `cli/template/project/shared/go.mod.gotmpl` (`go 1.27.0`)
- Test: `cli/scaffold_model_test.go`

**Interfaces:**
- Consumes: Task 3's layering files and `WithAuth`.
- Produces: `type modelSpec struct { Name, ExportedName, Table string }`; `func modelSpecs(names []string) ([]modelSpec, error)`; `func tableName(name string) string`; `newContext.Models []modelSpec`; `--model` flag. Templates reference `.Name`, `.ExportedName`, `.Table`, `.WithAuth`.

- [ ] **Step 1: Write the failing tests**

```go
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

	if !strings.Contains(readFile(t, filepath.Join(dir, "handler/key.http.go")), ".Use(validateSession)") {
		t.Error("a model handler in an auth-enabled project must mount validateSession")
	}
}

func TestNewModelNoAuthNoMiddleware(t *testing.T) {
	dir := scaffoldProjectDir(t, "modelnoauth", "--http", "--fiber=false", "--no-grpc", "--no-auth", "--model", "key")

	if strings.Contains(readFile(t, filepath.Join(dir, "handler/key.http.go")), ".Use(validateSession)") {
		t.Error("--no-auth must leave model handlers without the middleware")
	}

	if strings.Contains(readFile(t, filepath.Join(dir, "handler/http.go")), "func validateSession") {
		t.Error("--no-auth must not define validateSession")
	}
}

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

func TestScaffoldMakefileTidies(t *testing.T) {
	dir := scaffoldProjectDir(t, "tidy", "--http", "--fiber=false", "--no-grpc")
	makefile := readFile(t, filepath.Join(dir, "Makefile"))

	for _, want := range []string{"deps:", "$(GO) mod tidy", "all: summary proto deps fmt build"} {
		if !strings.Contains(makefile, want) {
			t.Errorf("Makefile missing %q", want)
		}
	}
}

func TestScaffoldGoDirective(t *testing.T) {
	dir := scaffoldProjectDir(t, "goversion", "--http", "--fiber=false", "--no-grpc")
	if !strings.Contains(readFile(t, filepath.Join(dir, "go.mod")), "go 1.27.0") {
		t.Error("generated go.mod must target Go 1.27")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cli -run 'TestNewWithModels|TestNewModelRequiresHTTP|TestNewRejectsAuthModel|TestScaffoldMakefileTidies|TestScaffoldGoDirective' -v`
Expected: FAIL — `--model` is unknown.

- [ ] **Step 3: Implement model specs and the flag**

```go
type modelSpec struct {
	Name         string
	ExportedName string
	Table        string
}

func modelSpecs(names []string) ([]modelSpec, error) {
	out := make([]modelSpec, 0, len(names))

	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		if err := validateSchematicName(name); err != nil {
			return nil, err
		}

		if strings.EqualFold(name, "auth") {
			return nil, fmt.Errorf("model name %q is reserved for the auth example", name)
		}

		out = append(out, modelSpec{Name: name, ExportedName: exportName(name), Table: tableName(name)})
	}

	return out, nil
}

func tableName(name string) string {
	return strings.ToLower(strings.NewReplacer("-", "_", ".", "_").Replace(name))
}
```

In `newRun`: `modelFlag := fs.StringSlice("model", nil, "Generate bun models (comma separated)")`; set `nc.Models`; after `interactivePrompt` and before `validateNewContext`, reject `len(nc.Models) > 0 && !nc.WithHTTP` with `--model requires an HTTP server (--http)`.

- [ ] **Step 4: Add resource templates and render them**

`template/entity/model.go.gotmpl` follows spec §7.9: `bun.BaseModel` with `table:<Table>`, `ID uuid.UUID` with `default:uuid_generate_v4()`, `Name string` with a TODO, `CreatedAt`/`UpdatedAt`, `TableName`, `String`, `Validate`, the six DB methods, and `InitDB(ctx, dropTable bool) error` creating `idx_<table>_name`, `idx_<table>_created_at`, `idx_<table>_updated_at`.

`template/entity/service.go.gotmpl`:

```go
type {{.ExportedName}} struct{}
```

`template/entity/handler.http.go.gotmpl` follows spec §7.10. With `.WithAuth`, `Register` is:

```go
g := router.NewGroup("/{{.Name}}").Use(validateSession)
```

Without it:

```go
g := router.NewGroup("/{{.Name}}")
```

Routes: `GET /list`, `POST /`, `GET /:id`, `PUT /:id`, `DELETE /:id`; handlers return `nil` with TODOs.

`request.decls.go.gotmpl` / `response.decls.go.gotmpl` render only the per-model types (no `package` clause), so the same fragments can be appended later.

In `scaffoldProject`, after the static files:
- for each model, render the three full templates into `model/<name>.go`, `service/<name>.go`, `handler/<name>.http.go` through the same header/footer/format pipeline;
- build `handler/request/http.go` = rendered base file + all `request.decls` fragments; same for `response`.

- [ ] **Step 5: Wire bun into the generated main/config**

`main.go.gotmpl`: when `.Models` is non-empty, add the `infra`, `logger`, and `model` imports, `MustInfra: []string{"bun"}`, the `FlagSwitch`, one `httpSrv.Handle(handler.NewHTTP<Name>())` per model, and `initdb()`:

```go
func initdb() error {
	ctx := context.Background()

	if infra.Bun == nil {
		logger.Logger.FatalContext(ctx, "database connection is not initialized")
	}

	if _, err := infra.Bun.ExecContext(ctx, `CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`); err != nil {
		return err
	}

	{{range .Models}}if err := (model.{{.ExportedName}}{}).InitDB(ctx, false); err != nil {
		return err
	}

	{{end}}os.Exit(0)

	return nil
}
```

`config.json.gotmpl`: when `.Models` is non-empty, add `infra.bun` with `driver: postgres` and the placeholder DSN from spec §7.2.

`Makefile.gotmpl`: add the `deps` target and the dependency edges from spec §7.12. `go.mod.gotmpl`: change `go 1.26.0` to `go 1.27.0`.

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./cli -run 'TestNewWithModels|TestNewModelRequiresHTTP|TestNewRejectsAuthModel|TestScaffoldMakefileTidies|TestScaffoldGoDirective' -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add cli/model.go cli/new.go cli/template/entity cli/template/project/standard cli/template/project/shared cli/scaffold_model_test.go
git commit -m "feat : add --model with bun wiring to sicky new"
```

---

### Task 5: Wiring helpers for existing projects

**Files:**
- Create: `cli/wire.go`
- Test: `cli/wire_test.go`

**Interfaces:**
- Consumes: nothing from earlier tasks except the generated `main.go` shape.
- Produces:
  - `func wireConfigBun(path string) error`
  - `func wireMainGo(path, module, modelName, exportedName string) error`
  - `func appendDecls(path, decls string) error`
  - `func mainGoHasValidateSession(path string) bool`

- [ ] **Step 1: Write the failing tests**

```go
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
```

Helpers (the latter two already exist in `scaffold_errors_test.go`; reuse them):

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cli -run 'TestWireMainGo|TestWireConfigBun|TestAppendDecls' -v`
Expected: FAIL — `wireMainGo`, `wireConfigBun`, `appendDecls` undefined.

- [ ] **Step 3: Implement `cli/wire.go`**

`wireMainGo(path, module, modelName, exportedName string) error`:

1. Read and `parser.ParseFile`.
2. Preflight (all checks before any write): find `sicky.Init` call and `&sicky.Options{}` literal; find `httpSrv := srvHTTP.New` (optional); find or plan `initdb`.
3. Mutate the AST: add the `<module>/model` import; add/patch `MustInfra`; append `&sicky.FlagSwitch{...}` to `sicky.Init`; create `initdb` when absent; append the `CREATE EXTENSION` + `(model.<ExportedName>{}).InitDB(ctx, false)` statements; append `httpSrv.Handle(handler.NewHTTP<ExportedName>())` after the last `httpSrv.Handle` (or after the `httpSrv` assignment).
4. `format.Node` into a buffer and write atomically via `writeAtomic(path, buf.Bytes())`.

`wireConfigBun(path)`:

1. Decode into `map[string]any`; reject a non-object root.
2. Ensure `sicky` and `sicky.infra` and `sicky.infra.bun`; set `driver: "postgres"` and the placeholder DSN only when absent.
3. `json.MarshalIndent(cfg, "", "    ")` + newline, atomic write.

`appendDecls(path, decls)`:

1. Parse the target file and the snippet as a file (`package x` prefix if needed).
2. Collect declared type names in the target; append only snippet `TypeSpec`s whose name is absent.
3. `format.Node`, atomic write.

`mainGoHasValidateSession(path)` parses `handler/http.go` and reports whether a `validateSession` func exists.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cli -run 'TestWireMainGo|TestWireConfigBun|TestAppendDecls' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cli/wire.go cli/wire_test.go
git commit -m "feat : add project wiring helpers for models"
```

---

### Task 6: `sicky generate model`

**Files:**
- Create: `cli/generate_model.go`
- Modify: `cli/generate.go` (dispatch + embed `template/entity/*.gotmpl`)
- Modify: `cli/generate_flags.go` (add `--no-wire` to the shared flags if useful, or keep it local)
- Test: `cli/generate_model_test.go`

**Interfaces:**
- Consumes: Task 4's resource templates; Task 5's `wireConfigBun`, `wireMainGo`, `appendDecls`, `mainGoHasValidateSession`.
- Produces: `func generateModel(args []string) int` and the `model` schematic in `generateRun`'s switch/help.

- [ ] **Step 1: Write the failing tests**

```go
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

	// --force rerun must not duplicate DTO declarations.
	runGenerate(t, dir, "model", "train", "--force")
	if got := strings.Count(readFile(t, filepath.Join(dir, "handler/response/http.go")), "type HttpTrain struct"); got != 1 {
		t.Errorf("HttpTrain declared %d times after --force, want 1", got)
	}
}

func TestGenerateModelNoWire(t *testing.T) {
	dir := scaffoldProjectDir(t, "nowire", "--http", "--fiber=false", "--no-grpc")
	cfgBefore := readFile(t, filepath.Join(dir, "config.json"))
	mainBefore := readFile(t, filepath.Join(dir, "main.go"))

	runGenerate(t, dir, "model", "train", "--no-wire")

	if readFile(t, filepath.Join(dir, "config.json")) != cfgBefore {
		t.Error("--no-wire changed config.json")
	}

	if readFile(t, filepath.Join(dir, "main.go")) != mainBefore {
		t.Error("--no-wire changed main.go")
	}
}

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

	if _, err := os.Stat(filepath.Join(dir, "model/key.go")); err == nil {
		t.Error("a rejected generate model must not write files")
	}
}

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
```

Helpers (new file): `runGenerate`, `runGenerateRaw` execute `scaffoldBinary(t)` with `cmd.Dir = dir`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cli -run 'TestGenerateModel' -v`
Expected: FAIL — unknown schematic `model`.

- [ ] **Step 3: Implement the command**

```go
func generateModel(args []string) int {
	fs, gf, err := parseGenerateFlags("generate model", args)
	if err != nil { ... }

	noWire := fs.Bool("no-wire", false, "Write files only; do not edit config.json or main.go")
	// require one positional name
	// validateSchematicName + reserved name check
	// preflight: go.mod, main.go, handler/http.go, handler/request/http.go, handler/response/http.go
	// render model/service/handler from template/entity with a generateContext extended by Table/WithAuth
	// append request/response decls
	// unless noWire: wireConfigBun, wireMainGo
}
```

Use the existing `renderGenerateTemplate` for the three full files (it already injects headers, footers, formats, and guards the path). For the DTO fragments, render `template/entity/request.decls.go.gotmpl` / `response.decls.go.gotmpl` with `text/template` directly (no header), then `appendDecls`.

Add `"model"` to `generateRun`'s switch and both help strings. Add `template/entity/*.gotmpl` to `generateTemplates`' embed.

`--force` semantics: preflight existing files before writing: if any of the three resource files exists and `--force` is false, print `sicky generate model: <path> exists (use --force)` and return 1 without writing.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cli -run 'TestGenerateModel' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cli/generate_model.go cli/generate.go cli/generate_model_test.go
git commit -m "feat : add sicky generate model"
```

---

### Task 7: `generate handler` naming alignment

**Files:**
- Modify: `cli/generate.go` (`generateHandler`)
- Modify: `cli/template/handler/handler.http.go.gotmpl`
- Test: `cli/generate_handler_test.go`

**Interfaces:**
- Consumes: `validateSchematicName`, `exportName`, `renderGenerateTemplate`.
- Produces: `func handlerIdent(name, transport string) (ident, route string)`.

- [ ] **Step 1: Write the failing tests**

```go
func TestGenerateHandlerHTTPNaming(t *testing.T) {
	for _, input := range []string{"key", "key.http"} {
		t.Run(input, func(t *testing.T) {
			dir := t.TempDir()
			runGenerate(t, dir, "handler", input, "--type", "http")

			path := filepath.Join(dir, "handler", input+".go")
			src := readFile(t, path)

			for _, want := range []string{"type HTTPKey struct{}", "func NewHTTPKey()", `return "key.http"`} {
				if !strings.Contains(src, want) {
					t.Errorf("%s missing %q", input, want)
				}
			}

			if strings.Contains(src, "KeyHttp") {
				t.Errorf("%s still uses the old KeyHttp naming", input)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cli -run TestGenerateHandlerHTTPNaming -v`
Expected: FAIL — template still emits `KeyHttp`.

- [ ] **Step 3: Implement naming**

```go
func handlerIdent(name, transport string) (ident, route string) {
	base := strings.TrimSuffix(name, "."+transport)
	route = base + "." + transport

	switch transport {
	case "http":
		ident = "HTTP" + exportName(base)
	case "fiber":
		ident = exportName(base) + "Fiber"
	}

	return ident, route
}
```

`generateHandler` passes the derived ident as `.ExportedName` and the route as `.Name` (or set both in the template context). Keep the output path `handler/<name>.go` and the file name `handler/key.http.go` for input `key.http`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cli -run TestGenerateHandlerHTTPNaming -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cli/generate.go cli/template/handler/handler.http.go.gotmpl cli/generate_handler_test.go
git commit -m "feat : align generated HTTP handler naming"
```

---

### Task 8: Docs, config test, build smoke, full gate

**Files:**
- Create: `cli/template/project/standard/AGENTS.md.gotmpl`
- Modify: `cli/template/doc/readme.md.gotmpl`
- Modify: `cli/scaffold_config_test.go`
- Create: `cli/scaffold_build_test.go`
- Test: all of `./cli`

**Interfaces:**
- Consumes: everything above.
- Produces: generated `AGENTS.md`, updated `README.md`, config-schema coverage for `app` and `infra.bun`, and a build smoke test.

- [ ] **Step 1: Write the failing config test update**

The `app` schema was added in Task 3. Add a model case to `TestScaffoldConfigKeysAllResolve` (it also validates `sicky.infra.bun`):

```go
{projectType: "standard", args: []string{"--http", "--fiber=false", "--no-grpc", "--model", "key"}, withHTTP: true},
```

Add `TestScaffoldDocs`:

```go
func TestScaffoldDocs(t *testing.T) {
	dir := scaffoldProjectDir(t, "documented", "--http", "--fiber=false", "--no-grpc", "--model", "key")

	for _, name := range []string{"README.md", "AGENTS.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}

	if !strings.Contains(readFile(t, filepath.Join(dir, "AGENTS.md")), "--initdb") {
		t.Error("AGENTS.md must document --initdb")
	}

	if !strings.Contains(readFile(t, filepath.Join(dir, "README.md")), "model/") {
		t.Error("README.md must document the layered structure")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cli -run 'TestScaffoldConfigKeysAllResolve|TestScaffoldDocs' -v`
Expected: FAIL — no AGENTS.md, no app schema.

- [ ] **Step 3: Add the templates and schema**

`AGENTS.md.gotmpl` follows spec §7.11. `readme.md.gotmpl` updates the structure block and the `--model`/`generate model`/empty-credentials notes.

- [ ] **Step 4: Add the build smoke test**

```go
func TestScaffoldBuilds(t *testing.T) {
	if testing.Short() {
		t.Skip("build smoke test needs the module cache/network")
	}

	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "plain", args: []string{"--http", "--fiber=false", "--no-grpc"}},
		{name: "noauth", args: []string{"--http", "--fiber=false", "--no-grpc", "--no-auth"}},
		{name: "models", args: []string{"--http", "--fiber=false", "--no-grpc", "--model", "key"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := scaffoldProjectDir(t, tc.name, tc.args...)

			// Point the generated module at this checkout so the test builds
			// the code it just rendered, not a published sicky.
			mod := readFile(t, filepath.Join(dir, "go.mod"))
			mod += "\nrequire github.com/go-sicky/sicky v0.0.0\n\nreplace github.com/go-sicky/sicky => " + repoRoot() + "\n"
			writeFile(t, filepath.Join(dir, "go.mod"), mod)

			for _, args := range [][]string{{"mod", "tidy"}, {"build", "./..."}} {
				cmd := exec.Command("go", args...)
				cmd.Dir = dir
				cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")

				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, out)
				}
			}
		})
	}
}
```

If `go mod tidy` fails for lack of network, `t.Skipf` with the captured output (the design allows the skip). Keep the failure visible: only skip on `dial tcp`/`proxy.golang.org`/`no such host`.

- [ ] **Step 5: Run the full gate**

Run: `make verify`
Expected: PASS (fmt-check, vet, lint, build, race).

- [ ] **Step 6: Commit**

```bash
git add cli/template/project/standard/AGENTS.md.gotmpl cli/template/doc/readme.md.gotmpl cli/scaffold_config_test.go cli/scaffold_build_test.go
git commit -m "chore : document scaffold layering and pin its config"
```

---

## Self-Review

- Spec coverage:
  - §7.1–§7.2 main/config → Task 3/4.
  - §7.3–§7.8 app/auth/request/response/service → Task 3.
  - §7.9–§7.10 model/service/handler → Tasks 4/6.
  - §7.11–§7.12 docs/Makefile → Tasks 4/8.
  - §8.1 `--model`/`--no-auth`/embed → Tasks 3/4.
  - §8.2 `generate model` → Tasks 5/6.
  - §8.3 handler naming → Task 7.
  - §9 format → Tasks 1/3/4.
  - §10 compatibility → Tasks 4/8.
  - §11 validation → every task plus Task 8.
- Type consistency: `modelSpec`, `WithAuth`, `wireMainGo`/`wireConfigBun`/`appendDecls`, `handlerIdent`, and `scaffoldProjectDir`/`readFile`/`readJSON` are introduced before their uses.
- Review Focus lines each map to a task: (1) Task 8 build matrix, (2) Tasks 4/6 reserved-name tests, (3) Task 5 preflight, (4) Tasks 5/6 dedupe, (5) Task 4 `TestNewModelRequiresHTTP`.
