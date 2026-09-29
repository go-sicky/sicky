/**
 * @file run_forward_test.go
 * @package cli
 * @author Dr.NP <np@herewe.tech>
 * @since 09/30/2026
 */

package cli

import (
	"slices"
	"strings"
	"testing"
)

// countSep returns how many times "--" appears in args.
func countSep(args []string) int {
	n := 0
	for _, a := range args {
		if a == "--" {
			n++
		}
	}

	return n
}

func TestBuildForwardArgsOmitsDefaults(t *testing.T) {
	got := buildForwardArgs("", "", nil)
	if want := []string{"run", "."}; !slices.Equal(got, want) {
		t.Fatalf("bare invocation = %v, want %v", got, want)
	}

	// The flag defaults are "config" and "json"; naming either explicitly
	// must not add a separator, because nothing is being forwarded.
	for _, tc := range []struct {
		name       string
		config     string
		configType string
		want       []string
	}{
		{"default config name", "config", "", []string{"run", "."}},
		{"default config type", "", "json", []string{"run", "."}},
		{"both defaults", "config", "json", []string{"run", "."}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := buildForwardArgs(tc.config, tc.configType, nil)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("= %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBuildForwardArgsEmitsOneSeparator(t *testing.T) {
	for _, tc := range []struct {
		name       string
		config     string
		configType string
		rest       []string
		want       []string
	}{
		{
			name:   "config only, no rest",
			config: "app.yaml",
			rest:   nil,
			want:   []string{"run", ".", "--", "--config", "app.yaml"},
		},
		{
			name:       "config only, with rest",
			config:     "app.yaml",
			rest:       []string{"--port", "8080"},
			want:       []string{"run", ".", "--", "--config", "app.yaml", "--port", "8080"},
			configType: "",
		},
		{
			name:       "config type only, with rest",
			configType: "yaml",
			rest:       []string{"--port", "8080"},
			want:       []string{"run", ".", "--", "--config-type", "yaml", "--port", "8080"},
		},
		{
			name: "no flags, with rest",
			rest: []string{"--port", "8080"},
			want: []string{"run", ".", "--", "--port", "8080"},
		},
		{
			name:       "both flags, with rest",
			config:     "app.yaml",
			configType: "yaml",
			rest:       []string{"--port", "8080"},
			want: []string{
				"run", ".", "--", "--config", "app.yaml",
				"--config-type", "yaml", "--port", "8080",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := buildForwardArgs(tc.config, tc.configType, tc.rest)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("= %v, want %v", got, tc.want)
			}
		})
	}
}

// TestBuildForwardArgsNeverLeaksASecondSeparator is the C15 regression.
// The forwarded block reads ["run", ".", "--", "--config", "app.yaml"],
// so the old "is the second-from-last element --?" probe looked at
// "--config", concluded no separator was present, and appended a second
// "--" that the business binary then read as a positional argument.
func TestBuildForwardArgsNeverLeaksASecondSeparator(t *testing.T) {
	rest := []string{"--port", "8080"}

	for _, tc := range []struct {
		name       string
		config     string
		configType string
	}{
		{"config only", "app.yaml", ""},
		{"config type only", "", "yaml"},
		{"both flags", "app.yaml", "yaml"},
		{"neither flag", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := buildForwardArgs(tc.config, tc.configType, rest)
			if n := countSep(got); n != 1 {
				t.Fatalf("%d separators in %v, want exactly 1: a second -- lands in the child's positional args", n, got)
			}

			// The separator must also come before the business args, not
			// between them.
			sep := slices.Index(got, "--")
			if sep == -1 {
				t.Fatalf("no separator in %v", got)
			}

			for i := sep + 1; i < len(got); i++ {
				if got[i] == "--" {
					t.Fatalf("stray separator at %d in %v", i, got)
				}
			}

			tail := strings.Join(got[sep+1:], " ")
			if !strings.HasSuffix(tail, "--port 8080") {
				t.Fatalf("business args are not the tail of %v", got)
			}
		})
	}
}
