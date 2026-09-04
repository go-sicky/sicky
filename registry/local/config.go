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
	"path/filepath"
	"strings"
)

const (
	DefaultRegistryFilePath = "/tmp/sicky/registry"
)

// ErrLocalPathNotAbsolute is returned when RegistryFilePath is not absolute
// after cleaning. Relative paths resolve against the process working
// directory, which is a config-hijack vector.
var ErrLocalPathNotAbsolute = errors.New("local registry: registry_file_path must be absolute")

type Config struct {
	RegistryFilePath string `json:"registry_file_path" yaml:"registry_file_path" mapstructure:"registry_file_path"`
	CleanupOnStart   bool   `json:"cleanup_on_start" yaml:"cleanup_on_start" mapstructure:"cleanup_on_start"`
}

func DefaultConfig() *Config {
	return &Config{
		RegistryFilePath: DefaultRegistryFilePath,
		CleanupOnStart:   false,
	}
}

func (c *Config) Ensure() *Config {
	if c == nil {
		c = DefaultConfig()
	}

	if strings.TrimSpace(c.RegistryFilePath) == "" {
		c.RegistryFilePath = DefaultRegistryFilePath
	} else {
		c.RegistryFilePath = filepath.Clean(c.RegistryFilePath)
	}

	return c
}

// Validate rejects relative paths so the registry directory can never be
// resolved against an untrusted working directory.
func (c *Config) Validate() error {
	if c == nil {
		return nil
	}
	if !filepath.IsAbs(filepath.Clean(c.RegistryFilePath)) {
		return ErrLocalPathNotAbsolute
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
