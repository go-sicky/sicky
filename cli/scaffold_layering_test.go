/**
 * @file scaffold_layering_test.go
 * @package cli
 * @author Dr.NP <np@herewe.tech>
 * @since 10/19/2026
 */

package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The template tree mirrors the generated output tree, so the layout a
// maintainer edits is the layout a user sees. That only works while the
// embed directive reaches the nested directories: a pattern that stops at
// one level leaves the handler templates out of the binary, and the failure
// surfaces as a render error in a generated project rather than here.
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

// scaffoldProjectDir renders a standard project into a fresh temp dir and
// returns the project directory. args are appended after the common flags so
// each test states only what makes it different.
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

// TestNewStandardLayering pins the app/service/handler layering a standard
// HTTP project is born with. These files are the contract later tasks (and
// `sicky generate model`) build on: a missing layer here means a generated
// project that cannot be extended the way the design promises.
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

	httpSrc := readFile(t, filepath.Join(dir, "handler", "http.go"))
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

// TestNewWithoutAuth: --no-auth removes the auth-only files and the
// middleware block, but the base handler and the request/response
// aggregation files stay: `generate model` appends to them.
func TestNewWithoutAuth(t *testing.T) {
	dir := scaffoldProjectDir(t, "noauth", "--http", "--fiber=false", "--no-grpc", "--no-auth")

	for _, name := range []string{"app/session.go", "handler/auth.http.go", "service/auth.go"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			t.Errorf("%s must not be generated with --no-auth", name)
		}
	}

	httpSrc := readFile(t, filepath.Join(dir, "handler", "http.go"))
	if strings.Contains(httpSrc, "validateSession") || strings.Contains(httpSrc, "ctxKeyToken") {
		t.Error("--no-auth must remove the middleware block")
	}

	if strings.Contains(readFile(t, filepath.Join(dir, "main.go")), "NewHTTPAuth") {
		t.Error("--no-auth must not register the auth handler")
	}
}

// TestNewAuthDefaultsEmpty: a scaffold must not ship a login. The document
// files an empty credential block; the code must treat it as "not
// configured" rather than "accept anything".
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

// TestNewWithoutHTTPStaysLean: a project without an HTTP server must not
// receive HTTP layering files. `app/config.go` and the app block stay: the
// application configuration is not an HTTP concept.
func TestNewWithoutHTTPStaysLean(t *testing.T) {
	dir := scaffoldProjectDir(t, "nohttp", "--fiber", "--no-grpc", "--http=false")

	for _, name := range []string{
		"handler/http.go", "handler/auth.http.go",
		"handler/request/http.go", "handler/response/http.go",
		"service/auth.go",
	} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			t.Errorf("%s must not be generated without an HTTP server", name)
		}
	}

	cfg := readJSON(t, filepath.Join(dir, "config.json"))
	if _, ok := cfg["app"].(map[string]any); !ok {
		t.Error("config.json must keep the app block without HTTP")
	}
}
