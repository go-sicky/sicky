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
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"text/template"
	"unicode"

	"github.com/spf13/pflag"
)

//go:embed template/project template/license/*.gotmpl
var projectTemplates embed.FS

type newContext struct {
	Name         string
	ExportedName string
	Module       string
	AppName      string
	Type         string
	WithGRPC     bool
	WithFiber    bool
	WithHTTP     bool
	WithProto    bool
	OutputDir    string
	// Optional scaffolding hints used by `sicky config init` and
	// `sicky generate config/docker`. Kept for compat; templates
	// render from WithGRPC/WithFiber/WithHTTP, these are informational only.
	Servers  []string
	Registry string
	Tracer   string
	// DockerRegistry is deprecated: generated Docker tags carry no registry
	// prefix (docker.io is the implicit default). Kept for API compat.
	DockerRegistry string
	// GoMetricsVersion pins the armon/go-metrics workaround in the generated
	// go.mod (see resolveGoMetricsVersion). Empty omits the workaround.
	GoMetricsVersion string
	// License is the canonical license key (one of licenseChoices) written
	// into the generated LICENSE file and into every file header. It is
	// resolved by resolveLicense before scaffolding, so it never stays
	// unrecognized; empty means "do not write a LICENSE file".
	License string
	// SPDX is the identifier stamped on generated file headers, derived
	// from License. Kept separately because it differs from the key for
	// GPL-3.0, which is written as the unambiguous GPL-3.0-only.
	SPDX string
	// Holder is the copyright holder. Author is the "Name <email>" on the
	// @author line. Both come from git, are confirmed interactively, can be
	// overridden by --copyright/--author, and fall back to a visible
	// ${HOLDER}/${AUTHOR} placeholder when nothing is known.
	Holder string
	Author string
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
	fiberFlag := fs.Bool("fiber", true, "Add Fiber server (use --fiber=false to skip)")
	httpFlag := fs.Bool("http", false, "Add net/http (bunrouter) server")
	forceFlag := fs.Bool("force", false, "Overwrite an existing non-empty project directory")
	licenseFlag := fs.String("license", "", "License: MIT, Apache-2.0, GPL-3.0, BSD-3-Clause, Commercial")
	authorFlag := fs.String("author", "", `Author stamped on file headers, e.g. "Jane Doe <jane@example.com>"`)
	copyrightFlag := fs.String("copyright", "", "Copyright holder for the LICENSE file and file headers")
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
		WithFiber: *fiberFlag,
		WithHTTP:  *httpFlag,
		License:   *licenseFlag,
		Author:    *authorFlag,
		Holder:    *copyrightFlag,
	}

	if nc.Name == "" {
		fmt.Fprintln(os.Stderr, "sicky new: project name cannot be empty")

		return 1
	}

	interactivePrompt(fs, nc)

	// An unrecognized license — from the flag or from the prompt — is a
	// warning and a default rather than a failed run: it names nothing but
	// this project's own file, so a typo must not cost the user a scaffold.
	license := resolveLicense(nc.License)
	nc.License = license.key
	nc.SPDX = license.spdx

	// Everything below interpolates these values into a Makefile, a
	// Dockerfile, go.mod and Go sources, and uses the name to build a
	// path - all of which the validators keep from escaping.
	if err := validateNewContext(nc); err != nil {
		fmt.Fprintf(os.Stderr, "sicky new: %s\n", err.Error())

		return 1
	}

	nc.AppName = nc.Name + ".example.sicky"
	nc.ExportedName = exportName(nc.Name)

	projectDir := filepath.Join(nc.OutputDir, nc.Name)
	if err := scaffoldProject(projectDir, nc, *forceFlag); err != nil {
		fmt.Fprintf(os.Stderr, "sicky new: failed to create project: %s\n", err.Error())

		return 1
	}

	gitInit(projectDir)

	fmt.Println()
	fmt.Printf("  \u2714 Created %s/\n", nc.Name)
	fmt.Println()
	fmt.Println("  Next steps:")
	fmt.Printf("    cd %s\n", nc.Name)
	if nc.WithProto {
		fmt.Println("    make proto   # requires protoc + protoc-gen-go (see `make grpc`)")
	}
	fmt.Println("    make build")
	fmt.Println("    make run")
	fmt.Println()

	return 0
}

func interactivePrompt(fs *pflag.FlagSet, nc *newContext) {
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

	askGRPC := !fs.Changed("no-grpc")
	askFiber := !fs.Changed("fiber")
	askHTTP := !fs.Changed("http")

	switch nc.Type {
	case "standard":
		if askGRPC {
			fmt.Print("  ? Add gRPC server? [Y/n]: ")
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if strings.EqualFold(input, "n") || strings.EqualFold(input, "no") {
				nc.WithGRPC = false
			} else {
				nc.WithGRPC = true
			}
		}

		if askFiber {
			fmt.Print("  ? Add Fiber server? (fasthttp, :3000) [Y/n]: ")
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if strings.EqualFold(input, "n") || strings.EqualFold(input, "no") {
				nc.WithFiber = false
			} else {
				nc.WithFiber = true
			}
		}

		if askHTTP {
			fmt.Print("  ? Add net/http server? (bunrouter, :9980) [Y/n]: ")
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if strings.EqualFold(input, "n") || strings.EqualFold(input, "no") {
				nc.WithHTTP = false
			} else {
				nc.WithHTTP = true
			}
		}
	case "mcp":
		nc.WithGRPC = false
		if askFiber {
			fmt.Print("  ? Add Fiber server (for SSE transport)? [Y/n]: ")
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if strings.EqualFold(input, "n") || strings.EqualFold(input, "no") {
				nc.WithFiber = false
			} else {
				nc.WithFiber = true
			}
		}

		if askHTTP {
			fmt.Print("  ? Add net/http server (for SSE transport)? [y/N]: ")
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if strings.EqualFold(input, "y") || strings.EqualFold(input, "yes") {
				nc.WithHTTP = true
			}
		}
	default:
		nc.WithGRPC = false
		nc.WithFiber = false
		nc.WithHTTP = false
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
	} else {
		nc.WithProto = false
	}

	if !fs.Changed("license") {
		fmt.Println("  ? License:")
		for i, choice := range licenseChoices {
			fmt.Printf("    %d. %s\n", i+1, choice.display)
		}
		fmt.Print("    [1]: ")

		input, _ := reader.ReadString('\n')
		nc.License = licenseInput(strings.TrimSpace(input))
	}

	if !fs.Changed("author") {
		detected := detectAuthor()
		if detected != "" {
			fmt.Printf("  ? Author [%s]: ", detected)
		} else {
			fmt.Print("  ? Author [Name <email>]: ")
		}

		input, _ := reader.ReadString('\n')
		if input = strings.TrimSpace(input); input != "" {
			nc.Author = input
		} else {
			nc.Author = detected
		}
	}

	if !fs.Changed("copyright") {
		detected := detectHolder()
		if detected != "" {
			fmt.Printf("  ? Copyright holder [%s]: ", detected)
		} else {
			fmt.Print("  ? Copyright holder: ")
		}

		input, _ := reader.ReadString('\n')
		if input = strings.TrimSpace(input); input != "" {
			nc.Holder = input
		} else {
			nc.Holder = detected
		}
	}
}

// licenseInput accepts both the interactive menu number and any spelling
// the --license flag would take. An unrecognized answer is returned
// verbatim so resolveLicense can warn using the words the user actually
// typed, rather than a normalized form of them.
func licenseInput(input string) string {
	if input == "" {
		return defaultLicense().key
	}

	if n, err := strconv.Atoi(input); err == nil && n >= 1 && n <= len(licenseChoices) {
		return licenseChoices[n-1].key
	}

	if c, ok := lookupLicense(input); ok {
		return c.key
	}

	return input
}

// gitInit initializes a git repository in dir. It is best-effort: a missing
// git binary or any init failure only warns and never fails scaffolding.
func gitInit(dir string) {
	if _, err := exec.LookPath("git"); err != nil {
		fmt.Fprintln(os.Stderr, "sicky new: warn: git not found, skipped `git init`")

		return
	}

	cmd := exec.Command("git", "init")
	cmd.Dir = dir

	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "sicky new: warn: `git init` failed: %s: %s\n", err.Error(), strings.TrimSpace(string(out)))

		return
	}

	fmt.Println("  \u2714 Git initialized")
}

// defaultGoMetricsVersion is the fallback when the CLI build info is
// unavailable (exotic builds). It must track sicky/go.mod's replace pin.
const defaultGoMetricsVersion = "v0.6.1"

// resolveGoMetricsVersion returns the hashicorp/go-metrics version currently
// selected by sicky's own replace directive, read from the running CLI
// binary's build info. It returns "" when the armon/go-metrics quirk is gone
// (no replace needed), so generated go.mod files track sicky/go.mod without
// hardcoded versions.
func resolveGoMetricsVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return defaultGoMetricsVersion
	}

	for _, d := range info.Deps {
		if d.Path != "github.com/armon/go-metrics" {
			continue
		}

		if d.Replace == nil {
			return ""
		}

		if v := d.Replace.Version; v != "" {
			return v
		}

		return defaultGoMetricsVersion
	}

	return ""
}

func scaffoldProject(dir string, nc *newContext, force bool) error {
	// Scaffolding overwrites by name (Makefile, go.mod, main.go, ...).
	// Refuse to touch a populated directory unless --force says so.
	if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 && !force {
		return fmt.Errorf("%s already exists and is not empty (use --force to overwrite)", dir)
	}

	// Pin the go-metrics workaround before rendering go.mod so `go mod tidy`
	// works in the fresh project without manual replace/exclude edits.
	nc.GoMetricsVersion = resolveGoMetricsVersion()

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

	// The license file is the one generated file carrying no header of its
	// own: it *is* the license, and prepending anything would corrupt the
	// standard text every downstream tool recognizes.
	if nc.License != "" {
		body, err := renderProjectTemplate("template/license/"+nc.License+".gotmpl", newFileMeta(nc.Name, nc.SPDX, nc.Holder, nc.Author))
		if err != nil {
			return fmt.Errorf("license %s: %w", nc.License, err)
		}

		if err := os.WriteFile(filepath.Join(dir, "LICENSE"), []byte(body), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", filepath.Join(dir, "LICENSE"), err)
		}
	}

	// The brief column feeds the @brief line of a .go header. Build scripts
	// get the shorter two-line head and files outside the header policy get
	// no comment at all, so their entry stays empty.
	files := []struct {
		path     string
		template string
		brief    string
		skip     func(*newContext) bool
	}{
		{filepath.Join(dir, "main.go"), "template/project/" + nc.Type + "/main.go.gotmpl", "This file contains the main entry point for the service.", nil},
		{filepath.Join(dir, "config.go"), "template/project/" + nc.Type + "/config.go.gotmpl", "This file contains the application configuration.", nil},
		{filepath.Join(dir, "config.json"), "template/project/" + nc.Type + "/config.json.gotmpl", "", nil},
		{filepath.Join(dir, "handler", "fiber.go"), "template/project/" + nc.Type + "/handler/fiber.go.gotmpl", "This file defines the Fiber HTTP handlers.", func(nc *newContext) bool { return !nc.WithFiber }},
		{filepath.Join(dir, "handler", "http.go"), "template/project/" + nc.Type + "/handler/http.go.gotmpl", "This file defines the net/http handlers.", func(nc *newContext) bool { return !nc.WithHTTP }},
		{filepath.Join(dir, "handler", "grpc.go"), "template/project/" + nc.Type + "/handler/grpc.go.gotmpl", "This file defines the gRPC handlers.", func(nc *newContext) bool { return !nc.WithGRPC }},
		// MCP service handler (Tools/Resources/Prompts) is independent of
		// the SSE transport servers and always rendered for mcp projects.
		{filepath.Join(dir, "handler", "handler.go"), "template/project/mcp/handler/handler.go.gotmpl", "This file defines the MCP tool, resource and prompt handlers.", func(nc *newContext) bool { return nc.Type != "mcp" }},
		{filepath.Join(dir, "Makefile"), "template/project/shared/Makefile.gotmpl", "", nil},
		{filepath.Join(dir, "Dockerfile"), "template/project/shared/Dockerfile.gotmpl", "", nil},
		{filepath.Join(dir, ".dockerignore"), "template/project/shared/dockerignore.gotmpl", "", nil},
		{filepath.Join(dir, ".gitignore"), "template/project/shared/gitignore.gotmpl", "", nil},
		{filepath.Join(dir, "go.mod"), "template/project/shared/go.mod.gotmpl", "", nil},
	}

	// Identity and date are fixed for the whole scaffold so every file in
	// the project agrees on its copyright line, even across a run that
	// straddles midnight.
	baseMeta := newFileMeta(nc.Name, nc.SPDX, nc.Holder, nc.Author)

	for _, f := range files {
		if f.skip != nil && f.skip(nc) {
			continue
		}

		content, err := renderProjectTemplate(f.template, nc)
		if err != nil {
			// Per-protocol handler templates are optional: types without
			// split files (e.g. interactive) fall back to handler.go.gotmpl.
			if strings.HasSuffix(f.template, "/fiber.go.gotmpl") || strings.HasSuffix(f.template, "/http.go.gotmpl") || strings.HasSuffix(f.template, "/grpc.go.gotmpl") {
				continue
			}

			return fmt.Errorf("template %s: %w", f.template, err)
		}

		// The header is injected after rendering, not kept in the template:
		// @file and @package must describe the file that exists on disk
		// (which depends on the selected servers), and one policy applied
		// in code cannot drift across 27 templates.
		fm := baseMeta
		fm.File = relativeFile(dir, f.path)
		fm.Brief = f.brief

		if isGoFile(f.path) {
			fm.Package = packageOf(content)
		}

		content = injectFileHeader(f.path, content, &fm)
		content = injectFileFooter(f.path, content)

		// Normalize rendered Go sources so every server combination is
		// gofmt-clean (conditional blocks defeat static alignment).
		if strings.HasSuffix(f.path, ".go") {
			formatted, ferr := format.Source([]byte(content))
			if ferr != nil {
				return fmt.Errorf("template %s: format: %w", f.template, ferr)
			}

			content = string(formatted)
		}

		if err := os.WriteFile(f.path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", f.path, err)
		}
	}

	// Legacy fallback: types (or selections) without any per-protocol
	// handler file render the single handler.go.gotmpl instead.
	// Selections without servers need no handler at all.
	entries, _ := os.ReadDir(filepath.Join(dir, "handler"))
	if len(entries) == 0 {
		content, err := renderProjectTemplate("template/project/"+nc.Type+"/handler/handler.go.gotmpl", nc)
		if err != nil {
			if !nc.WithFiber && !nc.WithHTTP && !nc.WithGRPC {
				return nil
			}

			return fmt.Errorf("template %s: %w", "template/project/"+nc.Type+"/handler/handler.go.gotmpl", err)
		}

		handlerPath := filepath.Join(dir, "handler", "handler.go")
		fm := baseMeta
		fm.File = relativeFile(dir, handlerPath)
		fm.Package = packageOf(content)
		fm.Brief = "This file defines the service handlers."

		content = injectFileHeader(handlerPath, content, &fm)
		content = injectFileFooter(handlerPath, content)

		if formatted, ferr := format.Source([]byte(content)); ferr != nil {
			return fmt.Errorf("template %s: format: %w", "template/project/"+nc.Type+"/handler/handler.go.gotmpl", ferr)
		} else {
			content = string(formatted)
		}

		if err := os.WriteFile(handlerPath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", handlerPath, err)
		}
	}

	// Proto starter: grpc handlers reference the generated package, so ship
	// a compilable .proto source next to it. Run `make proto` (protoc +
	// `make grpc` toolchains) to generate the Go code before `make build`.
	if nc.WithProto {
		protoName := strings.ToLower(nc.Name)
		svc := exportName(nc.Name)
		protoSrc := "syntax = \"proto3\";\n\npackage " + protoName + ";\n\noption go_package = \"./;pb\";\n\n" +
			"service " + svc + " {\n  rpc Ping (PingRequest) returns (PingReply);\n}\n\n" +
			"message PingRequest {\n  string message = 1;\n}\n\n" +
			"message PingReply {\n  string message = 1;\n}\n"
		protoPath := filepath.Join(dir, "proto", protoName+".proto")
		if err := os.WriteFile(protoPath, []byte(protoSrc), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", protoPath, err)
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
