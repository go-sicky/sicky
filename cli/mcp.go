/**
 * @file mcp.go
 * @package cli
 * @author Dr.NP <np@herewe.tech>
 * @since 09/06/2026
 */

package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/pflag"

	"github.com/go-sicky/sicky"
	"github.com/go-sicky/sicky/service"
	mcp "github.com/go-sicky/sicky/service/mcp"
	prtcl "github.com/go-sicky/sicky/service/mcp/protocol"
)

// mcpRun starts the sicky MCP meta-server for AI assistants.
// This is NOT the user business service; use `sicky run` for that.
func mcpRun(cmdName string, args []string) int {
	return serveMCP(cmdName, args)
}

// serveMCP holds the shared stdio/http MCP serving logic used by both
// `sicky serve` and `sicky mcp` (kept as an alias for discoverability).
func serveMCP(cmdName string, args []string) int {
	fs := pflag.NewFlagSet(cmdName, pflag.ContinueOnError)
	transport := fs.String("transport", "stdio", "Transport: stdio, http")
	listen := fs.String("listen", ":3000", "HTTP listen address")
	name := fs.String("name", "sicky-mcp-server", "MCP server name")
	version := fs.String("version", Version, "MCP server version")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return 0
		}

		fmt.Fprintf(os.Stderr, "sicky %s: %s\n", cmdName, err.Error())

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
			serveErr <- errors.New("failed to create transport")
			cancel()

			return
		}

		if err := mcpServer.Serve(ctx, trans); err != nil && !errors.Is(err, context.Canceled) {
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
