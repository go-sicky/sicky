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
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

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

	var cmd *exec.Cmd
	start := func() {
		args := append([]string{"run", "."}, extra...)
		cmd = exec.Command("go", args...) //nolint:gosec // G204: dev --watch loop; binary is fixed, extra args come from the invoking operator
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "sicky run --watch: %s\n", err.Error())
			cmd = nil
		}
	}

	stop := func() {
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
			cmd = nil
		}
	}

	start()

	var timer *time.Timer
	restart := func() {
		if timer != nil {
			timer.Stop()
		}

		timer = time.AfterFunc(500*time.Millisecond, func() {
			fmt.Println("  ↻ change detected, restarting…")
			stop()
			start()
		})
	}

	for {
		select {
		case ev, ok := <-watcher.Events:
			if !ok {
				stop()

				return 0
			}

			if strings.HasSuffix(ev.Name, ".go") {
				restart()
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				stop()

				return 0
			}

			fmt.Fprintf(os.Stderr, "sicky run --watch: %s\n", err.Error())
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
