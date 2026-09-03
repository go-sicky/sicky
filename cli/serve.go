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
	"errors"
	"fmt"
	"os"

	"github.com/go-sicky/sicky"
	"github.com/go-sicky/sicky/service"
	mcp "github.com/go-sicky/sicky/service/mcp"
	prtcl "github.com/go-sicky/sicky/service/mcp/protocol"
	"github.com/spf13/pflag"
)

func serveRun(args []string) int {
	fs := pflag.NewFlagSet("serve", pflag.ContinueOnError)
	transport := fs.String("transport", "stdio", "Transport: stdio, http")
	listen := fs.String("listen", ":3000", "HTTP listen address")
	name := fs.String("name", "sicky-mcp-server", "MCP server name")
	version := fs.String("version", Version, "MCP server version")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "sicky serve: %s\n", err.Error())
		return 1
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := sicky.Init(
		&sicky.Options{
			AppName:   "sicky.mcp",
			Version:   Version,
			Branch:    Branch,
			Commit:    Commit,
			BuildTime: BuildTime,
			Context:   ctx,
		},
	); err != nil {
		if errors.Is(err, sicky.ErrVersionShown) {
			return 0
		}
		fmt.Fprintf(os.Stderr, "Init failed: %s\n", err.Error())
		return 1
	}

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

	serveErr := make(chan error, 1)
	go func() {
		var trans prtcl.Transport

		switch *transport {
		case "stdio":
			trans = prtcl.NewStdioTransport()

		case "http":
			trans = prtcl.NewHTTPTransport(*listen)
		default:
			serveErr <- fmt.Errorf("unknown transport: %s", *transport)
			cancel()
			return
		}

		if trans == nil {
			serveErr <- fmt.Errorf("failed to create transport")
			cancel()
			return
		}

		if err := mcpServer.Serve(ctx, trans); err != nil && err != context.Canceled {
			serveErr <- err
			cancel()
			return
		}
		serveErr <- nil
	}()

	if err := sicky.Run(&sicky.Config{
		LogLevel: "info",
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Run failed: %s\n", err.Error())
		return 1
	}
	select {
	case err := <-serveErr:
		if err != nil {
			fmt.Fprintf(os.Stderr, "MCP server error: %s\n", err.Error())
			return 1
		}
	default:
	}
	return 0
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
