/**
 * @file doctor.go
 * @package cli
 * @author Dr.NP <np@herewe.tech>
 * @since 09/06/2026
 */

package cli

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/pflag"
)

func doctorRun(args []string) int {
	fs := pflag.NewFlagSet("doctor", pflag.ContinueOnError)
	configFlag := fs.StringP("config", "C", "config", "Config file to validate")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return 0
		}

		fmt.Fprintf(os.Stderr, "sicky doctor: %s\n", err.Error())

		return 1
	}

	fail := 0
	check := func(name string, ok bool, detail string) {
		status := "ok"
		if !ok {
			status = "FAIL"
			fail++
		}

		fmt.Printf("  [%s] %s %s\n", status, name, detail)
	}

	// go toolchain.
	if p, err := exec.LookPath("go"); err != nil {
		check("go", false, "not in PATH")
	} else {
		out, _ := exec.Command("go", "version").CombinedOutput()
		check("go", true, string(out)+fmt.Sprintf("(%s)", p))
	}

	// protoc (optional).
	if _, err := exec.LookPath("protoc"); err != nil {
		check("protoc", true, "not found (optional; needed for `sicky proto build`)")
	} else {
		check("protoc", true, "found")
	}

	// docker (optional).
	if _, err := exec.LookPath("docker"); err != nil {
		check("docker", true, "not found (optional)")
	} else {
		check("docker", true, "found")
	}

	// go.mod presence.
	if _, err := os.Stat("go.mod"); err != nil {
		check("go.mod", true, "not found (run `sicky new` or run inside a project)")
	} else {
		check("go.mod", true, "found ("+detectModule()+")")
	}

	// config validation (best effort).
	if _, _, err := loadConfigFile(*configFlag, "json"); err != nil {
		check("config", true, fmt.Sprintf("no readable %s.json in CWD (optional): %s", *configFlag, err.Error()))
	} else {
		check("config", true, *configFlag+".json readable")
	}

	// Common ports.
	for _, port := range []string{":3000", ":8888"} {
		ln, err := net.Listen("tcp", port)
		if err != nil {
			check("port "+port, true, "in use (server may already run)")
			continue
		}

		_ = ln.Close()
		check("port "+port, true, "free")
	}

	_ = runtime.GOOS
	if fail > 0 {
		return 1
	}

	fmt.Println("  doctor: all required checks passed")

	return 0
}

func infoRun(args []string) int {
	_ = args
	fmt.Printf("sicky %s (%s, %s)\n", Version, Branch, Commit)
	fmt.Printf("  built:   %s\n", BuildTime)
	fmt.Printf("  go:      %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	fmt.Printf("  module:  %s\n", detectModule())
	fmt.Printf("  project: %s\n", detectProjectName())

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
