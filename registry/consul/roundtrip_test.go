package consul

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/go-sicky/sicky/registry"
)

// A peer registered through consul must come back with the scalars consul's
// registration shape cannot express natively.
//
// AdvertiseAddress is the field with a live consumer: client/grpc's resolver
// falls back to Instance.AdvertiseAddress when a server entry carries none,
// so before this the fallback was always "" under consul — a peer relying on
// it resolved to no addresses at all, while the same peer resolved fine under
// redis and local (both of which marshal the whole Instance).
//
// This goes through Register and Load rather than the pure encode/decode
// helpers: removing both call sites leaves the helper tests green, so they
// alone would prove nothing about the wiring.
func TestRegisterLoadRoundTripKeepsEveryInstanceField(t *testing.T) {
	var (
		mu      sync.Mutex
		svcByID = map[string]json.RawMessage{}
	)

	// Just enough of the agent API for a genuine round trip: record whatever
	// PUT /v1/agent/service/register sends, serve the same records back from
	// GET /v1/agent/services.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/v1/agent/service/register":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)

				return
			}

			// A real agent stores the registration under its own shape, where
			// the service name is "Service" rather than the registration's
			// "Name". Translate it, or Load reads back an empty name and the
			// failure looks like the code's when it is the fake's.
			var reg map[string]any
			if err := json.Unmarshal(body, &reg); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)

				return
			}

			if name, ok := reg["Name"]; ok {
				reg["Service"] = name
			}

			svc, err := json.Marshal(reg)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)

				return
			}

			mu.Lock()
			svcByID[fmt.Sprint(reg["ID"])] = json.RawMessage(svc)
			mu.Unlock()

			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/agent/services":
			mu.Lock()
			out := svcByID
			mu.Unlock()

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(out)
		default:
			http.Error(w, "unsupported", http.StatusNotImplemented)
		}
	}))
	defer srv.Close()

	rg := New(&registry.Options{Name: "roundtrip"}, &Config{Endpoint: srv.URL})
	if rg == nil {
		t.Fatal("New returned nil")
	}

	want := &registry.Instance{
		ID:               uuid.New(),
		ServiceName:      "svc",
		Type:             "standard",
		AdvertiseAddress: "10.0.0.9:1234",
		ManagerAddress:   "127.0.0.1",
		ManagerPort:      8888,
		Weight:           7,
		Status:           3,
		CheckEntryPoint:  "/healthz",
		TTL:              30,
	}

	if err := rg.Register(want); err != nil {
		t.Fatalf("register: %v", err)
	}

	got, err := rg.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("Load returned %d instances, want 1", len(got))
	}

	ins := got[0]

	for _, f := range []struct {
		name     string
		got, exp any
	}{
		{"ID", ins.ID, want.ID},
		{"ServiceName", ins.ServiceName, want.ServiceName},
		{"Type", ins.Type, want.Type},
		{"AdvertiseAddress", ins.AdvertiseAddress, want.AdvertiseAddress},
		{"ManagerAddress", ins.ManagerAddress, want.ManagerAddress},
		{"ManagerPort", ins.ManagerPort, want.ManagerPort},
		{"Weight", ins.Weight, want.Weight},
		{"Status", ins.Status, want.Status},
		{"CheckEntryPoint", ins.CheckEntryPoint, want.CheckEntryPoint},
		{"TTL", ins.TTL, want.TTL},
	} {
		if f.got != f.exp {
			t.Errorf("%s = %v, want %v: the consul round trip dropped it", f.name, f.got, f.exp)
		}
	}
}

// An instance that sets none of the six must not gain a metadata key: the
// common case should cost nothing against the agent's per-service budget, and
// a record written this way is byte-identical to one from before the change.
func TestRegisterWritesNoInstanceKeyWhenNothingIsSet(t *testing.T) {
	var (
		mu      sync.Mutex
		rawBody []byte
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.URL.Path == "/v1/agent/service/register" {
			body, _ := io.ReadAll(r.Body)
			mu.Lock()
			rawBody = body
			mu.Unlock()

			w.WriteHeader(http.StatusOK)

			return
		}

		http.Error(w, "unsupported", http.StatusNotImplemented)
	}))
	defer srv.Close()

	rg := New(&registry.Options{Name: "nokey"}, &Config{Endpoint: srv.URL})
	if rg == nil {
		t.Fatal("New returned nil")
	}

	if err := rg.Register(&registry.Instance{
		ID:             uuid.New(),
		ServiceName:    "svc",
		ManagerAddress: "127.0.0.1",
		ManagerPort:    8888,
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	mu.Lock()
	body := string(rawBody)
	mu.Unlock()

	var probe struct {
		Meta map[string]string `json:"Meta"`
	}

	if err := json.Unmarshal([]byte(body), &probe); err != nil {
		t.Fatalf("registration body is not valid JSON: %v", err)
	}

	if _, present := probe.Meta[instanceFieldsKey]; present {
		t.Errorf("an all-default instance wrote Meta[%q] = %q; it must write no key at all",
			instanceFieldsKey, probe.Meta[instanceFieldsKey])
	}
}
