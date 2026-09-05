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
	forward := []string{"run", "."}
	if *configFlag != "" && *configFlag != "config" {
		forward = append(forward, "--", "--config", *configFlag)
		// NOTE: `go run . -- --config x` keeps sicky.Init pflag parsing intact.
		// Default value is omitted so `go run .` stays clean.
		// --config-type must also be forwarded when non-default, otherwise
		// a custom --config with e.g. yaml would silently parse as json.
		if *configTypeFlag != "" && *configTypeFlag != "json" {
			forward = append(forward, "--config-type", *configTypeFlag)
		}
	} else if *configTypeFlag != "" && *configTypeFlag != "json" {
		forward = append(forward, "--", "--config-type", *configTypeFlag)
	}

	// Any leftover positional args are treated as extra business args.
	// `sicky run -- --port 8080` -> `go run . -- --port 8080`.
	// `sicky run --port 8080` (no -- separator) also forwards for convenience.
	rest := fs.Args()
	if len(rest) > 0 {
		if len(forward) >= 2 && forward[len(forward)-2] == "--" {
			forward = append(forward, rest...)
		} else {
			forward = append(forward, "--")
			forward = append(forward, rest...)
		}
	}

	if *watchFlag {
		return runWatch(forward)
	}

	cmd := exec.Command("go", forward...)
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
