/**
 * @file scaffold_build_test.go
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
)

// TestScaffoldBuilds is the only test that compiles what the templates render.
// Every other scaffold test asserts on file contents, which cannot catch a
// template that is internally inconsistent — a handler referring to a symbol
// another template stopped declaring, or an import that no longer exists.
//
// The generated go.mod intentionally carries only module/go, so the test adds
// the require/replace pair that points the project at this checkout. Without
// it the build would pull a published sicky and prove nothing about the tree
// under test.
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
		// --model and --no-auth together: the resource handler must not mount
		// the validateSession the --no-auth project does not define.
		{name: "modelsnoauth", args: []string{"--http", "--fiber=false", "--no-grpc", "--no-auth", "--model", "key"}},
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

				out, err := cmd.CombinedOutput()
				if err == nil {
					continue
				}

				// A sandbox without module access cannot resolve the
				// dependencies; that says nothing about the templates, so it
				// is the one failure that may skip. A compile error must not.
				if networkResolutionFailure(string(out)) {
					t.Skipf("go %s needs the module cache/network: %v\n%s", strings.Join(args, " "), err, out)
				}

				t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		})
	}
}

// networkResolutionFailure reports whether go's output is a failure to reach
// the module source rather than a failure to compile what was downloaded.
func networkResolutionFailure(out string) bool {
	out = strings.ToLower(out)

	for _, marker := range []string{"dial tcp", "proxy.golang.org", "no such host"} {
		if strings.Contains(out, marker) {
			return true
		}
	}

	return false
}
