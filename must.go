/**
 * @file must.go
 * @package sicky
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package sicky

import (
	"maps"
	"strings"
	"sync"

	"github.com/go-sicky/sicky/infra"
)

// mustMu guards MustInfra, MustBroker and MustRegistry.
//
// These three are exported package-level values, so a reader outside this
// package can reach them at any time — a /debug handler, an operator's script,
// a second goroutine wiring up a service. Run used to mutate MustInfra as it
// went, flipping one entry per successfully initialized component, which put
// those readers in a concurrent read/write on a map. That is not a race the
// runtime recovers from: it is `fatal error: concurrent map read and map
// write`, and the process is gone with no stack from the offending code.
//
// Run no longer keeps its bookkeeping here at all — see mustInfraPending —
// so the values below are published at defined instants rather than mutated
// throughout startup. The mutex is what makes those instants safe against the
// accessors in this file.
//
// Direct reads of the exported variables are still unsynchronized. That cannot
// be fixed without removing them, which would break every caller, so the
// accessors here are the supported path.
var mustMu sync.RWMutex

// MustInfraSnapshot returns a copy of the required-infrastructure set.
//
// This is the safe way to read MustInfra. The returned map is the caller's to
// keep and mutate; the framework's own state is unaffected.
func MustInfraSnapshot() map[string]bool {
	mustMu.RLock()
	defer mustMu.RUnlock()

	out := make(map[string]bool, len(MustInfra))
	maps.Copy(out, MustInfra)

	return out
}

// MustBrokerRequired reports whether a broker is required.
func MustBrokerRequired() bool {
	mustMu.RLock()
	defer mustMu.RUnlock()

	return MustBroker
}

// MustRegistryRequired reports whether a registry is required.
func MustRegistryRequired() bool {
	mustMu.RLock()
	defer mustMu.RUnlock()

	return MustRegistry
}

// mustInfraPending returns a mutable copy of the requirement set for the
// caller to work on, merging in the names from Options.MustInfra and
// republishing the result so a reader polling MustInfraSnapshot sees what is
// about to be required.
//
// Names are lowercased and trimmed. An entry is either a component name
// ("s3") or a named instance ("s3:public"); the latter requires that exact
// instance to be registered. A blank entry is silently dropped — it is not a
// typo — while an unrecognized component, or a colon with no instance name,
// is reported through warn with the string the caller wrote, not the
// normalized form, so the warning quotes what is actually in the
// configuration.
func mustInfraPending(required []string, warn func(string)) map[string]bool {
	mustMu.RLock()
	pending := make(map[string]bool, len(MustInfra)+len(required))
	maps.Copy(pending, MustInfra)

	mustMu.RUnlock()

	for _, name := range required {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" {
			continue
		}

		component, instance, hasInstance := strings.Cut(key, ":")
		if !isKnownInfraComponent(component) || (hasInstance && strings.TrimSpace(instance) == "") {
			warn(name)

			continue
		}

		if hasInstance {
			key = component + ":" + strings.TrimSpace(instance)
		}

		pending[key] = true
	}

	publishMustInfra(pending)

	return pending
}

// mustInfraInstanceExists reports whether a "component:instance" requirement
// is registered. The component is already known when this is called.
func mustInfraInstanceExists(key string) bool {
	component, instance, ok := strings.Cut(key, ":")
	if !ok || instance == "" {
		return false
	}

	switch component {
	case componentBadger:
		return infra.GetBadger(instance) != nil
	case componentBun:
		return infra.GetBun(instance) != nil
	case componentClickhouse:
		return infra.GetClickHouse(instance) != nil
	case componentElastic:
		return infra.GetElastic(instance) != nil
	case componentMQTT:
		return infra.GetMQTT(instance) != nil
	case componentMongo:
		return infra.GetMongo(instance) != nil
	case componentNATS:
		return infra.GetNATS(instance) != nil
	case componentRedis:
		return infra.GetRedis(instance) != nil
	case componentRistretto:
		return infra.GetRistretto(instance) != nil
	case componentS3:
		return infra.GetS3(instance) != nil
	default:
		return false
	}
}

// publishMustInfra replaces the exported set with a snapshot of pending.
func publishMustInfra(pending map[string]bool) {
	next := make(map[string]bool, len(pending))
	maps.Copy(next, pending)

	mustMu.Lock()
	MustInfra = next
	mustMu.Unlock()
}

// consumeMustInfra records that one component was satisfied. It republishes
// the whole set so a reader polling MustInfraSnapshot sees the requirement
// being met, which is what flipping the exported entry used to do.
//
// Publishing on every component is what makes this safe: Run's own writes
// touch its private copy, so the exported map changes at most once per
// component instead of once per statement, and never while Run holds no lock.
func consumeMustInfra(pending map[string]bool, component string) {
	if !pending[component] {
		return
	}

	pending[component] = false

	publishMustInfra(pending)
}

// setMustFlags publishes the broker and registry requirements.
func setMustFlags(broker, registry bool) {
	mustMu.Lock()
	MustBroker = broker
	MustRegistry = registry
	mustMu.Unlock()
}

// knownInfraComponents is the set accepted in Options.MustInfra.
var knownInfraComponents = map[string]struct{}{
	componentBadger:     {},
	componentBun:        {},
	componentClickhouse: {},
	componentElastic:    {},
	componentMQTT:       {},
	componentMongo:      {},
	componentNATS:       {},
	componentRedis:      {},
	componentRistretto:  {},
	componentS3:         {},
}

func isKnownInfraComponent(name string) bool {
	_, ok := knownInfraComponents[name]

	return ok
}

// consumeMustFlag clears one of the broker/registry requirements and
// republishes both together, so a reader polling the accessors never sees a
// half-updated pair.
func consumeMustFlag(broker, registry bool) {
	mustMu.Lock()
	if broker {
		MustBroker = false
	}

	if registry {
		MustRegistry = false
	}

	mustMu.Unlock()
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
