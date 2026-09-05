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
// CA file.
var (
	ErrMQTTBrokerEmpty    = errors.New("infra: mqtt broker is empty")
	ErrMQTTConnectTimeout = errors.New("infra: mqtt connect timed out")
	ErrMQTTOptionInvalid  = errors.New("infra: mqtt option invalid")
	ErrMQTTCAUnreadable   = errors.New("infra: mqtt ca_file unreadable")
	ErrMQTTPasswordOrphan = errors.New("infra: mqtt password without username is discarded")
)

// InitMQTT is part of the public API.
func InitMQTT(cfg *MQTTConfig) (mqtt.Client, error) {
	if cfg == nil {
		return nil, nil
	}

	client, err := initMQTT(cfg)
	metrics.CountInfraInit("mqtt", err)

	return client, err
}

func initMQTT(cfg *MQTTConfig) (mqtt.Client, error) {
	if cfg == nil {
		return nil, nil
	}

	cfg.Ensure()
	if err := cfg.Validate(); err != nil {
		logger.Logger.Error(
			"MQTT config invalid",
			"error", err.Error(),
		)

		return nil, err
	}

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
					"MQTT CA cert unreadable",
					"ca_file", c.CAFile,
					"error", err.Error(),
				)

				return nil, fmt.Errorf("%w: %s", ErrMQTTCAUnreadable, c.CAFile)
			}

			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(pem) {
				logger.Logger.Error(
					"MQTT CA cert has no valid certificates",
					"ca_file", c.CAFile,
				)

				return nil, fmt.Errorf("%w: %s", ErrMQTTCAUnreadable, c.CAFile)
			}

			tlsCfg.RootCAs = pool
		}

		opts.SetTLSConfig(tlsCfg)
	}

	client := mqtt.NewClient(opts)

	token := client.Connect()
	if !token.WaitTimeout(time.Duration(c.ConnectTimeoutSec) * time.Second) {
		logger.Logger.Error(
			"MQTT connect timed out",
			"broker", redactDSN(c.Broker),
		)
		client.Disconnect(0)

		return nil, ErrMQTTConnectTimeout
	}

	if token.Error() != nil {
		logger.Logger.Error(
			"MQTT connect failed",
			"broker", redactDSN(c.Broker),
			"error", token.Error().Error(),
		)
		// paho auto-retries failed connects in the background; a
		// permanently bad broker must not keep that goroutine alive.
		client.Disconnect(0)

		return nil, token.Error()
	}

	mu.Lock()
	defer mu.Unlock()
	if MQTT != nil {
		// First-wins: keep the existing singleton and drop the duplicate
		// instead of leaking it.
		logger.Logger.Warn("MQTT already initialized, closing duplicate connection")
		client.Disconnect(250)

		return MQTT, nil
	}

	MQTT = client

	logger.Logger.InfoContext(
		context.Background(),
		"Init MQTT successful",
		"broker", redactDSN(c.Broker),
		"client_id", c.ClientID,
		"tls", c.EnableTLS,
	)

	return client, nil
}

// Ensure fills zero-valued fields with defaults and returns the receiver (nil-safe).
func (c *MQTTConfig) Ensure() *MQTTConfig {
	if c == nil {
		c = new(MQTTConfig)
	}

	// NOTE: ClientID default ("sicky::"+uuid) is generated inside InitMQTT
	// per call and intentionally stays out of Ensure to keep it deterministic.

	// Zero fills defaults; negative stays negative so Validate aborts.
	if c.KeepAliveSec == 0 {
		c.KeepAliveSec = DefaultMQTTKeepAliveSec
	}

	if c.ConnectTimeoutSec == 0 {
		c.ConnectTimeoutSec = DefaultMQTTConnectTimeoutSec
	}

	return c
}

// Validate rejects half-configured or illegal values.
func (c *MQTTConfig) Validate() error {
	if c == nil {
		return nil
	}

	if strings.TrimSpace(c.Broker) == "" {
		return ErrMQTTBrokerEmpty
	}

	if strings.TrimSpace(c.Username) == "" && c.Password != "" {
		return ErrMQTTPasswordOrphan
	}

	if c.KeepAliveSec < 0 || c.ConnectTimeoutSec < 0 {
		return ErrMQTTOptionInvalid
	}

	if strings.TrimSpace(c.CAFile) != "" {
		if _, err := os.Stat(c.CAFile); err != nil {
			return fmt.Errorf("%w: %s", ErrMQTTCAUnreadable, c.CAFile)
		}
	}

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
