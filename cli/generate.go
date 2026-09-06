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

//go:embed template/handler/*.gotmpl template/tool/*.gotmpl template/resource/*.gotmpl template/prompt/*.gotmpl template/doc/*.gotmpl
var generateTemplates embed.FS

func generateRun(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "sicky generate: missing schematic name")
		fmt.Fprintln(os.Stderr, "Usage: sicky generate <schematic> [name] [flags]")
		fmt.Fprintln(os.Stderr, "Schematics: handler, tool, resource, prompt, doc, server, client, service, broker, job, middleware, proto, config, docker, k8s")

		return 1
	}

	schematic := strings.ToLower(args[0])
	schematicArgs := args[1:]

	switch schematic {
	case "handler":
		return generateHandler(schematicArgs)
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
		fmt.Fprintln(os.Stderr, "Schematics: handler, tool, resource, prompt, doc, server, client, service, broker, job, middleware, proto, config, docker, k8s")

		return 1
	}
}

type generateContext struct {
	Name         string
	ExportedName string
	Module       string
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
	gc := &generateContext{
		Name:         name,
		ExportedName: exportName(name),
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

	gc := &generateContext{
		Name:         detectProjectName(),
		ExportedName: exportName(detectProjectName()),
		Module:       detectModule(),
	}

	outputPath := "README.md"
	if err := renderGenerateTemplate("template/doc/readme.md.gotmpl", outputPath, gc); err != nil {
		fmt.Fprintf(os.Stderr, "sicky generate doc: %s\n", err.Error())

		return 1
	}

	fmt.Printf("  \u2714 Created %s\n", outputPath)

	return 0
}

func renderGenerateTemplate(tmplName, outputPath string, data any) error {
	tmplBytes, err := generateTemplates.ReadFile(tmplName)
	if err != nil {
		return fmt.Errorf("template %q: %w", tmplName, err)
	}

	tmpl, err := template.New(filepath.Base(tmplName)).Parse(string(tmplBytes))
	if err != nil {
		return fmt.Errorf("template %q parse: %w", tmplName, err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("render %s: %w", outputPath, err)
	}

	content := buf.String()
	if strings.HasSuffix(outputPath, ".go") {
		formatted, ferr := format.Source([]byte(content))
		if ferr != nil {
			return fmt.Errorf("render %s: format: %w", outputPath, ferr)
		}

		content = string(formatted)
	}

	if err := os.WriteFile(outputPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("create %s: %w", outputPath, err)
	}

	return nil
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
