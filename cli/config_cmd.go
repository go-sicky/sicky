/**
 * @file config_cmd.go
 * @package cli
 * @author Dr.NP <np@herewe.tech>
 * @since 09/06/2026
 */

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/go-sicky/sicky/utils"
)

func configRun(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "sicky config: missing subcommand")
		fmt.Fprintln(os.Stderr, "Usage: sicky config <validate|init|show> [flags]")

		return 1
	}

	sub := strings.ToLower(args[0])
	rest := args[1:]
	switch sub {
	case "validate":
		return configValidate(rest)
	case "init":
		return configInit(rest)
	case "show":
		return configShow(rest)
	default:
		fmt.Fprintf(os.Stderr, "sicky config: unknown subcommand %q\n", sub)

		return 1
	}
}

func configValidate(args []string) int {
	fs := pflag.NewFlagSet("config validate", pflag.ContinueOnError)
	configFlag := fs.StringP("config", "C", "config", "Config file base name or path")
	configType := fs.String("config-type", "json", "Config format")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return 0
		}

		fmt.Fprintf(os.Stderr, "sicky config validate: %s\n", err.Error())

		return 1
	}

	v, file, err := loadConfigFile(*configFlag, *configType)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sicky config validate: %s\n", err.Error())

		return 1
	}

	warns := lintConfig(v)
	fmt.Printf("  ok: %s (%d warnings)\n", file, len(warns))
	for _, w := range warns {
		fmt.Printf("  warn: %s\n", w)
	}

	if len(warns) > 0 {
		return 0
	}

	return 0
}

func configInit(args []string) int {
	fs := pflag.NewFlagSet("config init", pflag.ContinueOnError)
	output := fs.StringP("output", "o", ".", "Output directory")
	force := fs.Bool("force", false, "Overwrite existing config.json")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return 0
		}

		fmt.Fprintf(os.Stderr, "sicky config init: %s\n", err.Error())

		return 1
	}

	nc := &newContext{
		Name:           detectProjectName(),
		Module:         detectModule(),
		Type:           "standard",
		Servers:        []string{"fiber", "grpc"},
		Registry:       "local",
		Tracer:         "none",
		DockerRegistry: "docker.io",
	}

	nc.AppName = nc.Name + ".example.sicky"
	nc.ExportedName = exportName(nc.Name)
	_ = normalizeNewContext(nc)
	content, err := renderProjectTemplate("template/project/standard/config.json.gotmpl", nc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sicky config init: %s\n", err.Error())

		return 1
	}

	// Reject parent-directory traversal before joining: an output root
	// typed by hand (or by a script) must stay inside the tree.
	if err := validateOutputRoot(*output); err != nil {
		fmt.Fprintf(os.Stderr, "sicky config init: %s\n", err.Error())

		return 1
	}

	out := filepath.Join(*output, "config.json")
	if _, err := os.Stat(out); err == nil && !*force {
		fmt.Fprintf(os.Stderr, "sicky config init: %s exists (use --force)\n", out)

		return 1
	}

	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "sicky config init: %s\n", err.Error())

		return 1
	}

	if err := os.WriteFile(out, []byte(content), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "sicky config init: %s\n", err.Error())

		return 1
	}

	fmt.Printf("  Created %s\n", out)

	return 0
}

func configShow(args []string) int {
	fs := pflag.NewFlagSet("config show", pflag.ContinueOnError)
	configFlag := fs.StringP("config", "C", "config", "Config file base name or path")
	configType := fs.String("config-type", "json", "Config format")
	showSecrets := fs.Bool("show-secrets", false, "Show secrets in clear text (default redacted)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return 0
		}

		fmt.Fprintf(os.Stderr, "sicky config show: %s\n", err.Error())

		return 1
	}

	v, _, err := loadConfigFile(*configFlag, *configType)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sicky config show: %s\n", err.Error())

		return 1
	}

	m := v.AllSettings()
	if !*showSecrets {
		// Same tree sanitizer as the Manager's /config endpoint: the CLI's
		// own list was shorter and passed arrays, private keys and
		// `tracer.headers.Authorization` straight through.
		redacted, _ := utils.SanitizeValue("", m).(map[string]any)
		m = redacted
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "sicky config show: %s\n", err.Error())

		return 1
	}

	fmt.Println(string(data))

	return 0
}

func loadConfigFile(base, configType string) (*viper.Viper, string, error) {
	v := viper.New()
	v.SetConfigType(configType)
	// Accept explicit path (with or without extension) or base name in CWD.
	if strings.Contains(base, string(os.PathSeparator)) || strings.HasSuffix(base, "."+configType) {
		dir := filepath.Dir(base)
		name := strings.TrimSuffix(filepath.Base(base), "."+configType)
		if dir == "" {
			dir = "."
		}

		v.SetConfigName(name)
		v.AddConfigPath(dir)
		if err := v.ReadInConfig(); err != nil {
			return nil, "", err
		}

		used := v.ConfigFileUsed()

		return v, used, nil
	}

	v.SetConfigName(base)
	v.AddConfigPath(".")
	if err := v.ReadInConfig(); err != nil {
		return nil, "", err
	}

	used := v.ConfigFileUsed()

	return v, used, nil
}

// lintConfig performs presence-means-enabled sanity checks without importing
// the full sicky config graph (CLI stays light).
func lintConfig(v *viper.Viper) []string {
	var warns []string
	if v.GetString("sicky.log_level") == "" {
		warns = append(warns, "sicky.log_level empty, will default to info")
	}

	tracerType := strings.ToLower(v.GetString("sicky.tracer.type"))
	switch tracerType {
	case "", "none", "grpc", "http", "stdout", "uptrace":
	default:
		warns = append(warns, "sicky.tracer.type unknown: "+tracerType)
	}

	if tracerType == "uptrace" && v.GetString("sicky.tracer.dsn") == "" {
		warns = append(warns, "uptrace tracer requires sicky.tracer.dsn")
	}

	return warns
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
