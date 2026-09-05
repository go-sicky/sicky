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
 * @file new.go
 * @package cli
 * @author Dr.NP <np@herewe.tech>
 * @since 07/06/2026
 */

package cli

import (
	"bufio"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"unicode"

	"github.com/spf13/pflag"
)

//go:embed template/project/**/*.gotmpl
var projectTemplates embed.FS

type newContext struct {
	Name         string
	ExportedName string
	Module       string
	AppName      string
	Type         string
	WithGRPC     bool
	WithFiber    bool
	WithProto    bool
	OutputDir    string
	// Optional scaffolding hints used by `sicky config init` and
	// `sicky generate config/docker`. Kept for compat; templates
	// render from WithGRPC/WithFiber, these are informational only.
	Servers        []string
	Registry       string
	Tracer         string
	DockerRegistry string
}

// normalizeNewContext fills defaults for scaffolding contexts.
// It never fails today; the error return keeps call-site symmetry.
//
//nolint:unparam // error return is reserved for future validation; callers check it
func normalizeNewContext(nc *newContext) error {
	if nc.Type == "" {
		nc.Type = "standard"
	}

	return nil
}

func newRun(args []string) int {
	fs := pflag.NewFlagSet("new", pflag.ContinueOnError)
	typeFlag := fs.String("type", "", "Service type: standard, mcp, interactive")
	moduleFlag := fs.String("module", "", "Go module path")
	outputFlag := fs.StringP("output", "o", ".", "Output directory")
	noGrpcFlag := fs.Bool("no-grpc", false, "Skip gRPC server")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "sicky new: %s\n", err.Error())

		return 1
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "sicky new: missing project name")
		fmt.Fprintln(os.Stderr, "Usage: sicky new <project-name> [flags]")

		return 1
	}

	nc := &newContext{
		Name:      fs.Arg(0),
		Type:      *typeFlag,
		Module:    *moduleFlag,
		OutputDir: *outputFlag,
		WithGRPC:  !*noGrpcFlag,
		WithFiber: true,
	}

	if nc.Name == "" {
		fmt.Fprintln(os.Stderr, "sicky new: project name cannot be empty")

		return 1
	}

	interactivePrompt(nc)

	nc.AppName = nc.Name + ".example.sicky"
	nc.ExportedName = exportName(nc.Name)

	projectDir := filepath.Join(nc.OutputDir, nc.Name)
	if err := scaffoldProject(projectDir, nc); err != nil {
		fmt.Fprintf(os.Stderr, "sicky new: failed to create project: %s\n", err.Error())

		return 1
	}

	fmt.Println()
	fmt.Printf("  \u2714 Created %s/\n", nc.Name)
	fmt.Println()
	fmt.Println("  Next steps:")
	fmt.Printf("    cd %s\n", nc.Name)
	fmt.Println("    make build")
	fmt.Println("    make run")
	fmt.Println()

	return 0
}

func interactivePrompt(nc *newContext) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Printf("  \u25b8 Creating go-sicky project: %s\n", nc.Name)
	fmt.Println()

	if nc.Type == "" {
		fmt.Println("  ? Service type:")
		fmt.Println("    1. Standard    — full microservice with servers/brokers/jobs")
		fmt.Println("    2. MCP         — Model Context Protocol server")
		fmt.Println("    3. Interactive — CLI REPL service")
		fmt.Print("    [1]: ")

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		switch input {
		case "2":
			nc.Type = "mcp"
		case "3":
			nc.Type = "interactive"
		default:
			nc.Type = "standard"
		}
	}

	if nc.Module == "" {
		defaultModule := "github.com/myorg/" + nc.Name
		fmt.Printf("  ? Module path [%s]: ", defaultModule)

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input != "" {
			nc.Module = input
		} else {
			nc.Module = defaultModule
		}
	}

	switch nc.Type {
	case "standard":
		fmt.Print("  ? Add gRPC server? [Y/n]: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if strings.EqualFold(input, "n") || strings.EqualFold(input, "no") {
			nc.WithGRPC = false
		}

		if nc.WithGRPC {
			nc.WithFiber = true
		} else {
			fmt.Print("  ? Add HTTP/Fiber server? [Y/n]: ")
			input, _ = reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if strings.EqualFold(input, "n") || strings.EqualFold(input, "no") {
				nc.WithFiber = false
			}
		}
	case "mcp":
		nc.WithGRPC = false
		fmt.Print("  ? Add HTTP/Fiber server (for SSE transport)? [Y/n]: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if strings.EqualFold(input, "n") || strings.EqualFold(input, "no") {
			nc.WithFiber = false
		}
	default:
		nc.WithGRPC = false
		nc.WithFiber = false
	}

	if nc.WithGRPC {
		fmt.Print("  ? Generate protobuf files? [Y/n]: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if strings.EqualFold(input, "n") || strings.EqualFold(input, "no") {
			nc.WithProto = false
		} else {
			nc.WithProto = true
		}
	}
}

func scaffoldProject(dir string, nc *newContext) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Join(dir, "handler"), 0o755); err != nil {
		return err
	}

	if nc.WithProto {
		if err := os.MkdirAll(filepath.Join(dir, "proto"), 0o755); err != nil {
			return err
		}
	}

	files := []struct {
		path     string
		template string
	}{
		{filepath.Join(dir, "main.go"), "template/project/" + nc.Type + "/main.go.gotmpl"},
		{filepath.Join(dir, "config.go"), "template/project/" + nc.Type + "/config.go.gotmpl"},
		{filepath.Join(dir, "config.json"), "template/project/" + nc.Type + "/config.json.gotmpl"},
		{filepath.Join(dir, "handler", "handler.go"), "template/project/" + nc.Type + "/handler.go.gotmpl"},
		{filepath.Join(dir, "Makefile"), "template/project/shared/Makefile.gotmpl"},
		{filepath.Join(dir, ".gitignore"), "template/project/shared/gitignore.gotmpl"},
		{filepath.Join(dir, "go.mod"), "template/project/shared/go.mod.gotmpl"},
	}

	for _, f := range files {
		if !nc.WithGRPC && strings.Contains(f.template, "grpc") {
			continue
		}

		content, err := renderProjectTemplate(f.template, nc)
		if err != nil {
			return fmt.Errorf("template %s: %w", f.template, err)
		}

		if err := os.WriteFile(f.path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", f.path, err)
		}
	}

	return nil
}

func renderProjectTemplate(name string, data any) (string, error) {
	tmplBytes, err := projectTemplates.ReadFile(name)
	if err != nil {
		return "", fmt.Errorf("template %q: %w", name, err)
	}

	tmpl, err := template.New(filepath.Base(name)).Parse(string(tmplBytes))
	if err != nil {
		return "", err
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func exportName(name string) string {
	if name == "" {
		return ""
	}

	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == '-' || r == '_' || r == '.'
	})

	for i, part := range parts {
		if part == "" {
			continue
		}

		runes := []rune(part)
		runes[0] = unicode.ToUpper(runes[0])

		parts[i] = string(runes)
	}

	return strings.Join(parts, "")
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
