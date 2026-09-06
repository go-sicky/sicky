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
 * @file sicky.go
 * @package cli
 * @author Dr.NP <np@herewe.tech>
 * @since 07/06/2026
 */

package cli

import (
	"fmt"
	"os"
	"strings"
)

var (
	// Version is a shared cli value.
	Version = "dev"
	// Branch is a shared cli value.
	Branch = "main"
	// Commit is a shared cli value.
	Commit = ""
	// BuildTime is a shared cli value.
	BuildTime = ""
)

// Run runs the component.
func Run() int {
	args := os.Args[1:]

	if len(args) < 1 {
		return serveRun(args)
	}

	cmd := strings.ToLower(args[0])
	cmdArgs := args[1:]

	switch cmd {
	case "serve", "s":
		return serveRun(cmdArgs)

	case "mcp":
		return mcpRun("mcp", cmdArgs)

	case "run", "r":
		return runRun(cmdArgs)

	case "doctor":
		return doctorRun(cmdArgs)

	case "info", "i":
		return infoRun(cmdArgs)

	case "config", "c":
		return configRun(cmdArgs)

	case "proto", "p":
		return protoRun(cmdArgs)

	case "new", "n":
		return newRun(cmdArgs)

	case "generate", "g":
		return generateRun(cmdArgs)

	case "version", "v":
		versionRun()

		return 0

	case "help", "-h", "--help":
		helpRun()

		return 0

	default:
		if strings.HasPrefix(cmd, "-") {
			return serveRun(args)
		}

		fmt.Fprintf(os.Stderr, "sicky: unknown command %q\n", cmd)
		fmt.Fprintf(os.Stderr, "Run 'sicky help' for usage.\n")

		return 1
	}
}

func helpRun() {
	fmt.Println(`Sicky CLI — MCP server & project scaffolding tool

Usage:
  sicky                   Run as MCP server (stdio)
  sicky serve             Run as MCP server (explicit)
  sicky mcp               Alias of serve (MCP meta-server)
  sicky run [--watch]     Run the business project (go run .)
  sicky new <name>        Scaffold a new project (interactive)
  sicky generate          Generate code in an existing project
  sicky proto <build|new> Build .proto files or scaffold a new one
  sicky config <sub>      Validate/init/show config
  sicky doctor            Check toolchain, ports and config
  sicky info              Print runtime info
  sicky version           Print version information
  sicky help              Show this help

Commands:
  serve, s     Start MCP server
    --transport string   Transport: stdio, http (default "stdio")
    --listen string      HTTP listen address (default ":3000")
    --name string        Server name
    --version string     Server version

  mcp          Alias of serve (same flags)

  run, r       Run business project via 'go run .'
    -C, --config string       Config base name or path (forwarded)
    --config-type string      Config format (forwarded, default "json")
    -w, --watch               Watch *.go files and restart on change

  new, n       Scaffold a new project
    --type string        standard, mcp, interactive
    --module string      Go module path
    --output, -o string  Output directory (default ".")
    --no-grpc            Skip gRPC server
    --fiber              Add Fiber server (default true, use --fiber=false to skip)
    --http               Add net/http (bunrouter) server (default false)

  generate, g  Generate code
    handler  <name> [--type fiber|http]  Generate a handler
    tool     <name>      Generate a tool definition
    resource <name>      Generate a resource definition
    prompt   <name>      Generate a prompt definition
    doc                  Generate documentation
    server   <type> [name]   Wire a server instance (fiber, http, grpc, tcp, udp, websocket)
    client   <type> [name]   Wrap an outbound client
    service  <type>           Wire a service (standard, mcp, interactive)
    broker   <type>           Subscribe to a broker (nats, jetstream, nsq)
    job      <type> [name]    Implement a background job (cron, ticker)
    middleware <name>    Generate an HTTP middleware stub
    proto    <name>           Scaffold a .proto file (also via 'sicky proto new')
    config               Generate config.json from project defaults
    docker               Generate Dockerfile + .dockerignore
    k8s [name]           Generate k8s deployment + service

  proto, p    Protobuf helpers
    build [--dir proto] [--dry-run]   Run protoc over .proto files
    new <name>                        Scaffold proto/<name>.proto

  config, c   Config helpers
    validate [--config C] [--config-type json]
    init [-o dir] [--force]
    show [--config C] [--config-type json] [--show-secrets]

  doctor       Check go/protoc/docker, go.mod, config, ports
  info, i      Print version + runtime + module info

  version, v   Print version information`)
}

func versionRun() {
	fmt.Printf("sicky version %s\n", Version)
	fmt.Printf("  branch:    %s\n", Branch)
	fmt.Printf("  commit:    %s\n", Commit)
	fmt.Printf("  built:     %s\n", BuildTime)
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
