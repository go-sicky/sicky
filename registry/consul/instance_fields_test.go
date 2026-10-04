package consul

import (
	"testing"

	"github.com/google/uuid"

	"github.com/go-sicky/sicky/registry"
)

// consul's registration shape has no field for six of the thirteen Instance
// scalars, and they were neither written nor read — a silent one-way loss on
// the only backend that does not marshal the whole Instance (redis and local
// both json.Marshal it).
//
// AdvertiseAddress is the one with a live consumer: client/grpc's resolver
// falls back to it when a server entry carries none, so a peer relying on
// that fallback resolved to no addresses under consul and resolved fine
// under the other two.
func TestInstanceFieldsRoundTrip(t *testing.T) {
	want := registry.Instance{
		ID:               uuid.New(),
		ServiceName:      "svc",
		Type:             "standard",
		AdvertiseAddress: "10.0.0.9:1234",
		Weight:           7,
		Status:           3,
		CheckEntryPoint:  "/healthz",
		TTL:              30,
	}

	raw, present := encodeInstanceFields(&want)
	if !present {
		t.Fatal("an instance with all six fields set must produce a value")
	}

	meta := map[string]string{instanceFieldsKey: raw}

	var got registry.Instance
	if !decodeInstanceFields(meta, &got) {
		t.Fatal("decode reported the key as unreadable")
	}

	for _, f := range []struct {
		name     string
		got, exp any
	}{
		{"Type", got.Type, want.Type},
		{"AdvertiseAddress", got.AdvertiseAddress, want.AdvertiseAddress},
		{"Weight", got.Weight, want.Weight},
		{"Status", got.Status, want.Status},
		{"CheckEntryPoint", got.CheckEntryPoint, want.CheckEntryPoint},
		{"TTL", got.TTL, want.TTL},
	} {
		if f.got != f.exp {
			t.Errorf("%s = %v, want %v: the consul round trip dropped it", f.name, f.got, f.exp)
		}
	}
}

// A record written by an older version carries no such key. It must decode to
// the zero values it always had rather than failing, or an upgrade would drop
// every peer registered before it.
func TestDecodeWithoutTheKeyIsANoOpNotAnError(t *testing.T) {
	ins := &registry.Instance{ServiceName: "svc", AdvertiseAddress: "untouched"}

	if decodeInstanceFields(map[string]string{}, ins) {
		t.Error("a missing key must report nothing applied")
	}

	if ins.AdvertiseAddress != "untouched" {
		t.Errorf("AdvertiseAddress = %q, want it left alone", ins.AdvertiseAddress)
	}
}

// A key that is present but unreadable must be reported so the caller can
// warn, rather than being swallowed into a silent zero.
func TestDecodeReportsAMalformedValue(t *testing.T) {
	ins := &registry.Instance{}

	if decodeInstanceFields(map[string]string{instanceFieldsKey: "{not json"}, ins) {
		t.Error("a malformed value must not report as applied")
	}
}

// An instance with none of the six set must write no key at all, so the common
// case does not grow the record or spend agent metadata budget.
func TestEncodeOmitsAnAllZeroInstance(t *testing.T) {
	if _, present := encodeInstanceFields(&registry.Instance{ServiceName: "svc"}); present {
		t.Error("an all-zero instance must not produce a key")
	}

	if _, present := encodeInstanceFields(nil); present {
		t.Error("a nil instance must not produce a key")
	}
}

// One set field is enough to make the record carry the rest, so a peer that
// only sets AdvertiseAddress still round-trips whatever else it set.
func TestEncodeWritesOnASingleNonZeroField(t *testing.T) {
	raw, present := encodeInstanceFields(&registry.Instance{AdvertiseAddress: "10.0.0.9:1234"})
	if !present {
		t.Fatal("a single set field must produce a value")
	}

	var got registry.Instance
	decodeInstanceFields(map[string]string{instanceFieldsKey: raw}, &got)

	if got.AdvertiseAddress != "10.0.0.9:1234" {
		t.Errorf("AdvertiseAddress = %q, want it preserved", got.AdvertiseAddress)
	}
}
