/**
 * @file decode.go
 * @package redis
 * @author Dr.NP <np@herewe.tech>
 * @since 10/04/2026
 */

package redis

import (
	"encoding/json"

	"github.com/go-sicky/sicky/registry"
)

// decodeInstancesFn pins decodeInstances' signature.
//
// Load's named error is what the metrics defer reads, so a decoder that
// returned an error could put a single corrupt value back in charge of every
// later Load's result. A named function type is what enforces that: giving
// decodeInstances a second return value stops this from compiling.
//
// A bare `var _ = decodeInstances` would not do — it still compiles after the
// signature changes, which is the whole thing being guarded.
type decodeInstancesFn = func(map[string]string, func(string, error)) []*registry.Instance

var _ decodeInstancesFn = decodeInstances

// decodeInstances turns the raw hash values into instances, reporting each
// value it could not decode through onError and skipping it.
//
// It is a function rather than a loop inside Load because Load's error is a
// named return that the metrics defer reads. The loop used to assign its
// per-entry decode failures to that same variable: one corrupt value left it
// non-nil, so the defer recorded every subsequent Load as an error result and
// stopped publishing the instance gauge — permanently, for as long as the bad
// value sat in the hash — while Load itself returned nil and every caller saw
// success. The metrics said the backend was failing; the return value said it
// was fine.
//
// Kept separate so the decode cannot reach the named error at all.
func decodeInstances(raw map[string]string, onError func(field string, err error)) []*registry.Instance {
	instances := make([]*registry.Instance, 0, len(raw))

	for field, value := range raw {
		var ins registry.Instance

		if err := json.Unmarshal([]byte(value), &ins); err != nil {
			if onError != nil {
				onError(field, err)
			}

			continue
		}

		instances = append(instances, &ins)
	}

	return instances
}
