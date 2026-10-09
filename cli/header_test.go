/*
 * Copyright (c) 2026, The go-sicky Authors
 * SPDX-License-Identifier: MIT
 *
 * @file    header_test.go
 * @package cli
 * @author  Dr.NP <np@herewe.tech>
 * @since   10/09/2026
 */

package cli

import (
	"encoding/json"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The header policy used to live inside 18 templates, each carrying its own
// copy of the MIT text and none carrying the @file block or the modeline
// this repository's own sources use. These tests pin the policy itself, and
// the last two drive the real binary so a change that only breaks the
// rendered project cannot stay green.

func TestResolveLicense(t *testing.T) {
	cases := []struct {
		in      string
		wantKey string
		wantSPX string
	}{
		{"", "mit", "MIT"},
		{"mit", "mit", "MIT"},
		{"MIT", "mit", "MIT"},
		{"apache-2.0", "apache-2.0", "Apache-2.0"},
		{"Apache2", "apache-2.0", "Apache-2.0"},
		{"gpl-3.0", "gpl-3.0", "GPL-3.0-only"},
		{"bsd-3-clause", "bsd-3", "BSD-3-Clause"},
		{"commercial", "commercial", "LicenseRef-Proprietary"},
		{"proprietary", "commercial", "LicenseRef-Proprietary"},
		// An unrecognized license falls back rather than failing: it names
		// nothing but this project's own file, so a typo must not cost the
		// user a scaffold.
		{"not-a-license", "mit", "MIT"},
	}

	for _, c := range cases {
		got := resolveLicense(c.in)

		if got.key != c.wantKey || got.spdx != c.wantSPX {
			t.Errorf("resolveLicense(%q) = {%q, %q}, want {%q, %q}", c.in, got.key, got.spdx, c.wantKey, c.wantSPX)
		}
	}
}

// TestLicenseInputAcceptsMenuNumbers covers the interactive path: the menu
// offers numbers, so a "3" must reach the same choice as a "gpl-3.0". An
// unrecognized answer must come back verbatim so resolveLicense can warn
// using the words the user actually typed.
func TestLicenseInputAcceptsMenuNumbers(t *testing.T) {
	cases := map[string]string{
		"":            "mit",
		"1":           "mit",
		"2":           "apache-2.0",
		"3":           "gpl-3.0",
		"4":           "bsd-3",
		"5":           "commercial",
		"Apache-2.0":  "apache-2.0",
		"not a thing": "not a thing",
		"0":           "0",
		"6":           "6",
	}

	for in, want := range cases {
		if got := licenseInput(in); got != want {
			t.Errorf("licenseInput(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestIdentifyShippedLicenses is the round trip that makes `sicky generate`
// safe inside an existing project: every text this CLI can write must be
// recognized as the license it claims to be, or a generated file would join
// the project under the wrong SPDX line.
func TestIdentifyShippedLicenses(t *testing.T) {
	if len(licenseChoices) != 5 {
		t.Fatalf("licenseChoices has %d entries, want 5", len(licenseChoices))
	}

	for _, c := range licenseChoices {
		body, err := projectTemplates.ReadFile("template/license/" + c.key + ".gotmpl")
		if err != nil {
			t.Fatalf("read license template %s: %v", c.key, err)
		}

		got, ok := identifyLicense(string(body))
		if !ok {
			t.Errorf("%s: not identified at all", c.key)

			continue
		}

		if got.key != c.key {
			t.Errorf("%s: identified as %q", c.key, got.key)
		}
	}
}

func TestIdentifyUnknownLicenseBody(t *testing.T) {
	if _, ok := identifyLicense("All rights reserved. Ask us first."); ok {
		t.Error("an unrecognized license body must not be identified as any known license")
	}
}

// TestInjectionClassifiesFiles pins the scope decision: .go gets a header
// and a modeline, build scripts get a header only, and everything else is
// left alone. config.json is the case that matters most — a comment in it
// makes encoding/json reject the whole document, which is why JSON is out
// of scope rather than overlooked.
func TestInjectionClassifiesFiles(t *testing.T) {
	fm := fileMeta{
		Name: "demo", SPDX: "MIT", Holder: "Example Corp", Year: "2026",
		Author: "Ada <ada@example.com>", File: "main.go", Package: "main",
		Since: "10/09/2026",
	}

	goSrc := "package main\n\nfunc main() {}\n"
	got := injectFileHeader("main.go", goSrc, &fm)

	for _, want := range []string{
		"* Copyright (c) 2026 Example Corp",
		"* SPDX-License-Identifier: MIT",
		"* @file main.go",
		"* @package main",
		"* @author Ada <ada@example.com>",
		"* @since 10/09/2026",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("go header missing %q:\n%s", want, got)
		}
	}

	if !strings.HasPrefix(got, "/*\n") {
		t.Errorf("go header must start the file, got: %.40q", got)
	}

	makeGot := injectFileHeader("Makefile", "PROJECT := demo\n", &fm)
	if !strings.HasPrefix(makeGot, "# Copyright (c) 2026 Example Corp\n# SPDX-License-Identifier: MIT\n\n") {
		t.Errorf("script header wrong:\n%s", makeGot)
	}

	if strings.Contains(makeGot, "@file") {
		t.Error("a Makefile has no package and no @file line")
	}

	jsonBody := "{\n    \"a\": 1\n}\n"
	if unchanged := injectFileHeader("config.json", jsonBody, &fm); unchanged != jsonBody {
		t.Errorf("config.json must be untouched, got:\n%s", unchanged)
	}

	if unchanged := injectFileHeader("go.mod", "module x\n", &fm); unchanged != "module x\n" {
		t.Errorf("go.mod must be untouched, got:\n%s", unchanged)
	}
}

// TestInjectionIsIdempotent keeps the injector safe to call from more than
// one path: a double header would duplicate the copyright and hand gofmt a
// file with two package doc comments.
func TestInjectionIsIdempotent(t *testing.T) {
	fm := fileMeta{SPDX: "MIT", Holder: "H", Year: "2026", Author: "A", File: "main.go", Package: "main", Since: "10/09/2026"}

	once := injectFileHeader("main.go", "package main\n", &fm)
	if twice := injectFileHeader("main.go", once, &fm); twice != once {
		t.Errorf("second header injection changed the file:\n%s", twice)
	}

	footerOnce := injectFileFooter("main.go", once)
	if footerTwice := injectFileFooter("main.go", footerOnce); footerTwice != footerOnce {
		t.Errorf("second footer injection changed the file:\n%s", footerTwice)
	}
}

// TestGeneratedGoIsGofmtClean proves the injected header and modeline are
// not merely present but stable: scaffoldProject runs format.Source over
// every .go file after injection, so a header gofmt would rewrite would
// fail the scaffold rather than ship a dirty file.
func TestGeneratedGoIsGofmtClean(t *testing.T) {
	fm := fileMeta{SPDX: "MIT", Holder: "H", Year: "2026", Author: "A", File: "main.go", Package: "main", Since: "10/09/2026"}

	raw := injectFileFooter("main.go", injectFileHeader("main.go", "package main\n\nfunc main() {}\n", &fm))

	formatted, err := format.Source([]byte(raw))
	if err != nil {
		t.Fatalf("format: %v\n%s", err, raw)
	}

	if string(formatted) != raw {
		t.Errorf("header + modeline is not gofmt-clean:\n--- got ---\n%s\n--- want ---\n%s", raw, formatted)
	}
}

func TestInjectFileFooterScope(t *testing.T) {
	body := "package main\n"

	if got := injectFileFooter("Makefile", body); got != body {
		t.Errorf("a build script must not carry a vim modeline, got:\n%s", got)
	}

	if got := injectFileFooter("main.go", body); !strings.Contains(got, "vim600: sw=4 ts=4 fdm=marker") {
		t.Errorf("a .go file must carry the modeline, got:\n%s", got)
	}
}

func TestPackageOf(t *testing.T) {
	if got := packageOf("package handler\n\nfunc F() {}\n"); got != "handler" {
		t.Errorf("packageOf = %q, want handler", got)
	}

	// A source with no readable package clause yields the placeholder
	// rather than a wrong value.
	if got := packageOf("no package here"); got != placeholderPackage {
		t.Errorf("packageOf = %q, want %q", got, placeholderPackage)
	}
}

// TestValidateHeaderText covers the one field that is an error rather than
// a warn-and-default: a name is interpolated into every generated file, and
// a "*/" or a newline there closes the comment block and becomes source.
func TestValidateHeaderText(t *testing.T) {
	for _, bad := range []string{"bad */ name", "line1\nline2", "carriage\rreturn"} {
		if err := validateHeaderText("author", bad); err == nil {
			t.Errorf("validateHeaderText(%q) accepted a value that breaks the comment block", bad)
		}
	}

	if err := validateHeaderText("author", strings.Repeat("x", 201)); err == nil {
		t.Error("validateHeaderText accepted an over-long value")
	}

	for _, good := range []string{"", "Ada Lovelace <ada@example.com>", "Example Corp"} {
		if err := validateHeaderText("author", good); err != nil {
			t.Errorf("validateHeaderText(%q) = %v, want nil", good, err)
		}
	}
}

// TestScaffoldWritesLicenseAndFileHeaders drives the real binary, because
// the wiring — flags, injection, LICENSE rendering — is exactly what a
// template-level test cannot see.
func TestScaffoldWritesLicenseAndFileHeaders(t *testing.T) {
	dir := t.TempDir()

	full := []string{
		"new", "hdrdemo",
		"--module", "example.com/hdrdemo",
		"--type", "standard",
		"--no-grpc", "--fiber=false", "--http",
		"--license", "apache-2.0",
		"--author", "Ada <ada@example.com>",
		"--copyright", "Example Corp",
		"-o", dir,
	}

	if out, err := exec.Command(scaffoldBinary(t), full...).CombinedOutput(); err != nil {
		t.Fatalf("scaffold: %v\n%s", err, out)
	}

	project := filepath.Join(dir, "hdrdemo")

	license, err := os.ReadFile(filepath.Join(project, "LICENSE"))
	if err != nil {
		t.Fatalf("read LICENSE: %v", err)
	}

	if !strings.HasPrefix(string(license), "Apache License\n") {
		t.Errorf("LICENSE is not the Apache-2.0 text:\n%.60s", license)
	}

	if !strings.Contains(string(license), "Copyright [2026] [Example Corp]") {
		t.Errorf("LICENSE does not carry the selected copyright line:\n%.200s", license)
	}

	raw, err := os.ReadFile(filepath.Join(project, "main.go"))
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}

	mainGo := string(raw)

	for _, want := range []string{
		"* Copyright (c) 2026 Example Corp",
		"* SPDX-License-Identifier: Apache-2.0",
		"* @file main.go",
		"* @package main",
		"* @author Ada <ada@example.com>",
		"vim600: sw=4 ts=4 fdm=marker",
	} {
		if !strings.Contains(mainGo, want) {
			t.Errorf("main.go missing %q:\n%s", want, mainGo)
		}
	}

	if strings.Contains(mainGo, "The MIT License (MIT)") {
		t.Error("main.go still carries the old hardcoded MIT text")
	}

	if formatted, ferr := format.Source([]byte(mainGo)); ferr != nil || string(formatted) != mainGo {
		t.Errorf("main.go is not gofmt-clean: %v", ferr)
	}

	// The build script gets the two-line head and, by decision, no modeline.
	makeRaw, err := os.ReadFile(filepath.Join(project, "Makefile"))
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}

	makefile := string(makeRaw)

	if !strings.HasPrefix(makefile, "# Copyright (c) 2026 Example Corp\n# SPDX-License-Identifier: Apache-2.0\n\n") {
		t.Errorf("Makefile header wrong:\n%.200s", makefile)
	}

	if strings.Contains(makefile, "vim600") {
		t.Error("Makefile must not carry a vim modeline")
	}

	// config.json stays parseable: a comment in it would make the
	// generated service unable to read its own configuration.
	cfgRaw, err := os.ReadFile(filepath.Join(project, "config.json"))
	if err != nil {
		t.Fatalf("read config.json: %v", err)
	}

	var cfg map[string]any
	if jerr := json.Unmarshal(cfgRaw, &cfg); jerr != nil {
		t.Fatalf("config.json no longer parses: %v\n%s", jerr, cfgRaw)
	}

	if strings.Contains(string(cfgRaw), "SPDX") {
		t.Error("config.json must not carry a header comment")
	}
}

// TestScaffoldUnknownLicenseWarnsAndDefaultsToMIT is the invalid-value
// contract: a typo in --license is a warning and a default, not a failed
// run, and the warning must name the value the user typed.
func TestScaffoldUnknownLicenseWarnsAndDefaultsToMIT(t *testing.T) {
	dir := t.TempDir()

	full := []string{
		"new", "hdrdemo",
		"--module", "example.com/hdrdemo",
		"--type", "standard",
		"--no-grpc", "--fiber=false", "--http",
		"--license", "not-a-license",
		"-o", dir,
	}

	cmd := exec.Command(scaffoldBinary(t), full...)

	var stdout, stderr strings.Builder

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("scaffold: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}

	if !strings.Contains(stderr.String(), `unknown license "not-a-license"`) {
		t.Errorf("stderr does not warn about the rejected value:\n%s", stderr.String())
	}

	license, err := os.ReadFile(filepath.Join(dir, "hdrdemo", "LICENSE"))
	if err != nil {
		t.Fatalf("read LICENSE: %v", err)
	}

	if !strings.HasPrefix(string(license), "MIT License\n") {
		t.Errorf("an unknown license must fall back to MIT:\n%.60s", license)
	}
}

// TestScaffoldRejectsHeaderInjection is the negative control for
// validateHeaderText reaching the binary: a "*/" in --author would close
// the comment block in every generated file and turn a name into code.
func TestScaffoldRejectsHeaderInjection(t *testing.T) {
	dir := t.TempDir()

	full := []string{
		"new", "hdrdemo",
		"--module", "example.com/hdrdemo",
		"--type", "standard",
		"--no-grpc", "--fiber=false", "--http",
		"--author", "bad */ name",
		"-o", dir,
	}

	cmd := exec.Command(scaffoldBinary(t), full...)

	var stderr strings.Builder

	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatal("scaffold accepted an author that closes the comment block")
	}

	if !strings.Contains(stderr.String(), "must not contain") {
		t.Errorf("stderr does not explain the rejection:\n%s", stderr.String())
	}

	if _, statErr := os.Stat(filepath.Join(dir, "hdrdemo")); statErr == nil {
		t.Error("a rejected scaffold must not leave a project behind")
	}
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
