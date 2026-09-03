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
 * @file misc.go
 * @package utils
 * @author Dr.NP <np@herewe.tech>
 * @since 11/29/2023
 */

package utils

import (
	"bytes"
	"crypto/md5"
	crand "crypto/rand"
	"crypto/sha256"
	"fmt"
	"runtime"
	"strconv"

	// For submodule upgrade
	_ "google.golang.org/genproto/protobuf/api"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func RandomString(length int) string {
	if length <= 0 {
		return ""
	}
	// Rejection-sample uniformly without per-byte big.Int allocs.
	max := 256 - (256 % len(letterBytes))
	b := make([]byte, 0, length)
	raw := make([]byte, length*2)
	for len(b) < length {
		if _, err := crand.Read(raw); err != nil {
			// crypto/rand must never silently degrade; fail fast.
			panic(fmt.Sprintf("crypto/rand unavailable: %s", err.Error()))
		}
		for _, v := range raw {
			if int(v) >= max {
				continue
			}
			b = append(b, letterBytes[int(v)%len(letterBytes)])
			if len(b) == length {
				break
			}
		}
	}

	return string(b)
}

func RandomHex(length int) []byte {
	if length <= 0 {
		return nil
	}
	b := make([]byte, length)
	if _, err := crand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand unavailable: %s", err.Error()))
	}

	return b
}

func MD5String(input string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(input)))
}

func GoroutineID() uint64 {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	n, _ := strconv.ParseUint(string(b), 10, 64)

	return n
}

func CryptoPassword(original, salt string) string {
	h := sha256.New()
	h.Write([]byte(original + "@@" + salt))

	return fmt.Sprintf("%x", h.Sum(nil))
}

func EnsureStatus(status, bit int64) bool {
	if bit < 0 || bit > 63 {
		return false
	}

	return (status & (1 << bit)) != 0
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
