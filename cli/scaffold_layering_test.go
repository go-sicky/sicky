/**
 * @file scaffold_layering_test.go
 * @package cli
 * @author Dr.NP <np@herewe.tech>
 * @since 10/19/2026
 */

package cli

import "testing"

// The template tree mirrors the generated output tree, so the layout a
// maintainer edits is the layout a user sees. That only works while the
// embed directive reaches the nested directories: a pattern that stops at
// one level leaves the handler templates out of the binary, and the failure
// surfaces as a render error in a generated project rather than here.
func TestProjectTemplatesEmbedNestedFiles(t *testing.T) {
	for _, name := range []string{
		"template/project/standard/handler/fiber.go.gotmpl",
		"template/project/standard/handler/http.go.gotmpl",
		"template/project/standard/handler/grpc.go.gotmpl",
		"template/project/mcp/handler/handler.go.gotmpl",
		"template/project/interactive/handler/handler.go.gotmpl",
	} {
		if _, err := projectTemplates.ReadFile(name); err != nil {
			t.Errorf("%s is not embedded: %v", name, err)
		}
	}
}
