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
 * @file s3.go
 * @package infra
 * @author Dr.NP <np@herewe.tech>
 * @since 12/20/2025
 */

package infra

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/metrics"
)

// S3Config is a infra component.
type S3Config struct {
	// Region is required: without it every S3 call fails at request time,
	// so a missing region aborts startup instead of failing late.
	Region string `json:"region" mapstructure:"region" yaml:"region"`
	// Endpoint overrides the service endpoint (MinIO, LocalStack, ...).
	Endpoint string `json:"endpoint" mapstructure:"endpoint" yaml:"endpoint"`
	// Static credentials. Empty means fall back to the default chain
	// (env, shared config, IMDS).
	AccessKey    string `json:"access_key"    mapstructure:"access_key"    yaml:"access_key"`
	SecretKey    string `json:"secret_key"    mapstructure:"secret_key"    yaml:"secret_key"`
	SessionToken string `json:"session_token" mapstructure:"session_token" yaml:"session_token"`
	// Bucket is optional: it may be selected at call time instead.
	Bucket string `json:"bucket" mapstructure:"bucket" yaml:"bucket"`
	// UsePathStyle forces path-style addressing (required by MinIO).
	UsePathStyle bool `json:"use_path_style" mapstructure:"use_path_style" yaml:"use_path_style"`
	// Timeout bounds LoadDefaultConfig (IMDS lookups can stall for
	// seconds outside AWS). Seconds, defaults to DefaultInitTimeoutSec.
	Timeout int `json:"timeout" mapstructure:"timeout" yaml:"timeout"`
	// RequestTimeoutSec bounds whole S3 API calls (headers + body) via the
	// HTTP client. 0 keeps the SDK default (unbounded); negative aborts.
	// Prefer a context deadline per call when you need tighter control.
	RequestTimeoutSec int `json:"request_timeout_sec" mapstructure:"request_timeout_sec" yaml:"request_timeout_sec"`
	// DefaultInstance names the instance that also serves as the shared
	// singleton when the top-level fields do not form a complete config.
	DefaultInstance string `json:"default_instance" mapstructure:"default_instance" yaml:"default_instance"`
	// NoDefault suppresses the shared singleton on a fresh init; only Instances are created.
	NoDefault bool `json:"no_default" mapstructure:"no_default" yaml:"no_default"`
	// Instances are complete S3 configurations. They do not inherit fields
	// from the block above.
	Instances map[string]*S3Config `json:"instances" mapstructure:"instances" yaml:"instances"`
}

// s3Bucket mirrors the configured bucket for PingS3. Guarded by mu.
var s3Bucket string

// GetS3Bucket returns the bucket configured for the shared S3 client, or for
// the named instance when a name is given. It returns "" when the component or
// instance is not configured, or when that instance has no bucket. An empty
// name returns the shared singleton; "default" is not a reserved alias — only
// an instance literally named "default".
func GetS3Bucket(name ...string) string {
	if n := firstInstanceName(name); n != "" {
		instance, ok := s3Instances.get(n)
		if !ok {
			return ""
		}

		return instance.bucket
	}

	mu.RLock()
	defer mu.RUnlock()

	return s3Bucket
}

// PingS3 checks the shared singleton for /health; nil means not configured
// (healthy-by-absence, following the other local checks). With a name it
// checks that named instance instead.
func PingS3(ctx context.Context, name ...string) error {
	if n := firstInstanceName(name); n != "" {
		instance, ok := s3Instances.get(n)
		if !ok {
			return fmt.Errorf("%w: %s", ErrInstanceNotFound, n)
		}

		if instance.client == nil || instance.bucket == "" {
			return nil
		}

		return pingS3Bucket(ctx, instance.client, instance.bucket, "s3:"+n)
	}

	mu.RLock()
	client := S3
	bucket := s3Bucket
	mu.RUnlock()

	if client == nil || bucket == "" {
		return nil
	}

	return pingS3Bucket(ctx, client, bucket, "s3")
}

func pingS3Bucket(ctx context.Context, client *s3.Client, bucket, label string) error {
	start := time.Now()
	_, err := client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &bucket})
	metrics.ObserveInfraOp(label, "head_bucket", start, err)
	if err != nil {
		return fmt.Errorf("infra: s3 head bucket (bucket %s): %w", bucket, err)
	}

	return nil
}

// ErrS3RegionEmpty aborts startup: a non-nil S3Config means "enable s3".
// ErrS3TimeoutInvalid aborts startup on a negative load timeout.
var (
	ErrS3RegionEmpty    = errors.New("infra: s3 region is empty")
	ErrS3SecretEmpty    = errors.New("infra: s3 secret_key is empty")
	ErrS3TimeoutInvalid = errors.New("infra: s3 timeout is negative")
)

// S3 is a shared infra value.
var S3 *s3.Client

// InitS3 is part of the public API.
func InitS3(cfg *S3Config) (*s3.Client, error) {
	if cfg == nil {
		return nil, nil
	}

	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()

	client, err := initS3(cfg)
	metrics.CountInfraInit("s3", err)
	if err == nil && client == nil {
		// no_default: the component initialized, but there is no shared
		// singleton to report as up.
		metrics.SetInfraUp("s3", false)
	}

	return client, err
}

func initS3(cfg *S3Config) (*s3.Client, error) {
	if cfg == nil {
		return nil, nil
	}

	cfg.Ensure()
	if err := cfg.Validate(); err != nil {
		logger.Logger.Error(
			"init S3 config invalid",
			"error", err.Error(),
		)

		return nil, err
	}

	instances, names, err := cfg.resolveInstances()
	if err != nil {
		logger.Logger.Error(
			"init S3 instances invalid",
			"error", err.Error(),
		)

		return nil, err
	}

	identityErr := cfg.validateIdentity()
	selection, err := resolveDefaultSelection(cfg.NoDefault, cfg.DefaultInstance, names, identityErr == nil, identityErr)
	if err != nil {
		logger.Logger.Error(
			"init S3 default selection invalid",
			"error", err.Error(),
		)

		return nil, err
	}

	if selection.Ignored != "" {
		logger.Logger.Warn(
			"s3 default_instance ignored because the top-level config is complete",
			"instance", selection.Ignored,
		)
	}

	if cfg.NoDefault {
		logger.Logger.Info("s3 no_default is set; this block does not create a shared instance")
	}

	if !selection.UseTopLevel && selection.Instance != "" && cfg.DefaultInstance == "" {
		if len(names) == 1 {
			logger.Logger.Info(
				"s3 using the only named instance as the default",
				"instance", selection.Instance,
			)
		} else {
			logger.Logger.Warn(
				"s3 no default_instance configured; using the first instance in sorted order",
				"instance", selection.Instance,
			)
		}
	}

	// Build first, register second: a failure never leaves a half-installed
	// component behind. Existing first-wins entries are not rebuilt.
	mu.RLock()
	existingDefault := S3
	mu.RUnlock()

	var topLevel *s3.Client

	if selection.UseTopLevel && existingDefault == nil {
		topLevel, err = newS3Client(cfg)
		if err != nil {
			logger.Logger.Error(
				"init S3 failed",
				"region", cfg.Region,
				"endpoint", cfg.Endpoint,
				"error", err.Error(),
			)

			return nil, err
		}
	}

	built := make(map[string]*s3.Client, len(names))
	for _, name := range names {
		if _, ok := s3Instances.get(name); ok {
			logger.Logger.Warn("s3 instance already initialized, keeping existing client", "instance", name)

			continue
		}

		client, berr := newS3Client(instances[name])
		if berr != nil {
			metrics.CountInfraInit("s3:"+name, berr)
			logger.Logger.Error(
				"init S3 instance failed",
				"instance", name,
				"error", berr.Error(),
			)

			rollbackS3Clients(built)

			return nil, fmt.Errorf("s3 instance %s: %w", name, berr)
		}

		built[name] = client
		metrics.CountInfraInit("s3:"+name, nil)
	}

	// Register under the lock, release before anything that can block.
	if topLevel != nil {
		stored := false

		mu.Lock()
		if S3 == nil {
			S3 = topLevel
			s3Bucket = strings.TrimSpace(cfg.Bucket)
			s3DefaultInstance = ""
			stored = true
		}
		mu.Unlock()

		if !stored {
			logger.Logger.Warn("s3 already initialized, keeping existing client")
		}
	}

	for _, name := range names {
		client, ok := built[name]
		if !ok {
			continue
		}

		instance := s3Instance{
			client: client,
			bucket: strings.TrimSpace(instances[name].Bucket),
		}

		if _, stored := s3Instances.putFirst(name, instance); !stored {
			logger.Logger.Warn("s3 instance already initialized, keeping existing client", "instance", name)

			continue
		}

		logger.Logger.InfoContext(
			context.Background(),
			"init S3 instance successful",
			"instance", name,
			"region", instances[name].Region,
			"endpoint", instances[name].Endpoint,
			"bucket", instances[name].Bucket,
		)
	}

	// A named default is installed only when the shared singleton is still
	// free; first-wins keeps an earlier top-level or alias default.
	if !selection.UseTopLevel && selection.Instance != "" {
		instance, ok := s3Instances.get(selection.Instance)
		if ok && instance.client != nil {
			mu.Lock()
			if S3 == nil {
				S3 = instance.client
				s3Bucket = instance.bucket
				s3DefaultInstance = selection.Instance
			}
			mu.Unlock()
		}
	}

	mu.RLock()
	client := S3
	bucket := s3Bucket
	mu.RUnlock()

	if topLevel != nil {
		logger.Logger.InfoContext(
			context.Background(),
			"init S3 successful",
			"region", cfg.Region,
			"endpoint", cfg.Endpoint,
			"bucket", bucket,
		)
	}

	return client, nil
}

// rollbackS3Clients removes the metrics for handles built in a failed init.
// S3 clients have no Close, so there is nothing else to tear down.
func rollbackS3Clients(built map[string]*s3.Client) {
	for name := range built {
		metrics.SetInfraUp("s3:"+name, false)
	}
}

func newS3Client(cfg *S3Config) (*s3.Client, error) {
	// Bound the config load: outside AWS the default chain probes IMDS,
	// which can stall startup for seconds.
	lctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Timeout)*time.Second)
	defer cancel()

	loadOpts := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.Region),
	}

	if cfg.RequestTimeoutSec > 0 {
		loadOpts = append(loadOpts, config.WithHTTPClient(&http.Client{
			Timeout: time.Duration(cfg.RequestTimeoutSec) * time.Second,
		}))
	}

	if strings.TrimSpace(cfg.AccessKey) != "" {
		loadOpts = append(loadOpts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, cfg.SessionToken),
		))
	}

	c, err := config.LoadDefaultConfig(lctx, loadOpts...)
	if err != nil {
		return nil, err
	}

	clientOpts := func(o *s3.Options) {
		if strings.TrimSpace(cfg.Endpoint) != "" {
			endpoint := strings.TrimSpace(cfg.Endpoint)
			o.BaseEndpoint = &endpoint
		}

		o.UsePathStyle = cfg.UsePathStyle
	}

	return s3.NewFromConfig(c, clientOpts), nil
}

// Ensure fills zero-valued fields with defaults and returns the receiver (nil-safe).
func (c *S3Config) Ensure() *S3Config {
	if c == nil {
		c = new(S3Config)
	}

	if c.Timeout == 0 {
		c.Timeout = DefaultInitTimeoutSec
	}

	return c
}

// Validate rejects half-configured or illegal values and resolves the named
// instances (without mutating the caller's config).
func (c *S3Config) Validate() error {
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

func (c *S3Config) validateShape() error {
	if c == nil {
		return nil
	}

	if strings.TrimSpace(c.AccessKey) != "" && strings.TrimSpace(c.SecretKey) == "" {
		return ErrS3SecretEmpty
	}

	if c.Timeout < 0 || c.RequestTimeoutSec < 0 {
		return ErrS3TimeoutInvalid
	}

	return nil
}

func (c *S3Config) validateIdentity() error {
	if c == nil {
		return nil
	}

	if strings.TrimSpace(c.Region) == "" {
		return ErrS3RegionEmpty
	}

	return nil
}

func (c *S3Config) validateBase() error {
	if err := c.validateShape(); err != nil {
		return err
	}

	return c.validateIdentity()
}

// resolveInstances validates every declared instance and returns ensured
// copies keyed by the normalized name. The caller's config is not mutated.
func (c *S3Config) resolveInstances() (resolved map[string]*S3Config, names []string, err error) {
	normalized, names, err := normalizeInstanceNames(c.Instances)
	if err != nil {
		return nil, nil, err
	}

	resolved = make(map[string]*S3Config, len(names))
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

// ShutdownS3 clears the shared client and every named instance. S3 clients
// hold no closable resources; the AWS SDK manages its HTTP transports. Named
// instances are detached before clearing so a reader cannot observe a stale
// handle through the registry.
func ShutdownS3() error {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()

	instances := s3Instances.clear()

	mu.Lock()
	alias := s3DefaultInstance
	defaultClient := S3
	S3 = nil
	s3Bucket = ""
	s3DefaultInstance = ""
	mu.Unlock()

	if len(instances) == 0 && defaultClient == nil {
		return nil
	}

	logger.Logger.InfoContext(
		context.Background(),
		"s3 clients shutdown (connections managed by AWS SDK)",
		"instances", len(instances),
		"default", defaultClient != nil,
		"default_instance", alias,
	)

	for name := range instances {
		metrics.SetInfraUp("s3:"+name, false)
	}

	metrics.SetInfraUp("s3", false)

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
