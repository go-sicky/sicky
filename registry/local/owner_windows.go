//go:build windows

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
 * @file owner_windows.go
 * @package local
 * @author Dr.NP <np@herewe.tech>
 * @since 09/27/2026
 */

package local

import "io/fs"

// sameOwner: NTFS ACLs are not modeled by the permission bits Go
// reports, so ownership is left to the OS access check. The symlink and
// "not a directory" rules still apply.
func sameOwner(fs.FileInfo) bool { return true }

// writableByOthers: the unix group/other write bits have no ACL
// equivalent on Windows; writing is governed by the ACL, which the OS
// enforces.
func writableByOthers(fs.FileMode) bool { return false }

// worldWritable: see writableByOthers.
func worldWritable(fs.FileMode) bool { return false }
