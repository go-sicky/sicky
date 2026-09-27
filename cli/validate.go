/**
 * @file validate.go
 * @package cli
 * @author Dr.NP <np@herewe.tech>
 * @since 09/27/2026
 */

package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Names and paths coming from the command line end up interpolated into
// a Makefile, a Dockerfile, go.mod and generated Go sources, and are
// used to build filesystem paths. Each validator below closes one of the
// ways that could go wrong.

var (
	// projectNamePattern accepts what survives as a docker tag, a make
	// identifier and a proto package at once.
	projectNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
	// identPattern is the Go identifier grammar.
	identPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	// schematicNamePattern is the safe token for names that become file
	// names and land inside Go string literals.
	schematicNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	// modulePathPattern is the printable subset of a Go module path.
	modulePathPattern = regexp.MustCompile(`^[A-Za-z0-9._~/-]+$`)
	// serviceTypes are the scaffoldable service kinds.
	serviceTypes = []string{"standard", "mcp", "interactive"}
)

// validateProjectName rejects a project name that could escape into a
// shell command or a path.
//
// The name is rendered as `PROJECT := {{.Name}}` in the Makefile - make
// expands `$(shell ...)` the moment it parses the file - and inside the
// Dockerfile's -ldflags quoting, so shell metacharacters ($, backquotes,
// quotes, spaces, newlines) are an arbitrary command execution waiting
// for the next `make` or `docker build`. Separators and ".." turn the
// same value into a path traversal for filepath.Join.
func validateProjectName(name string) error {
	if name == "" {
		return errors.New("project name is empty")
	}

	if !projectNamePattern.MatchString(name) {
		return fmt.Errorf("invalid project name %q: use lowercase letters, digits, '.', '_' or '-' (64 chars max)", name)
	}

	if strings.Contains(name, "..") {
		return fmt.Errorf("invalid project name %q: %q is not allowed", name, "..")
	}

	return nil
}

// validateIdent rejects a name that is not a Go identifier.
//
// exportName() only re-cases the parts split on '-', '_' and '.'; it
// keeps quotes, semicolons and whitespace, so a name such as
// `Foo = "x"; func init(){...}` compiles into the generated file and
// runs at package init.
func validateIdent(name string) error {
	if name == "" {
		return errors.New("name is empty")
	}

	if !identPattern.MatchString(name) {
		return fmt.Errorf("invalid name %q: a Go identifier is required (letters, digits and '_', not starting with a digit)", name)
	}

	return nil
}

// validateModulePath rejects a module path that could inject directives
// into go.mod: the file is rendered as `module {{.Module}}`, and a
// newline there adds arbitrary require/replace lines.
func validateModulePath(module string) error {
	if module == "" {
		return errors.New("module path is empty")
	}

	if !modulePathPattern.MatchString(module) {
		return fmt.Errorf("invalid module path %q: only letters, digits and '.', '_', '~', '/', '-' are allowed", module)
	}

	if strings.Contains(module, "//") || strings.HasPrefix(module, "/") || strings.HasSuffix(module, "/") {
		return fmt.Errorf("invalid module path %q", module)
	}

	for segment := range strings.SplitSeq(module, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fmt.Errorf("invalid module path %q: empty or relative path segment", module)
		}
	}

	return nil
}

// validateSchematicName validates a name that becomes both a file name
// and a value inside generated Go string literals.
//
// Templates render `{{.Name}}` inside quotes (return "{{.Name}}",
// fmt.Errorf("{{.Name}}: ...")), so a quote, a backslash or a newline in
// the name closes the literal and injects statements that gofmt happily
// accepts. The derived identifier is checked as well: exportName only
// re-cases, so a leading digit would still produce `const 123Foo`.
func validateSchematicName(name string) error {
	if name == "" {
		return errors.New("name is empty")
	}

	if !schematicNamePattern.MatchString(name) {
		return fmt.Errorf("invalid name %q: letters, digits, '.', '_' or '-' only (64 chars max)", name)
	}

	if err := validateIdent(exportName(name)); err != nil {
		return fmt.Errorf("invalid name %q: derived identifier is invalid: %s", name, err.Error())
	}

	return nil
}

// validateOutputRoot rejects parent-directory traversal in an output
// directory. Absolute paths stay allowed: they are explicit user intent,
// while "../../" hides inside otherwise routine-looking commands.
func validateOutputRoot(root string) error {
	if root == "" {
		return nil
	}

	if slices.Contains(strings.Split(filepath.ToSlash(root), "/"), "..") {
		return fmt.Errorf("output directory %q must not contain %q", root, "..")
	}

	return nil
}

// checkWritePath rejects a destination that escapes the working tree.
//
// The check runs on the raw path: filepath.Abs/Clean would resolve ".."
// away before anyone could look at it.
func checkWritePath(path string) error {
	if path == "" {
		return errors.New("empty output path")
	}

	if err := validateOutputRoot(path); err != nil {
		return err
	}

	return nil
}

// validateNewContext validates everything a scaffolded project renders
// from: the name (Makefile, Dockerfile, go.mod and the output path), the
// service type (selects the template directory) and the module path
// (go.mod).
func validateNewContext(nc *newContext) error {
	if err := validateProjectName(nc.Name); err != nil {
		return err
	}

	if err := validateOutputRoot(nc.OutputDir); err != nil {
		return err
	}

	if err := validateModulePath(nc.Module); err != nil {
		return err
	}

	if slices.Contains(serviceTypes, nc.Type) {
		return nil
	}

	return fmt.Errorf("unknown service type %q (valid: %s)", nc.Type, strings.Join(serviceTypes, ", "))
}

// requireName validates a schematic name before it is interpolated into
// generated source, returning a process exit code (0 when valid).
func requireName(schematic, name string) int {
	if err := validateSchematicName(name); err != nil {
		fmt.Fprintf(os.Stderr, "sicky generate %s: %s\n", schematic, err.Error())

		return 1
	}

	return 0
}

// requireIdent validates a name that must be a Go (or proto) identifier
// verbatim, returning a process exit code (0 when valid).
func requireIdent(schematic, name string) int {
	if err := validateIdent(name); err != nil {
		fmt.Fprintf(os.Stderr, "sicky generate %s: %s\n", schematic, err.Error())

		return 1
	}

	return 0
}
