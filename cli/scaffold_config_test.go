/**
 * @file scaffold_config_test.go
 * @package cli
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/go-sicky/sicky"
	srvFiber "github.com/go-sicky/sicky/server/fiber"
	srvGRPC "github.com/go-sicky/sicky/server/grpc"
	srvHTTP "github.com/go-sicky/sicky/server/http"
	"github.com/go-sicky/sicky/service/interactive"
	"github.com/go-sicky/sicky/service/mcp"
	"github.com/go-sicky/sicky/service/standard"
)

// The config.json a project is born with is the only configuration most of
// them will ever have, and a key matching no field is dropped without a word.
// That has shipped twice: the scaffold shipped no manager block at all, so
// every generated service had no /metrics and no /health; and when the block
// was added it shipped `services_path` where the field is `service_pool_path`.
//
// Reading struct tags catches neither. Rendering the real template and
// checking every key against the real struct does — including the
// application's own blocks, which the framework's runtime unknown-key check
// deliberately skips because it cannot know what an application puts there.

// serverFields mirrors the `server` block the template generates. It is
// restated rather than imported because the generated struct lives in package
// main of another module, and restating it is the contract: if the template
// changes shape, this stops matching and the test fails.
type serverFields struct {
	Fiber *srvFiber.Config `mapstructure:"fiber"`
	GRPC  *srvGRPC.Config  `mapstructure:"grpc"`
	HTTP  *srvHTTP.Config  `mapstructure:"http"`
}

// scaffoldConfigStruct reproduces the ConfigDef main.go is generated with, for
// the given enabled protocols.
//
// The service field is concrete rather than an interface on purpose: a field
// typed `any` has no structure to walk, so every key under `service` would go
// unchecked and the test would pass for the wrong reason.
func scaffoldConfigStruct(projectType string, withGRPC, withFiber, withHTTP bool) any {
	var servers serverFields

	if withFiber {
		servers.Fiber = &srvFiber.Config{}
	}

	if withGRPC {
		servers.GRPC = &srvGRPC.Config{}
	}

	if withHTTP {
		servers.HTTP = &srvHTTP.Config{}
	}

	switch projectType {
	case "mcp":
		return struct {
			Server  serverFields  `mapstructure:"server"`
			Service *mcp.Config   `mapstructure:"service"`
			Sicky   *sicky.Config `mapstructure:"sicky"`
		}{Server: servers, Service: &mcp.Config{}}

	case "interactive":
		return struct {
			Server  serverFields        `mapstructure:"server"`
			Service *interactive.Config `mapstructure:"service"`
			Sicky   *sicky.Config       `mapstructure:"sicky"`
		}{Server: servers, Service: &interactive.Config{}}

	default:
		return struct {
			Server  serverFields     `mapstructure:"server"`
			Service *standard.Config `mapstructure:"service"`
			Sicky   *sicky.Config    `mapstructure:"sicky"`
		}{Server: servers, Service: &standard.Config{}}
	}
}

// scaffoldConfigJSON renders a project and returns its parsed config.json.
func scaffoldConfigJSON(t *testing.T, projectType string, args ...string) map[string]any {
	t.Helper()

	dir := t.TempDir()

	full := append([]string{
		"new", projectType,
		"--module", "example.com/" + projectType,
		"--type", projectType,
		"-o", filepath.Join(dir, "out"),
	}, args...)

	if out, err := exec.Command(scaffoldBinary(t), full...).CombinedOutput(); err != nil {
		t.Fatalf("scaffold %s: %v\n%s", projectType, err, out)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "out", projectType, "config.json"))
	if err != nil {
		t.Fatalf("read config.json: %v", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("parse config.json: %v\n%s", err, raw)
	}

	return cfg
}

// TestScaffoldConfigKeysAllResolve renders each template and checks every key
// it writes against the struct meant to receive it.
func TestScaffoldConfigKeysAllResolve(t *testing.T) {
	for _, tc := range []struct {
		projectType                   string
		args                          []string
		withGRPC, withFiber, withHTTP bool
	}{
		{projectType: "standard", args: []string{"--http", "--fiber=false", "--no-grpc"}, withHTTP: true},
		{projectType: "standard", args: []string{"--http"}, withFiber: true, withHTTP: true},
		{projectType: "standard", args: nil, withFiber: true, withGRPC: true},
		{projectType: "mcp", args: []string{"--http", "--fiber=false", "--no-grpc"}, withHTTP: true},
		{projectType: "mcp", args: []string{"--fiber", "--no-grpc", "--http=false"}, withFiber: true},
		{projectType: "interactive", args: nil},
	} {
		t.Run(tc.projectType+strings.Join(tc.args, " "), func(t *testing.T) {
			cfg := scaffoldConfigJSON(t, tc.projectType, tc.args...)
			target := scaffoldConfigStruct(tc.projectType, tc.withGRPC, tc.withFiber, tc.withHTTP)

			nodes := schemaNodes(reflect.TypeOf(target))
			for _, key := range unknownJSONKeys(cfg, "", nodes) {
				t.Errorf("config.json sets %q but no field maps to it: viper drops it "+
					"silently, so the generated service is not configured the way its "+
					"own file claims", key)
			}
		})
	}
}

// The manager block is what makes /metrics and /health exist: Enabled reads a
// nil block as disabled, so its absence is silent. It gets its own test rather
// than being folded into the key check, because "key resolves" and "key is
// present" are different failures.
func TestScaffoldConfigEnablesTheManager(t *testing.T) {
	for _, projectType := range []string{"standard", "mcp", "interactive"} {
		t.Run(projectType, func(t *testing.T) {
			var args []string
			if projectType != "interactive" {
				args = []string{"--http", "--fiber=false", "--no-grpc"}
			}

			sickyBlock, ok := scaffoldConfigJSON(t, projectType, args...)["sicky"].(map[string]any)
			if !ok {
				t.Fatal("config.json has no sicky block")
			}

			manager, present := sickyBlock["manager"].(map[string]any)
			if !present {
				t.Fatal(`no "manager" block: its presence is the enable signal, so ` +
					"without it the generated service serves no /metrics, /health, /live " +
					"or /ready, and registers an empty manager address into the registry")
			}

			if enable, has := manager["enable"]; has && enable == false {
				t.Error(`"manager.enable" is false: the scaffold must not ship a ` +
					"service with its observability endpoints switched off")
			}

			if addr, _ := manager["address"].(string); !isLoopbackAddr(addr) {
				t.Errorf("manager address = %q; the scaffold must bind loopback", addr)
			}

			if expose, has := manager["expose_config"]; has && expose == true {
				t.Error(`"expose_config" is true: /config publishes the whole ` +
					"configuration and must not be on by default")
			}

			if token, has := manager["auth_token"]; has && token != "" {
				t.Errorf("auth_token = %v; the scaffold must not ship a credential", token)
			}
		})
	}
}

// schemaNodes records every mapstructure path a type can hold.
func schemaNodes(t reflect.Type) map[string]struct{} {
	nodes := map[string]struct{}{}

	var walk func(reflect.Type, string)

	walk = func(t reflect.Type, prefix string) {
		for t != nil && t.Kind() == reflect.Pointer {
			t = t.Elem()
		}

		if t == nil || t.Kind() != reflect.Struct {
			return
		}

		for field := range t.Fields() {
			if !field.IsExported() {
				continue
			}

			name, options, _ := strings.Cut(field.Tag.Get("mapstructure"), ",")

			if name == "-" {
				continue
			}

			if name == "" {
				name = field.Name
			}

			if options == "squash" {
				walk(field.Type, prefix)

				continue
			}

			path := name
			if prefix != "" {
				path = prefix + "." + name
			}

			nodes[path] = struct{}{}
			walk(field.Type, path)
		}
	}

	walk(t, "")

	return nodes
}

// unknownJSONKeys returns every leaf path in doc that resolves to no field,
// reported at the first segment that does not resolve — naming a leaf the
// operator never wrote helps nobody.
func unknownJSONKeys(doc map[string]any, prefix string, nodes map[string]struct{}) []string {
	var unknown []string

	for key, value := range doc {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		if nested, ok := value.(map[string]any); ok {
			unknown = append(unknown, unknownJSONKeys(nested, path, nodes)...)

			continue
		}

		if _, ok := nodes[path]; ok {
			continue
		}

		segments := strings.Split(path, ".")

		for i := range segments {
			candidate := strings.Join(segments[:i+1], ".")
			if _, ok := nodes[candidate]; ok {
				continue
			}

			unknown = append(unknown, candidate)

			break
		}
	}

	slices.Sort(unknown)

	return slices.Compact(unknown)
}

func isLoopbackAddr(addr string) bool {
	return strings.HasPrefix(addr, "127.0.0.1:") || strings.HasPrefix(addr, "localhost:")
}
