package sicky

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/spf13/viper"

	"github.com/go-sicky/sicky/service"
)

// managerProbe records the manager address that Run advertises for it at
// the moment a service starts. Run starts the manager before any service,
// so a non-empty address here proves the manager was bound and live; an
// empty one proves Enable turned it off. The capture happens on Run's own
// goroutine and the test reads it only after Run returns, so no extra
// synchronization is needed beyond the mutex the test already holds.
type managerProbe struct {
	*orderedService

	mu         *sync.Mutex
	advertised *string
	seen       *bool
}

func (p *managerProbe) Start() []error {
	errs := p.orderedService.Start()

	p.mu.Lock()
	*p.advertised = serviceToRegistryInstance(p).ManagerAddress
	*p.seen = true
	p.mu.Unlock()

	return errs
}

// TestRunHonorsManagerEnableTriState pins the tri-state contract: a nil
// receiver (no manager block) is off, a nil Enable (block present) is
// on, and only an explicit false is off.
func TestManagerConfigEnabledTriState(t *testing.T) {
	var nilCfg *ManagerConfig

	tests := []struct {
		name string
		cfg  *ManagerConfig
		want bool
	}{
		{name: "no manager block", cfg: nil, want: false},
		{name: "block present, enable unset", cfg: &ManagerConfig{Address: ":9999"}, want: true},
		{name: "explicitly enabled", cfg: &ManagerConfig{Enable: new(true)}, want: true},
		{name: "explicitly disabled", cfg: &ManagerConfig{Enable: new(false)}, want: false},
		{name: "disabled with an address set", cfg: &ManagerConfig{Enable: new(false), Address: "0.0.0.0:8888"}, want: false},
		{name: "nil receiver is a total call", cfg: nilCfg, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.Enabled(); got != tc.want {
				t.Fatalf("Enabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestManagerConfigEnsureKeepsExplicitDisable guards the reason this is a
// pointer rather than a bool: Ensure fills every other zero-valued field
// with a default, so if it also filled Enable it would overwrite an
// operator's explicit "off" and start a manager they asked not to have.
func TestManagerConfigEnsureKeepsExplicitDisable(t *testing.T) {
	c := (&ManagerConfig{Enable: new(false)}).Ensure()
	if c.Enable == nil {
		t.Fatal("Ensure must not invent an Enable value: nil means the block is present, which is on")
	}

	if *c.Enable {
		t.Fatal("Ensure flipped an explicit enable=false to true: the manager would start on an address the operator disabled")
	}

	if c.Enabled() {
		t.Fatal("an explicitly disabled manager must stay disabled after Ensure")
	}
}

// TestManagerConfigEnableSurvivesViperDecode is the regression the
// tri-state exists for. viper's AutomaticEnv only resolves keys it
// already knows, and a plain bool cannot distinguish "key absent" from
// "key false", so {"manager": {"enable": false}} used to be
// indistinguishable from a block that never said anything.
func TestManagerConfigEnableSurvivesViperDecode(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantNil bool
		wantOn  bool
	}{
		{
			name:    "absent enable leaves the pointer nil, which means on",
			raw:     `{"manager": {"address": "0.0.0.0:8888"}}`,
			wantNil: true,
			wantOn:  true,
		},
		{
			name:    "explicit false decodes to a pointer to false, which means off",
			raw:     `{"manager": {"enable": false, "address": "0.0.0.0:8888"}}`,
			wantNil: false,
			wantOn:  false,
		},
		{
			name:    "explicit true decodes to a pointer to true",
			raw:     `{"manager": {"enable": true}}`,
			wantNil: false,
			wantOn:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := viper.New()
			v.SetConfigType("json")

			if err := v.ReadConfig(bytes.NewBufferString(tc.raw)); err != nil {
				t.Fatalf("read config: %v", err)
			}

			var cfg Config
			if err := v.Unmarshal(&cfg); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			if cfg.Manager == nil {
				t.Fatal("manager block decoded to nil")
			}

			if gotNil := cfg.Manager.Enable == nil; gotNil != tc.wantNil {
				t.Fatalf("Enable == nil is %v, want %v (value %v)", gotNil, tc.wantNil, cfg.Manager.Enable)
			}

			if got := cfg.Manager.Enabled(); got != tc.wantOn {
				t.Fatalf("Enabled() = %v, want %v for %s", got, tc.wantOn, tc.raw)
			}
		})
	}
}

// TestRunHonorsManagerEnableTriState is the end-to-end check: Run must
// start the manager for a present block and must not start it for an
// explicit false. The address is a public bind on purpose, because that
// is the case that must never come up.
func TestRunHonorsManagerEnableTriState(t *testing.T) {
	tests := []struct {
		name      string
		enable    *bool
		wantStart bool
	}{
		{name: "block present starts the manager", enable: nil, wantStart: true},
		{name: "explicit false does not", enable: new(false), wantStart: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			restore := snapshotGlobals()
			defer restore()

			ctx, cancel := context.WithCancel(context.Background())

			options = testOptions()
			options.Context = ctx

			// Observe whether the manager was actually up by capturing the
			// address Run advertises for it, not by inspecting managerApp
			// afterwards: Run drops that global when it shuts down, so a
			// post-run read can no longer answer the question. A service
			// started by Run is the right vantage point because Run starts
			// the manager before it starts any service, so the address is
			// already bound when the service records it. Start runs on
			// Run's own goroutine and the test only reads after Run
			// returns, so the capture needs no extra synchronization.
			var (
				mu         sync.Mutex
				advertised string
				recordSeen bool
			)

			service.Set(&managerProbe{
				orderedService: probeService("manager-enable-probe"),
				mu:             &mu,
				advertised:     &advertised,
				seen:           &recordSeen,
			})

			go func() {
				time.Sleep(150 * time.Millisecond)
				cancel()
			}()

			cfg := &Config{Manager: &ManagerConfig{Enable: tc.enable, Address: "127.0.0.1:0"}}
			if err := Run(cfg); err != nil {
				t.Fatalf("Run: %v", err)
			}

			mu.Lock()
			got, seen := advertised, recordSeen
			mu.Unlock()

			if !seen {
				t.Fatal("the probe service never ran, so the manager's state was not observed")
			}

			if gotStart := got != ""; gotStart != tc.wantStart {
				t.Fatalf("manager started = %v (advertised %q), want %v (enable %v)", gotStart, got, tc.wantStart, tc.enable)
			}
		})
	}
}
