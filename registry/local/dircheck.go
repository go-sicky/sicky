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
 * @file dircheck.go
 * @package local
 * @author Dr.NP <np@herewe.tech>
 * @since 09/27/2026
 */

package local

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

var (
	// ErrLocalPathSymlink is returned when a registry path is a symlink.
	// Writing through one hands the file to whoever owns the target.
	ErrLocalPathSymlink = errors.New("local registry: path is a symlink")
	// ErrLocalPathNotDir is returned when a registry path is a file.
	ErrLocalPathNotDir = errors.New("local registry: path is not a directory")
	// ErrLocalPathNotOwner is returned when the registry directory belongs
	// to another user.
	ErrLocalPathNotOwner = errors.New("local registry: path is not owned by the current user")
	// ErrLocalPathWorldWritable is returned when other users can write the
	// registry directory, which lets them replace <uuid>.json and redirect
	// service discovery.
	ErrLocalPathWorldWritable = errors.New("local registry: path is writable by other users")
	// ErrLocalPathUntrustedParent is returned when a directory we would
	// create the registry under belongs to someone else and is
	// world-writable without the sticky bit.
	ErrLocalPathUntrustedParent = errors.New("local registry: parent directory is untrusted")
	// ErrLocalInstanceTooLarge is returned when a registry file exceeds
	// maxInstanceBytes; a giant file would be read into memory in full.
	ErrLocalInstanceTooLarge = errors.New("local registry: instance file too large")
)

// maxInstanceBytes caps a single instance file. A real registration is a
// few kilobytes; anything near this size is a swap, not a service.
const maxInstanceBytes = 1 << 20 // 1 MiB

// checkRegistryDir validates the directory instances are stored in.
//
// The default location lives under a shared temporary root, where any
// local user could pre-create the directory (or a parent), own it, and
// serve poisoned <uuid>.json files to the watcher - hijacking service
// discovery. Rules, starting at the target directory and walking up:
//
//   - the target must exist as a real directory (no symlink), be owned by
//     us and not be writable by group or others;
//   - an ancestor owned by someone else must not be world-writable
//     without the sticky bit, otherwise it can replace what we create
//     inside it (a sticky /tmp passes, an attacker-created /tmp/sicky
//     does not);
//   - missing components are fine: MkdirAll creates them with 0700.
func checkRegistryDir(path string) error {
	target := filepath.Clean(path)
	dir := target

	for {
		info, err := os.Lstat(dir)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("local registry: stat %s: %w", dir, err)
			}

			parent := filepath.Dir(dir)
			if parent == dir {
				// Reached the filesystem root: every missing component
				// below it will be created with 0700.
				return nil
			}

			dir = parent

			continue
		}

		if info.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%w: %s", ErrLocalPathSymlink, dir)
		}

		if !info.IsDir() {
			return fmt.Errorf("%w: %s", ErrLocalPathNotDir, dir)
		}

		owner := sameOwner(info)
		if dir == target {
			if !owner {
				return fmt.Errorf("%w: %s", ErrLocalPathNotOwner, dir)
			}

			if writableByOthers(info.Mode()) {
				return fmt.Errorf("%w: %s", ErrLocalPathWorldWritable, dir)
			}

			return nil
		}

		if !owner && worldWritable(info.Mode()) && info.Mode()&fs.ModeSticky == 0 {
			return fmt.Errorf("%w: %s", ErrLocalPathUntrustedParent, dir)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return nil
		}

		dir = parent
	}
}
