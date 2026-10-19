/**
 * @file model.go
 * @package cli
 * @author Dr.NP <np@herewe.tech>
 * @since 10/19/2026
 */

package cli

import (
	"fmt"
	"strings"
)

// modelSpec is one bun resource `sicky new --model` generates. Name is the
// command-line spelling, ExportedName the Go identifier derived from it, and
// Table the PostgreSQL table and index prefix.
type modelSpec struct {
	Name         string
	ExportedName string
	Table        string
}

// modelSpecs turns the --model values into specs. Every name lands in a file
// name and inside generated Go string literals, so each one goes through
// validateSchematicName; "auth" is rejected because the auth example already
// declares a handler and a service under that name.
func modelSpecs(names []string) ([]modelSpec, error) {
	out := make([]modelSpec, 0, len(names))

	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		if err := validateSchematicName(name); err != nil {
			return nil, err
		}

		if strings.EqualFold(name, "auth") {
			return nil, fmt.Errorf("model name %q is reserved for the auth example", name)
		}

		out = append(out, modelSpec{Name: name, ExportedName: exportName(name), Table: tableName(name)})
	}

	return out, nil
}

// tableName normalizes a model name into a PostgreSQL identifier: lower case,
// with the separators exportName splits on mapped to underscores.
func tableName(name string) string {
	return strings.ToLower(strings.NewReplacer("-", "_", ".", "_").Replace(name))
}

// modelDeclarations renders the per-model request and response fragments.
// They carry declarations only (no package clause) so they can be appended to
// the aggregated DTO files, and the caller runs the header/format pipeline
// once on the combined source.
func modelDeclarations(nc *newContext) (request, response string, err error) {
	var requests, responses strings.Builder

	for _, m := range nc.Models {
		data := modelTemplateData{modelSpec: m, Module: nc.Module, WithAuth: nc.WithAuth}

		req, err := renderProjectTemplate("template/entity/request.decls.go.gotmpl", data)
		if err != nil {
			return "", "", fmt.Errorf("template %s: %w", "template/entity/request.decls.go.gotmpl", err)
		}

		resp, err := renderProjectTemplate("template/entity/response.decls.go.gotmpl", data)
		if err != nil {
			return "", "", fmt.Errorf("template %s: %w", "template/entity/response.decls.go.gotmpl", err)
		}

		requests.WriteString(req)
		responses.WriteString(resp)
	}

	return requests.String(), responses.String(), nil
}

// modelTemplateData is the render context of one entity template. The model
// spec is embedded so {{.Name}} names the resource, not the project, while
// Module and WithAuth come from the surrounding scaffold.
type modelTemplateData struct {
	modelSpec

	Module   string
	WithAuth bool
}
