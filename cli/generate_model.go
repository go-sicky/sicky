/**
 * @file generate_model.go
 * @package cli
 * @author Dr.NP <np@herewe.tech>
 * @since 10/19/2026
 */

package cli

import (
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/spf13/pflag"
)

// generateModel adds one bun resource to an existing standard project: the
// model, its service and its HTTP handler, the request/response DTO
// declarations, and — unless --no-wire — the config and main.go wiring.
//
// Every check runs before the first file is written. A project that does not
// carry the standard HTTP layering, a main.go the wiring cannot understand, or
// an existing resource file without --force all fail without leaving a partial
// model behind.
func generateModel(args []string) int {
	fs := pflag.NewFlagSet("generate model", pflag.ContinueOnError)
	output := fs.StringP("output", "o", "", "Project directory (default: current directory)")
	force := fs.Bool("force", false, "Overwrite existing model files")
	noWire := fs.Bool("no-wire", false, "Write files only; do not edit config.json or main.go")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "sicky generate model: %s\n", err.Error())

		return 1
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "sicky generate model: missing model name")
		fmt.Fprintln(os.Stderr, "Usage: sicky generate model <name> [--output <dir>] [--force] [--no-wire]")

		return 1
	}

	if fs.NArg() > 1 {
		fmt.Fprintf(os.Stderr, "sicky generate model: unexpected argument %q\n", fs.Arg(1))

		return 1
	}

	// The shared validator owns the reserved names (auth, http), the
	// identifier grammar and the table-name collisions, so generate model
	// accepts exactly the names `sicky new --model` accepts.
	specs, err := modelSpecs([]string{fs.Arg(0)})
	if err != nil {
		fmt.Fprintf(os.Stderr, "sicky generate model: %s\n", err.Error())

		return 1
	}

	if len(specs) != 1 {
		fmt.Fprintln(os.Stderr, "sicky generate model: missing model name")

		return 1
	}

	spec := specs[0]

	dir := *output
	if dir == "" {
		dir = "."
	}

	module, err := preflightModelProject(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sicky generate model: %s\n", err.Error())

		return 1
	}

	resources := modelResourceFiles(dir, spec)

	// A repeat without --force is all-or-nothing: report the first occupied
	// path and return before anything is written or wired.
	if !*force {
		for _, resource := range resources {
			if _, err := os.Stat(resource.path); err == nil {
				fmt.Fprintf(os.Stderr, "sicky generate model: %s exists (use --force)\n", resource.path)

				return 1
			}
		}
	}

	for _, sub := range []string{"model", "service"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "sicky generate model: %s\n", err.Error())

			return 1
		}
	}

	// Identity is resolved once so the three files agree even when license
	// detection walks the tree; renderGenerateTemplate would repeat it per
	// file.
	gc := &generateContext{
		Name:         spec.Name,
		ExportedName: spec.ExportedName,
		Module:       module,
		Table:        spec.Table,
		WithAuth:     mainGoHasValidateSession(filepath.Join(dir, "handler", "http.go")),
	}
	fillGenerateIdentity(gc)

	for _, resource := range resources {
		data := *gc
		data.Brief = resource.brief

		if err := renderGenerateTemplate(resource.template, resource.path, &data); err != nil {
			fmt.Fprintf(os.Stderr, "sicky generate model: %s\n", err.Error())

			return 1
		}

		fmt.Printf("  \u2714 Created %s\n", resource.path)
	}

	// The aggregated DTO files carry a package and imports already; only the
	// declared types are appended, and appendDecls skips names that exist so
	// a --force rerun cannot duplicate them.
	requestDecls, responseDecls, err := renderModelDeclarations(gc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sicky generate model: %s\n", err.Error())

		return 1
	}

	for _, dto := range []struct {
		path  string
		decls string
	}{
		{filepath.Join(dir, "handler", "request", "http.go"), requestDecls},
		{filepath.Join(dir, "handler", "response", "http.go"), responseDecls},
	} {
		if err := appendDecls(dto.path, dto.decls); err != nil {
			fmt.Fprintf(os.Stderr, "sicky generate model: %s\n", err.Error())

			return 1
		}

		fmt.Printf("  \u2714 Ensured declarations in %s\n", dto.path)
	}

	if *noWire {
		printModelManualWiring(spec)

		return 0
	}

	configPath := filepath.Join(dir, "config.json")
	if err := wireConfigBun(configPath); err != nil {
		fmt.Fprintf(os.Stderr, "sicky generate model: %s\n", err.Error())

		return 1
	}

	fmt.Printf("  \u2714 Wired %s\n", configPath)

	mainPath := filepath.Join(dir, "main.go")
	if err := wireMainGo(mainPath, module, spec.Name, spec.ExportedName); err != nil {
		fmt.Fprintf(os.Stderr, "sicky generate model: %s\n", err.Error())

		return 1
	}

	fmt.Printf("  \u2714 Wired %s\n", mainPath)

	return 0
}

// modelResource is one full file generate model owns.
type modelResource struct {
	path     string
	template string
	brief    string
}

// modelResourceFiles names the three per-resource files in the order the
// command reports them.
func modelResourceFiles(dir string, spec modelSpec) []modelResource {
	return []modelResource{
		{
			path:     filepath.Join(dir, "model", spec.Name+".go"),
			template: "template/entity/model.go.gotmpl",
			brief:    "This file defines the " + spec.ExportedName + " bun model.",
		},
		{
			path:     filepath.Join(dir, "service", spec.Name+".go"),
			template: "template/entity/service.go.gotmpl",
			brief:    "This file defines the " + spec.ExportedName + " service.",
		},
		{
			path:     filepath.Join(dir, "handler", spec.Name+".http.go"),
			template: "template/entity/handler.http.go.gotmpl",
			brief:    "This file defines the HTTP handlers for the " + spec.Name + " resource.",
		},
	}
}

// preflightModelProject checks everything the command needs before it writes:
// the module path, the standard HTTP layering, and a main.go with the anchors
// wireMainGo edits. The returned module path is what the generated imports use.
func preflightModelProject(dir string) (string, error) {
	module, err := moduleFromGoMod(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", err
	}

	missing := make([]string, 0, 3)

	for _, name := range []string{
		filepath.Join("handler", "http.go"),
		filepath.Join("handler", "request", "http.go"),
		filepath.Join("handler", "response", "http.go"),
	} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			missing = append(missing, filepath.ToSlash(name))
		}
	}

	if len(missing) > 0 {
		return "", fmt.Errorf(
			"missing %s; generate model needs the standard HTTP layering (create the files, or scaffold with `sicky new --http`)",
			strings.Join(missing, ", "),
		)
	}

	if err := checkGenerateMainGo(filepath.Join(dir, "main.go")); err != nil {
		return "", err
	}

	return module, nil
}

// moduleFromGoMod reads the module directive of the project being extended.
// detectModule reads the process working directory, which is not necessarily
// the directory --output names.
func moduleFromGoMod(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}

	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "module" {
			return fields[1], nil
		}
	}

	return "", errors.New("go.mod has no module directive")
}

// checkGenerateMainGo parses main.go and verifies the anchors wireMainGo needs.
// It is the preflight half of wireMainGo: the same messages, but no edits and
// no write, so a rejected run cannot leave a half-wired project behind.
func checkGenerateMainGo(path string) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read main.go: %w", err)
	}

	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("main.go does not look like a sicky scaffold: %w", err)
	}

	mainFn := funcDeclaration(file, "main")
	if mainFn == nil || mainFn.Body == nil {
		return errors.New("main.go does not look like a sicky scaffold: func main")
	}

	initCall := findInitCall(mainFn)
	if initCall == nil {
		return errors.New("main.go does not look like a sicky scaffold: sicky.Init")
	}

	if optionsComposite(initCall) == nil {
		return errors.New("main.go does not look like a sicky scaffold: sicky.Options")
	}

	return nil
}

// renderModelDeclarations renders the request and response fragments for one
// resource. They are declarations only: no package clause, no imports, and no
// header, because appendDecls merges them into the aggregated DTO files.
func renderModelDeclarations(data *generateContext) (request, response string, err error) {
	request, err = renderEntityFragment("template/entity/request.decls.go.gotmpl", data)
	if err != nil {
		return "", "", err
	}

	response, err = renderEntityFragment("template/entity/response.decls.go.gotmpl", data)
	if err != nil {
		return "", "", err
	}

	return request, response, nil
}

// renderEntityFragment renders a template without the header/footer/format
// pipeline. The fragment is not a file yet; the target file's format pass runs
// after appendDecls merges it.
func renderEntityFragment(name string, data any) (string, error) {
	raw, err := generateTemplates.ReadFile(name)
	if err != nil {
		return "", fmt.Errorf("template %q: %w", name, err)
	}

	tmpl, err := template.New(filepath.Base(name)).Parse(string(raw))
	if err != nil {
		return "", fmt.Errorf("template %q parse: %w", name, err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render %s: %w", name, err)
	}

	return buf.String(), nil
}

// printModelManualWiring lists the three places --no-wire leaves untouched.
func printModelManualWiring(spec modelSpec) {
	fmt.Println()
	fmt.Println("  Manual wiring required (--no-wire):")
	fmt.Println("    1. config.json: add sicky.infra.bun (driver + dsn).")
	fmt.Printf("    2. main.go: add MustInfra: []string{\"bun\"}, the --initdb FlagSwitch, and (model.%s{}).InitDB(ctx, false) inside initdb().\n", spec.ExportedName)
	fmt.Printf("    3. main.go: register the handler with httpSrv.Handle(handler.NewHTTP%s()).\n", spec.ExportedName)
}
