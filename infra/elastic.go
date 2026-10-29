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
	"github.com/go-sicky/sicky/metrics"
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
	// DefaultInstance names the instance that also serves as the shared
	// singleton when the top-level fields do not form a complete config.
	DefaultInstance string `json:"default_instance" mapstructure:"default_instance" yaml:"default_instance"`
	// NoDefault suppresses the shared singleton on a fresh init; only Instances are created.
	NoDefault bool `json:"no_default" mapstructure:"no_default" yaml:"no_default"`
	// Instances are complete Elasticsearch configurations. They do not
	// inherit fields from the block above.
	Instances map[string]*ElasticConfig `json:"instances" mapstructure:"instances" yaml:"instances"`
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

	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()

	client, err := initElastic(cfg)
	metrics.CountInfraInit("elastic", err)
	if err == nil && client == nil {
		// no_default: the component initialized, but there is no shared
		// singleton to report as up.
		metrics.SetInfraUp("elastic", false)
	}

	return client, err
}

func initElastic(cfg *ElasticConfig) (*elasticsearch.Client, error) {
	if cfg == nil {
		return nil, nil
	}

	cfg.Ensure()
	if err := cfg.Validate(); err != nil {
		logger.Logger.Error(
			"elasticsearch config invalid",
			"error", err.Error(),
		)

		return nil, err
	}

	instances, names, err := cfg.resolveInstances()
	if err != nil {
		logger.Logger.Error(
			"init elasticsearch instances invalid",
			"error", err.Error(),
		)

		return nil, err
	}

	identityErr := cfg.validateIdentity()
	selection, err := resolveDefaultSelection(cfg.NoDefault, cfg.DefaultInstance, names, identityErr == nil, identityErr)
	if err != nil {
		logger.Logger.Error(
			"init elasticsearch default selection invalid",
			"error", err.Error(),
		)

		return nil, err
	}

	if selection.Ignored != "" {
		logger.Logger.Warn(
			"elasticsearch default_instance ignored because the top-level config is complete",
			"instance", selection.Ignored,
		)
	}

	if cfg.NoDefault {
		logger.Logger.Info("elasticsearch no_default is set; this block does not create a shared instance")
	}

	if !selection.UseTopLevel && selection.Instance != "" && cfg.DefaultInstance == "" {
		if len(names) == 1 {
			logger.Logger.Info(
				"elasticsearch using the only named instance as the default",
				"instance", selection.Instance,
			)
		} else {
			logger.Logger.Warn(
				"elasticsearch no default_instance configured; using the first instance in sorted order",
				"instance", selection.Instance,
			)
		}
	}

	// Build first, register second: a failure never leaves a half-installed
	// component behind. Existing first-wins entries are not rebuilt.
	mu.RLock()
	existingDefault := Elastic
	mu.RUnlock()

	var topLevel *elasticsearch.Client

	if selection.UseTopLevel && existingDefault == nil {
		topLevel, err = newElasticClient(cfg, "elastic")
		if err != nil {
			logger.Logger.Error(
				"init elasticsearch failed",
				"endpoint", cfg.endpoint(),
				"error", err.Error(),
			)

			return nil, err
		}
	}

	built := make(map[string]*elasticsearch.Client, len(names))
	for _, name := range names {
		if _, ok := elasticInstances.get(name); ok {
			logger.Logger.Warn("elasticsearch instance already initialized, keeping existing client", "instance", name)

			continue
		}

		client, berr := newElasticClient(instances[name], "elastic:"+name)
		if berr != nil {
			metrics.CountInfraInit("elastic:"+name, berr)
			logger.Logger.Error(
				"init elasticsearch instance failed",
				"instance", name,
				"error", berr.Error(),
			)

			rollbackElasticClients(topLevel, built)

			return nil, fmt.Errorf("elastic instance %s: %w", name, berr)
		}

		metrics.CountInfraInit("elastic:"+name, nil)
		built[name] = client
	}

	// Register under the lock, release before anything that can block. This
	// mu is a single package-global write lock guarding all ten
	// singletons, so holding it across a duplicate's teardown (a 5s
	// Disconnect, a Close that flushes, paho's quiesce sleep) stalls
	// every Get* reader and every manager health probe for the duration.
	if topLevel != nil {
		stored := false

		mu.Lock()
		if Elastic == nil {
			Elastic = topLevel
			elasticDefaultInstance = ""
			stored = true
		}

		mu.Unlock()

		if !stored {
			logger.Logger.Warn("elasticsearch already initialized, closing duplicate client")
			closeElasticClient(topLevel, "elastic")
		}
	}

	for _, name := range names {
		client, ok := built[name]
		if !ok {
			continue
		}

		if _, stored := elasticInstances.putFirst(name, client); !stored {
			logger.Logger.Warn("elasticsearch instance already initialized, closing duplicate client", "instance", name)
			closeElasticClient(client, "elastic:"+name)

			continue
		}

		logger.Logger.InfoContext(
			context.Background(),
			"elasticsearch instance initialized",
			"instance", name,
			"endpoint", instances[name].endpoint(),
		)
	}

	// A named default is installed only when the shared singleton is still
	// free; first-wins keeps an earlier top-level or alias default.
	if !selection.UseTopLevel && selection.Instance != "" {
		if client, ok := elasticInstances.get(selection.Instance); ok && client != nil {
			mu.Lock()
			if Elastic == nil {
				Elastic = client
				elasticDefaultInstance = selection.Instance
			}
			mu.Unlock()
		}
	}

	mu.RLock()
	client := Elastic
	mu.RUnlock()

	if topLevel != nil {
		logger.Logger.InfoContext(
			context.Background(),
			"init Elasticsearch successful",
			"endpoint", cfg.endpoint(),
		)
	}

	return client, nil
}

// newElasticClient builds one client and runs the startup cluster check. It
// touches no package-level state, so a build that fails leaves nothing
// registered. label names the instance in logs: "elastic" for the shared
// singleton, "elastic:<name>" for a named instance.
func newElasticClient(cfg *ElasticConfig, label string) (*elasticsearch.Client, error) {
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
				"elasticsearch CA cert unreadable",
				"instance", label,
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
			"elasticsearch initialize failed",
			"instance", label,
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
			"elasticsearch cluster check failed",
			"instance", label,
			"endpoint", cfg.endpoint(),
			"error", err.Error(),
		)

		closeElasticClient(client, label)

		return nil, err
	}

	return client, nil
}

// closeElasticClient closes one client and logs a failure instead of
// returning it: the caller is already on an error or duplicate path.
func closeElasticClient(client *elasticsearch.Client, label string) {
	if client == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), DefaultInitTimeoutSec*time.Second)
	defer cancel()

	if cerr := client.Close(ctx); cerr != nil {
		logger.Logger.Error(
			"elasticsearch close failed",
			"instance", label,
			"error", cerr.Error(),
		)
	}
}

// rollbackElasticClients closes everything one init call built but will not
// register, so a failed build leaves no transports or "up" series behind.
func rollbackElasticClients(defaultClient *elasticsearch.Client, built map[string]*elasticsearch.Client) {
	closeElasticClient(defaultClient, "elastic")

	for name, client := range built {
		closeElasticClient(client, "elastic:"+name)
		metrics.SetInfraUp("elastic:"+name, false)
	}
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
func PingElastic(ctx context.Context, name ...string) error {
	if n := firstInstanceName(name); n != "" {
		client, ok := elasticInstances.get(n)
		if !ok {
			return fmt.Errorf("%w: %s", ErrInstanceNotFound, n)
		}

		if client == nil {
			return nil
		}

		start := time.Now()
		err := pingElastic(ctx, client)
		metrics.ObserveInfraOp("elastic:"+n, "ping", start, err)

		return err
	}

	c := GetElastic()
	if c == nil {
		return nil
	}

	start := time.Now()
	err := pingElastic(ctx, c)
	metrics.ObserveInfraOp("elastic", "ping", start, err)

	return err
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

// Validate rejects half-configured or illegal values and resolves the named
// instances (without mutating the caller's config). A missing top-level
// endpoint is only an error when no instance can serve as the default, so the
// identity verdict is handed to resolveDefaultSelection rather than returned
// immediately.
func (c *ElasticConfig) Validate() error {
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
func (c *ElasticConfig) validateShape() error {
	if c == nil {
		return nil
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

// validateIdentity requires an endpoint: addresses or a CloudID.
func (c *ElasticConfig) validateIdentity() error {
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

	return nil
}

func (c *ElasticConfig) validateBase() error {
	if err := c.validateShape(); err != nil {
		return err
	}

	return c.validateIdentity()
}

// resolveInstances validates every declared instance and returns ensured
// copies keyed by the normalized name. The caller's config is not mutated.
func (c *ElasticConfig) resolveInstances() (resolved map[string]*ElasticConfig, names []string, err error) {
	normalized, names, err := normalizeInstanceNames(c.Instances)
	if err != nil {
		return nil, nil, err
	}

	resolved = make(map[string]*ElasticConfig, len(names))
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

// ShutdownElastic closes the shared elastic client and every named instance,
// then clears their references. Named instances are detached before closing
// so a reader cannot observe a closed handle through the registry; a default
// alias is closed exactly once. One 5s deadline bounds the whole component.
func ShutdownElastic() error {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()

	instances := elasticInstances.clear()

	mu.Lock()
	alias := elasticDefaultInstance
	defaultClient := Elastic
	Elastic = nil
	elasticDefaultInstance = ""
	mu.Unlock()

	if len(instances) == 0 && defaultClient == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var errs []error

	for name, client := range instances {
		if client == nil {
			continue
		}

		if cerr := client.Close(ctx); cerr != nil {
			errs = append(errs, fmt.Errorf("elastic instance %s: %w", name, cerr))
		}

		metrics.SetInfraUp("elastic:"+name, false)
	}

	if defaultNeedsClose(alias, instances) && defaultClient != nil {
		if cerr := defaultClient.Close(ctx); cerr != nil {
			errs = append(errs, fmt.Errorf("elastic: %w", cerr))
		}
	}

	metrics.SetInfraUp("elastic", false)

	return errors.Join(errs...)
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
