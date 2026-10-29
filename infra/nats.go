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
 * @file nats.go
 * @package infra
 * @author Dr.NP <np@herewe.tech>
 * @since 11/29/2023
 */

package infra

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/metrics"
)

// NATSConfig holds NATS connection settings.
type NATSConfig struct {
	URL string `json:"url" mapstructure:"url" yaml:"url"`
	// Auth: set at most one of Token / Username+Password / CredsFile /
	// NkeyFile. Credentials are never logged.
	Token     string `json:"token"      mapstructure:"token"      yaml:"token"`
	Username  string `json:"username"   mapstructure:"username"   yaml:"username"`
	Password  string `json:"password"   mapstructure:"password"   yaml:"password"`
	CredsFile string `json:"creds_file" mapstructure:"creds_file" yaml:"creds_file"`
	NkeyFile  string `json:"nkey_file"  mapstructure:"nkey_file"  yaml:"nkey_file"`
	// EnableTLS wraps the connection in TLS 1.2+. RootCAFile optionally
	// pins a custom CA bundle (PEM path).
	EnableTLS  bool   `json:"enable_tls"   mapstructure:"enable_tls"   yaml:"enable_tls"`
	RootCAFile string `json:"root_ca_file" mapstructure:"root_ca_file" yaml:"root_ca_file"`
	// TimeoutSec bounds the initial connect. Seconds.
	TimeoutSec int `json:"timeout_sec" mapstructure:"timeout_sec" yaml:"timeout_sec"`
	// ReconnectWaitSec is the delay between reconnect attempts. Seconds.
	ReconnectWaitSec int `json:"reconnect_wait_sec" mapstructure:"reconnect_wait_sec" yaml:"reconnect_wait_sec"`
	// MaxReconnects caps reconnect attempts. 0 fills 60 (the library
	// default); -1 retries forever (opt-in: a permanently down server
	// then never surfaces as a startup error); other negatives abort.
	MaxReconnects int `json:"max_reconnects" mapstructure:"max_reconnects" yaml:"max_reconnects"`
	// DefaultInstance names the instance that also serves as the shared
	// singleton when the top-level fields do not form a complete config.
	DefaultInstance string `json:"default_instance" mapstructure:"default_instance" yaml:"default_instance"`
	// NoDefault suppresses the shared singleton on a fresh init; only Instances are created.
	NoDefault bool `json:"no_default" mapstructure:"no_default" yaml:"no_default"`
	// Instances are complete NATS configurations. They do not inherit fields
	// from the block above.
	Instances map[string]*NATSConfig `json:"instances" mapstructure:"instances" yaml:"instances"`
}

// Deprecated: use NATSConfig.
type NatsConfig = NATSConfig

// Defaults for NATS timing.
const (
	DefaultNatsTimeoutSec       = 5
	DefaultNatsReconnectWaitSec = 2
	DefaultNatsMaxReconnects    = 60
)

// ErrNATSURLEmpty aborts startup: a non-nil NATSConfig means "enable nats",
// and an empty URL would otherwise silently connect to the client default
// (nats://127.0.0.1:4222). ErrNATSFileUnreadable aborts on missing auth/CA
// files. ErrNATSOptionInvalid aborts on negative timing.
// ErrNATSAuthConflict aborts when more than one auth method is set.
// ErrNATSCAWithoutTLS aborts when RootCAFile is set without EnableTLS.
var (
	ErrNATSURLEmpty       = errors.New("infra: nats url is empty")
	ErrNATSFileUnreadable = errors.New("infra: nats file unreadable")
	ErrNATSOptionInvalid  = errors.New("infra: nats option invalid")
	ErrNATSAuthConflict   = errors.New("infra: nats at most one of token/username+password/creds_file/nkey_file")
	ErrNATSCAWithoutTLS   = errors.New("infra: nats root_ca_file requires enable_tls")
)

var (
	// Deprecated: use ErrNATSURLEmpty.
	ErrNatsURLEmpty = ErrNATSURLEmpty
	// Deprecated: use ErrNATSFileUnreadable.
	ErrNatsFileUnreadable = ErrNATSFileUnreadable
	// Deprecated: use ErrNATSOptionInvalid.
	ErrNatsOptionInvalid = ErrNATSOptionInvalid
	// Deprecated: use ErrNATSAuthConflict.
	ErrNatsAuthConflict = ErrNATSAuthConflict
	// Deprecated: use ErrNATSCAWithoutTLS.
	ErrNatsCAWithoutTLS = ErrNATSCAWithoutTLS
)

// NATS is the shared connection singleton.
var NATS *nats.Conn

// Deprecated: use NATS. Kept in sync by InitNATS/ClearNATS.
var Nats *nats.Conn

// InitNATS connects and stores the shared singleton (first-wins, nil cfg disables).
func InitNATS(cfg *NATSConfig) (*nats.Conn, error) {
	if cfg == nil {
		return nil, nil
	}

	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()

	conn, err := initNATS(cfg)
	metrics.CountInfraInit("nats", err)
	if err == nil && conn == nil {
		// no_default: the component initialized, but there is no shared
		// singleton to report as up.
		metrics.SetInfraUp("nats", false)
	}

	return conn, err
}

func initNATS(cfg *NATSConfig) (*nats.Conn, error) {
	if cfg == nil {
		return nil, nil
	}

	cfg.Ensure()
	if err := cfg.Validate(); err != nil {
		logger.Logger.Error(
			"init NATS config invalid",
			"error", err.Error(),
		)

		return nil, err
	}

	instances, names, err := cfg.resolveInstances()
	if err != nil {
		logger.Logger.Error(
			"init NATS instances invalid",
			"error", err.Error(),
		)

		return nil, err
	}

	identityErr := cfg.validateIdentity()
	selection, err := resolveDefaultSelection(cfg.NoDefault, cfg.DefaultInstance, names, identityErr == nil, identityErr)
	if err != nil {
		logger.Logger.Error(
			"init NATS default selection invalid",
			"error", err.Error(),
		)

		return nil, err
	}

	if selection.Ignored != "" {
		logger.Logger.Warn(
			"nats default_instance ignored because the top-level config is complete",
			"instance", selection.Ignored,
		)
	}

	if cfg.NoDefault {
		logger.Logger.Info("nats no_default is set; this block does not create a shared instance")
	}

	if !selection.UseTopLevel && selection.Instance != "" && cfg.DefaultInstance == "" {
		if len(names) == 1 {
			logger.Logger.Info(
				"nats using the only named instance as the default",
				"instance", selection.Instance,
			)
		} else {
			logger.Logger.Warn(
				"nats no default_instance configured; using the first instance in sorted order",
				"instance", selection.Instance,
			)
		}
	}

	// Build first, register second: a failure never leaves a half-installed
	// component behind. Existing first-wins entries are not rebuilt.
	mu.RLock()
	existingDefault := NATS
	mu.RUnlock()

	var topLevel *nats.Conn

	if selection.UseTopLevel && existingDefault == nil {
		topLevel, err = newNATSClient(cfg, "nats")
		if err != nil {
			// newNATSClient already logged the failure with the same
			// instance label; a second identical line adds nothing.
			return nil, err
		}
	}

	built := make(map[string]*nats.Conn, len(names))
	for _, name := range names {
		if _, ok := natsInstances.get(name); ok {
			logger.Logger.Warn("nats instance already initialized, keeping existing connection", "instance", name)

			continue
		}

		conn, berr := newNATSClient(instances[name], "nats:"+name)
		if berr != nil {
			metrics.CountInfraInit("nats:"+name, berr)
			logger.Logger.Error(
				"init NATS instance failed",
				"instance", name,
				"error", berr.Error(),
			)

			rollbackNATSConnections(topLevel, built)

			return nil, fmt.Errorf("nats instance %s: %w", name, berr)
		}

		metrics.CountInfraInit("nats:"+name, nil)
		built[name] = conn
	}

	// Register under the lock, release before anything that can block. This
	// mu is a single package-global write lock guarding all ten
	// singletons, so holding it across a duplicate's teardown (a 5s
	// Disconnect, a Close that flushes, paho's quiesce sleep) stalls
	// every Get* reader and every manager health probe for the duration.
	if topLevel != nil {
		stored := false

		mu.Lock()
		if NATS == nil {
			NATS = topLevel
			Nats = topLevel
			natsDefaultInstance = ""
			stored = true
		}

		mu.Unlock()

		if !stored {
			logger.Logger.Warn("nats already initialized, closing duplicate connection")
			closeNATSConnection(topLevel)
		}
	}

	for _, name := range names {
		conn, ok := built[name]
		if !ok {
			continue
		}

		if _, stored := natsInstances.putFirst(name, conn); !stored {
			logger.Logger.Warn("nats instance already initialized, closing duplicate connection", "instance", name)
			closeNATSConnection(conn)

			continue
		}

		logger.Logger.InfoContext(
			context.Background(),
			"nats instance initialized",
			"instance", name,
			"url", redactDSN(instances[name].URL),
			"tls", instances[name].EnableTLS,
		)
	}

	// A named default is installed only when the shared singleton is still
	// free; first-wins keeps an earlier top-level or alias default.
	if !selection.UseTopLevel && selection.Instance != "" {
		if conn, ok := natsInstances.get(selection.Instance); ok && conn != nil {
			mu.Lock()
			if NATS == nil {
				NATS = conn
				Nats = conn
				natsDefaultInstance = selection.Instance
			}
			mu.Unlock()
		}
	}

	mu.RLock()
	conn := NATS
	mu.RUnlock()

	if topLevel != nil {
		logger.Logger.InfoContext(
			context.Background(),
			"init NATS successful",
			"instance", "nats",
			"url", redactDSN(cfg.URL),
		)
	}

	return conn, nil
}

// newNATSClient connects one connection from cfg. It touches no package-level
// state, so a build that fails leaves nothing registered. label names the
// component series: "nats" for the shared singleton, "nats:<name>" for a named
// instance.
func newNATSClient(cfg *NATSConfig, label string) (*nats.Conn, error) {
	opts := []nats.Option{
		nats.Timeout(time.Duration(cfg.TimeoutSec) * time.Second),
		nats.ReconnectWait(time.Duration(cfg.ReconnectWaitSec) * time.Second),
		nats.MaxReconnects(cfg.MaxReconnects),
	}

	if strings.TrimSpace(cfg.Token) != "" {
		opts = append(opts, nats.Token(cfg.Token))
	}

	if strings.TrimSpace(cfg.Username) != "" {
		opts = append(opts, nats.UserInfo(cfg.Username, cfg.Password))
	}

	if strings.TrimSpace(cfg.CredsFile) != "" {
		opts = append(opts, nats.UserCredentials(cfg.CredsFile))
	}

	if strings.TrimSpace(cfg.NkeyFile) != "" {
		nkeyOpt, err := nats.NkeyOptionFromSeed(cfg.NkeyFile)
		if err != nil {
			logger.Logger.Error(
				"init NATS nkey failed",
				"instance", label,
				"nkey_file", cfg.NkeyFile,
				"error", err.Error(),
			)

			return nil, fmt.Errorf("%w: %s", ErrNATSFileUnreadable, cfg.NkeyFile)
		}

		opts = append(opts, nkeyOpt)
	}

	if strings.TrimSpace(cfg.RootCAFile) != "" {
		opts = append(opts, nats.RootCAs(cfg.RootCAFile))
	}

	if cfg.EnableTLS {
		opts = append(opts, nats.Secure(&tls.Config{MinVersion: tls.VersionTLS12}))
	}

	nc, err := nats.Connect(cfg.URL, opts...)
	if err != nil {
		logger.Logger.Error(
			"init NATS failed",
			"instance", label,
			"url", redactDSN(cfg.URL),
			"error", err.Error(),
		)

		return nil, err
	}

	return nc, nil
}

// closeNATSConnection closes one connection. nats.Conn.Close returns nothing,
// so there is no failure to log or return.
func closeNATSConnection(conn *nats.Conn) {
	if conn == nil {
		return
	}

	conn.Close()
}

// rollbackNATSConnections closes everything one init call built but will not
// register, so a failed build leaves no sockets or "up" series behind.
func rollbackNATSConnections(defaultConn *nats.Conn, built map[string]*nats.Conn) {
	closeNATSConnection(defaultConn)

	for name, conn := range built {
		closeNATSConnection(conn)
		metrics.SetInfraUp("nats:"+name, false)
	}
}

// Deprecated: use InitNATS.
func InitNats(cfg *NATSConfig) (*nats.Conn, error) {
	return InitNATS(cfg)
}

// Ensure fills zero-valued fields with defaults and returns the receiver (nil-safe).
func (c *NATSConfig) Ensure() *NATSConfig {
	if c == nil {
		c = new(NATSConfig)
	}

	// Zero fills defaults; negative stays negative so Validate aborts.
	if c.TimeoutSec == 0 {
		c.TimeoutSec = DefaultNatsTimeoutSec
	}

	if c.ReconnectWaitSec == 0 {
		c.ReconnectWaitSec = DefaultNatsReconnectWaitSec
	}

	if c.MaxReconnects == 0 {
		c.MaxReconnects = DefaultNatsMaxReconnects
	}

	return c
}

// Validate rejects half-configured or illegal values and resolves the named
// instances (without mutating the caller's config).
func (c *NATSConfig) Validate() error {
	if c == nil {
		return nil
	}

	if err := c.validateShape(); err != nil {
		return err
	}

	_, names, err := c.resolveInstances()
	if err != nil {
		return err
	}

	identityErr := c.validateIdentity()
	if _, err := resolveDefaultSelection(c.NoDefault, c.DefaultInstance, names, identityErr == nil, identityErr); err != nil {
		return err
	}

	return nil
}

// validateShape checks every value that does not depend on the connection
// identity, so an instance can be rejected before anything dials.
func (c *NATSConfig) validateShape() error {
	if c == nil {
		return nil
	}

	if c.TimeoutSec < 0 || c.ReconnectWaitSec < 0 || c.MaxReconnects < -1 {
		return ErrNATSOptionInvalid
	}

	set := 0
	if strings.TrimSpace(c.Token) != "" {
		set++
	}

	if strings.TrimSpace(c.Username) != "" || strings.TrimSpace(c.Password) != "" {
		set++
	}

	if strings.TrimSpace(c.CredsFile) != "" {
		set++
	}

	if strings.TrimSpace(c.NkeyFile) != "" {
		set++
	}

	if set > 1 {
		return ErrNATSAuthConflict
	}

	if strings.TrimSpace(c.RootCAFile) != "" && !c.EnableTLS {
		return ErrNATSCAWithoutTLS
	}

	for _, f := range []string{c.CredsFile, c.NkeyFile, c.RootCAFile} {
		if strings.TrimSpace(f) != "" {
			if _, err := os.Stat(f); err != nil {
				return fmt.Errorf("%w: %s", ErrNATSFileUnreadable, f)
			}
		}
	}

	return nil
}

// validateIdentity requires the field that addresses the server.
func (c *NATSConfig) validateIdentity() error {
	if c == nil {
		return nil
	}

	if strings.TrimSpace(c.URL) == "" {
		return ErrNATSURLEmpty
	}

	return nil
}

func (c *NATSConfig) validateBase() error {
	if err := c.validateShape(); err != nil {
		return err
	}

	return c.validateIdentity()
}

// resolveInstances validates every declared instance and returns ensured
// copies keyed by the normalized name. The caller's config is not mutated.
func (c *NATSConfig) resolveInstances() (resolved map[string]*NATSConfig, names []string, err error) {
	normalized, names, err := normalizeInstanceNames(c.Instances)
	if err != nil {
		return nil, nil, err
	}

	resolved = make(map[string]*NATSConfig, len(names))
	for _, name := range names {
		raw := normalized[name]
		if raw.Instances != nil || raw.DefaultInstance != "" || raw.NoDefault {
			return nil, nil, fmt.Errorf("%w: instances.%s", ErrInstanceNested, name)
		}

		instance := *raw
		instance.Instances = nil
		instance.DefaultInstance = ""
		instance.NoDefault = false
		instance.Ensure()

		if err := instance.validateBase(); err != nil {
			return nil, nil, fmt.Errorf("instances.%s: %w", name, err)
		}

		resolved[name] = &instance
	}

	return resolved, names, nil
}

// ShutdownNATS closes the shared connection and every named instance, then
// clears their references. Named instances close first so a default alias is
// closed exactly once.
func ShutdownNATS() error {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()

	instances := natsInstances.clear()

	mu.Lock()
	alias := natsDefaultInstance
	defaultConn := NATS
	NATS = nil
	Nats = nil
	natsDefaultInstance = ""
	mu.Unlock()

	if len(instances) == 0 && defaultConn == nil {
		return nil
	}

	for name, conn := range instances {
		closeNATSConnection(conn)
		metrics.SetInfraUp("nats:"+name, false)
	}

	if defaultNeedsClose(alias, instances) {
		closeNATSConnection(defaultConn)
	}

	metrics.SetInfraUp("nats", false)

	return nil
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
