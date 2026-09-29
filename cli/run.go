/**
 * @file run.go
 * @package cli
 * @author Dr.NP <np@herewe.tech>
 * @since 09/06/2026
 */

package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/pflag"
)

// buildForwardArgs assembles the `go run` argument list. The "--" that
// separates go run's own flags from the business program's appears at most
// once: it is tracked with hasSep rather than probed in the slice, because
// the forwarded block reads ["run", ".", "--", "--config", "app.yaml"], so
// the second-from-last element is "--config" and probing there appended a
// second "--" that leaked into the child's positional args.
//
// configFlag and configTypeFlag are the raw flag values; empty and the
// defaults ("config" / "json") are omitted so a bare `go run .` stays clean.
// rest are the leftover positional args from the flag set.
func buildForwardArgs(configFlag, configTypeFlag string, rest []string) []string {
	forward := []string{"run", "."}

	hasSep := false

	if configFlag != "" && configFlag != "config" {
		forward = append(forward, "--", "--config", configFlag)
		hasSep = true
		// NOTE: `go run . -- --config x` keeps sicky.Init pflag parsing intact.
		// --config-type must also be forwarded when non-default, otherwise
		// a custom --config with e.g. yaml would silently parse as json.
		if configTypeFlag != "" && configTypeFlag != "json" {
			forward = append(forward, "--config-type", configTypeFlag)
		}
	} else if configTypeFlag != "" && configTypeFlag != "json" {
		forward = append(forward, "--", "--config-type", configTypeFlag)
		hasSep = true
	}

	// Any leftover positional args are treated as extra business args.
	// `sicky run -- --port 8080` -> `go run . -- --port 8080`.
	// `sicky run --port 8080` (no -- separator) also forwards for convenience.
	if len(rest) > 0 {
		if !hasSep {
			forward = append(forward, "--")
		}

		forward = append(forward, rest...)
	}

	return forward
}

// runRun executes the user business project in the current directory by
// delegating to `go run .`. It only passes through framework-relevant flags
// (--config/-C, --config-type); the target binary parses them via sicky.Init.
func runRun(args []string) int {
	fs := pflag.NewFlagSet("run", pflag.ContinueOnError)
	configFlag := fs.StringP("config", "C", "config", "Config definition (forwarded to the business binary)")
	configTypeFlag := fs.String("config-type", "json", "Configuration data format (forwarded to the business binary)")
	watchFlag := fs.BoolP("watch", "w", false, "Watch *.go files and restart on change")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return 0
		}

		fmt.Fprintf(os.Stderr, "sicky run: %s\n", err.Error())

		return 1
	}

	// go run . <forwarded flags> [-- extra args]
	forward := buildForwardArgs(*configFlag, *configTypeFlag, fs.Args())

	if *watchFlag {
		return runWatch(forward)
	}

	cmd := exec.Command("go", forward...) //nolint:gosec // G204: dev CLI; the binary is fixed (go) and forward is built by buildForwardArgs from explicit operator flags
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		exitErr := &exec.ExitError{}
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}

		fmt.Fprintf(os.Stderr, "sicky run: %s\n", err.Error())

		return 1
	}

	return 0
}

// runWatch restarts `go run` whenever a *.go file under CWD changes.
// It uses fsnotify when available; falls back to a clear error otherwise.
// Implemented in run_watch.go so `run.go` stays dependency-light to read.
func runWatch(forward []string) int {
	// Strip leading "run ." prefix for the watcher helper.
	extra := []string{}
	if idx := indexOf(forward, "--"); idx >= 0 {
		extra = forward[idx:]
	}

	return runWatchLoop(extra)
}

func indexOf(ss []string, v string) int {
	for i, s := range ss {
		if strings.TrimSpace(s) == v {
			return i
		}
	}

	return -1
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
