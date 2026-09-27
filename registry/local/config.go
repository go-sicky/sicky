/*
 * The MIT License (MIT)
 *
 * Copyright (c) 2024 HereweTech Co.LTD
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy of
 * this software and associated documentation files (the "Software"), to deal in
 * the Software without restriction, including without limitation the rights to
 * use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
 * the Software, and to permit persons to whom the Software is furnished to do so,
 * subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
 * FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
 * COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
 * IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
 * CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 */

/**
 * @file config.go
 * @package local
 * @author Dr.NP <np@herewe.tech>
 * @since 08/19/2024
 */

package local

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const (
	// DefaultRegistryFilePath is the fallback registry location, used
	// when XDG_RUNTIME_DIR is unset. Prefer defaultRegistryDir() in new
	// code: a shared temporary root is writable by every local user.
	DefaultRegistryFilePath = "/tmp/sicky/registry"
	// registrySubdir is appended to XDG_RUNTIME_DIR.
	registrySubdir = "sicky/registry"
)

// defaultRegistryDir prefers the per-user runtime directory: the shared
// temporary fallback is world-writable, so the directory (and every
// registration inside it) can be pre-created or swapped by another local
// user. Validate() rejects such a directory when one is found.
func defaultRegistryDir() string {
	if dir := strings.TrimSpace(os.Getenv("XDG_RUNTIME_DIR")); dir != "" && filepath.IsAbs(dir) {
		return filepath.Join(filepath.Clean(dir), registrySubdir)
	}

	return DefaultRegistryFilePath
}

// ErrLocalPathNotAbsolute is returned when RegistryFilePath is not absolute
// after cleaning. Relative paths resolve against the process working
// directory, which is a config-hijack vector.
var ErrLocalPathNotAbsolute = errors.New("local registry: registry_file_path must be absolute")

// Config is a local component.
type Config struct {
	RegistryFilePath string `json:"registry_file_path" mapstructure:"registry_file_path" yaml:"registry_file_path"`
	CleanupOnStart   bool   `json:"cleanup_on_start"   mapstructure:"cleanup_on_start"   yaml:"cleanup_on_start"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		RegistryFilePath: defaultRegistryDir(),
		CleanupOnStart:   false,
	}
}

// Ensure fills zero-valued fields with defaults and returns the receiver (nil-safe).
func (c *Config) Ensure() *Config {
	if c == nil {
		c = DefaultConfig()
	}

	if strings.TrimSpace(c.RegistryFilePath) == "" {
		c.RegistryFilePath = defaultRegistryDir()
	} else {
		c.RegistryFilePath = filepath.Clean(c.RegistryFilePath)
	}

	return c
}

// Validate rejects relative paths so the registry directory can never be
// resolved against an untrusted working directory, and rejects a
// directory another user controls: a poisoned registry directory lets
// any local process redirect service discovery.
func (c *Config) Validate() error {
	if c == nil {
		return nil
	}

	if !filepath.IsAbs(filepath.Clean(c.RegistryFilePath)) {
		return ErrLocalPathNotAbsolute
	}

	return checkRegistryDir(c.RegistryFilePath)
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
