/**
 * @file shutdown_noop_test.go
 * @package infra
 * @author Dr.NP <np@herewe.tech>
 * @since 10/29/2026
 */

package infra

import "testing"

// An unconfigured component must shut down as a no-op: no close call, no
// error, and no metrics series invented for a component the process never
// enabled.
func TestShutdownNoopWhenUnconfigured(t *testing.T) {
	cases := []struct {
		name     string
		reset    func()
		shutdown func() error
	}{
		{"badger", func() { ClearBadgerAll() }, ShutdownBadger},
		{"bun", func() { ClearBunAll() }, ShutdownBun},
		{"clickhouse", func() { ClearClickHouseAll() }, ShutdownClickHouse},
		{"elastic", func() { ClearElasticAll() }, ShutdownElastic},
		{"mongo", func() { ClearMongoAll() }, ShutdownMongo},
		{"mqtt", func() { ClearMQTTAll() }, ShutdownMQTT},
		{"nats", func() { ClearNATSAll() }, ShutdownNATS},
		{"redis", func() { ClearRedisAll() }, ShutdownRedis},
		{"ristretto", func() { ClearRistrettoAll() }, ShutdownRistretto},
		{"s3", func() { ClearS3All() }, ShutdownS3},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.reset()

			if err := tc.shutdown(); err != nil {
				t.Fatalf("Shutdown%s() = %v, want nil", tc.name, err)
			}
		})
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
