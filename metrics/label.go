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

// UnknownCommand is the label value for a Redis command outside the
// known set. It bounds the series count for sicky_infra_ops_total{infra=
// "redis"} the same way UnknownMethod bounds the HTTP server series.
const UnknownCommand = "OTHER"

// OtherHost is the label value for every non-empty outbound destination.
// The outbound host is caller-supplied, so any per-destination value would
// let a caller mint an unbounded number of permanent series; see
// NormalizeClientHost.
const OtherHost = "other"

// UnknownHost is the label value for an outbound request with no host at
// all. Kept distinct from OtherHost so a misconfigured client does not share
// a series with every other one.
const UnknownHost = "unknown"

// knownRedisCommands is the closed set of commands that keep their own
// label. Every typed go-redis method issues one of these with a
// compile-time constant name, so they are already bounded; Do and
// NewCmd take caller-supplied args whose first element becomes the
// command name, and that is the unbounded path this set closes.
var knownRedisCommands = map[string]struct{}{
	"ACL": {}, "APPEND": {}, "AUTH": {}, "BGSAVE": {}, "BITCOUNT": {},
	"BITFIELD": {}, "BLPOP": {}, "BRPOP": {}, "CLIENT": {}, "CLUSTER": {},
	"COMMAND": {}, "CONFIG": {}, "DBSIZE": {}, "DECR": {}, "DEL": {},
	"DISCARD": {}, "DUMP": {}, "ECHO": {}, "EVAL": {}, "EVALSHA": {},
	"EXISTS": {}, "EXPIRE": {}, "FLUSHALL": {}, "FLUSHDB": {}, "GEOADD": {},
	"GEODIST": {}, "GEOPOS": {}, "GET": {}, "GETRANGE": {}, "HDEL": {},
	"HGET": {}, "HGETALL": {}, "HINCRBY": {}, "HKEYS": {}, "HLEN": {},
	"HMGET": {}, "HMSET": {}, "HRANDFIELD": {}, "HSCAN": {}, "HSET": {},
	"HSETNX": {}, "HSTRLEN": {}, "HVALS": {}, "INCR": {}, "INCRBY": {},
	"INFO": {}, "KEYS": {}, "LASTSAVE": {}, "LPOP": {}, "LPUSH": {},
	"LPUSHX": {}, "LRANGE": {}, "LLEN": {}, "LREM": {}, "LSET": {},
	"LTRIM": {}, "MGET": {}, "MOVE": {}, "MSET": {}, "MSETNX": {},
	"MULTI": {}, "PERSIST": {}, "PEXPIRE": {}, "PING": {}, "PSETEX": {},
	"PTTL": {}, "PUBLISH": {}, "PUBSUB": {}, "RANDOMKEY": {}, "RENAME": {},
	"RENAMENX": {}, "RPOP": {}, "RPOPLPUSH": {}, "RPUSH": {}, "RPUSHX": {},
	"SADD": {}, "SAVE": {}, "SCAN": {}, "SCARD": {}, "SDIFF": {},
	"SDIFFSTORE": {}, "SELECT": {}, "SET": {}, "SETEX": {}, "SETNX": {},
	"SHUTDOWN": {}, "SINTER": {}, "SINTERSTORE": {}, "SISMEMBER": {},
	"SLAVEOF": {}, "SMEMBERS": {}, "SMISMEMBER": {}, "SMOVE": {},
	"SORT": {}, "SPOP": {}, "SRANDMEMBER": {}, "SREM": {}, "SSCAN": {},
	"STRLEN": {}, "SUBSCRIBE": {}, "SUNION": {}, "SUNIONSTORE": {},
	"SWAPDB": {}, "TIME": {}, "TTL": {}, "TYPE": {}, "UNLINK": {},
	"UNSUBSCRIBE": {}, "UNWATCH": {}, "WATCH": {}, "ZADD": {}, "ZCARD": {},
	"ZCOUNT": {}, "ZINCRBY": {}, "ZINTER": {}, "ZINTERSTORE": {},
	"ZLEXCOUNT": {}, "ZPOPMAX": {}, "ZPOPMIN": {}, "ZRANDMEMBER": {},
	"ZRANGE": {}, "ZRANGEBYLEX": {}, "ZRANGEBYSCORE": {}, "ZRANK": {},
	"ZREM": {}, "ZREMRANGEBYLEX": {}, "ZREMRANGEBYRANK": {},
	"ZREMRANGEBYSCORE": {}, "ZREVRANGE": {}, "ZREVRANGEBYSCORE": {},
	"ZREVRANK": {}, "ZSCAN": {}, "ZSCORE": {},
}

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

// The host label on outbound client metrics is the one place where the
// destination had no normalizer while every sibling label did
// (NormalizeHTTPMethod, NormalizeRouteLabel, NormalizeRedisCommand). Callers
// of client/http.Do in a gateway or webhook relay supply it, so each distinct
// host minted a permanent series — a memory allocator paid for by the caller.
// client/http now routes it through NormalizeClientHost.

// NormalizeClientHost bounds the host label on outbound client metrics.
//
// The host is chosen by the caller of client/http.Do, which in a gateway, a
// webhook relay or any fetch-a-URL feature is attacker-controlled. Without
// this, one request per distinct host mints a permanent Prometheus series:
// pairCache memoizes the child, the CounterVec/HistogramVec children are never
// released, and the process grows a memory allocator and a scrape body that
// grows with the attacker's request count.
//
// It collapses to OtherHost rather than a registrable suffix on purpose. A
// suffix still lets an attacker buy a series per registered domain, and a
// host name is the wrong aggregation dimension anyway: it splits the series
// across instances whose DNS resolves differently. The per-destination signal
// belongs in the span name (client/http starts one per "<method> <host>"),
// which is already bounded by the exporter rather than by this process.
//
// An empty host is a caller bug rather than an attack, so it is reported as
// UnknownHost instead of being folded into the same bucket as a real
// destination — otherwise a broken client silently shares a series with every
// other broken client.
func NormalizeClientHost(host string) string {
	if host == "" {
		return UnknownHost
	}

	return OtherHost
}

// NormalizeRedisCommand bounds a Redis command name before it reaches a
// metric label. go-redis derives cmd.Name() from the FIRST ARGUMENT, so
// the typed methods are safe (their command is a compile-time constant)
// but Do and NewCmd take caller-supplied args: a sharded key or a
// pseudo-command routed through them mints a permanent new series per
// distinct value, which grows the Prometheus registry without bound and
// eventually OOMs the process or times out every scrape.
//
// Only call this for Redis command labels.
func NormalizeRedisCommand(cmd string) string {
	if cmd == "" {
		return UnknownCommand
	}

	c := strings.ToUpper(cmd)
	if _, ok := knownRedisCommands[c]; ok {
		return c
	}

	return UnknownCommand
}
