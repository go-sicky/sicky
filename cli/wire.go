/**
 * @file wire.go
 * @package cli
 * @author Dr.NP <np@herewe.tech>
 * @since 10/19/2026
 */

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// placeholderBunDSN is the connection string `sicky generate model` writes
// when the project has no bun block yet. It is a placeholder, not a secret,
// and points at the same local PostgreSQL the scaffold documents.
//
//nolint:gosec // G101: local-development placeholder, not a credential.
const placeholderBunDSN = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"

// sourceEdit replaces src[start:end] with text. start == end inserts text
// without removing anything.
type sourceEdit struct {
	start int
	end   int
	text  string
}

// wireMainGo appends one model to an existing project's main.go. Everything is
// parsed and checked before the first edit is applied, so a main.go that does
// not look like a sicky scaffold is rejected without being touched. The
// function is idempotent: wiring the same model twice reports success and
// leaves the file byte-identical.
func wireMainGo(path, module, modelName, exportedName string) error {
	if module == "" {
		return errors.New("wire main.go: empty module path")
	}

	if exportedName == "" {
		return fmt.Errorf("wire main.go: model %q has no exported name", modelName)
	}

	src, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read main.go: %w", err)
	}

	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("main.go does not look like a sicky scaffold: %w", err)
	}

	mainFn := funcDeclaration(file, "main")
	if mainFn == nil || mainFn.Body == nil {
		return errors.New("main.go does not look like a sicky scaffold: func main")
	}

	initCall := findInitCall(mainFn)
	if initCall == nil {
		return errors.New("main.go does not look like a sicky scaffold: sicky.Init")
	}

	options := optionsComposite(initCall)
	if options == nil {
		return errors.New("main.go does not look like a sicky scaffold: sicky.Options")
	}

	var edits []sourceEdit

	imports := importPaths(file)
	missingImports := make([]string, 0, 5)

	wantImport := func(importPath string) {
		if imports[importPath] {
			return
		}

		imports[importPath] = true

		missingImports = append(missingImports, importPath)
	}

	wantImport(module + "/model")

	edits = append(edits, mustInfraEdits(src, fset, options)...)

	if !hasInitDBFlagSwitch(initCall) {
		edits = append(edits, flagSwitchEdit(src, fset, initCall))
	}

	initdb := funcDeclaration(file, "initdb")
	if initdb == nil {
		// The generated initdb talks to PostgreSQL through infra, reports a
		// missing connection through the logger, and exits with os.Exit.
		wantImport("context")
		wantImport("os")
		wantImport("github.com/go-sicky/sicky/infra")
		wantImport("github.com/go-sicky/sicky/logger")

		edits = append(edits, insertInitDB(src, fset, file, exportedName))
	} else if !hasModelInitDB(initdb, exportedName) {
		edits = append(edits, insertModelInitDB(src, fset, initdb, exportedName))
	}

	assign, lastHandle := httpServerStatements(mainFn)
	if !hasHTTPHandler(mainFn, exportedName) {
		if assign == nil {
			fmt.Fprintf(
				os.Stderr,
				"warning: main.go has no httpSrv := srvHTTP.New(...) assignment; add httpSrv.Handle(handler.NewHTTP%s()) manually for model %q\n",
				exportedName,
				modelName,
			)
		} else {
			wantImport(module + "/handler")

			anchor := ast.Stmt(assign)
			if lastHandle != nil && lastHandle.Pos() > assign.Pos() {
				anchor = lastHandle
			}

			edits = append(edits, appendStatement(src, fset, anchor, "httpSrv.Handle(handler.NewHTTP"+exportedName+"())"))
		}
	}

	if len(missingImports) > 0 {
		edits = append(edits, importEdits(src, fset, file, missingImports)...)
	}

	if len(edits) == 0 {
		return nil
	}

	out, err := applyEdits(src, edits)
	if err != nil {
		return fmt.Errorf("wire main.go: %w", err)
	}

	if err := writeAtomic(path, out); err != nil {
		return fmt.Errorf("write main.go: %w", err)
	}

	return nil
}

// wireConfigBun ensures config.json carries the bun block the generated model
// needs. Existing values are never overwritten: an operator who already chose
// a database keeps it. The file is left untouched when nothing is missing, so
// a second run is byte-for-byte identical.
func wireConfigBun(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config.json: %w", err)
	}

	cfg := make(map[string]any)

	// UseNumber keeps integers beyond float64's exact range (and their
	// original spelling) intact when the file is written back.
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()

	if err := decoder.Decode(&cfg); err != nil {
		return fmt.Errorf("config.json: %w", err)
	}

	if cfg == nil {
		return errors.New("config.json: the root must be a JSON object")
	}

	sickyCfg, err := jsonObject(cfg, "sicky", "sicky")
	if err != nil {
		return fmt.Errorf("config.json: %w", err)
	}

	infraCfg, err := jsonObject(sickyCfg, "infra", "sicky.infra")
	if err != nil {
		return fmt.Errorf("config.json: %w", err)
	}

	bunCfg, err := jsonObject(infraCfg, "bun", "sicky.infra.bun")
	if err != nil {
		return fmt.Errorf("config.json: %w", err)
	}

	changed := false

	if _, ok := bunCfg["driver"]; !ok {
		bunCfg["driver"] = "postgres"
		changed = true
	}

	if _, ok := bunCfg["dsn"]; !ok {
		bunCfg["dsn"] = placeholderBunDSN
		changed = true
	}

	if !changed {
		return nil
	}

	out, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return fmt.Errorf("config.json: %w", err)
	}

	out = append(out, '\n')

	if err := writeAtomic(path, out); err != nil {
		return fmt.Errorf("write config.json: %w", err)
	}

	return nil
}

// appendDecls appends the type declarations in decls that path does not
// already declare. The snippet may omit its package clause, which is the form
// `sicky generate model` renders. Duplicate names are skipped by name, so a
// forced re-run cannot declare the same DTO twice.
func appendDecls(path, decls string) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	snippetSrc, snippetFset, snippet, err := parseDeclarations(decls)
	if err != nil {
		return err
	}

	existing := declaredTypes(file)
	chunks := make([]string, 0, len(snippet.Decls))

	for _, decl := range snippet.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}

		for _, spec := range gen.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || existing[typeSpec.Name.Name] {
				continue
			}

			existing[typeSpec.Name.Name] = true

			chunks = append(chunks, declarationText(snippetSrc, snippetFset, gen, typeSpec))
		}
	}

	if len(chunks) == 0 {
		return nil
	}

	offset := trailingCommentOffset(src, fset, file)
	text := strings.Join(chunks, "\n\n") + "\n"

	// Keep the separator out of the modeline: a trailing comment starts at the
	// beginning of its line, and the declarations must come before it.
	if offset > 0 && src[offset-1] != '\n' {
		text = "\n\n" + text
	} else {
		text = "\n" + text
	}

	out, err := applyEdits(src, []sourceEdit{{start: offset, end: offset, text: text}})
	if err != nil {
		return fmt.Errorf("append declarations to %s: %w", path, err)
	}

	if err := writeAtomic(path, out); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}

// mainGoHasValidateSession reports whether the project's handler package still
// defines the auth middleware. path is handler/http.go: --no-auth projects do
// not have it, and resource handlers mount validateSession only when it
// exists.
func mainGoHasValidateSession(path string) bool {
	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return false
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Recv == nil && fn.Name.Name == "validateSession" {
			return true
		}
	}

	return false
}

// mustInfraEdits adds bun to the Options literal, either as a new MustInfra
// field or as one more entry in the existing slice.
func mustInfraEdits(src []byte, fset *token.FileSet, options *ast.CompositeLit) []sourceEdit {
	if kv := compositeKey(options, "MustInfra"); kv != nil {
		slice, ok := kv.Value.(*ast.CompositeLit)
		if !ok {
			// The value is an expression this helper cannot inspect, so it
			// cannot prove bun is missing; leave the caller's choice alone.
			return nil
		}

		for _, elt := range slice.Elts {
			if value, ok := stringLiteral(elt); ok && value == "bun" {
				return nil
			}
		}

		return []sourceEdit{appendCompositeString(src, fset, slice, `"bun"`)}
	}

	return []sourceEdit{addCompositeField(src, fset, options, `MustInfra: []string{"bun"}`)}
}

// appendCompositeString adds one string element to a string slice literal.
func appendCompositeString(src []byte, fset *token.FileSet, lit *ast.CompositeLit, value string) sourceEdit {
	openPos := fset.Position(lit.Lbrace)
	closePos := fset.Position(lit.Rbrace)

	if closePos.Line > openPos.Line {
		lineStart := lineStartOffset(closePos)
		indent := string(src[lineStart:closePos.Offset])

		return sourceEdit{start: lineStart, end: lineStart, text: indent + "\t" + value + ",\n"}
	}

	if len(lit.Elts) == 0 {
		return sourceEdit{start: closePos.Offset, end: closePos.Offset, text: value}
	}

	return sourceEdit{start: closePos.Offset, end: closePos.Offset, text: ", " + value}
}

// addCompositeField adds a key/value field to a struct literal.
func addCompositeField(src []byte, fset *token.FileSet, lit *ast.CompositeLit, field string) sourceEdit {
	openPos := fset.Position(lit.Lbrace)
	closePos := fset.Position(lit.Rbrace)

	if closePos.Line > openPos.Line {
		lineStart := lineStartOffset(closePos)
		indent := string(src[lineStart:closePos.Offset])

		return sourceEdit{start: lineStart, end: lineStart, text: indent + "\t" + field + ",\n"}
	}

	prefix := ""
	if len(lit.Elts) > 0 {
		prefix = ", "
	}

	return sourceEdit{start: closePos.Offset, end: closePos.Offset, text: prefix + field}
}

// flagSwitchEdit appends the --initdb switch to sicky.Init. The separator
// region between the last argument and the closing parenthesis is replaced as
// a whole, which handles both the one-line and the multiline call shape
// without doubling the trailing comma.
func flagSwitchEdit(src []byte, fset *token.FileSet, call *ast.CallExpr) sourceEdit {
	last := call.Args[len(call.Args)-1]

	start := fset.Position(last.End()).Offset
	end := fset.Position(call.Rparen).Offset
	baseIndent := lineIndent(src, end)
	argIndent := baseIndent + "\t"
	fieldIndent := argIndent + "\t"

	text := ",\n" + argIndent + "&sicky.FlagSwitch{\n" +
		fieldIndent + `Flag:     "initdb",` + "\n" +
		fieldIndent + `Usage:    "Initialize the database tables and exit",` + "\n" +
		fieldIndent + "Callback: initdb,\n" +
		argIndent + "},\n" +
		baseIndent

	return sourceEdit{start: start, end: end, text: text}
}

// insertInitDB adds the initdb function before the trailing modeline, so the
// editor directive stays last.
func insertInitDB(src []byte, fset *token.FileSet, file *ast.File, exportedName string) sourceEdit {
	offset := trailingCommentOffset(src, fset, file)

	return sourceEdit{start: offset, end: offset, text: "\n" + initDBFunc(exportedName) + "\n"}
}

// initDBFunc renders the initdb function body. uuid_generate_v4() comes from
// uuid-ossp, which a clean PostgreSQL database does not have installed, so the
// extension is created before the first table.
func initDBFunc(exportedName string) string {
	const createExtension = "`CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"`"

	return fmt.Sprintf(`// initdb creates the uuid-ossp extension and every model's table, then exits
// so the process does not proceed to serve. It runs from the --initdb flag
// after infrastructure initialization.
func initdb() error {
	ctx := context.Background()

	if infra.Bun == nil {
		logger.Logger.FatalContext(ctx, "database connection is not initialized")
	}

	// uuid_generate_v4() comes from uuid-ossp, which a clean PostgreSQL
	// database does not have installed.
	if _, err := infra.Bun.ExecContext(ctx, %s); err != nil {
		return err
	}

	if err := (model.%s{}).InitDB(ctx, false); err != nil {
		return err
	}

	os.Exit(0)

	return nil
}`, createExtension, exportedName)
}

// insertModelInitDB adds one InitDB call to an existing initdb, ahead of the
// exit so it runs with the rest of the table setup.
func insertModelInitDB(src []byte, fset *token.FileSet, fn *ast.FuncDecl, exportedName string) sourceEdit {
	statement := fmt.Sprintf("if err := (model.%s{}).InitDB(ctx, false); err != nil {\n\treturn err\n}", exportedName)

	anchor := exitStatement(fn)
	if anchor != nil {
		pos := fset.Position(anchor.Pos())
		lineStart := lineStartOffset(pos)
		indent := string(src[lineStart:pos.Offset])

		return sourceEdit{start: lineStart, end: lineStart, text: indent + statement + "\n\n"}
	}

	closePos := fset.Position(fn.Body.Rbrace)
	lineStart := lineStartOffset(closePos)
	indent := string(src[lineStart:closePos.Offset])

	return sourceEdit{start: lineStart, end: lineStart, text: indent + statement + "\n"}
}

// exitStatement returns the statement the new InitDB call must precede: an
// os.Exit call when the function ends with one, otherwise the last return.
func exitStatement(fn *ast.FuncDecl) ast.Stmt {
	if fn.Body == nil {
		return nil
	}

	var lastReturn ast.Stmt

	for _, stmt := range slices.Backward(fn.Body.List) {
		if expr, ok := stmt.(*ast.ExprStmt); ok && isCallNamed(expr.X, "Exit") {
			return stmt
		}

		if _, ok := stmt.(*ast.ReturnStmt); ok && lastReturn == nil {
			lastReturn = stmt
		}
	}

	return lastReturn
}

// hasModelInitDB reports whether fn already initializes the model's table.
func hasModelInitDB(fn *ast.FuncDecl, exportedName string) bool {
	found := false

	ast.Inspect(fn.Body, func(node ast.Node) bool {
		if found {
			return false
		}

		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "InitDB" {
			return true
		}

		// The scaffold writes `(model.Key{}).InitDB(...)`; the receiver is
		// parenthesized so the literal stays addressable.
		lit, ok := unparen(sel.X).(*ast.CompositeLit)
		if !ok || !isModelType(lit.Type, exportedName) {
			return true
		}

		found = true

		return false
	})

	return found
}

// httpServerStatements locates the `httpSrv := srvHTTP.New(...)` assignment
// and the last `httpSrv.Handle(...)` call in main.
func httpServerStatements(fn *ast.FuncDecl) (*ast.AssignStmt, *ast.ExprStmt) {
	if fn.Body == nil {
		return nil, nil
	}

	var assign *ast.AssignStmt
	var lastHandle *ast.ExprStmt

	for _, stmt := range fn.Body.List {
		switch typed := stmt.(type) {
		case *ast.AssignStmt:
			if isHTTPNewAssignment(typed) {
				assign = typed
			}
		case *ast.ExprStmt:
			if isHTTPHandleCall(typed.X) {
				lastHandle = typed
			}
		}
	}

	return assign, lastHandle
}

// hasHTTPHandler reports whether main already registers the model's handler.
func hasHTTPHandler(fn *ast.FuncDecl, exportedName string) bool {
	if fn.Body == nil {
		return false
	}

	found := false

	ast.Inspect(fn.Body, func(node ast.Node) bool {
		if found {
			return false
		}

		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}

		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Handle" || len(call.Args) != 1 {
			return true
		}

		httpSrv, ok := selector.X.(*ast.Ident)
		if !ok || httpSrv.Name != "httpSrv" {
			return true
		}

		handlerCall, ok := call.Args[0].(*ast.CallExpr)
		if !ok {
			return true
		}

		handlerSel, ok := handlerCall.Fun.(*ast.SelectorExpr)
		if !ok || handlerSel.Sel.Name != "NewHTTP"+exportedName {
			return true
		}

		pkg, ok := handlerSel.X.(*ast.Ident)
		if ok && pkg.Name == "handler" {
			found = true

			return false
		}

		return true
	})

	return found
}

// appendStatement inserts one statement after anchor, at the same indentation.
func appendStatement(src []byte, fset *token.FileSet, anchor ast.Stmt, statement string) sourceEdit {
	end := fset.Position(anchor.End()).Offset
	indent := lineIndent(src, fset.Position(anchor.Pos()).Offset)

	return sourceEdit{start: end, end: end, text: "\n" + indent + statement}
}

// importEdits adds the missing imports to the file's import block. Standard
// library paths go into the first group and module paths into the last one, so
// the result keeps the grouping the scaffold ships. Paths are sorted, and
// gofmt then orders them inside their group.
func importEdits(src []byte, fset *token.FileSet, file *ast.File, paths []string) []sourceEdit {
	sort.Strings(paths)

	decl := importDeclaration(file)
	if decl == nil {
		offset := fset.Position(file.Name.End()).Offset
		standard, external := splitImportPaths(paths)

		var build strings.Builder

		build.WriteString("\n\nimport (\n")

		if len(standard) > 0 {
			build.WriteString(formatImportPaths(standard))
			build.WriteString("\n")

			if len(external) > 0 {
				build.WriteString("\n")
			}
		}

		if len(external) > 0 {
			build.WriteString(formatImportPaths(external))
			build.WriteString("\n")
		}

		build.WriteString(")")

		return []sourceEdit{{start: offset, end: offset, text: build.String()}}
	}

	if !decl.Lparen.IsValid() {
		offset := fset.Position(decl.End()).Offset

		var build strings.Builder
		for _, importPath := range paths {
			fmt.Fprintf(&build, "\nimport %q", importPath)
		}

		return []sourceEdit{{start: offset, end: offset, text: build.String()}}
	}

	standard, external := splitImportPaths(paths)

	var edits []sourceEdit

	if len(standard) > 0 {
		edits = append(edits, standardImportEdit(src, fset, decl, standard))
	}

	if len(external) > 0 {
		edits = append(edits, externalImportEdit(src, fset, decl, external))
	}

	return edits
}

// standardImportEdit inserts the standard library paths after the last
// standard import in the block, creating a leading group when there is none.
func standardImportEdit(src []byte, fset *token.FileSet, decl *ast.GenDecl, paths []string) sourceEdit {
	var last *ast.ImportSpec

	for _, spec := range decl.Specs {
		importSpec, ok := spec.(*ast.ImportSpec)
		if !ok {
			continue
		}

		path, err := strconv.Unquote(importSpec.Path.Value)
		if err == nil && isStandardImportPath(path) {
			last = importSpec
		}
	}

	lines := formatImportPaths(paths)
	if last == nil {
		offset := fset.Position(decl.Lparen).Offset + 1

		return sourceEdit{start: offset, end: offset, text: "\n" + lines + "\n"}
	}

	// Insert at the end of the last standard import's line, keeping any
	// trailing comment attached to its own spec.
	offset := fset.Position(last.End()).Offset
	for offset < len(src) && src[offset] != '\n' {
		offset++
	}

	return sourceEdit{start: offset, end: offset, text: "\n" + lines}
}

// externalImportEdit appends module paths to the block's last group.
func externalImportEdit(src []byte, fset *token.FileSet, decl *ast.GenDecl, paths []string) sourceEdit {
	offset := fset.Position(decl.Rparen).Offset

	var build strings.Builder

	if offset > 0 && src[offset-1] != '\n' {
		build.WriteString("\n")
	}

	// A block that only has standard imports needs a separating blank line
	// before its first external group.
	if !hasExternalImport(decl) {
		build.WriteString("\n")
	}

	build.WriteString(formatImportPaths(paths))
	build.WriteString("\n")

	return sourceEdit{start: offset, end: offset, text: build.String()}
}

// splitImportPaths separates standard library paths from everything else. A
// first path segment containing a dot belongs to a host name, so it is not
// part of the standard library.
func splitImportPaths(paths []string) (standard, external []string) {
	for _, importPath := range paths {
		if isStandardImportPath(importPath) {
			standard = append(standard, importPath)
		} else {
			external = append(external, importPath)
		}
	}

	return standard, external
}

// isStandardImportPath reports whether path belongs to the standard library.
func isStandardImportPath(path string) bool {
	first, _, _ := strings.Cut(path, "/")

	return !strings.Contains(first, ".")
}

// hasExternalImport reports whether the block already has a module path.
func hasExternalImport(decl *ast.GenDecl) bool {
	for _, spec := range decl.Specs {
		importSpec, ok := spec.(*ast.ImportSpec)
		if !ok {
			continue
		}

		path, err := strconv.Unquote(importSpec.Path.Value)
		if err == nil && !isStandardImportPath(path) {
			return true
		}
	}

	return false
}

// formatImportPaths renders one tab-indented quoted path per line.
func formatImportPaths(paths []string) string {
	lines := make([]string, 0, len(paths))
	for _, importPath := range paths {
		lines = append(lines, "\t"+strconv.Quote(importPath))
	}

	return strings.Join(lines, "\n")
}

// applyEdits applies replacements from the end of the file backwards so every
// original offset stays valid, then formats the result with gofmt.
func applyEdits(src []byte, edits []sourceEdit) ([]byte, error) {
	ordered := append([]sourceEdit(nil), edits...)

	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].start > ordered[j].start })

	out := append([]byte(nil), src...)

	for _, edit := range ordered {
		if edit.start < 0 || edit.end < edit.start || edit.end > len(src) {
			return nil, fmt.Errorf("invalid edit range %d:%d for a %d-byte file", edit.start, edit.end, len(src))
		}

		size := max(len(out)+len(edit.text)-(edit.end-edit.start), 0)

		next := make([]byte, 0, size)
		next = append(next, out[:edit.start]...)
		next = append(next, edit.text...)
		next = append(next, out[edit.end:]...)
		out = next
	}

	formatted, err := format.Source(out)
	if err != nil {
		return nil, fmt.Errorf("format: %w", err)
	}

	return formatted, nil
}

// writeAtomic replaces path with data through a temporary file in the same
// directory, so a failed write cannot leave a half-written source file. The
// original permissions are preserved when the file already exists.
func writeAtomic(path string, data []byte) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".sicky-wire-*")
	if err != nil {
		return err
	}

	tmpName := tmp.Name()

	defer func() {
		_ = os.Remove(tmpName) // no-op after a successful rename
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()

		return err
	}

	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()

		return err
	}

	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmpName, path)
}

// jsonObject returns parent[key] as an object, creating it when absent. A
// value of any other type is an error rather than a silent overwrite.
func jsonObject(parent map[string]any, key, path string) (map[string]any, error) {
	value, ok := parent[key]
	if !ok || value == nil {
		child := make(map[string]any)
		parent[key] = child

		return child, nil
	}

	child, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be a JSON object, got %T", path, value)
	}

	return child, nil
}

// parseDeclarations parses a declaration snippet, adding a package clause
// when the snippet is a bare list of declarations.
func parseDeclarations(decls string) ([]byte, *token.FileSet, *ast.File, error) {
	snippetSrc := []byte(decls)
	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, "decls.go", snippetSrc, parser.ParseComments)
	if err == nil {
		return snippetSrc, fset, file, nil
	}

	snippetSrc = append([]byte("package decls\n\n"), decls...)
	fset = token.NewFileSet()

	file, err = parser.ParseFile(fset, "decls.go", snippetSrc, parser.ParseComments)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parse declarations: %w", err)
	}

	return snippetSrc, fset, file, nil
}

// declarationText extracts the source of one declared type. A grouped
// `type (...)` may contain types the target already has, so each missing spec
// gets its own `type` keyword instead of copying the whole group.
func declarationText(src []byte, fset *token.FileSet, gen *ast.GenDecl, spec *ast.TypeSpec) string {
	if len(gen.Specs) == 1 {
		start := gen.Pos()
		if gen.Doc != nil {
			start = gen.Doc.Pos()
		}

		return string(src[fset.Position(start).Offset:fset.Position(gen.End()).Offset])
	}

	start := spec.Pos()
	if spec.Doc != nil {
		start = spec.Doc.Pos()
	}

	return "type " + string(src[fset.Position(start).Offset:fset.Position(spec.End()).Offset])
}

// declaredTypes collects the top-level type names path already declares.
func declaredTypes(file *ast.File) map[string]bool {
	names := make(map[string]bool)

	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}

		for _, spec := range gen.Specs {
			if typeSpec, ok := spec.(*ast.TypeSpec); ok {
				names[typeSpec.Name.Name] = true
			}
		}
	}

	return names
}

// trailingCommentOffset returns the start of a trailing modeline comment, or
// the end of the file when there is none. New declarations go here so the vim
// directive keeps applying to the whole file.
func trailingCommentOffset(src []byte, fset *token.FileSet, file *ast.File) int {
	var lastDeclEnd token.Pos
	if len(file.Decls) > 0 {
		lastDeclEnd = file.Decls[len(file.Decls)-1].End()
	}

	for _, group := range file.Comments {
		if !strings.Contains(group.Text(), "vim600:") {
			continue
		}

		if lastDeclEnd.IsValid() && group.Pos() < lastDeclEnd {
			continue
		}

		return lineStartOffset(fset.Position(group.Pos()))
	}

	return len(src)
}

// funcDeclaration returns the top-level function with the given name.
func funcDeclaration(file *ast.File, name string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == name {
			return fn
		}
	}

	return nil
}

// findInitCall locates the sicky.Init call inside main.
func findInitCall(fn *ast.FuncDecl) *ast.CallExpr {
	var found *ast.CallExpr

	ast.Inspect(fn.Body, func(node ast.Node) bool {
		if found != nil {
			return false
		}

		call, ok := node.(*ast.CallExpr)
		if !ok || !isSickyCall(call, "Init") {
			return true
		}

		found = call

		return false
	})

	return found
}

// optionsComposite returns the &sicky.Options literal passed to sicky.Init.
func optionsComposite(call *ast.CallExpr) *ast.CompositeLit {
	if len(call.Args) == 0 {
		return nil
	}

	lit := compositeLiteral(call.Args[0])
	if lit == nil || !isSickyType(lit.Type, "Options") {
		return nil
	}

	return lit
}

// hasInitDBFlagSwitch reports whether sicky.Init already carries the initdb
// switch, in which case the callback is someone else's to own.
func hasInitDBFlagSwitch(call *ast.CallExpr) bool {
	for _, arg := range call.Args {
		lit := compositeLiteral(arg)
		if lit == nil || !isSickyType(lit.Type, "FlagSwitch") {
			continue
		}

		kv := compositeKey(lit, "Flag")
		if kv == nil {
			continue
		}

		if value, ok := stringLiteral(kv.Value); ok && value == "initdb" {
			return true
		}
	}

	return false
}

// isHTTPNewAssignment matches `httpSrv := srvHTTP.New(...)`.
func isHTTPNewAssignment(stmt *ast.AssignStmt) bool {
	if stmt.Tok != token.DEFINE && stmt.Tok != token.ASSIGN {
		return false
	}

	if len(stmt.Lhs) != 1 || len(stmt.Rhs) != 1 {
		return false
	}

	ident, ok := stmt.Lhs[0].(*ast.Ident)
	if !ok || ident.Name != "httpSrv" {
		return false
	}

	call, ok := stmt.Rhs[0].(*ast.CallExpr)
	if !ok {
		return false
	}

	selector, ok := call.Fun.(*ast.SelectorExpr)

	return ok && selector.Sel.Name == "New"
}

// isHTTPHandleCall matches `httpSrv.Handle(...)`.
func isHTTPHandleCall(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}

	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Handle" {
		return false
	}

	ident, ok := selector.X.(*ast.Ident)

	return ok && ident.Name == "httpSrv"
}

// isSickyCall matches a `<name>(...)` call on the sicky package.
func isSickyCall(call *ast.CallExpr, name string) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	pkg, ok := selector.X.(*ast.Ident)

	return ok && pkg.Name == "sicky" && selector.Sel.Name == name
}

// isCallNamed reports whether expr is a method call with the given name.
func isCallNamed(expr ast.Expr, name string) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}

	selector, ok := call.Fun.(*ast.SelectorExpr)

	return ok && selector.Sel.Name == name
}

// isSickyType matches the `sicky.<name>` type expression.
func isSickyType(expr ast.Expr, name string) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	pkg, ok := selector.X.(*ast.Ident)

	return ok && pkg.Name == "sicky" && selector.Sel.Name == name
}

// isModelType matches the `model.<name>` type expression.
func isModelType(expr ast.Expr, name string) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	pkg, ok := selector.X.(*ast.Ident)

	return ok && pkg.Name == "model" && selector.Sel.Name == name
}

// unparen strips any number of parentheses around an expression.
func unparen(expr ast.Expr) ast.Expr {
	for {
		paren, ok := expr.(*ast.ParenExpr)
		if !ok {
			return expr
		}

		expr = paren.X
	}
}

// compositeLiteral unwraps an optional & and returns the literal, if any.
func compositeLiteral(expr ast.Expr) *ast.CompositeLit {
	if unary, ok := expr.(*ast.UnaryExpr); ok && unary.Op == token.AND {
		expr = unary.X
	}

	lit, _ := expr.(*ast.CompositeLit)

	return lit
}

// compositeKey returns the key/value element with the given field name.
func compositeKey(lit *ast.CompositeLit, name string) *ast.KeyValueExpr {
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}

		key, ok := kv.Key.(*ast.Ident)
		if ok && key.Name == name {
			return kv
		}
	}

	return nil
}

// stringLiteral returns the unquoted value of a string literal.
func stringLiteral(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}

	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}

	return value, true
}

// importPaths collects the paths the file already imports.
func importPaths(file *ast.File) map[string]bool {
	paths := make(map[string]bool, len(file.Imports))

	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}

		paths[path] = true
	}

	return paths
}

// importDeclaration returns the file's first import declaration.
func importDeclaration(file *ast.File) *ast.GenDecl {
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if ok && gen.Tok == token.IMPORT {
			return gen
		}
	}

	return nil
}

// lineStartOffset returns the byte offset of the first character of pos's
// line.
func lineStartOffset(pos token.Position) int {
	return pos.Offset - (pos.Column - 1)
}

// lineIndent returns the leading whitespace of the line containing offset.
func lineIndent(src []byte, offset int) string {
	start := offset
	for start > 0 && src[start-1] != '\n' {
		start--
	}

	end := start
	for end < len(src) && (src[end] == ' ' || src[end] == '\t') {
		end++
	}

	return string(src[start:end])
}
