/**
 * @file configschema.go
 * @package sicky
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package sicky

import (
	"reflect"
	"slices"
	"strings"
)

// mapstructureSquash is the mapstructure option that folds a struct's fields
// into the parent's key path. Spelled once because three call sites here and
// in configprefix.go must agree on it.
const mapstructureSquash = "squash"

// configSchema is the set of viper key paths a target's sicky.Config fields
// can hold, keyed by the prefix each Config was found at.
type configSchema struct {
	// prefixes are the paths at which a Config was found, "" included.
	prefixes []string
	// nodes is every path those Configs occupy: leaves and the intermediate
	// segments on the way to them.
	nodes map[string]struct{}
}

// buildConfigSchema walks the caller's type and records where its Config
// fields live and which keys each of them can hold.
//
// It is what makes a mistyped configuration key visible. viper silently drops
// a key it cannot map, so `{"sicky": {"infra": {"bu": {"dsn": ...}}}}` starts
// a service with no database and says nothing: the block simply is not there.
// An operator who misspells `address` as `adresss` gets the same silence.
func buildConfigSchema(raw any) configSchema {
	schema := configSchema{nodes: map[string]struct{}{}}

	var walk func(t reflect.Type, prefix string, depth int)

	walk = func(t reflect.Type, prefix string, depth int) {
		if depth > 12 {
			return
		}

		for t != nil && t.Kind() == reflect.Pointer {
			t = t.Elem()
		}

		if t == nil {
			return
		}

		if t == reflect.TypeFor[Config]() {
			schema.prefixes = append(schema.prefixes, prefix)
			schema.recordPath(prefix)
			schema.record(prefix, t, depth)

			return
		}

		if t.Kind() != reflect.Struct {
			return
		}

		for field := range t.Fields() {
			if !field.IsExported() {
				continue
			}

			name, options, _ := strings.Cut(field.Tag.Get("mapstructure"), ",")

			if name == "-" {
				continue
			}

			if name == "" {
				name = field.Name
			}

			if options == mapstructureSquash {
				walk(field.Type, prefix, depth+1)

				continue
			}

			walk(field.Type, joinKey(prefix, name), depth+1)
		}
	}

	walk(reflect.TypeOf(raw), "", 0)

	if !slices.Contains(schema.prefixes, "") {
		schema.prefixes = append(schema.prefixes, "")
	}

	return schema
}

// recordPath adds every segment of the path that leads to a Config, so
// "sicky" and "app.sicky" are known nodes even though no field declares them.
// Without this the first unknown segment of any nested key would be the
// prefix itself, which tells the operator nothing.
func (s *configSchema) recordPath(prefix string) {
	segments := strings.Split(prefix, ".")
	for i := range segments {
		s.nodes[strings.Join(segments[:i+1], ".")] = struct{}{}
	}
}

// record adds every path a struct can occupy under prefix. Both leaves and
// intermediates are recorded because a file legitimately contains
// `sicky.infra` as well as `sicky.infra.bun.dsn`.
func (s *configSchema) record(prefix string, t reflect.Type, depth int) {
	if t.Kind() != reflect.Struct {
		return
	}

	for field := range t.Fields() {
		if !field.IsExported() {
			continue
		}

		name, options, _ := strings.Cut(field.Tag.Get("mapstructure"), ",")

		if name == "-" {
			continue
		}

		if name == "" {
			name = field.Name
		}

		if options == mapstructureSquash {
			s.record(prefix, field.Type, depth+1)

			continue
		}

		path := joinKey(prefix, name)
		s.nodes[path] = struct{}{}

		if ft := indirect(field.Type); ft != nil && ft.Kind() == reflect.Struct {
			s.record(path, ft, depth+1)
		}
	}
}

func indirect(t reflect.Type) reflect.Type {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	return t
}

// owns reports whether a key belongs to the framework's own configuration
// rather than to the application's. Only those can be checked: the framework
// has no idea what fields the application put under `server` or `service`, so
// warning about them would be guessing.
func (s configSchema) owns(key string) bool {
	for _, prefix := range s.prefixes {
		if prefix == "" {
			continue
		}

		if key == prefix || strings.HasPrefix(key, prefix+".") {
			return true
		}
	}

	return false
}

// unknownKeys returns the keys viper knows that fall under a framework prefix
// but match no field. Sorted and deduplicated, so the warning is stable across
// runs and a log diff is meaningful.
//
// Each key is reported at the first segment that does not resolve rather than
// at the leaf. viper only ever knows leaves, so `{"sicky":{"infra":{"bu":{}}}}`
// arrives here as `sicky.infra.bu.dsn`; reporting that names a key the
// operator never wrote, while `sicky.infra.bu` names the block they did.
func (s configSchema) unknownKeys(keys []string) []string {
	var unknown []string

	for _, key := range keys {
		if !s.owns(key) {
			continue
		}

		if _, ok := s.nodes[key]; ok {
			continue
		}

		if bad := s.firstUnknownSegment(key); bad != "" {
			unknown = append(unknown, bad)
		}
	}

	slices.Sort(unknown)

	return slices.Compact(unknown)
}

// firstUnknownSegment returns the shortest prefix of key that is not a node,
// which is the first thing in it that names nothing.
func (s configSchema) firstUnknownSegment(key string) string {
	segments := strings.Split(key, ".")

	for i := range segments {
		candidate := strings.Join(segments[:i+1], ".")
		if _, ok := s.nodes[candidate]; ok {
			continue
		}

		return candidate
	}

	return ""
}
