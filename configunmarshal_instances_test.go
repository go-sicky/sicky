/**
 * @file configunmarshal_instances_test.go
 * @package sicky
 * @author Dr.NP <np@herewe.tech>
 * @since 10/29/2026
 */

package sicky

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// ConfigUnmarshal is the public path Init/Run use. Exercise the typed instance
// map through it rather than only through a direct viper.Unmarshal, so the
// env binding and unknown-key schema walk are part of the same test.
func TestConfigUnmarshalDecodesInstances(t *testing.T) {
	previous := configIns
	configIns = viper.New()
	configIns.SetConfigType("json")

	t.Cleanup(func() { configIns = previous })

	if err := configIns.ReadConfig(strings.NewReader(`{
		"sicky": {
			"infra": {
				"s3": {
					"default_instance": "gallery",
					"instances": {
						"gallery": {"region": "eu-west-1", "bucket": "gallery"},
						"avatars": {"region": "us-east-1", "bucket": "avatars"}
					}
				},
				"redis": {
					"no_default": true,
					"instances": {
						"cache": {"addr": "127.0.0.1:6380", "db": 2}
					}
				}
			}
		}
	}`)); err != nil {
		t.Fatalf("read config: %v", err)
	}

	cfg := &nestedAppConfig{Sicky: &Config{}}
	if err := ConfigUnmarshal(cfg); err != nil {
		t.Fatalf("ConfigUnmarshal: %v", err)
	}

	s3 := cfg.Sicky.Infra.S3
	if s3 == nil || s3.DefaultInstance != "gallery" || len(s3.Instances) != 2 {
		t.Fatalf("s3 block = %+v, want gallery + two instances", s3)
	}

	if s3.Instances["gallery"].Bucket != "gallery" {
		t.Errorf("gallery bucket = %q, want gallery", s3.Instances["gallery"].Bucket)
	}

	redis := cfg.Sicky.Infra.Redis
	if redis == nil || !redis.NoDefault || redis.Instances["cache"] == nil {
		t.Fatalf("redis block = %+v, want no_default + cache", redis)
	}

	if redis.Instances["cache"].DB != 2 {
		t.Errorf("cache db = %d, want 2", redis.Instances["cache"].DB)
	}
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
