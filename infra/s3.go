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
}

// s3Bucket mirrors the configured bucket for PingS3. Guarded by mu.
var s3Bucket string

// PingS3 checks the shared singleton for /health; nil means not configured
// (healthy-by-absence, following the other local checks). With a bucket
// configured it issues HeadBucket; without one the client holds no
// connection state to probe, so non-nil alone counts as healthy.
func PingS3(ctx context.Context) error {
	mu.RLock()
	client := S3
	bucket := s3Bucket
	mu.RUnlock()

	if client == nil || bucket == "" {
		return nil
	}

	start := time.Now()
	_, err := client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &bucket})
	metrics.ObserveInfraOp("s3", "head_bucket", start, err)
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

	client, err := initS3(cfg)
	metrics.CountInfraInit("s3", err)

	return client, err
}

func initS3(cfg *S3Config) (*s3.Client, error) {
	if cfg == nil {
		return nil, nil
	}

	cfg.Ensure()
	if err := cfg.Validate(); err != nil {
		logger.Logger.Error(
			"Init S3 config invalid",
			"error", err.Error(),
		)

		return nil, err
	}

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
		logger.Logger.Error(
			"Init S3 failed",
			"region", cfg.Region,
			"endpoint", cfg.Endpoint,
			"error", err.Error(),
		)

		return nil, err
	}

	clientOpts := func(o *s3.Options) {
		if strings.TrimSpace(cfg.Endpoint) != "" {
			endpoint := strings.TrimSpace(cfg.Endpoint)
			o.BaseEndpoint = &endpoint
		}

		o.UsePathStyle = cfg.UsePathStyle
	}

	client := s3.NewFromConfig(c, clientOpts)

	mu.Lock()
	defer mu.Unlock()
	if S3 != nil {
		// First-wins: keep the existing singleton. The S3 client holds
		// no closeable resources, so there is nothing to drop.
		logger.Logger.Warn("S3 already initialized, keeping existing client")

		return S3, nil
	}

	S3 = client
	s3Bucket = strings.TrimSpace(cfg.Bucket)

	logger.Logger.InfoContext(
		context.Background(),
		"Init S3 successful",
		"region", cfg.Region,
		"endpoint", cfg.Endpoint,
		"bucket", cfg.Bucket,
	)

	return client, nil
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

// Validate rejects half-configured or illegal values.
func (c *S3Config) Validate() error {
	if c == nil {
		return nil
	}

	if strings.TrimSpace(c.Region) == "" {
		return ErrS3RegionEmpty
	}

	if strings.TrimSpace(c.AccessKey) != "" && strings.TrimSpace(c.SecretKey) == "" {
		return ErrS3SecretEmpty
	}

	if c.Timeout < 0 || c.RequestTimeoutSec < 0 {
		return ErrS3TimeoutInvalid
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
