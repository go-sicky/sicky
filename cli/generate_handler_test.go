/**
 * @file generate_handler_test.go
 * @package cli
 * @author Dr.NP <np@herewe.tech>
 * @since 10/19/2026
 */

package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestGenerateHandlerHTTPNaming pins the reference-project naming: the
// exported identifier gains an HTTP prefix while Name() reports the route,
// whether or not the user spelled the transport in the argument.
func TestGenerateHandlerHTTPNaming(t *testing.T) {
	for _, input := range []string{"key", "key.http"} {
		t.Run(input, func(t *testing.T) {
			dir := t.TempDir()
			runGenerate(t, dir, "handler", input, "--type", "http")

			path := filepath.Join(dir, "handler", input+".go")
			src := readFile(t, path)

			for _, want := range []string{"type HTTPKey struct{}", "func NewHTTPKey()", `return "key.http"`} {
				if !strings.Contains(src, want) {
					t.Errorf("%s missing %q", input, want)
				}
			}

			if strings.Contains(src, "KeyHttp") {
				t.Errorf("%s still uses the old KeyHttp naming", input)
			}
		})
	}
}

// TestGenerateHandlerFiberNamingKeepsTheSuffix pins the decision that the
// HTTP rename leaves Fiber alone: the template still appends ".fiber" itself.
func TestGenerateHandlerFiberNamingKeepsTheSuffix(t *testing.T) {
	dir := t.TempDir()
	runGenerate(t, dir, "handler", "key", "--type", "fiber")

	src := readFile(t, filepath.Join(dir, "handler", "key.go"))

	for _, want := range []string{"type KeyFiber struct{}", "func NewKeyFiber()", `return "key.fiber"`} {
		if !strings.Contains(src, want) {
			t.Errorf("key missing %q", want)
		}
	}
}
