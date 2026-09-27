package cli

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestWatchProcessStopKillsProcessGroup: the loop used to kill only the
// `go run` parent, leaving the compiled binary running - still holding
// the port, so the next start failed and processes piled up. The child
// now runs in its own process group and stop() signals the group.
func TestWatchProcessStopKillsProcessGroup(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns a shell")
	}

	pidFile := filepath.Join(t.TempDir(), "grandchild.pid")
	// A grandchild stands in for the binary `go run` compiles and spawns.
	script := `sleep 60 & echo $! > "` + pidFile + `"; wait`

	p := &watchProcess{}
	p.start("/bin/sh", []string{"-c", script})
	if !p.running() {
		t.Fatal("child did not start")
	}

	var grandchild int
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(pidFile)
		if err == nil {
			pid, perr := strconv.Atoi(strings.TrimSpace(string(data)))
			if perr == nil {
				grandchild = pid

				break
			}
		}

		time.Sleep(20 * time.Millisecond)
	}

	if grandchild == 0 {
		p.stop(time.Second)
		t.Fatal("grandchild never started")
	}

	start := time.Now()
	p.stop(2 * time.Second)
	if elapsed := time.Since(start); elapsed > 4*time.Second {
		t.Fatalf("stop took %s, want a prompt exit", elapsed)
	}

	if p.running() {
		t.Fatal("child still marked running after stop")
	}

	// A second stop must be a no-op, never a double Wait.
	p.stop(time.Second)

	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(grandchild, 0); err != nil {
			return // ESRCH: the group kill reached it
		}

		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("grandchild %d survived the group kill", grandchild)
}

// TestWatchProcessStopWithoutChild: stopping an idle watcher must not
// block or panic.
func TestWatchProcessStopWithoutChild(t *testing.T) {
	p := &watchProcess{}
	p.stop(time.Millisecond)

	if p.running() {
		t.Fatal("unexpected child after stop")
	}
}

// TestWatchProcessReportsChildExit: a child that dies on its own (build
// error, crash) has to be observable - the loop used to keep watching a
// port nothing listened on any more.
func TestWatchProcessReportsChildExit(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns a shell")
	}

	p := &watchProcess{}
	p.start("sh", []string{"-c", "exit 3"})

	select {
	case <-p.exited():
	case <-time.After(5 * time.Second):
		t.Fatal("child exit was not observed")
	}

	if !p.running() {
		t.Fatal("a self-exited child still owns the handle until takeExit")
	}

	err := p.takeExit()
	if err == nil {
		t.Fatal("takeExit lost the child's exit status")
	}

	// Second report is a no-op: the channel is cleared, so the select in
	// the watch loop cannot spin on it.
	if p.exited() != nil {
		t.Fatal("takeExit must clear the exit channel")
	}

	if again := p.takeExit(); again != nil {
		t.Fatalf("second takeExit = %v, want nil", again)
	}
}
