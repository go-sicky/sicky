/*
 * Copyright (c) 2026, The go-sicky Authors
 * SPDX-License-Identifier: MIT
 *
 * @file    generate_k8s_test.go
 * @package cli
 * @author  Dr.NP <np@herewe.tech>
 * @since   09/29/2026
 */

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGenerateK8sRejectsInjectedName covers the manifest-injection hole:
// generateK8sFiles interpolates the project name into raw YAML at five
// points (metadata.name, both matchLabels, template labels and the
// container image) and writeGuard only checks the output path, never the
// content. Every other generator calls requireName before templating;
// this one had to as well, or a newline in the name injects arbitrary
// deployment keys that a later `kubectl apply -f` executes.
func TestGenerateK8sRejectsInjectedName(t *testing.T) {
	const injected = "x\n  hostNetwork: true\n  containers:\n  - name: p\n    image: attacker/root:latest\n"

	dir := t.TempDir()

	code := generateK8sFiles([]string{injected, "--output", dir})
	if code == 0 {
		t.Fatalf("generate k8s %q: want a non-zero exit code, got 0", injected)
	}

	// A rejected name must not leave a manifest behind at all.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read output dir: %v", err)
	}

	for _, e := range entries {
		data, readErr := os.ReadFile(filepath.Join(dir, e.Name()))
		if readErr != nil {
			t.Fatalf("read %s: %v", e.Name(), readErr)
		}

		if strings.Contains(string(data), "attacker/root") || strings.Contains(string(data), "hostNetwork") {
			t.Fatalf("%s contains injected manifest content:\n%s", e.Name(), data)
		}
	}
}

// TestGenerateK8sAcceptsValidName is the negative half: the guard must
// not reject a legitimate name, or the fix would be a footgun.
func TestGenerateK8sAcceptsValidName(t *testing.T) {
	dir := t.TempDir()

	if code := generateK8sFiles([]string{"my-app.v2", "--output", dir}); code != 0 {
		t.Fatalf("generate k8s my-app.v2: exit %d, want 0", code)
	}

	deploy := filepath.Join(dir, "deployment.yaml")

	data, err := os.ReadFile(deploy)
	if err != nil {
		t.Fatalf("read %s: %v", deploy, err)
	}

	for _, want := range []string{"name: my-app.v2", "image: my-app.v2:latest"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("deployment.yaml missing %q:\n%s", want, data)
		}
	}
}
