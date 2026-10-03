/**
 * @file leak_test.go
 * @package cli
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"go.uber.org/goleak"

	"github.com/go-sicky/sicky/internal/leakcheck"
)

// sickyBin is the CLI binary the scaffolding tests drive. It is built once per
// test binary rather than once per test: the suite renders real projects
// through the real `sicky new`, and rebuilding the binary each time cost
// about seven seconds a call — fourteen calls, so a minute and a half under
// -race, which was enough to make a full `make verify` run flaky on load.
var sickyBin string

// TestMain builds the CLI once, runs the suite, then checks for leaked
// goroutines.
//
// The check is done by hand rather than with goleak.VerifyTestMain because
// that helper calls os.Exit itself, which would skip the temp-dir cleanup
// and leave a built binary behind in every run. The semantics are the same:
// leaks are only reported when the run was otherwise successful, since a
// package that already failed elsewhere says nothing about leaks.
//
// The options are built before m.Run so IgnoreCurrent snapshots the
// goroutines that existed before the first test rather than after the last
// one, which would ignore everything the tests leaked.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "sicky-cli-test")
	if err != nil {
		fmt.Fprintln(os.Stderr, "cli tests: temp dir:", err)
		os.Exit(1)
	}

	sickyBin = filepath.Join(dir, "sicky")

	build := exec.Command("go", "build", "-o", sickyBin, "github.com/go-sicky/sicky/cmd/sicky")
	build.Dir = repoRoot()

	if out, berr := build.CombinedOutput(); berr != nil {
		os.RemoveAll(dir)
		fmt.Fprintln(os.Stderr, "cli tests: build:", berr, "\n"+string(out))
		os.Exit(1)
	}

	options := leakcheck.Options()

	code := m.Run()

	if code == 0 {
		if err := goleak.Find(options...); err != nil {
			fmt.Fprintln(os.Stderr, "cli tests: goroutine leak:", err)
			code = 1
		}
	}

	// Explicit rather than deferred: os.Exit below would skip a defer, and
	// this is the only thing keeping a built binary out of /tmp between runs.
	os.RemoveAll(dir)

	os.Exit(code)
}
