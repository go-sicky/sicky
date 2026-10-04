/**
 * @file encode.go
 * @package consul
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package consul

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"

	"github.com/go-sicky/sicky/registry"
)

// Consul's Meta is a map, so anything spread across one key per element comes
// back in Go's randomized map order. The tags used to be written that way —
// Meta["tag-0"], Meta["tag-1"] — and read back by appending in iteration
// order, so a service's tag order was different on every Load. Load runs on
// every pool purge tick, which makes it worse than a one-time loss: two
// identical reads can disagree, and any consumer indexing the slice sees
// flapping values.
//
// The other two backends do not have this problem, because they JSON-marshal
// the whole Instance and the slice order rides along.
//
// Tags now travel as a single JSON array under one Meta key, which is exact.
// The per-index keys are still read on the way back, because a cluster
// registered by an older version carries those and would otherwise silently
// lose every tag on upgrade. Writing them is stopped, so the Meta pair count
// stops growing with the tag count.
const (
	metaTagPrefix = "tag-"
	metaTagsKey   = "tags"
)

// encodeTags renders an instance's tags as one Meta value. A nil or empty
// slice yields present=false, so no empty key is written.
func encodeTags(tags []string) (string, bool) {
	if len(tags) == 0 {
		return "", false
	}

	data, err := json.Marshal(tags)
	if err != nil {
		// A []string cannot fail to marshal. Returning the error anyway
		// keeps the caller from writing a half-encoded value.
		return "", false
	}

	return string(data), true
}

// decodeTags reconstructs the tag slice from a service's Meta.
//
// The modern key wins. The legacy per-index keys are ordered by their index
// rather than by map iteration, which is what a previous version of this
// backend meant to write.
func decodeTags(meta map[string]string) []string {
	if raw, present := meta[metaTagsKey]; present {
		var tags []string
		if err := json.Unmarshal([]byte(raw), &tags); err == nil {
			return tags
		}

		// A malformed value must not silently produce no tags: fall through
		// to the legacy keys, which may still hold them.
	}

	legacy := make(map[int]string, len(meta))

	for k, v := range meta {
		after, ok := strings.CutPrefix(k, metaTagPrefix)
		if !ok {
			continue
		}

		idx, err := strconv.Atoi(after)
		if err != nil || idx < 0 {
			// Negative indices are not a thing the writer ever produced —
			// it ranged over a slice — so a negative one is a hand-written
			// key, not a tag. Accepting it would inject a tag that sorts
			// ahead of every real one.
			continue
		}

		legacy[idx] = v
	}

	if len(legacy) == 0 {
		return nil
	}

	indexes := make([]int, 0, len(legacy))
	for idx := range legacy {
		indexes = append(indexes, idx)
	}

	slices.Sort(indexes)

	tags := make([]string, 0, len(indexes))
	for _, idx := range indexes {
		tags = append(tags, legacy[idx])
	}

	return tags
}

// instanceTags is the accessor the decode path uses, kept separate so the
// ordering fix can be pinned without a live Consul agent.
func instanceTags(meta map[string]string) []string {
	return decodeTags(meta)
}

// instanceFieldsKey holds the Instance scalars that consul's own registration
// shape cannot express.
//
// A consul registration carries Name/Address/Port natively and gives Tags and
// Meta for everything else, so the other six Instance fields had nowhere to
// go: they were neither written nor read. That is a silent one-way loss on
// the only backend that does not marshal the whole Instance — redis and local
// both json.Marshal it and lose nothing.
//
// AdvertiseAddress is not cosmetic. client/grpc's resolver falls back to
// Instance.AdvertiseAddress when a server entry has none, so a peer relying
// on that fallback resolved to NO addresses under consul while resolving
// fine under the other two.
//
// One JSON value rather than six Meta keys: fewer entries against the agent's
// per-service metadata budget, atomic, and a version field can be added later
// without a second migration. Both directions are additive — an older reader
// ignores an unknown key, and an older writer's records simply lack it.
const instanceFieldsKey = "inst"

// instanceFields is the persisted subset. Every field is omitempty so a record
// carrying nothing but a zero weight does not spend bytes saying so.
type instanceFields struct {
	Type             string `json:"type,omitempty"`
	AdvertiseAddress string `json:"advertise_address,omitempty"`
	Weight           int    `json:"weight,omitempty"`
	Status           int    `json:"status,omitempty"`
	CheckEntryPoint  string `json:"check_entry_point,omitempty"`
	TTL              int    `json:"ttl,omitempty"`
}

// encodeInstanceFields renders the scalars consul cannot carry natively.
// An instance where all six are zero yields present=false, so no key is
// written and the record stays exactly as small as it was.
func encodeInstanceFields(ins *registry.Instance) (string, bool) {
	if ins == nil {
		return "", false
	}

	f := instanceFields{
		Type:             ins.Type,
		AdvertiseAddress: ins.AdvertiseAddress,
		Weight:           ins.Weight,
		Status:           ins.Status,
		CheckEntryPoint:  ins.CheckEntryPoint,
		TTL:              ins.TTL,
	}

	if f == (instanceFields{}) {
		return "", false
	}

	data, err := json.Marshal(f)
	if err != nil {
		// A struct of strings and ints cannot fail to marshal. Returning the
		// error anyway keeps the caller from writing a half-encoded value,
		// which is the same discipline encodeTags follows.
		return "", false
	}

	return string(data), true
}

// decodeInstanceFields applies the persisted scalars onto ins.
//
// The second return distinguishes "no key" (an older writer, or an instance
// that never had any of the six set) from "key present but unreadable" — the
// caller logs the second so a hand-edited value is not silently ignored.
func decodeInstanceFields(meta map[string]string, ins *registry.Instance) bool {
	if ins == nil {
		return false
	}

	raw, present := meta[instanceFieldsKey]
	if !present {
		return false
	}

	var f instanceFields
	if err := json.Unmarshal([]byte(raw), &f); err != nil {
		return false
	}

	ins.Type = f.Type
	ins.AdvertiseAddress = f.AdvertiseAddress
	ins.Weight = f.Weight
	ins.Status = f.Status
	ins.CheckEntryPoint = f.CheckEntryPoint
	ins.TTL = f.TTL

	return true
}
