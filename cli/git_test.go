/**
 * @file git_test.go
 * @package cli
 * @author Dr.NP <np@herewe.tech>
 * @since 10/21/2026
 */

package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestNewSkipsGitInitInsideWorkTree: `sicky new` inside an existing working
// tree must not create a nested repository. A nested .git makes the new
// project a separate history that the enclosing repository cannot see, so the
// files it just wrote are invisible to the monorepo the user is adding them
// to. The enclosing repository is the one that should track the result.
func TestNewSkipsGitInitInsideWorkTree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed; the CLI skips `git init` entirely")
	}

	root := t.TempDir()
	gitCmd(t, root, "init", "-q")

	out, err := exec.Command(scaffoldBinary(t),
		"new", "svc", "--type", "standard", "--module", "example.com/svc",
		"--http", "--fiber=false", "--no-grpc", "-o", root).CombinedOutput()
	if err != nil {
		t.Fatalf("sicky new: %v\n%s", err, out)
	}

	project := filepath.Join(root, "svc")
	if _, err := os.Stat(filepath.Join(project, ".git")); err == nil {
		t.Errorf("`sicky new` created a nested repository inside an existing work tree:\n%s", out)
	}

	// Not initializing is only useful if the enclosing repository sees the
	// generated files; an ignored or skipped directory would be just as lost.
	if status := gitCmd(t, root, "status", "--porcelain"); !strings.Contains(status, "svc/") {
		t.Errorf("the parent repository does not see the generated project:\n%s", status)
	}
}

// TestNewInitializesGitOutsideWorkTree pins the behavior the skip must not
// swallow: outside any working tree the generated project still gets its own
// repository, which is what every standalone `sicky new` has always done.
func TestNewInitializesGitOutsideWorkTree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed; the CLI skips `git init` entirely")
	}

	root := t.TempDir()
	if gitIsWorkTree(t, root) {
		t.Skip("the test temp directory is itself inside a git working tree")
	}

	out, err := exec.Command(scaffoldBinary(t),
		"new", "svc", "--type", "standard", "--module", "example.com/svc",
		"--http", "--fiber=false", "--no-grpc", "-o", root).CombinedOutput()
	if err != nil {
		t.Fatalf("sicky new: %v\n%s", err, out)
	}

	if _, err := os.Stat(filepath.Join(root, "svc", ".git")); err != nil {
		t.Errorf("`sicky new` outside a work tree must still initialize a repository: %v\n%s", err, out)
	}
}

// gitCmd runs one git command against dir and returns its trimmed output.
func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()

	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}

	return strings.TrimSpace(string(out))
}

// gitIsWorkTree asks git directly rather than through the helper under test,
// so a broken production check cannot turn the fixture guard into a skip.
func gitIsWorkTree(t *testing.T, dir string) bool {
	t.Helper()

	out, err := exec.Command("git", "-C", dir, "rev-parse", "--is-inside-work-tree").Output()
	if err != nil {
		return false
	}

	return strings.TrimSpace(string(out)) == "true"
}
