/*
 * Copyright (c) 2026, The go-sicky Authors
 * SPDX-License-Identifier: MIT
 *
 * @file    header.go
 * @package cli
 * @author  Dr.NP <np@herewe.tech>
 * @since   10/09/2026
 */

// Package cli implements the `sicky` command line. This file owns the
// file-header and license policy applied to everything the CLI scaffolds.
package cli

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Placeholders written into a generated file when the value could not be
// confirmed at generation time. They stay visible on purpose: a project
// whose copyright holder is unknown must not silently inherit someone
// else's name, and a literal the author can grep beats a wrong one.
const (
	placeholderHolder  = "${HOLDER}"
	placeholderAuthor  = "${AUTHOR}"
	placeholderPackage = "${PACKAGE}"
)

// licenseChoice is one selectable license: the template file it renders
// from (template/license/<key>.gotmpl), the SPDX identifier written into
// generated file headers, and the spellings accepted on the command line.
type licenseChoice struct {
	key     string
	spdx    string
	display string
	labels  []string
}

// licenseChoices is the full selection offered by `sicky new --license`.
// The order is the interactive menu order; index 0 is the default.
var licenseChoices = []licenseChoice{
	{key: "mit", spdx: "MIT", display: "MIT", labels: []string{"mit", "mit-license"}},
	{key: "apache-2.0", spdx: "Apache-2.0", display: "Apache-2.0", labels: []string{"apache-2.0", "apache2", "apache"}},
	{key: "gpl-3.0", spdx: "GPL-3.0-only", display: "GPL-3.0", labels: []string{"gpl-3.0", "gpl3", "gpl", "gpl-3.0-only"}},
	{key: "bsd-3", spdx: "BSD-3-Clause", display: "BSD-3-Clause", labels: []string{"bsd-3-clause", "bsd-3", "bsd3", "bsd"}},
	{key: "commercial", spdx: "LicenseRef-Proprietary", display: "Commercial License", labels: []string{"commercial", "commercial-license", "proprietary"}},
}

// licenseSignatures identify a license body by phrases unique to it. Order
// matters: every entry must be specific enough not to match an earlier one,
// so a generic phrase is never tested before the specific phrase that
// contains it.
var licenseSignatures = []struct {
	key     string
	markers []string
}{
	{key: "gpl-3.0", markers: []string{"GNU GENERAL PUBLIC LICENSE", "Version 3"}},
	{key: "apache-2.0", markers: []string{"Apache License", "Version 2.0, January 2004"}},
	{key: "bsd-3", markers: []string{"Redistribution and use in source and binary forms", "Neither the name of"}},
	{key: "commercial", markers: []string{"licensing@herewe.tech", "commercial license"}},
	{key: "mit", markers: []string{"Permission is hereby granted, free of charge"}},
}

// licenseFileNames are the files searched for a project's existing license.
var licenseFileNames = []string{"LICENSE", "LICENSE.md", "LICENSE.txt", "COPYING"}

// defaultLicense returns the selection used when nothing was asked for.
func defaultLicense() licenseChoice {
	return licenseChoices[0]
}

// licenseForKey resolves a canonical key or SPDX identifier.
func licenseForKey(key string) (licenseChoice, bool) {
	v := strings.ToLower(strings.TrimSpace(key))
	if v == "" {
		return licenseChoice{}, false
	}

	for _, c := range licenseChoices {
		if c.key == v || strings.EqualFold(c.spdx, v) || slices.Contains(c.labels, v) {
			return c, true
		}
	}

	return licenseChoice{}, false
}

// lookupLicense resolves a user-typed license name. It is a pure lookup:
// an empty value is not a match, so callers decide whether empty means
// "unspecified" (quiet default) or "invalid" (warn).
func lookupLicense(input string) (licenseChoice, bool) {
	v := strings.ToLower(strings.TrimSpace(input))
	if v == "" {
		return licenseChoice{}, false
	}

	for _, c := range licenseChoices {
		if strings.EqualFold(c.display, v) || slices.Contains(c.labels, v) {
			return c, true
		}
	}

	return licenseChoice{}, false
}

// resolveLicense turns a `--license` value or an interactive answer into a
// choice, warning and falling back to MIT on an unrecognized one. An empty
// value is the default rather than a mistake, so it resolves quietly.
func resolveLicense(input string) licenseChoice {
	if c, ok := lookupLicense(input); ok {
		return c
	}

	if strings.TrimSpace(input) != "" {
		fmt.Fprintf(os.Stderr, "sicky: warn: unknown license %q, using %s\n", strings.TrimSpace(input), defaultLicense().display)
	}

	return defaultLicense()
}

// fileMeta carries everything a generated file's header and footer renders
// from. Holder, Author and Package fall back to a visible placeholder when
// the value could not be confirmed.
type fileMeta struct {
	Name    string
	SPDX    string
	Holder  string
	Year    string
	Author  string
	File    string
	Package string
	Since   string
}

// newFileMeta builds the time- and identity-dependent part of a header.
// Year and Since are always known (they are the generation date); Holder
// and Author may come back as placeholders.
func newFileMeta(name, spdx, holder, author string) fileMeta {
	now := time.Now()

	return fileMeta{
		Name:   name,
		SPDX:   spdx,
		Holder: orPlaceholder(holder, placeholderHolder),
		Year:   now.Format("2006"),
		Author: orPlaceholder(author, placeholderAuthor),
		Since:  now.Format("01/02/2006"),
	}
}

// orPlaceholder returns the confirmed value, or the placeholder when the
// value is absent or blank.
func orPlaceholder(value, placeholder string) string {
	if strings.TrimSpace(value) == "" {
		return placeholder
	}

	return value
}

// goHeader renders the comment block prepended to every generated .go
// file: the copyright and SPDX lines the file's licensing hangs on, then
// the Doxygen-style descriptor block this repository already uses.
func goHeader(fm *fileMeta) string {
	var b strings.Builder

	b.WriteString("/*\n")
	fmt.Fprintf(&b, " * Copyright (c) %s %s\n", fm.Year, fm.Holder)
	fmt.Fprintf(&b, " * SPDX-License-Identifier: %s\n", fm.SPDX)
	b.WriteString(" */\n\n")
	b.WriteString("/**\n")
	fmt.Fprintf(&b, " * @file %s\n", fm.File)
	fmt.Fprintf(&b, " * @package %s\n", fm.Package)
	fmt.Fprintf(&b, " * @author %s\n", fm.Author)
	fmt.Fprintf(&b, " * @since %s\n", fm.Since)
	b.WriteString(" */\n\n")

	return b.String()
}

// scriptHeader renders the two-line head for Makefile and Dockerfile.
// Neither has a package clause, and by decision neither carries a vim
// modeline.
func scriptHeader(fm *fileMeta) string {
	return fmt.Sprintf("# Copyright (c) %s %s\n# SPDX-License-Identifier: %s\n\n", fm.Year, fm.Holder, fm.SPDX)
}

// vimFooter is the editor modeline sicky's own sources end with, so a
// generated file opens with the same indentation rules as the framework it
// imports.
func vimFooter() string {
	return "\n" +
		"/*\n" +
		" * Local variables:\n" +
		" * tab-width: 4\n" +
		" * c-basic-offset: 4\n" +
		" * End:\n" +
		" * vim600: sw=4 ts=4 fdm=marker\n" +
		" * vim<600: sw=4 ts=4\n" +
		" */\n"
}

// isGoFile reports whether path is a Go source file.
func isGoFile(path string) bool {
	return strings.HasSuffix(path, ".go")
}

// isScriptFile reports whether path is one of the build scripts that carry
// a comment header under this policy.
func isScriptFile(path string) bool {
	base := filepath.Base(path)

	return base == "Makefile" || base == "Dockerfile"
}

// injectFileHeader prepends the header belonging to the file's kind.
//
// Files outside the policy (config.json, go.mod, .gitignore, README) come
// back untouched: a comment inside config.json makes encoding/json reject
// the whole document, which is why JSON is deliberately out of scope rather
// than an oversight.
func injectFileHeader(path, content string, fm *fileMeta) string {
	// Idempotent: a file that already carries a header keeps it, so the
	// function is safe to call from more than one code path.
	if strings.Contains(content, "SPDX-License-Identifier:") {
		return content
	}

	switch {
	case isGoFile(path):
		return goHeader(fm) + content
	case isScriptFile(path):
		return scriptHeader(fm) + content
	default:
		return content
	}
}

// injectFileFooter appends the vim modeline. Only .go files get one.
func injectFileFooter(path, content string) string {
	if !isGoFile(path) || strings.Contains(content, "vim600:") {
		return content
	}

	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	return content + vimFooter()
}

var packagePattern = regexp.MustCompile(`(?m)^package[ \t]+([A-Za-z_][A-Za-z0-9_]*)`)

// packageOf reads the package clause out of a rendered .go source so the
// @package line describes the file instead of guessing. A source without a
// readable clause yields the placeholder rather than a wrong value.
func packageOf(content string) string {
	if m := packagePattern.FindStringSubmatch(content); m != nil {
		return m[1]
	}

	return placeholderPackage
}

var spdxPattern = regexp.MustCompile(`SPDX-License-Identifier:[ \t]*([A-Za-z0-9.+_-]+)`)

// gitConfigValue reads one git config key. It is best-effort: no git, no
// repository and no value all mean "unknown", never an error.
func gitConfigValue(key string) string {
	out, err := exec.Command("git", "config", "--get", key).Output() //nolint:gosec // G204: the binary is fixed (git) and key is one of the literal names callers pass
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(out))
}

// detectAuthor derives the @author line from git, as "Name <email>", or
// "" when git cannot answer.
func detectAuthor() string {
	name := gitConfigValue("user.name")
	if name == "" {
		return ""
	}

	if email := gitConfigValue("user.email"); email != "" {
		return name + " <" + email + ">"
	}

	return name
}

// detectHolder derives the suggested copyright holder from git. It is only
// a suggestion: the interactive prompt confirms it, and --copyright
// overrides it, because git's user.name is a person and a holder is often
// a company.
func detectHolder() string {
	return gitConfigValue("user.name")
}

// hasAll reports whether body contains every marker.
func hasAll(body string, markers []string) bool {
	for _, m := range markers {
		if !strings.Contains(body, m) {
			return false
		}
	}

	return true
}

// identifyLicense matches a license body against the texts sicky ships, so
// a file generated inside an existing project joins that project's license
// instead of contradicting it.
func identifyLicense(body string) (licenseChoice, bool) {
	for _, sig := range licenseSignatures {
		if hasAll(body, sig.markers) {
			return licenseForKey(sig.key)
		}
	}

	return licenseChoice{}, false
}

// detectSPDXInSources scans the working tree for an SPDX line the project
// already stamps on its sources. The walk stops at the first match and at a
// hard file cap, so `sicky generate` never turns into a full-tree read.
func detectSPDXInSources() string {
	const maxFiles = 200

	skipDirs := map[string]bool{
		".git": true, "vendor": true, "node_modules": true,
		"dist": true, "bin": true, "obj": true,
	}

	seen := 0
	found := ""

	_ = filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || found != "" {
			return nil //nolint:nilerr // an unreadable entry is skipped, not fatal
		}

		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}

			return nil
		}

		// A symlink is not one of the project's own sources, and following
		// it would read whatever it points at.
		if d.Type()&fs.ModeSymlink != 0 || !strings.HasSuffix(path, ".go") {
			return nil
		}

		seen++
		if seen > maxFiles {
			return filepath.SkipAll
		}

		// Unreadable entries are skipped: this is a best-effort scan for a
		// marker, and one file we cannot open must not abort it.
		data, readErr := os.ReadFile(path) //nolint:gosec // G122: symlinks are skipped above; only regular .go files in the working tree are read
		if readErr != nil {
			return nil //nolint:nilerr // the scan continues by design
		}

		if m := spdxPattern.FindSubmatch(data); m != nil {
			found = string(m[1])
		}

		return nil
	})

	return found
}

// detectProjectLicense decides which license a file generated in the
// current directory should carry: the project's own license file first,
// then a line an existing source file already uses, then MIT with a
// warning. Every failure is loud because a wrong SPDX line is a claim about
// licensing the file's owner never made.
func detectProjectLicense() licenseChoice {
	for _, name := range licenseFileNames {
		body, err := os.ReadFile(name)
		if err != nil {
			continue
		}

		if c, ok := identifyLicense(string(body)); ok {
			return c
		}
	}

	if c, ok := licenseForKey(detectSPDXInSources()); ok {
		return c
	}

	fmt.Fprintln(os.Stderr, "sicky generate: warn: could not confirm the project license, using MIT")

	return defaultLicense()
}

// relativeFile renders a scaffold path the way the @file line should read:
// slash-separated and relative to the project root, never an absolute path
// that would leak the builder's directory layout into a committed file.
func relativeFile(dir, path string) string {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return filepath.Base(path)
	}

	return filepath.ToSlash(rel)
}

// validateHeaderText rejects a value that would break out of the comment
// block it is rendered into: a newline splits the block and "*/" closes it,
// either of which turns a name into source code. This is why an invalid
// author or holder is an error rather than a warn-and-default like an
// unknown license — an unknown license names a file we control, a bad name
// writes into code we do not.
func validateHeaderText(name, value string) error {
	if value == "" {
		return nil
	}

	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%s must not contain a newline", name)
	}

	if strings.Contains(value, "*/") {
		return fmt.Errorf("%s must not contain %q", name, "*/")
	}

	if len(value) > 200 {
		return fmt.Errorf("%s is too long (200 characters max)", name)
	}

	return nil
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
