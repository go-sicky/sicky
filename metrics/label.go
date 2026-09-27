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
 * @file label.go
 * @package metrics
 * @author Dr.NP <np@herewe.tech>
 * @since 09/27/2026
 */

package metrics

import "strings"

// UnknownMethod is the label value for every method outside the standard
// set. It bounds the series count to len(knownHTTPMethods)+1 no matter
// what a client puts on the request line.
const UnknownMethod = "OTHER"

// UnmatchedRoute is the label value for requests that matched no route.
const UnmatchedRoute = "unmatched"

// knownHTTPMethods is the closed set of methods that keep their own
// label. Anything else (net/http accepts any token, including
// `GET1 x`) collapses into UnknownMethod.
var knownHTTPMethods = map[string]struct{}{
	"CONNECT": {},
	"DELETE":  {},
	"GET":     {},
	"HEAD":    {},
	"OPTIONS": {},
	"PATCH":   {},
	"POST":    {},
	"PUT":     {},
	"TRACE":   {},
}

// NormalizeHTTPMethod bounds an HTTP method before it reaches a label or
// a span name. The method is attacker-controlled: without this a client
// can mint an unbounded number of time series and tracing entries with
// nothing but a loop of requests.
//
// Only call this for HTTP server labels - gRPC and the raw transports
// pass their own fixed values (full method path, "datagram", ...) which
// are already bounded.
func NormalizeHTTPMethod(method string) string {
	if method == "" {
		return UnknownMethod
	}

	m := strings.ToUpper(method)
	if _, ok := knownHTTPMethods[m]; ok {
		return m
	}

	return UnknownMethod
}

// NormalizeRouteLabel bounds a route label. Route templates registered by
// the application are bounded by construction and stay verbatim; an
// empty route (unmatched request) collapses to UnmatchedRoute instead of
// carrying the raw request path, which is unbounded client input.
func NormalizeRouteLabel(route string) string {
	if route == "" {
		return UnmatchedRoute
	}

	return route
}
