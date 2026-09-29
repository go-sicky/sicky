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
	"strings"

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

// newHTTPTransport builds the HTTP transport from the parsed flags.
func newHTTPTransport(listen, token, allowedHosts string) *prtcl.HTTPTransport {
	trans := prtcl.NewHTTPTransport(listen)
	trans.AuthToken = token

	for h := range strings.SplitSeq(allowedHosts, ",") {
		if h = strings.TrimSpace(h); h != "" {
			trans.AllowedHosts = append(trans.AllowedHosts, h)
		}
	}

	return trans
}

// consumeSubcommandArgs rewrites os.Args to program name plus the
// positional leftovers of a locally parsed flag set, so sicky.Init's
// global parse (ExitOnError) never sees this subcommand's flags.
func consumeSubcommandArgs(fs *pflag.FlagSet) {
	os.Args = append(os.Args[:1], fs.Args()...)
}

// serveMCP holds the shared stdio/http MCP serving logic used by both
// `sicky serve` and `sicky mcp` (kept as an alias for discoverability).
func serveMCP(cmdName string, args []string) int {
	fs := pflag.NewFlagSet(cmdName, pflag.ContinueOnError)
	transport := fs.String("transport", "stdio", "Transport: stdio, http")
	// Loopback by default: without a token the transport only accepts
	// loopback peers, and a wildcard bind would still be closed to them.
	listen := fs.String("listen", "127.0.0.1:3000", "HTTP listen address")
	token := fs.String("token", os.Getenv("SICKY_MCP_TOKEN"), "Bearer token for the HTTP transport (default: SICKY_MCP_TOKEN)")
	allowedHosts := fs.String("allowed-hosts", "", "Extra Host header values accepted without a token (comma separated)")
	name := fs.String("name", "sicky-mcp-server", "MCP server name")
	version := fs.String("version", Version, "MCP server version")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return 0
		}

		fmt.Fprintf(os.Stderr, "sicky %s: %s\n", cmdName, err.Error())

		return 1
	}

	// sicky.Init re-parses os.Args with the global flag set, which is
	// ExitOnError: leaving these subcommand flags in place made
	// `sicky serve --transport http` die with "unknown flag" (exit 2)
	// before anything was served. They are parsed above - hand Init only
	// what is left.
	consumeSubcommandArgs(fs)

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
			// The meta-server is configured by its own flags, not by a
			// business config file: requiring one made `sicky serve`// fail with "Config File not Found" on a fresh checkout.
			DisableConfig: true,
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
		&mcp.Config{},
	)

	mcpServer := svc.MCPServer()

	serveErr := make(chan error, 1)
	go func() {
		var trans prtcl.Transport

		switch *transport {
		case "stdio":
			trans = prtcl.NewStdioTransport()

		case "http":
			trans = newHTTPTransport(*listen, *token, *allowedHosts)
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
