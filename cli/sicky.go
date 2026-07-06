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
	Version   = "dev"
	Branch    = "main"
	Commit    = ""
	BuildTime = ""
)

func Run() {
	args := os.Args[1:]

	if len(args) < 1 {
		serveRun(args)

		return
	}

	cmd := strings.ToLower(args[0])
	cmdArgs := args[1:]

	switch cmd {
	case "serve", "s":
		serveRun(cmdArgs)

	case "new", "n":
		newRun(cmdArgs)

	case "generate", "g":
		generateRun(cmdArgs)

	case "version", "v":
		versionRun()

	case "help", "-h", "--help":
		helpRun()

	default:
		if strings.HasPrefix(cmd, "-") {
			serveRun(args)
		} else {
			fmt.Fprintf(os.Stderr, "sicky: unknown command %q\n", cmd)
			fmt.Fprintf(os.Stderr, "Run 'sicky help' for usage.\n")
			os.Exit(1)
		}
	}
}

func helpRun() {
	fmt.Println(`Sicky CLI — MCP server & project scaffolding tool

Usage:
  sicky                   Run as MCP server (stdio)
  sicky serve             Run as MCP server (explicit)
  sicky new <name>        Scaffold a new project (interactive)
  sicky generate          Generate code in an existing project
  sicky version           Print version information
  sicky help              Show this help

Commands:
  serve, s     Start MCP server
    --transport string   Transport: stdio, http (default "stdio")
    --listen string      HTTP listen address (default ":3000")
    --name string        Server name
    --version string     Server version

  new, n       Scaffold a new project
    --type string        standard, mcp, interactive
    --module string      Go module path
    --output, -o string  Output directory (default ".")
    --no-grpc            Skip gRPC server

  generate, g  Generate code
    handler  <name>      Generate a handler
    tool     <name>      Generate a tool definition
    resource <name>      Generate a resource definition
    doc                  Generate documentation

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
