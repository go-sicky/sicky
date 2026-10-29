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
 * @file mqtt.go
 * @package infra
 * @author Dr.NP <np@herewe.tech>
 * @since 12/21/2025
 */

package infra

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"

	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/metrics"
)

// MQTT is a shared infra value.
var MQTT mqtt.Client

// MQTTConfig is a infra component.
type MQTTConfig struct {
	Broker   string `json:"broker"    mapstructure:"broker"    yaml:"broker"`
	ClientID string `json:"client_id" mapstructure:"client_id" yaml:"client_id"`
	Username string `json:"username"  mapstructure:"username"  yaml:"username"`
	Password string `json:"password"  mapstructure:"password"  yaml:"password"`
	// EnableTLS wraps the connection in TLS 1.2+. CAFile optionally pins
	// a custom CA bundle (PEM path).
	EnableTLS bool   `json:"enable_tls" mapstructure:"enable_tls" yaml:"enable_tls"`
	CAFile    string `json:"ca_file"    mapstructure:"ca_file"    yaml:"ca_file"`
	// KeepAliveSec is the paho keepalive interval. Seconds.
	KeepAliveSec int `json:"keep_alive_sec" mapstructure:"keep_alive_sec" yaml:"keep_alive_sec"`
	// ConnectTimeoutSec bounds the initial connect. Seconds.
	ConnectTimeoutSec int `json:"connect_timeout_sec" mapstructure:"connect_timeout_sec" yaml:"connect_timeout_sec"`
	// CleanSession toggles a clean session on connect (default true).
	CleanSession *bool `json:"clean_session" mapstructure:"clean_session" yaml:"clean_session"`
	// DefaultInstance names the instance that also serves as the shared
	// singleton when the top-level fields do not form a complete config.
	DefaultInstance string `json:"default_instance" mapstructure:"default_instance" yaml:"default_instance"`
	// NoDefault suppresses the shared singleton on a fresh init; only Instances are created.
	NoDefault bool `json:"no_default" mapstructure:"no_default" yaml:"no_default"`
	// Instances are complete MQTT configurations. They do not inherit fields
	// from the block above.
	Instances map[string]*MQTTConfig `json:"instances" mapstructure:"instances" yaml:"instances"`
}

// Defaults for MQTT timing in seconds.
const (
	DefaultMQTTKeepAliveSec      = 30
	DefaultMQTTConnectTimeoutSec = 5
)

// ErrMQTTBrokerEmpty aborts startup: a non-nil MQTTConfig means "enable
// mqtt", and an empty broker URL would otherwise produce an obscure error
// from the paho client. ErrMQTTConnectTimeout aborts startup when the
// broker does not answer within the connect deadline.
// ErrMQTTOptionInvalid aborts startup on negative timing or an unreadable
// CA file. ErrMQTTClientIDDuplicate aborts before anything connects: two
// clients sharing a client_id fight over one broker session.
var (
	ErrMQTTBrokerEmpty       = errors.New("infra: mqtt broker is empty")
	ErrMQTTConnectTimeout    = errors.New("infra: mqtt connect timed out")
	ErrMQTTOptionInvalid     = errors.New("infra: mqtt option invalid")
	ErrMQTTCAUnreadable      = errors.New("infra: mqtt ca_file unreadable")
	ErrMQTTPasswordOrphan    = errors.New("infra: mqtt password without username is discarded")
	ErrMQTTCAWithoutTLS      = errors.New("infra: mqtt ca_file requires enable_tls")
	ErrMQTTClientIDDuplicate = errors.New("infra: mqtt client_id is duplicated across instances")
)

// InitMQTT is part of the public API.
func InitMQTT(cfg *MQTTConfig) (mqtt.Client, error) {
	if cfg == nil {
		return nil, nil
	}

	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()

	client, err := initMQTT(cfg)
	metrics.CountInfraInit("mqtt", err)
	if err == nil && client == nil {
		// no_default: the component initialized, but there is no shared
		// singleton to report as up.
		metrics.SetInfraUp("mqtt", false)
	}

	return client, err
}

func initMQTT(cfg *MQTTConfig) (mqtt.Client, error) {
	if cfg == nil {
		return nil, nil
	}

	cfg.Ensure()
	if err := cfg.Validate(); err != nil {
		logger.Logger.Error(
			"mqtt config invalid",
			"error", err.Error(),
		)

		return nil, err
	}

	instances, names, err := cfg.resolveInstances()
	if err != nil {
		logger.Logger.Error(
			"init mqtt instances invalid",
			"error", err.Error(),
		)

		return nil, err
	}

	identityErr := cfg.validateIdentity()
	selection, err := resolveDefaultSelection(cfg.NoDefault, cfg.DefaultInstance, names, identityErr == nil, identityErr)
	if err != nil {
		logger.Logger.Error(
			"init mqtt default selection invalid",
			"error", err.Error(),
		)

		return nil, err
	}

	if selection.Ignored != "" {
		logger.Logger.Warn(
			"mqtt default_instance ignored because the top-level config is complete",
			"instance", selection.Ignored,
		)
	}

	if cfg.NoDefault {
		logger.Logger.Info("mqtt no_default is set; this block does not create a shared instance")
	}

	if !selection.UseTopLevel && selection.Instance != "" && cfg.DefaultInstance == "" {
		if len(names) == 1 {
			logger.Logger.Info(
				"mqtt using the only named instance as the default",
				"instance", selection.Instance,
			)
		} else {
			logger.Logger.Warn(
				"mqtt no default_instance configured; using the first instance in sorted order",
				"instance", selection.Instance,
			)
		}
	}

	// Reject a duplicated client_id before anything dials: paho would accept
	// both connections and the broker would silently close one of them.
	if err := cfg.validateClientIDs(selection, instances, names); err != nil {
		logger.Logger.Error(
			"init mqtt client_id conflict",
			"error", err.Error(),
		)

		return nil, err
	}

	// Build first, register second: a failure never leaves a half-installed
	// component behind. Existing first-wins entries are not rebuilt.
	mu.RLock()
	existingDefault := MQTT
	mu.RUnlock()

	var topLevel mqttInstance

	if selection.UseTopLevel && existingDefault == nil {
		topLevel, err = newMQTTClient(cfg, "mqtt")
		if err != nil {
			logger.Logger.Error(
				"init mqtt failed",
				"instance", "mqtt",
				"broker", redactDSN(cfg.Broker),
				"error", err.Error(),
			)

			return nil, err
		}
	}

	built := make(map[string]mqttInstance, len(names))
	for _, name := range names {
		if _, ok := mqttInstances.get(name); ok {
			logger.Logger.Warn("mqtt instance already initialized, keeping existing client", "instance", name)

			continue
		}

		client, berr := newMQTTClient(instances[name], "mqtt:"+name)
		if berr != nil {
			metrics.CountInfraInit("mqtt:"+name, berr)
			logger.Logger.Error(
				"init mqtt instance failed",
				"instance", name,
				"error", berr.Error(),
			)

			rollbackMQTTClients(topLevel, built)

			return nil, fmt.Errorf("mqtt instance %s: %w", name, berr)
		}

		metrics.CountInfraInit("mqtt:"+name, nil)
		built[name] = client
	}

	// Register under the lock, release before anything that can block. This
	// mu is a single package-global write lock guarding all ten
	// singletons, so holding it across a duplicate's teardown (a 5s
	// Disconnect, a Close that flushes, paho's quiesce sleep) stalls
	// every Get* reader and every manager health probe for the duration.
	if topLevel.client != nil {
		stored := false

		mu.Lock()
		if MQTT == nil {
			MQTT = topLevel.client
			mqttDefaultInstance = ""
			stored = true
		}

		mu.Unlock()

		if !stored {
			logger.Logger.Warn("mqtt already initialized, closing duplicate connection")
			closeMQTTClient(topLevel.client)
		}
	}

	for _, name := range names {
		instance, ok := built[name]
		if !ok {
			continue
		}

		if _, stored := mqttInstances.putFirst(name, instance); !stored {
			logger.Logger.Warn("mqtt instance already initialized, closing duplicate connection", "instance", name)
			closeMQTTClient(instance.client)

			continue
		}

		logger.Logger.InfoContext(
			context.Background(),
			"mqtt instance initialized",
			"instance", name,
			"broker", redactDSN(instances[name].Broker),
			"tls", instances[name].EnableTLS,
		)
	}

	// A named default is installed only when the shared singleton is still
	// free; first-wins keeps an earlier top-level or alias default.
	if !selection.UseTopLevel && selection.Instance != "" {
		if instance, ok := mqttInstances.get(selection.Instance); ok && instance.client != nil {
			mu.Lock()
			if MQTT == nil {
				MQTT = instance.client
				mqttDefaultInstance = selection.Instance
			}
			mu.Unlock()
		}
	}

	mu.RLock()
	client := MQTT
	mu.RUnlock()

	if topLevel.client != nil {
		logger.Logger.InfoContext(
			context.Background(),
			"init MQTT successful",
			"instance", "mqtt",
			"broker", redactDSN(cfg.Broker),
			"tls", cfg.EnableTLS,
		)
	}

	return client, nil
}

// validateClientIDs rejects an explicit client_id used by more than one of the
// clients this config block describes. The top-level config is part of the set
// only when it is the selected default, and empty ids are generated per client
// so they can never collide.
func (c *MQTTConfig) validateClientIDs(selection defaultSelection, instances map[string]*MQTTConfig, names []string) error {
	seen := make(map[string]string, len(names)+1)

	check := func(id, owner string) error {
		if strings.TrimSpace(id) == "" {
			return nil
		}

		if previous, ok := seen[id]; ok {
			return fmt.Errorf("%w: %q is used by %s and %s", ErrMQTTClientIDDuplicate, id, previous, owner)
		}

		seen[id] = owner

		return nil
	}

	// Live registered clients still occupy their broker session even though
	// this call will not rebuild them. Seed their effective ids so a new block
	// cannot silently reuse one.
	for name, instance := range mqttInstances.snapshot() {
		if err := check(instance.clientID, "instances."+name); err != nil {
			return err
		}
	}

	if selection.UseTopLevel {
		mu.RLock()
		existingDefault := MQTT
		mu.RUnlock()

		if existingDefault == nil {
			if err := check(c.ClientID, "the top-level config"); err != nil {
				return err
			}
		}
	}

	for _, name := range names {
		// An instance that is already registered will not be rebuilt, so a
		// client_id collision with the new config is not a real conflict.
		if _, ok := mqttInstances.get(name); ok {
			continue
		}

		if err := check(instances[name].ClientID, "instances."+name); err != nil {
			return err
		}
	}

	return nil
}

// newMQTTClient connects one client from cfg. It touches no package-level
// state, so a build that fails leaves nothing registered. label names the
// instance in logs: "mqtt" for the shared singleton, "mqtt:<name>" for a named
// instance.
func newMQTTClient(cfg *MQTTConfig, label string) (mqttInstance, error) {
	// Copy before filling the default ClientID: Init must not mutate the
	// caller's config (repeated calls would otherwise share/overwrite IDs).
	c := *cfg
	if strings.TrimSpace(c.ClientID) == "" {
		c.ClientID = "sicky::" + uuid.NewString()
	}

	opts := mqtt.NewClientOptions().AddBroker(c.Broker)
	opts.SetClientID(c.ClientID)
	// Username is safe to log; password never is.
	if strings.TrimSpace(c.Username) != "" {
		opts.SetUsername(c.Username)
		if c.Password != "" {
			opts.SetPassword(c.Password)
		}
	}

	opts.SetKeepAlive(time.Duration(c.KeepAliveSec) * time.Second)
	opts.SetConnectTimeout(time.Duration(c.ConnectTimeoutSec) * time.Second)
	if c.CleanSession != nil {
		opts.SetCleanSession(*c.CleanSession)
	}

	if c.EnableTLS {
		tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
		if strings.TrimSpace(c.CAFile) != "" {
			pem, err := os.ReadFile(c.CAFile)
			if err != nil {
				logger.Logger.Error(
					"mqtt CA cert unreadable",
					"instance", label,
					"ca_file", c.CAFile,
					"error", err.Error(),
				)

				return mqttInstance{}, fmt.Errorf("%w: %s", ErrMQTTCAUnreadable, c.CAFile)
			}

			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(pem) {
				logger.Logger.Error(
					"mqtt CA cert has no valid certificates",
					"instance", label,
					"ca_file", c.CAFile,
				)

				return mqttInstance{}, fmt.Errorf("%w: %s", ErrMQTTCAUnreadable, c.CAFile)
			}

			tlsCfg.RootCAs = pool
		}

		opts.SetTLSConfig(tlsCfg)
	}

	client := mqtt.NewClient(opts)

	token := client.Connect()
	if !token.WaitTimeout(time.Duration(c.ConnectTimeoutSec) * time.Second) {
		logger.Logger.Error(
			"mqtt connect timed out",
			"instance", label,
			"broker", redactDSN(c.Broker),
		)
		client.Disconnect(0)

		return mqttInstance{}, ErrMQTTConnectTimeout
	}

	if token.Error() != nil {
		logger.Logger.Error(
			"mqtt connect failed",
			"instance", label,
			"broker", redactDSN(c.Broker),
			"error", token.Error().Error(),
		)
		// paho auto-retries failed connects in the background; a
		// permanently bad broker must not keep that goroutine alive.
		client.Disconnect(0)

		return mqttInstance{}, token.Error()
	}

	return mqttInstance{client: client, clientID: c.ClientID}, nil
}

// closeMQTTClient disconnects one client. Disconnect has no return value, so
// there is no failure to log; 0 skips the quiesce wait because the caller is
// already on an error or duplicate path.
func closeMQTTClient(client mqtt.Client) {
	if client == nil {
		return
	}

	client.Disconnect(0)
}

// rollbackMQTTClients disconnects everything one init call built but will not
// register, so a failed build leaves no connections or "up" series behind.
func rollbackMQTTClients(defaultClient mqttInstance, built map[string]mqttInstance) {
	closeMQTTClient(defaultClient.client)

	for name, instance := range built {
		closeMQTTClient(instance.client)
		metrics.SetInfraUp("mqtt:"+name, false)
	}
}

// Ensure fills zero-valued fields with defaults and returns the receiver (nil-safe).
func (c *MQTTConfig) Ensure() *MQTTConfig {
	if c == nil {
		c = new(MQTTConfig)
	}

	// NOTE: ClientID default ("sicky::"+uuid) is generated per client inside
	// newMQTTClient and intentionally stays out of Ensure to keep it
	// deterministic and out of the caller's config.

	// Zero fills defaults; negative stays negative so Validate aborts.
	if c.KeepAliveSec == 0 {
		c.KeepAliveSec = DefaultMQTTKeepAliveSec
	}

	if c.ConnectTimeoutSec == 0 {
		c.ConnectTimeoutSec = DefaultMQTTConnectTimeoutSec
	}

	return c
}

// Validate rejects half-configured or illegal values and resolves the named
// instances (without mutating the caller's config).
func (c *MQTTConfig) Validate() error {
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
func (c *MQTTConfig) validateShape() error {
	if c == nil {
		return nil
	}

	if strings.TrimSpace(c.Username) == "" && c.Password != "" {
		return ErrMQTTPasswordOrphan
	}

	if c.KeepAliveSec < 0 || c.ConnectTimeoutSec < 0 {
		return ErrMQTTOptionInvalid
	}

	if strings.TrimSpace(c.CAFile) != "" {
		// newMQTTClient only reads the CA inside its `if c.EnableTLS` branch,
		// so a CA pinned without TLS is stat-checked and then silently
		// discarded: an operator who pins a private CA and forgets
		// enable_tls gets a plaintext connection that looks configured. nats
		// already rejects the same combination (ErrNATSCAWithoutTLS); this
		// closes the asymmetry rather than leaving one backend quietly
		// forgiving.
		if !c.EnableTLS {
			return ErrMQTTCAWithoutTLS
		}

		if _, err := os.Stat(c.CAFile); err != nil {
			return fmt.Errorf("%w: %s", ErrMQTTCAUnreadable, c.CAFile)
		}
	}

	return nil
}

// validateIdentity requires the field that addresses the broker.
func (c *MQTTConfig) validateIdentity() error {
	if c == nil {
		return nil
	}

	if strings.TrimSpace(c.Broker) == "" {
		return ErrMQTTBrokerEmpty
	}

	return nil
}

func (c *MQTTConfig) validateBase() error {
	if err := c.validateShape(); err != nil {
		return err
	}

	return c.validateIdentity()
}

// resolveInstances validates every declared instance and returns ensured
// copies keyed by the normalized name. The caller's config is not mutated.
func (c *MQTTConfig) resolveInstances() (resolved map[string]*MQTTConfig, names []string, err error) {
	normalized, names, err := normalizeInstanceNames(c.Instances)
	if err != nil {
		return nil, nil, err
	}

	resolved = make(map[string]*MQTTConfig, len(names))
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

// ShutdownMQTT disconnects the shared client and every named instance, then
// clears their references. Named instances are detached before closing so a
// reader cannot observe a closed handle through the registry; a default alias
// is disconnected exactly once.
func ShutdownMQTT() error {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()

	instances := mqttInstances.clear()

	mu.Lock()
	alias := mqttDefaultInstance
	defaultClient := MQTT
	MQTT = nil
	mqttDefaultInstance = ""
	mu.Unlock()

	if len(instances) == 0 && defaultClient == nil {
		return nil
	}

	for name, instance := range instances {
		if instance.client != nil {
			instance.client.Disconnect(250)
		}

		metrics.SetInfraUp("mqtt:"+name, false)
	}

	if defaultNeedsClose(alias, instances) && defaultClient != nil {
		defaultClient.Disconnect(250)
	}

	metrics.SetInfraUp("mqtt", false)

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
