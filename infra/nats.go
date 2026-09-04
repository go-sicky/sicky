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

	"github.com/go-sicky/sicky/logger"
	"github.com/nats-io/nats.go"
)

type NatsConfig struct {
	URL string `json:"url" yaml:"url" mapstructure:"url"`
	// Auth: set at most one of Token / Username+Password / CredsFile /
	// NkeyFile. Credentials are never logged.
	Token     string `json:"token" yaml:"token" mapstructure:"token"`
	Username  string `json:"username" yaml:"username" mapstructure:"username"`
	Password  string `json:"password" yaml:"password" mapstructure:"password"`
	CredsFile string `json:"creds_file" yaml:"creds_file" mapstructure:"creds_file"`
	NkeyFile  string `json:"nkey_file" yaml:"nkey_file" mapstructure:"nkey_file"`
	// EnableTLS wraps the connection in TLS 1.2+. RootCAFile optionally
	// pins a custom CA bundle (PEM path).
	EnableTLS  bool   `json:"enable_tls" yaml:"enable_tls" mapstructure:"enable_tls"`
	RootCAFile string `json:"root_ca_file" yaml:"root_ca_file" mapstructure:"root_ca_file"`
	// TimeoutSec bounds the initial connect. Seconds.
	TimeoutSec int `json:"timeout_sec" yaml:"timeout_sec" mapstructure:"timeout_sec"`
	// ReconnectWaitSec is the delay between reconnect attempts. Seconds.
	ReconnectWaitSec int `json:"reconnect_wait_sec" yaml:"reconnect_wait_sec" mapstructure:"reconnect_wait_sec"`
	// MaxReconnects caps reconnect attempts. 0 fills 60 (the library
	// default); -1 retries forever (opt-in: a permanently down server
	// then never surfaces as a startup error); other negatives abort.
	MaxReconnects int `json:"max_reconnects" yaml:"max_reconnects" mapstructure:"max_reconnects"`
}

// Defaults for NATS timing.
const (
	DefaultNatsTimeoutSec       = 5
	DefaultNatsReconnectWaitSec = 2
	DefaultNatsMaxReconnects    = 60
)

// ErrNatsURLEmpty aborts startup: a non-nil NatsConfig means "enable nats",
// and an empty URL would otherwise silently connect to the client default
// (nats://127.0.0.1:4222). ErrNatsFileUnreadable aborts on missing auth/CA
// files. ErrNatsOptionInvalid aborts on negative timing.
// ErrNatsAuthConflict aborts when more than one auth method is set.
// ErrNatsCAWithoutTLS aborts when RootCAFile is set without EnableTLS.
var (
	ErrNatsURLEmpty       = errors.New("infra: nats url is empty")
	ErrNatsFileUnreadable = errors.New("infra: nats file unreadable")
	ErrNatsOptionInvalid  = errors.New("infra: nats option invalid")
	ErrNatsAuthConflict   = errors.New("infra: nats at most one of token/username+password/creds_file/nkey_file")
	ErrNatsCAWithoutTLS   = errors.New("infra: nats root_ca_file requires enable_tls")
)

var Nats *nats.Conn

func InitNats(cfg *NatsConfig) (*nats.Conn, error) {
	if cfg == nil {
		return nil, nil
	}

	cfg.Ensure()
	if err := cfg.Validate(); err != nil {
		logger.Logger.Error(
			"Init NATS config invalid",
			"error", err.Error(),
		)

		return nil, err
	}

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
				"Init NATS nkey failed",
				"nkey_file", cfg.NkeyFile,
				"error", err.Error(),
			)

			return nil, fmt.Errorf("%w: %s", ErrNatsFileUnreadable, cfg.NkeyFile)
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
			"Init NATS failed",
			"url", redactDSN(cfg.URL),
			"error", err.Error(),
		)

		return nil, err
	}

	mu.Lock()
	defer mu.Unlock()
	if Nats != nil {
		// First-wins: keep the existing singleton and drop the duplicate
		// instead of leaking it.
		logger.Logger.Warn("NATS already initialized, closing duplicate connection")
		nc.Close()

		return Nats, nil
	}
	Nats = nc

	logger.Logger.InfoContext(
		context.Background(),
		"Init NATS successful",
		"url", redactDSN(cfg.URL),
	)

	return nc, nil
}

func (c *NatsConfig) Ensure() *NatsConfig {
	if c == nil {
		c = new(NatsConfig)
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

func (c *NatsConfig) Validate() error {
	if c == nil {
		return nil
	}

	if strings.TrimSpace(c.URL) == "" {
		return ErrNatsURLEmpty
	}

	if c.TimeoutSec < 0 || c.ReconnectWaitSec < 0 || c.MaxReconnects < -1 {
		return ErrNatsOptionInvalid
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
		return ErrNatsAuthConflict
	}

	if strings.TrimSpace(c.RootCAFile) != "" && !c.EnableTLS {
		return ErrNatsCAWithoutTLS
	}

	for _, f := range []string{c.CredsFile, c.NkeyFile, c.RootCAFile} {
		if strings.TrimSpace(f) != "" {
			if _, err := os.Stat(f); err != nil {
				return fmt.Errorf("%w: %s", ErrNatsFileUnreadable, f)
			}
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
