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

// The README template is shared with `sicky generate doc`, so it lives under
// template/doc; `sicky new` embeds it from there rather than keeping a second
// copy that could drift.
//
//go:embed template/project template/entity template/license/*.gotmpl template/doc/readme.md.gotmpl
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
	// WithAuth gates the auth/session example (app/session.go,
	// handler/auth.http.go, service/auth.go and the validateSession
	// middleware). It is on unless --no-auth is given.
	WithAuth bool
	// Models is the resource list --model asked for. A non-empty list is
	// what enables bun: config infra, MustInfra and the --initdb flag are
	// all conditional on it.
	Models    []modelSpec
	OutputDir string
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
	noAuthFlag := fs.Bool("no-auth", false, "Skip the auth/session example")
	modelFlag := fs.StringSlice("model", nil, "Generate bun models (comma separated)")
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
		WithAuth:  !*noAuthFlag,
		License:   *licenseFlag,
		Author:    *authorFlag,
		Holder:    *copyrightFlag,
	}

	if nc.Name == "" {
		fmt.Fprintln(os.Stderr, "sicky new: project name cannot be empty")

		return 1
	}

	interactivePrompt(fs, nc)

	models, err := modelSpecs(*modelFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sicky new: %s\n", err.Error())

		return 1
	}

	nc.Models = models

	// Models are generated from the standard layering: mcp and interactive
	// projects have no request/response files and no validateSession, so
	// their resource handler would not compile.
	if len(nc.Models) > 0 && nc.Type != "standard" {
		fmt.Fprintln(os.Stderr, "sicky new: --model requires --type standard")

		return 1
	}

	// A model is an HTTP resource: its handler, DTOs and the request/response
	// aggregation files only exist for the bunrouter stack. Rejecting the
	// combination before scaffoldProject writes anything keeps a failed run
	// from leaving a half-built project behind.
	if len(nc.Models) > 0 && !nc.WithHTTP {
		fmt.Fprintln(os.Stderr, "sicky new: --model requires an HTTP server (--http)")

		return 1
	}

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
	fmt.Println("    ./bin/svc")
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
			fmt.Print("  ? Add net/http server? (bunrouter, :9980) [y/N]: ")
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if strings.EqualFold(input, "y") || strings.EqualFold(input, "yes") {
				nc.WithHTTP = true
			} else {
				nc.WithHTTP = false
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

// scaffoldFile is one file scaffoldProject writes. data defaults to the
// scaffold context; extra is appended to the rendered template before the
// header/footer/format pass, which is how per-model DTO declarations join the
// aggregated request/response files.
type scaffoldFile struct {
	path     string
	template string
	brief    string
	skip     func(*newContext) bool
	data     any
	extra    string
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

	// Directories are created only for the files that will actually be
	// written: an HTTP-less project must not gain empty layering directories.
	if nc.Type == "standard" {
		if err := os.MkdirAll(filepath.Join(dir, "app"), 0o755); err != nil {
			return err
		}
	}

	if nc.Type == "standard" && nc.WithHTTP {
		for _, sub := range []string{"request", "response"} {
			if err := os.MkdirAll(filepath.Join(dir, "handler", sub), 0o755); err != nil {
				return err
			}
		}
	}

	if nc.Type == "standard" && nc.WithHTTP && nc.WithAuth {
		if err := os.MkdirAll(filepath.Join(dir, "service"), 0o755); err != nil {
			return err
		}
	}

	if nc.WithProto {
		if err := os.MkdirAll(filepath.Join(dir, "proto"), 0o755); err != nil {
			return err
		}
	}

	// Models write one service file each even when --no-auth removed the
	// auth service, so these directories follow the model list rather than
	// the auth block.
	if len(nc.Models) > 0 {
		for _, sub := range []string{"model", "service"} {
			if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
				return err
			}
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

	// Per-model DTO fragments are rendered once and appended to the base
	// request/response files before their single header/format pass, so the
	// aggregated files stay gofmt-clean no matter how many models there are.
	requestDecls, responseDecls, err := modelDeclarations(nc)
	if err != nil {
		return err
	}

	// The brief column feeds the @brief line of a .go header. Build scripts
	// get the shorter two-line head and files outside the header policy get
	// no comment at all, so their entry stays empty.
	files := []scaffoldFile{
		{path: filepath.Join(dir, "main.go"), template: "template/project/" + nc.Type + "/main.go.gotmpl", brief: "This file contains the main entry point for the service."},
		{path: filepath.Join(dir, "config.go"), template: "template/project/" + nc.Type + "/config.go.gotmpl", brief: "This file contains the application configuration."},
		{path: filepath.Join(dir, "config.json"), template: "template/project/" + nc.Type + "/config.json.gotmpl"},
		// README is rendered for every project type; AGENTS.md describes the
		// standard layering and has no mcp/interactive counterpart.
		{path: filepath.Join(dir, "README.md"), template: "template/doc/readme.md.gotmpl"},
		{path: filepath.Join(dir, "AGENTS.md"), template: "template/project/standard/AGENTS.md.gotmpl", skip: func(nc *newContext) bool { return nc.Type != "standard" }},
		{path: filepath.Join(dir, "handler", "fiber.go"), template: "template/project/" + nc.Type + "/handler/fiber.go.gotmpl", brief: "This file defines the Fiber HTTP handlers.", skip: func(nc *newContext) bool { return !nc.WithFiber }},
		{path: filepath.Join(dir, "handler", "http.go"), template: "template/project/" + nc.Type + "/handler/http.go.gotmpl", brief: "This file defines the net/http handlers.", skip: func(nc *newContext) bool { return !nc.WithHTTP }},
		{path: filepath.Join(dir, "handler", "grpc.go"), template: "template/project/" + nc.Type + "/handler/grpc.go.gotmpl", brief: "This file defines the gRPC handlers.", skip: func(nc *newContext) bool { return !nc.WithGRPC }},
		// Application layering (standard only). app/config.go is the
		// application's own configuration; session, auth handler, service
		// and the middleware they feed are the auth example, which
		// --no-auth removes. The request/response aggregation files stay
		// without auth: `sicky generate model` appends to them.
		{path: filepath.Join(dir, "app", "config.go"), template: "template/project/standard/app/config.go.gotmpl", brief: "This file contains the application configuration.", skip: func(nc *newContext) bool { return nc.Type != "standard" }},
		{path: filepath.Join(dir, "app", "session.go"), template: "template/project/standard/app/session.go.gotmpl", brief: "This file manages in-memory access tokens.", skip: func(nc *newContext) bool { return nc.Type != "standard" || !nc.WithAuth }},
		{path: filepath.Join(dir, "handler", "auth.http.go"), template: "template/project/standard/handler/auth.http.go.gotmpl", brief: "This file defines the authentication HTTP handlers.", skip: func(nc *newContext) bool { return nc.Type != "standard" || !nc.WithHTTP || !nc.WithAuth }},
		{path: filepath.Join(dir, "handler", "request", "http.go"), template: "template/project/standard/handler/request/http.go.gotmpl", brief: "This file contains the HTTP request payloads.", skip: func(nc *newContext) bool { return nc.Type != "standard" || !nc.WithHTTP }, extra: requestDecls},
		{path: filepath.Join(dir, "handler", "response", "http.go"), template: "template/project/standard/handler/response/http.go.gotmpl", brief: "This file contains the HTTP response codes and payloads.", skip: func(nc *newContext) bool { return nc.Type != "standard" || !nc.WithHTTP }, extra: responseDecls},
		{path: filepath.Join(dir, "service", "auth.go"), template: "template/project/standard/service/auth.go.gotmpl", brief: "This file defines the authentication service.", skip: func(nc *newContext) bool { return nc.Type != "standard" || !nc.WithHTTP || !nc.WithAuth }},
		// MCP service handler (Tools/Resources/Prompts) is independent of
		// the SSE transport servers and always rendered for mcp projects.
		{path: filepath.Join(dir, "handler", "handler.go"), template: "template/project/mcp/handler/handler.go.gotmpl", brief: "This file defines the MCP tool, resource and prompt handlers.", skip: func(nc *newContext) bool { return nc.Type != "mcp" }},
		{path: filepath.Join(dir, "Makefile"), template: "template/project/shared/Makefile.gotmpl"},
		{path: filepath.Join(dir, "Dockerfile"), template: "template/project/shared/Dockerfile.gotmpl"},
		{path: filepath.Join(dir, ".dockerignore"), template: "template/project/shared/dockerignore.gotmpl"},
		{path: filepath.Join(dir, ".gitignore"), template: "template/project/shared/gitignore.gotmpl"},
		{path: filepath.Join(dir, "go.mod"), template: "template/project/shared/go.mod.gotmpl"},
	}

	// One full resource per model: model, service and HTTP handler. The DTO
	// declarations were folded into the aggregated files above; every file
	// still takes the same header/footer/format path.
	for _, m := range nc.Models {
		data := modelTemplateData{modelSpec: m, Module: nc.Module, WithAuth: nc.WithAuth}
		files = append(files,
			scaffoldFile{
				path:     filepath.Join(dir, "model", m.Name+".go"),
				template: "template/entity/model.go.gotmpl",
				brief:    "This file defines the " + m.ExportedName + " bun model.",
				data:     data,
			},
			scaffoldFile{
				path:     filepath.Join(dir, "service", m.Name+".go"),
				template: "template/entity/service.go.gotmpl",
				brief:    "This file defines the " + m.ExportedName + " service.",
				data:     data,
			},
			scaffoldFile{
				path:     filepath.Join(dir, "handler", m.Name+".http.go"),
				template: "template/entity/handler.http.go.gotmpl",
				brief:    "This file defines the HTTP handlers for the " + m.Name + " resource.",
				data:     data,
			},
		)
	}

	// Identity and date are fixed for the whole scaffold so every file in
	// the project agrees on its copyright line, even across a run that
	// straddles midnight.
	baseMeta := newFileMeta(nc.Name, nc.SPDX, nc.Holder, nc.Author)

	for _, f := range files {
		if f.skip != nil && f.skip(nc) {
			continue
		}

		data := f.data
		if data == nil {
			data = nc
		}

		content, err := renderProjectTemplate(f.template, data)
		if err != nil {
			// Per-protocol handler templates are optional for project types
			// without split files (e.g. interactive), which fall back to
			// handler.go.gotmpl. The type gate keeps standard failures
			// visible: those templates always exist there, and its own
			// request/response files also end in /http.go.gotmpl, so a
			// suffix-only check would swallow a real render error.
			optional := nc.Type != "standard" &&
				(strings.HasSuffix(f.template, "/fiber.go.gotmpl") ||
					strings.HasSuffix(f.template, "/http.go.gotmpl") ||
					strings.HasSuffix(f.template, "/grpc.go.gotmpl"))
			if optional {
				continue
			}

			return fmt.Errorf("template %s: %w", f.template, err)
		}

		if f.extra != "" {
			content = strings.TrimRight(content, "\n") + "\n\n" + strings.TrimLeft(f.extra, "\n")
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
