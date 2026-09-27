/**
 * @file run_watch.go
 * @package cli
 * @author Dr.NP <np@herewe.tech>
 * @since 09/06/2026
 */

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

// watchStopGrace is how long a terminating child gets before it is killed.
const watchStopGrace = 3 * time.Second

// watchKillTimeout bounds the wait after SIGKILL (a stuck process would
// otherwise leak on every restart).
const watchKillTimeout = 5 * time.Second

// watchProcess owns the `go run` child of `sicky run --watch`.
//
// The child runs in its own process group: `go run` spawns the actual
// binary as its child, and killing only the parent (as this loop used to)
// left the compiled program running - still holding the port, so every
// restart failed with "address already in use" while processes piled up.
// Moving the child out of the terminal's group also means Ctrl-C no
// longer reaches it, so runWatchLoop forwards the signal explicitly.
type watchProcess struct {
	mu   sync.Mutex
	cmd  *exec.Cmd
	done chan error

	// exitCh is closed when the current child ends; exitErr carries
	// Wait's result. Both are cleared by takeExit so the select in the
	// watch loop does not spin on a channel that is already closed.
	exitCh  chan struct{}
	exitErr error
}

// start launches name with args in a new process group.
func (p *watchProcess) start(name string, args []string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cmd != nil {
		// Callers stop before starting; refusing keeps a handle from
		// being dropped without being waited on.
		return
	}

	cmd := exec.Command(name, args...) //nolint:gosec // G204: dev --watch loop; the binary is fixed (go) and extra args come from the invoking operator
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "sicky run --watch: %s\n", err.Error())

		return
	}

	done := make(chan error, 1)
	exitCh := make(chan struct{})
	go func() {
		// Exactly one Wait per started process: the channel also carries
		// a child that exits on its own (build error, crash).
		err := cmd.Wait()
		done <- err

		p.mu.Lock()
		p.exitErr = err
		close(exitCh)
		p.mu.Unlock()
	}()

	p.cmd = cmd
	p.done = done
	p.exitCh = exitCh
	p.exitErr = nil
}

// stop terminates the whole process group and reaps the child.
func (p *watchProcess) stop(grace time.Duration) {
	p.mu.Lock()
	cmd, done := p.cmd, p.done
	p.cmd, p.done = nil, nil
	p.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return
	}

	pid := cmd.Process.Pid

	// Negative pid signals the whole group (the compiled binary too).
	_ = syscall.Kill(-pid, syscall.SIGTERM)

	select {
	case <-done:
		return
	case <-time.After(grace):
	}

	_ = syscall.Kill(-pid, syscall.SIGKILL)

	select {
	case <-done:
	case <-time.After(watchKillTimeout):
		// Never block the restart loop forever on a stuck child.
		fmt.Fprintf(os.Stderr, "sicky run --watch: child process %d did not exit\n", pid)
	}
}

// running reports whether a child is currently owned.
func (p *watchProcess) running() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.cmd != nil
}

// exited returns the channel that closes when the current child ends, or
// nil when nothing is running (a nil channel blocks in select forever,
// which is exactly what a stopped child should do).
func (p *watchProcess) exited() <-chan struct{} {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.exitCh
}

// takeExit reports and clears a child exit so the watch loop does not
// spin on the closed channel.
func (p *watchProcess) takeExit() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	err := p.exitErr
	p.exitErr = nil
	p.exitCh = nil

	return err
}

// runWatchLoop runs `go run . [-- extra...]` and restarts it on *.go changes.
// Debounced at 500ms; watches CWD recursively (skips .git/bin/vendor).
func runWatchLoop(extra []string) int {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Fprintf(os.Stderr, "sicky run --watch: %s (run without --watch instead)\n", err.Error())

		return 1
	}

	defer func() { _ = watcher.Close() }()

	root, _ := os.Getwd()
	var dirs []string
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // unreadable paths are skipped by design; watch setup is best-effort
		}

		if !d.IsDir() {
			return nil
		}

		base := filepath.Base(p)
		if base == ".git" || base == "bin" || base == "vendor" || base == ".idea" {
			return filepath.SkipDir
		}

		dirs = append(dirs, p)

		return nil
	})
	for _, d := range dirs {
		_ = watcher.Add(d)
	}

	fmt.Println("  ▸ watching *.go (Ctrl-C to stop)")

	// The child is in its own process group (see watchProcess), so the
	// terminal no longer delivers Ctrl-C to it: forward it here.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)

	var child watchProcess
	watchArgs := append([]string{"run", "."}, extra...)
	child.start("go", watchArgs)

	var timer *time.Timer
	restart := func() {
		if timer != nil {
			timer.Stop()
		}

		timer = time.AfterFunc(500*time.Millisecond, func() {
			fmt.Println("  ↻ change detected, restarting…")
			child.stop(watchStopGrace)
			child.start("go", watchArgs)
		})
	}

	defer child.stop(watchStopGrace)

	for {
		select {
		case ev, ok := <-watcher.Events:
			if !ok {
				return 0
			}

			if strings.HasSuffix(ev.Name, ".go") {
				restart()
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return 0
			}

			fmt.Fprintf(os.Stderr, "sicky run --watch: %s\n", err.Error())
		case <-child.exited():
			// The child ended on its own (build error, crash). Report it
			// instead of silently watching a port nobody is listening on;
			// a child we stopped ourselves is not news.
			err := child.takeExit()
			if child.running() {
				fmt.Fprintf(os.Stderr, "  ▸ process exited: %v\n", err)
			}
		case sig := <-signals:
			fmt.Printf("\n  ▸ %s received, stopping\n", sig)

			return 0
		}
	}
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
