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
 * @file serve.go
 * @package cli
 * @author Dr.NP <np@herewe.tech>
 * @since 07/06/2026
 */

package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/go-sicky/sicky"
	mcp "github.com/go-sicky/sicky/service/mcp"
	prtcl "github.com/go-sicky/sicky/service/mcp/protocol"
	"github.com/go-sicky/sicky/service"
	"github.com/spf13/pflag"
)

func serveRun(args []string) {
	fs := pflag.NewFlagSet("serve", pflag.ExitOnError)
	transport := fs.String("transport", "stdio", "Transport: stdio, http")
	listen := fs.String("listen", ":3000", "HTTP listen address")
	name := fs.String("name", "sicky-mcp-server", "MCP server name")
	version := fs.String("version", Version, "MCP server version")
	_ = fs.Parse(args)

	ctx := context.Background()

	sicky.Init(
		&sicky.Options{
			AppName:   "sicky.mcp",
			Version:   Version,
			Branch:    Branch,
			Commit:    Commit,
			BuildTime: BuildTime,
			Context:   ctx,
		},
	)

	svc := mcp.New(
		&service.Options{
			Name:    *name,
			Version: *version,
			Context: ctx,
		},
		&mcp.Config{
			Transport: *transport,
			Listen:    *listen,
		},
	)

	mcpServer := svc.MCPServer()

	go func() {
		var trans prtcl.Transport

		switch *transport {
		case "stdio":
			trans = prtcl.NewStdioTransport()

		case "http":
			trans = prtcl.NewHTTPTransport(*listen)
		default:
			fmt.Fprintf(os.Stderr, "Unknown transport: %s\n", *transport)
			os.Exit(1)
		}

		if trans == nil {
			fmt.Fprintf(os.Stderr, "Failed to create transport\n")
			os.Exit(1)
		}

		if err := mcpServer.Serve(ctx, trans); err != nil && err != context.Canceled {
			fmt.Fprintf(os.Stderr, "MCP server error: %s\n", err.Error())
			os.Exit(1)
		}
	}()

	sicky.Run(&sicky.Config{
		LogLevel: "info",
	})
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
