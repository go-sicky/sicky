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
 * @file debug.go
 * @package utils
 * @author Dr.NP <np@herewe.tech>
 * @since 11/29/2023
 */

package utils

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"reflect"
	"strings"
	"unsafe"
)

// PrintContextInternals is a debugging helper.
func PrintContextInternals(ctx any, inner bool) {
	contextValues := reflect.ValueOf(ctx).Elem()
	contextKeys := reflect.TypeOf(ctx).Elem()
	if !inner {
		fmt.Printf("\nFields for %s.%s\n", contextKeys.PkgPath(), contextKeys.Name())
	}

	if contextKeys.Kind() == reflect.Struct {
		for i := range contextValues.NumField() {
			reflectValue := contextValues.Field(i)
			//nolint:gosec // G103: deliberate unsafe introspection in a debug-only printer; never on a hot path
			reflectValue = reflect.NewAt(reflectValue.Type(), unsafe.Pointer(reflectValue.UnsafeAddr())).Elem()

			reflectField := contextKeys.Field(i)

			if reflectField.Name == "Context" {
				PrintContextInternals(reflectValue.Interface(), true)
			} else {
				fmt.Printf("field name: %+v\n", reflectField.Name)
				fmt.Printf("value: %+v\n", reflectValue.Interface())
			}
		}
	} else {
		fmt.Printf("context is empty (int)\n")
	}
}

// JSONAny is a debugging helper.
func JSONAny(d any) {
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return
	}

	fmt.Println(string(b))
}

// JSONAnyString is a debugging helper.
func JSONAnyString(d any) string {
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return ""
	}

	return string(b)
}

// JSONAnyBytes is a debugging helper.
func JSONAnyBytes(d any) []byte {
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return nil
	}

	return b
}

// XMLAny is a debugging helper.
func XMLAny(d any) {
	b, err := xml.MarshalIndent(d, "", "  ")
	if err != nil {
		return
	}

	fmt.Println(string(b))
}

// XMLAnyString is a debugging helper.
func XMLAnyString(d any) string {
	b, err := xml.MarshalIndent(d, "", "  ")
	if err != nil {
		return ""
	}

	return string(b)
}

// XMLAnyBytes is a debugging helper.
func XMLAnyBytes(d any) []byte {
	b, err := xml.MarshalIndent(d, "", "  ")
	if err != nil {
		return nil
	}

	return b
}

// E is a debugging helper.
func E(level, format string, args ...any) {
	fmt.Println()

	switch strings.ToLower(level) {
	case "debug":
		// Color blue
		fmt.Printf("\033[34m[%s] %s\033[0m\n", level, fmt.Sprintf(format, args...))
	case "info":
		// Color green
		fmt.Printf("\033[32m[%s] %s\033[0m\n", level, fmt.Sprintf(format, args...))
	case "warn":
		// Color yellow
		fmt.Printf("\033[33m[%s] %s\033[0m\n", level, fmt.Sprintf(format, args...))
	case "error":
		// Color red
		fmt.Printf("\033[31m[%s] %s\033[0m\n", level, fmt.Sprintf(format, args...))
	default:
		if len(args) > 0 {
			fmt.Printf("[%s] %s\n", level, fmt.Sprintf(format, args...))
		} else {
			fmt.Printf("[%s] %s\n", level, format)
		}
	}

	fmt.Println()
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
