/**
 * @file proto_cmd.go
 * @package cli
 * @author Dr.NP <np@herewe.tech>
 * @since 09/06/2026
 */

package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"
)

func protoRun(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "sicky proto: missing subcommand")
		fmt.Fprintln(os.Stderr, "Usage: sicky proto <build|new> [flags]")

		return 1
	}

	sub := strings.ToLower(args[0])
	rest := args[1:]
	switch sub {
	case "build":
		return protoBuild(rest)
	case "new":
		return generateProtoFile(rest)
	default:
		// Back-compat: `sicky proto --dry-run` behaves like build.
		if strings.HasPrefix(sub, "-") {
			return protoBuild(args)
		}

		fmt.Fprintf(os.Stderr, "sicky proto: unknown subcommand %q\n", sub)

		return 1
	}
}

func protoBuild(args []string) int {
	fs := pflag.NewFlagSet("proto build", pflag.ContinueOnError)
	dir := fs.String("dir", "proto", "Directory with .proto files")
	goOut := fs.String("go-out", ".", "protoc --go_out value")
	grpcOut := fs.String("go-grpc-out", ".", "--go-grpc_out value")
	dryRun := fs.Bool("dry-run", false, "Print protoc commands without running")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return 0
		}

		fmt.Fprintf(os.Stderr, "sicky proto build: %s\n", err.Error())

		return 1
	}

	files, err := filepath.Glob(filepath.Join(*dir, "*.proto"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "sicky proto build: %s\n", err.Error())

		return 1
	}

	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "sicky proto build: no .proto files in %s (try `sicky proto new <name>`)\n", *dir)

		return 1
	}

	if _, err := exec.LookPath("protoc"); err != nil {
		fmt.Fprintln(os.Stderr, "sicky proto build: protoc not found in PATH")
		fmt.Fprintln(os.Stderr, "Install: https://grpc.io/docs/protoc-installation/ + protoc-gen-go(_grpc)")

		return 1
	}

	rc := 0
	for _, f := range files {
		cmdArgs := []string{"--go_out=" + *goOut, "--go-grpc_out=" + *grpcOut, f}
		if *dryRun {
			fmt.Printf("  dry-run: protoc %s\n", strings.Join(cmdArgs, " "))
			continue
		}

		cmd := exec.Command("protoc", cmdArgs...) //nolint:gosec // G204: dev CLI; binary is fixed, args are explicit operator flags/paths
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "sicky proto build: %s: %s\n", f, err.Error())
			rc = 1
		}
	}

	if rc == 0 && !*dryRun {
		fmt.Println("  Proto generated")
	}

	return rc
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
