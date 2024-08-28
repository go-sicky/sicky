/*
 * The MIT License (MIT)
 *
 * Copyright (c) 2021 HereweTech Co.LTD
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
 * @file metadata.go
 * @package utils
 * @author Dr.NP <np@herewe.tech>
 * @since 08/17/2024
 */

package utils

import "strings"

type Metadata map[string]string

func NewMetadata() Metadata {
	return make(Metadata)
}

func (md Metadata) Get(key string) (string, bool) {
	val, ok := md[key]

	return val, ok
}

func (md Metadata) Set(key, val string) {
	md[key] = val
}

func (md Metadata) Delete(key string) {
	delete(md, key)
}

func (md Metadata) Copy() Metadata {
	o := make(Metadata, len(md))
	for k, v := range md {
		o[k] = v
	}

	return o
}

func (md Metadata) Strings() []string {
	ret := make([]string, 0)
	for k, v := range md {
		ret = append(ret, k+"="+v)
	}

	return ret
}

func MetadataFromStrings(ss []string) Metadata {
	ret := make(Metadata)
	for _, line := range ss {
		parts := strings.Split(line, "=")
		if len(parts) == 2 {
			ret.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}

	return ret
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
