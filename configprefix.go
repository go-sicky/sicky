/**
 * @file configprefix.go
 * @package sicky
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package sicky

import (
	"reflect"
	"strings"
)

// configPrefixes returns the viper key paths at which a sicky Config lives
// inside the target passed to ConfigUnmarshal.
//
// The environment mechanism binds configuration keys, and a key is only
// meaningful relative to where the value sits in the tree being unmarshaled.
// The framework's own tests and the simplest possible caller unmarshal a
// Config directly, so `manager.auth_token` is the whole path and
// SICKY_MANAGER_AUTH_TOKEN is the variable that fills it. Every project this
// framework scaffolds instead nests it — `{"sicky": {..., "manager": {...}}}`
// — so the same variable needs the key `sicky.manager.auth_token`, and binding
// only the flat one silently drops it.
//
// That is not a theoretical gap. It disabled all 32 sensitiveEnvKeys entries
// for every generated application: SICKY_MANAGER_AUTH_TOKEN, SICKY_TRACER_DSN,
// SICKY_INFRA_BUN_DSN and the rest were accepted by the operator, logged by
// the ignored-variable check as known, and never read.
//
// Deriving the path from the caller's own type is what makes the mechanism
// work regardless of how deeply the application chooses to nest. The result
// always contains "" as a fallback, so a target that holds no Config at all
// keeps exactly the flat behavior it has today.
func configPrefixes(raw any) []string {
	var (
		found    []string
		seen     = map[string]struct{}{}
		walkType func(t reflect.Type, prefix string, depth int)
	)

	add := func(prefix string) {
		if _, ok := seen[prefix]; ok {
			return
		}

		seen[prefix] = struct{}{}
		found = append(found, prefix)
	}

	configType := reflect.TypeFor[Config]()

	walkType = func(t reflect.Type, prefix string, depth int) {
		// Bounded so a self-referential type cannot spin. Config is five
		// levels deep at most; the limit is generous rather than tight.
		if depth > 12 {
			return
		}

		for t != nil && t.Kind() == reflect.Pointer {
			t = t.Elem()
		}

		if t == nil {
			return
		}

		if t == configType {
			add(prefix)

			return
		}

		// Only a struct can carry a nested Config under a known key. A map
		// keys its values at runtime, so there is no single path to bind —
		// the flat fallback covers that shape.
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

			// A squash has no key of its own: its fields live at the
			// parent's path, which is what "prefix" already holds.
			if options == "squash" {
				walkType(field.Type, prefix, depth+1)

				continue
			}

			walkType(field.Type, joinKey(prefix, name), depth+1)
		}
	}

	walkType(reflect.TypeOf(raw), "", 0)

	// Always offer the flat path so behavior for a target that holds no
	// Config is unchanged from before this existed.
	add("")

	return found
}

// joinKey concatenates two viper key segments, tolerating an empty prefix.
func joinKey(prefix, key string) string {
	switch {
	case prefix == "":
		return key
	case key == "":
		return prefix
	default:
		return prefix + "." + key
	}
}
