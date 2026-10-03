/**
 * @file scaffold_errors_test.go
 * @package cli
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The generated main() discarded the error from sicky.Init, from
// sicky.ConfigUnmarshal and from sicky.Run. A service that failed to start —
// an unusable DSN, an unknown driver, a port already bound — therefore exited
// 0 having served nothing, which is the worst possible signal for a process
// supervisor: it reads as a clean shutdown.
//
// These tests run against the real templates through the real scaffolding path,
// so they fail if a template ever regresses to a bare call.
func scaffoldMain(t *testing.T, projectType string) string {
	t.Helper()

	dir := t.TempDir()

	// One scaffold per test-binary run is not enough here: the three
	// sub-tests each need their own output directory, and the project name is
	// what the directory is named after.
	out, err := exec.Command(scaffoldBinary(t),
		"new", projectType, "--module", "example.com/"+projectType,
		"--http", "--fiber=false", "--no-grpc", "--type", projectType,
		"-o", filepath.Join(dir, "out")).CombinedOutput()
	if err != nil {
		t.Fatalf("scaffold %s: %v\n%s", projectType, err, out)
	}

	return filepath.Join(dir, "out", projectType, "main.go")
}

// scaffoldBinary returns the CLI built once by TestMain, so a test never
// depends on the developer's local build state and the suite pays for one
// build rather than one per test.
func scaffoldBinary(t *testing.T) string {
	t.Helper()

	if sickyBin == "" {
		t.Fatal("the CLI binary was not built: TestMain did not run")
	}

	return sickyBin
}

// repoRoot walks up from the working directory (the package dir) to the
// module root, which is where the go.mod and cmd/sicky live.
func repoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return dir
		}

		dir = parent
	}
}

// TestScaffoldChecksEverySickyError is the regression test for the discarded
// errors. Each of the three calls must be inside an `if err :=` and must fail
// the process on a non-nil error.
func TestScaffoldChecksEverySickyError(t *testing.T) {
	for _, projectType := range []string{"standard", "mcp", "interactive"} {
		t.Run(projectType, func(t *testing.T) {
			src := readFile(t, scaffoldMain(t, projectType))

			for _, call := range []string{"sicky.Init(", "sicky.ConfigUnmarshal(", "sicky.Run("} {
				if !strings.Contains(src, "if err := "+call) {
					t.Errorf("%s: %q must be guarded by `if err :=`", projectType, call)
				}
			}

			// A guarded call that then drops the error is no better.
			for _, msg := range []string{"log.Fatalf(\"sicky: init:", "log.Fatalf(\"sicky: config unmarshal:", "log.Fatalf(\"sicky: run:"} {
				if !strings.Contains(src, msg) {
					t.Errorf("%s: missing %s — a failure must reach the exit status", projectType, msg)
				}
			}
		})
	}
}

// --version prints the version and returns ErrVersionShown, which is a success,
// not a failure. The generated main must exit 0 on it rather than through
// log.Fatalf — otherwise the documented flag would exit non-zero.
func TestScaffoldTreatsVersionShownAsSuccess(t *testing.T) {
	src := readFile(t, scaffoldMain(t, "standard"))

	const check = "errors.Is(err, sicky.ErrVersionShown)"

	idx := strings.Index(src, check)
	if idx < 0 {
		t.Fatal("the ErrVersionShown case is not handled: --version would exit non-zero")
	}

	// The branch body runs up to its closing brace; it must be a bare return.
	body := src[idx+len(check):]

	end := strings.IndexByte(body, '}')
	if end < 0 {
		t.Fatalf("unterminated ErrVersionShown branch at offset %d", idx)
	}

	if branch := body[:end]; !strings.Contains(branch, "return") {
		t.Errorf("branch body %q must return so --version exits 0", strings.TrimSpace(branch))
	}

	if strings.Contains(body[:end], "Fatal") {
		t.Error("--version must return cleanly, not log.Fatalf")
	}
}

// A discarded error teaches every generated service to discard errors, and the
// templates are what most users copy from. This pins the specific shape that
// caused it.
func TestScaffoldHasNoBareSickyCalls(t *testing.T) {
	src := readFile(t, scaffoldMain(t, "standard"))

	for line := range strings.SplitSeq(src, "\n") {
		trimmed := strings.TrimSpace(line)
		for _, call := range []string{"sicky.Init(", "sicky.ConfigUnmarshal(", "sicky.Run("} {
			if strings.HasPrefix(trimmed, call) {
				t.Errorf("bare call %q: %s", trimmed, call)
			}
		}
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	return string(b)
}
