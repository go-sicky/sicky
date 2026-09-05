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
 * @file elastic.go
 * @package infra
 * @author Dr.NP <np@herewe.tech>
 * @since 12/25/2025
 */

package infra

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v9"

	"github.com/go-sicky/sicky/logger"
)

// ElasticConfig is a infra component.
type ElasticConfig struct {
	Addresses []string `json:"addresses" mapstructure:"addresses" yaml:"addresses"`
	Username  string   `json:"username"  mapstructure:"username"  yaml:"username"`
	Password  string   `json:"password"  mapstructure:"password"  yaml:"password"`
	// CloudID targets Elastic Cloud; when set it takes precedence over
	// Addresses (following client semantics).
	CloudID string `json:"cloud_id" mapstructure:"cloud_id" yaml:"cloud_id"`
	// APIKey (base64) overrides Username/Password and ServiceToken.
	APIKey string `json:"api_key" mapstructure:"api_key" yaml:"api_key"`
	// ServiceToken overrides Username/Password.
	ServiceToken string `json:"service_token" mapstructure:"service_token" yaml:"service_token"`
	// CertificateFingerprint pins the server certificate (SHA256 hex).
	CertificateFingerprint string `json:"certificate_fingerprint" mapstructure:"certificate_fingerprint" yaml:"certificate_fingerprint"`
	// CACertFile loads a custom CA bundle (PEM file path).
	CACertFile string `json:"ca_cert_file" mapstructure:"ca_cert_file" yaml:"ca_cert_file"`
	// TimeoutSec bounds the startup Info() check. Seconds.
	// It does not bound regular API calls: pass a context deadline per
	// call (manager health uses a 2s context).
	TimeoutSec int `json:"timeout_sec" mapstructure:"timeout_sec" yaml:"timeout_sec"`
}

// ErrElasticNoEndpoint aborts startup: a non-nil ElasticConfig means
// "enable elasticsearch", and with neither Addresses nor CloudID the
// client would silently fall back to localhost:9200.
// ErrElasticAddressesEmpty is kept for compatibility; prefer
// ErrElasticNoEndpoint for new errors.Is checks.
// ErrElasticCACertUnreadable / ErrElasticTimeoutInvalid abort startup.
var (
	ErrElasticAddressesEmpty   = errors.New("infra: elastic addresses is empty")
	ErrElasticNoEndpoint       = errors.New("infra: elastic needs addresses or cloud_id")
	ErrElasticCACertUnreadable = errors.New("infra: elastic ca_cert_file unreadable")
	ErrElasticTimeoutInvalid   = errors.New("infra: elastic timeout_sec is negative")
	ErrElasticUnhealthy        = errors.New("infra: elastic cluster info check failed")
)

// DefaultElasticTimeoutSec bounds the startup Info() check.
const DefaultElasticTimeoutSec = 5

// Elastic is a shared infra value.
var Elastic *elasticsearch.Client

// InitElastic is part of the public API.
func InitElastic(cfg *ElasticConfig) (*elasticsearch.Client, error) {
	if cfg == nil {
		return nil, nil
	}

	cfg.Ensure()
	if err := cfg.Validate(); err != nil {
		logger.Logger.Error(
			"Elasticsearch config invalid",
			"error", err.Error(),
		)

		return nil, err
	}

	esCfg := elasticsearch.Config{
		Addresses:              cfg.Addresses,
		Username:               cfg.Username,
		Password:               cfg.Password,
		CloudID:                cfg.CloudID,
		APIKey:                 cfg.APIKey,
		ServiceToken:           cfg.ServiceToken,
		CertificateFingerprint: cfg.CertificateFingerprint,
	}

	if strings.TrimSpace(cfg.CACertFile) != "" {
		pem, err := os.ReadFile(cfg.CACertFile)
		if err != nil {
			logger.Logger.Error(
				"Elasticsearch CA cert unreadable",
				"ca_cert_file", cfg.CACertFile,
				"error", err.Error(),
			)

			return nil, fmt.Errorf("%w: %s", ErrElasticCACertUnreadable, cfg.CACertFile)
		}

		esCfg.CACert = pem
	}

	client, err := elasticsearch.NewClient(esCfg)
	if err != nil {
		logger.Logger.Error(
			"Elasticsearch initialize failed",
			"endpoint", cfg.endpoint(),
			"error", err.Error(),
		)

		return nil, err
	}

	// Fail-fast cluster check: without it a wrong address/auth would only
	// surface at first query time (and /health would lie).
	ictx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.TimeoutSec)*time.Second)
	defer cancel()

	if err := pingElastic(ictx, client); err != nil {
		logger.Logger.Error(
			"Elasticsearch cluster check failed",
			"endpoint", cfg.endpoint(),
			"error", err.Error(),
		)

		cctx, ccancel := context.WithTimeout(context.Background(), DefaultInitTimeoutSec*time.Second)
		defer ccancel()
		if cerr := client.Close(cctx); cerr != nil {
			logger.Logger.Error(
				"Elasticsearch close after failed check failed",
				"error", cerr.Error(),
			)
		}

		return nil, err
	}

	mu.Lock()
	defer mu.Unlock()
	if Elastic != nil {
		// First-wins: keep the existing singleton and drop the duplicate
		// instead of leaking its transport.
		logger.Logger.Warn("Elasticsearch already initialized, closing duplicate client")
		cctx, ccancel := context.WithTimeout(context.Background(), DefaultInitTimeoutSec*time.Second)
		defer ccancel()
		if cerr := client.Close(cctx); cerr != nil {
			logger.Logger.Error(
				"Elasticsearch duplicate close failed",
				"error", cerr.Error(),
			)
		}

		return Elastic, nil
	}

	Elastic = client

	logger.Logger.InfoContext(
		context.Background(),
		"Init Elasticsearch successful",
		"endpoint", cfg.endpoint(),
	)

	return client, nil
}

// endpoint renders a log-safe endpoint label: never credentials.
func (c *ElasticConfig) endpoint() string {
	if c == nil {
		return ""
	}

	if strings.TrimSpace(c.CloudID) != "" {
		return "cloud:***redacted***"
	}

	safe := make([]string, 0, len(c.Addresses))
	for _, a := range c.Addresses {
		safe = append(safe, redactDSN(strings.TrimSpace(a)))
	}

	return strings.Join(safe, ",")
}

// pingElastic runs a cluster Info check; shared by Init and /health.
func pingElastic(ctx context.Context, client *elasticsearch.Client) error {
	res, err := client.Info(client.Info.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrElasticUnhealthy, err)
	}

	defer func() { _ = res.Body.Close() }()

	if res.IsError() {
		return fmt.Errorf("%w: %s", ErrElasticUnhealthy, res.Status())
	}

	return nil
}

// PingElastic checks the shared singleton; nil means not configured.
func PingElastic(ctx context.Context) error {
	c := GetElastic()
	if c == nil {
		return nil
	}

	return pingElastic(ctx, c)
}

// Ensure fills zero-valued fields with defaults and returns the receiver (nil-safe).
func (c *ElasticConfig) Ensure() *ElasticConfig {
	if c == nil {
		c = new(ElasticConfig)
	}

	// Zero fills the default; negative stays negative so Validate aborts.
	if c.TimeoutSec == 0 {
		c.TimeoutSec = DefaultElasticTimeoutSec
	}

	return c
}

// Validate rejects half-configured or illegal values.
func (c *ElasticConfig) Validate() error {
	if c == nil {
		return nil
	}

	if strings.TrimSpace(c.CloudID) == "" {
		empty := true
		for _, a := range c.Addresses {
			if strings.TrimSpace(a) != "" {
				empty = false

				break
			}
		}

		if empty {
			return ErrElasticNoEndpoint
		}
	}

	if c.TimeoutSec < 0 {
		return ErrElasticTimeoutInvalid
	}

	if strings.TrimSpace(c.CACertFile) != "" {
		if _, err := os.Stat(c.CACertFile); err != nil {
			return fmt.Errorf("%w: %s", ErrElasticCACertUnreadable, c.CACertFile)
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
