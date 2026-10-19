/*
 * The MIT License (MIT)
 *
 * Copyright (c) 2024 HereweTech Co.LTD
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy of
 * this software and associated documentation files (the "Software"), to deal in
 * the Software without restriction, including without limitation the rights to
 * use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
 * the Software, and to permit persons to whom the Software is furnished to do so,
 * subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
 * FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
 * COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
 * IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
 * CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 */

/**
 * @file generate.go
 * @package cli
 * @author Dr.NP <np@herewe.tech>
 * @since 07/06/2026
 */

package cli

import (
	"embed"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/spf13/pflag"
)

//go:embed template/handler/*.gotmpl template/tool/*.gotmpl template/resource/*.gotmpl template/prompt/*.gotmpl template/doc/*.gotmpl template/entity/*.gotmpl
var generateTemplates embed.FS

func generateRun(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "sicky generate: missing schematic name")
		fmt.Fprintln(os.Stderr, "Usage: sicky generate <schematic> [name] [flags]")
		fmt.Fprintln(os.Stderr, "Schematics: handler, model, tool, resource, prompt, doc, server, client, service, broker, job, middleware, proto, config, docker, k8s")

		return 1
	}

	schematic := strings.ToLower(args[0])
	schematicArgs := args[1:]

	switch schematic {
	case "handler":
		return generateHandler(schematicArgs)
	case "model":
		return generateModel(schematicArgs)
	case "tool":
		return generateTool(schematicArgs)
	case "resource":
		return generateResource(schematicArgs)
	case "prompt":
		return generatePrompt(schematicArgs)
	case "doc":
		return generateDoc(schematicArgs)
	case "server":
		return generateServerStub(schematicArgs)
	case "client":
		return generateClientStub(schematicArgs)
	case "service":
		return generateServiceStub(schematicArgs)
	case "broker":
		return generateBrokerStub(schematicArgs)
	case "job":
		return generateJobStub(schematicArgs)
	case "middleware":
		return generateMiddlewareStub(schematicArgs)
	case "proto":
		return generateProtoFile(schematicArgs)
	case "config":
		return generateConfigFile(schematicArgs)
	case "docker":
		return generateDockerFiles(schematicArgs)
	case "k8s":
		return generateK8sFiles(schematicArgs)
	default:
		fmt.Fprintf(os.Stderr, "sicky generate: unknown schematic %q\n", schematic)
		fmt.Fprintln(os.Stderr, "Schematics: handler, model, tool, resource, prompt, doc, server, client, service, broker, job, middleware, proto, config, docker, k8s")

		return 1
	}
}

// generateContext is the data every `sicky generate` template renders
// from. SPDX, Holder and Author are not set by callers: renderGenerateTemplate
// fills them from the surrounding project, so a file generated inside an
// existing project joins that project's license instead of contradicting it.
type generateContext struct {
	Name         string
	ExportedName string
	// Route is the handler's Name() value derived from the command-line name
	// and its transport: both "key" and "key.http" with --type http yield
	// "key.http". Only the HTTP handler template reads it; the Fiber template
	// keeps appending its own ".fiber" suffix to Name.
	Route  string
	Module string
	SPDX   string
	Holder string
	Author string
	// Brief is the @brief sentence stamped on the generated header. It is
	// derived from the template path when a caller leaves it empty; callers
	// that know more about the file set it themselves.
	Brief string
	// Table is the PostgreSQL table a generated model owns. Only the entity
	// templates read it; every other schematic leaves it empty.
	Table string
	// WithAuth gates the validateSession middleware in a generated resource
	// handler. It mirrors the project's own handler/http.go.
	WithAuth bool
	// Root is the project directory identity is resolved from. Empty means
	// the process working directory, which is what every schematic that
	// generates in place wants; `generate model --output` sets it so the
	// generated headers describe the target project, not the caller.
	Root string
}

func generateHandler(args []string) int {
	fs := pflag.NewFlagSet("generate handler", pflag.ContinueOnError)
	transport := fs.String("type", "fiber", "Handler transport: fiber, http")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "sicky generate handler: %s\n", err.Error())

		return 1
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "sicky generate handler: missing handler name")

		return 1
	}

	kind := strings.ToLower(strings.TrimSpace(*transport))
	var tmplName string
	switch kind {
	case "fiber":
		tmplName = "template/handler/handler.fiber.go.gotmpl"
	case "http":
		tmplName = "template/handler/handler.http.go.gotmpl"
	default:
		fmt.Fprintf(os.Stderr, "sicky generate handler: unknown type %q (valid: fiber, http)\n", *transport)

		return 1
	}

	name := fs.Arg(0)
	ident, route := handlerIdent(name, kind)
	gc := &generateContext{
		Name:         name,
		ExportedName: ident,
		Route:        route,
		Module:       detectModule(),
	}

	dir := "handler"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "sicky generate handler: %s\n", err.Error())

		return 1
	}

	outputPath := filepath.Join(dir, name+".go")
	if err := renderGenerateTemplate(tmplName, outputPath, gc); err != nil {
		fmt.Fprintf(os.Stderr, "sicky generate handler: %s\n", err.Error())

		return 1
	}

	fmt.Printf("  \u2714 Created %s (%s)\n", outputPath, kind)

	return 0
}

// handlerIdent derives the exported type name and the Name() route from the
// command-line name and its transport. The transport may already be spelled
// in the name ("key.http"), so the suffix is stripped before it is exported
// and then re-attached exactly once.
func handlerIdent(name, transport string) (ident, route string) {
	base := strings.TrimSuffix(name, "."+transport)
	route = base + "." + transport

	switch transport {
	case "http":
		ident = "HTTP" + exportName(base)
	case "fiber":
		ident = exportName(base) + "Fiber"
	}

	return ident, route
}

func generateTool(args []string) int {
	fs := pflag.NewFlagSet("generate tool", pflag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "sicky generate tool: %s\n", err.Error())

		return 1
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "sicky generate tool: missing tool name")

		return 1
	}

	name := fs.Arg(0)
	gc := &generateContext{
		Name:         name,
		ExportedName: exportName(name),
		Module:       detectModule(),
	}

	dir := "tool"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "sicky generate tool: %s\n", err.Error())

		return 1
	}

	outputPath := filepath.Join(dir, name+".go")
	if err := renderGenerateTemplate("template/tool/tool.go.gotmpl", outputPath, gc); err != nil {
		fmt.Fprintf(os.Stderr, "sicky generate tool: %s\n", err.Error())

		return 1
	}

	fmt.Printf("  \u2714 Created %s\n", outputPath)

	return 0
}

func generateResource(args []string) int {
	fs := pflag.NewFlagSet("generate resource", pflag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "sicky generate resource: %s\n", err.Error())

		return 1
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "sicky generate resource: missing resource name")

		return 1
	}

	name := fs.Arg(0)
	gc := &generateContext{
		Name:         name,
		ExportedName: exportName(name),
		Module:       detectModule(),
	}

	dir := "resource"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "sicky generate resource: %s\n", err.Error())

		return 1
	}

	outputPath := filepath.Join(dir, name+".go")
	if err := renderGenerateTemplate("template/resource/resource.go.gotmpl", outputPath, gc); err != nil {
		fmt.Fprintf(os.Stderr, "sicky generate resource: %s\n", err.Error())

		return 1
	}

	fmt.Printf("  \u2714 Created %s\n", outputPath)

	return 0
}

func generatePrompt(args []string) int {
	fs := pflag.NewFlagSet("generate prompt", pflag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "sicky generate prompt: %s\n", err.Error())

		return 1
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "sicky generate prompt: missing prompt name")

		return 1
	}

	name := fs.Arg(0)
	gc := &generateContext{
		Name:         name,
		ExportedName: exportName(name),
		Module:       detectModule(),
	}

	dir := "prompt"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "sicky generate prompt: %s\n", err.Error())

		return 1
	}

	outputPath := filepath.Join(dir, name+".go")
	if err := renderGenerateTemplate("template/prompt/prompt.go.gotmpl", outputPath, gc); err != nil {
		fmt.Fprintf(os.Stderr, "sicky generate prompt: %s\n", err.Error())

		return 1
	}

	fmt.Printf("  \u2714 Created %s\n", outputPath)

	return 0
}

func generateDoc(args []string) int {
	fs := pflag.NewFlagSet("generate doc", pflag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "sicky generate doc: %s\n", err.Error())

		return 1
	}

	// The README renders from the same context `sicky new` uses. An existing
	// project has no scaffold context left, so the selection is read back from
	// the files that selection produced.
	dc := detectDocContext()

	outputPath := "README.md"
	if err := renderGenerateTemplate("template/doc/readme.md.gotmpl", outputPath, dc); err != nil {
		fmt.Fprintf(os.Stderr, "sicky generate doc: %s\n", err.Error())

		return 1
	}

	fmt.Printf("  \u2714 Created %s\n", outputPath)

	return 0
}

func renderGenerateTemplate(tmplName, outputPath string, data any) error {
	content, err := renderGenerateContent(tmplName, outputPath, data)
	if err != nil {
		return err
	}

	if err := writeAtomic(outputPath, []byte(content)); err != nil {
		return fmt.Errorf("create %s: %w", outputPath, err)
	}

	return nil
}

// renderGenerateContent runs the template, header, footer and format pipeline
// without touching the disk. Callers that must write several files run every
// render to bytes first, so a rejected run leaves the tree untouched.
func renderGenerateContent(tmplName, outputPath string, data any) (string, error) {
	// Both the destination and the name interpolated into the template
	// come from the command line: a "../" name would write outside the
	// working tree, and a name that is not an identifier can inject
	// statements into the generated file (gofmt only rejects syntax that
	// does not parse, not syntax that is valid but malicious).
	if err := checkWritePath(outputPath); err != nil {
		return "", err
	}

	if gc, ok := data.(*generateContext); ok {
		if err := validateSchematicName(gc.Name); err != nil {
			return "", fmt.Errorf("template %s: %w", tmplName, err)
		}
	}

	tmplBytes, err := generateTemplates.ReadFile(tmplName)
	if err != nil {
		return "", fmt.Errorf("template %q: %w", tmplName, err)
	}

	tmpl, err := template.New(filepath.Base(tmplName)).Parse(string(tmplBytes))
	if err != nil {
		return "", fmt.Errorf("template %q parse: %w", tmplName, err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render %s: %w", outputPath, err)
	}

	content := buf.String()

	// Header and footer policy lives here rather than in the templates:
	// one place decides, so a template added later cannot ship without a
	// header and the four .go templates cannot drift from the project files
	// `sicky new` writes.
	if gc, ok := data.(*generateContext); ok {
		if gc.SPDX == "" {
			fillGenerateIdentity(gc)
		}

		// The schematic is the middle segment of the embedded template path
		// (template/handler/...), which is the only place that still knows
		// it this deep; generateRun's local is long gone by now.
		if gc.Brief == "" {
			if schematic := templateSchematic(tmplName); schematic != "" {
				gc.Brief = "Generated " + schematic + " " + gc.Name + "."
			}
		}

		fm := newFileMeta(gc.Name, gc.SPDX, gc.Holder, gc.Author)
		fm.File = filepath.ToSlash(outputPath)
		fm.Package = packageOf(content)
		fm.Brief = gc.Brief

		content = injectFileHeader(outputPath, content, &fm)
		content = injectFileFooter(outputPath, content)
	}

	if strings.HasSuffix(outputPath, ".go") {
		formatted, ferr := format.Source([]byte(content))
		if ferr != nil {
			return "", fmt.Errorf("render %s: format: %w", outputPath, ferr)
		}

		content = string(formatted)
	}

	return content, nil
}

// templateSchematic returns the schematic an embedded generate template
// belongs to: "template/handler/handler.fiber.go.gotmpl" yields "handler".
// An unrecognized path yields "", and the caller then omits @brief rather
// than stamping a descriptor it cannot name.
func templateSchematic(tmplName string) string {
	parts := strings.Split(filepath.ToSlash(tmplName), "/")
	if len(parts) < 2 {
		return ""
	}

	return parts[1]
}

// fillGenerateIdentity resolves the license a generated file should carry
// and the identity to stamp on it. Detection reads the working tree, so it
// runs once per generated file rather than once per process — a generate
// command writes one file.
func fillGenerateIdentity(gc *generateContext) {
	license := detectProjectLicenseAt(gc.Root)

	gc.SPDX = license.spdx
	gc.Holder = detectHolderAt(gc.Root)
	gc.Author = detectAuthorAt(gc.Root)
}

func detectModule() string {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		return "github.com/myorg/my-app"
	}

	lines := strings.SplitSeq(string(data), "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "module "); ok {
			return strings.TrimSpace(after)
		}
	}

	return "github.com/myorg/my-app"
}

// detectDocContext reconstructs the scaffold selection from the project on
// disk, so `sicky generate doc` renders the same README `sicky new` would for
// this tree. A file that is absent simply means that option was not selected.
func detectDocContext() *newContext {
	nc := &newContext{
		Name:      detectProjectName(),
		Type:      detectDocProjectType(),
		Module:    detectModule(),
		WithFiber: fileExists("handler/fiber.go"),
		WithGRPC:  fileExists("handler/grpc.go"),
		WithHTTP:  fileExists("handler/http.go"),
		WithAuth:  fileExists("handler/auth.http.go"),
	}
	nc.ExportedName = exportName(nc.Name)
	nc.Models = detectDocModels()

	return nc
}

// detectDocProjectType reads the service constructor out of main.go. Failing
// to read the file yields the standard layout, which is the only type with the
// layered documentation this template can describe without hints.
func detectDocProjectType() string {
	data, err := os.ReadFile("main.go")
	if err != nil {
		return "standard"
	}

	src := string(data)

	switch {
	case strings.Contains(src, "service/interactive"):
		return "interactive"
	case strings.Contains(src, "service/mcp"):
		return "mcp"
	default:
		return "standard"
	}
}

// detectDocModels lists the generated model files. Only the documentation
// fields are needed: the resource name and its exported identifier.
func detectDocModels() []modelSpec {
	entries, err := os.ReadDir("model")
	if err != nil {
		return nil
	}

	var models []modelSpec

	for _, entry := range entries {
		name, ok := strings.CutSuffix(entry.Name(), ".go")
		if !ok || entry.IsDir() || strings.HasSuffix(name, "_test") {
			continue
		}

		models = append(models, modelSpec{Name: name, ExportedName: exportName(name), Table: tableName(name)})
	}

	return models
}

// fileExists reports whether path names an existing non-directory. Rebuilding
// a scaffold context treats absence as "not selected", never as an error.
func fileExists(path string) bool {
	info, err := os.Stat(path)

	return err == nil && !info.IsDir()
}

func detectProjectName() string {
	module := detectModule()
	parts := strings.Split(module, "/")

	return parts[len(parts)-1]
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
