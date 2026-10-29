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
	// mapValues records map-typed fields whose values are structs (or nil
	// for an open map). The next path segment after such a node is a
	// runtime key, not a declared field.
	mapValues map[string]reflect.Type
}

// buildConfigSchema walks the caller's type and records where its Config
// fields live and which keys each of them can hold.
//
// It is what makes a mistyped configuration key visible. viper silently drops
// a key it cannot map, so `{"sicky": {"infra": {"bu": {"dsn": ...}}}}` starts
// a service with no database and says nothing: the block simply is not there.
// An operator who misspells `address` as `adresss` gets the same silence.
func buildConfigSchema(raw any) configSchema {
	schema := configSchema{
		nodes:     map[string]struct{}{},
		mapValues: map[string]reflect.Type{},
	}

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

		ft := indirect(field.Type)
		if ft == nil {
			continue
		}

		if ft.Kind() == reflect.Struct {
			s.record(path, ft, depth+1)

			continue
		}

		if ft.Kind() == reflect.Map {
			vt := indirect(ft.Elem())
			if vt != nil && vt.Kind() == reflect.Struct {
				s.mapValues[path] = vt
			} else {
				// A map of scalars/interface values has no declared
				// children; treat every descendant as known rather than
				// inventing warnings for dynamic keys.
				s.mapValues[path] = nil
			}
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

		if known, bad := s.resolveMapKey(key); known {
			continue
		} else if bad != "" {
			unknown = append(unknown, bad)

			continue
		}

		if bad := s.firstUnknownSegment(key); bad != "" {
			unknown = append(unknown, bad)
		}
	}

	slices.Sort(unknown)

	return slices.Compact(unknown)
}

// resolveMapKey resolves a key that falls under a map-typed field. The segment
// after the map path is the runtime key; the rest must match the map's value
// struct. It returns the first unresolved path segment when the key is not
// valid.
func (s configSchema) resolveMapKey(key string) (known bool, bad string) {
	for path, valueType := range s.mapValues {
		if key == path {
			return true, ""
		}

		prefix := path + "."
		if !strings.HasPrefix(key, prefix) {
			continue
		}

		rest := strings.TrimPrefix(key, prefix)
		parts := strings.Split(rest, ".")
		if len(parts) == 1 {
			// The runtime key itself (the instance name).
			return true, ""
		}

		if valueType == nil {
			return true, ""
		}

		return resolveStructFields(valueType, parts[1:], prefix+parts[0]+".")
	}

	return false, ""
}

// resolveStructFields walks a struct type using mapstructure tags. prefix is
// the path already consumed, used to name the first unresolved segment.
func resolveStructFields(t reflect.Type, segments []string, prefix string) (known bool, bad string) {
	if len(segments) == 0 {
		return true, ""
	}

	t = indirect(t)
	if t == nil {
		return false, prefix + segments[0]
	}

	if t.Kind() != reflect.Struct {
		return false, prefix + segments[0]
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
			if ok, bad := resolveStructFields(field.Type, segments, prefix); ok {
				return true, ""
			} else if bad != "" {
				return false, bad
			}

			continue
		}

		if name != segments[0] {
			continue
		}

		ft := indirect(field.Type)
		if ft == nil {
			return false, prefix + segments[0]
		}

		if ft.Kind() == reflect.Map {
			if len(segments) == 1 {
				return true, ""
			}

			vt := indirect(ft.Elem())
			if vt == nil || vt.Kind() != reflect.Struct {
				return true, ""
			}

			return resolveStructFields(vt, segments[2:], prefix+segments[0]+"."+segments[1]+".")
		}

		if ft.Kind() == reflect.Struct {
			return resolveStructFields(ft, segments[1:], prefix+segments[0]+".")
		}

		if len(segments) == 1 {
			return true, ""
		}

		return false, prefix + segments[0]
	}

	return false, prefix + segments[0]
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

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
