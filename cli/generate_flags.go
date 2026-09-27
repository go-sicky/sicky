/**
 * @file generate_flags.go
 * @package cli
 * @since 09/06/2026
 */

package cli

import (
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"
)

// generateFlags carries the common flags shared by all
// `sicky generate <schematic>` stubs: output dir + force overwrite.
type generateFlags struct {
	output string
	force  bool
}

func parseGenerateFlags(name string, args []string) (*pflag.FlagSet, *generateFlags, error) {
	fs := pflag.NewFlagSet(name, pflag.ContinueOnError)
	gf := &generateFlags{}
	fs.StringVarP(&gf.output, "output", "o", "", "Output directory (default per schematic)")
	fs.BoolVar(&gf.force, "force", false, "Overwrite existing files")

	if err := fs.Parse(args); err != nil {
		return nil, nil, err
	}

	return fs, gf, nil
}

// writeGuard writes content to path, refusing to overwrite unless --force.
// Returns a process exit code (0 ok, 1 exists/error) for direct return.
func writeGuard(path, content string, gf *generateFlags, kind string) int {
	// The path is built from command-line arguments; a name carrying
	// "../" would otherwise write outside the working tree.
	if err := checkWritePath(path); err != nil {
		fmt.Fprintf(os.Stderr, "sicky generate %s: %s\n", kind, err.Error())

		return 1
	}

	// A generated .go file that does not parse breaks `go build` for a
	// reason nobody can see from the command that produced it: refuse to
	// write it rather than leave a broken package behind.
	if strings.HasSuffix(path, ".go") {
		if _, err := format.Source([]byte(content)); err != nil {
			fmt.Fprintf(os.Stderr, "sicky generate %s: %s\n", kind, err.Error())

			return 1
		}
	}

	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "sicky generate %s: %s\n", kind, err.Error())

			return 1
		}
	}

	if gf != nil && !gf.force {
		if _, err := os.Stat(path); err == nil {
			fmt.Fprintf(os.Stderr, "sicky generate %s: %s exists (use --force)\n", kind, path)

			return 1
		}
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "sicky generate %s: %s\n", kind, err.Error())

		return 1
	}

	fmt.Printf("  ✔ Created %s\n", path)

	return 0
}
