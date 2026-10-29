/**
 * @file clear_instances_test.go
 * @package infra
 * @author Dr.NP <np@herewe.tech>
 * @since 10/29/2026
 */

package infra

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/dgraph-io/badger/v4"
	"github.com/dgraph-io/ristretto/v2"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/elastic/go-elasticsearch/v9"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"github.com/uptrace/bun"
	"github.com/uptrace/go-clickhouse/ch"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// TestClearInstanceVariants pins the three reference operations for every
// component: ClearX() drops only the default (and its alias reference),
// ClearX(name) drops one named instance, and ClearXAll() drops everything.
// Zero-value handles are enough because none of these operations closes.
func TestClearInstanceVariants(t *testing.T) {
	cases := []struct {
		name       string
		reset      func()
		install    func()
		defaultSet func() bool
		namedSet   func(string) bool
		clearOne   func(...string)
		clearAll   func()
	}{
		{
			name:  "badger",
			reset: func() { ClearBadgerAll() },
			install: func() {
				db := new(badger.DB)
				mu.Lock()
				Badger = db
				badgerDefaultInstance = "cache"
				mu.Unlock()
				badgerInstances.putFirst("cache", badgerInstance{db: db})
				badgerInstances.putFirst("other", badgerInstance{db: new(badger.DB)})
			},
			defaultSet: func() bool { return GetBadger() != nil },
			namedSet:   func(name string) bool { return GetBadger(name) != nil },
			clearOne:   ClearBadger,
			clearAll:   ClearBadgerAll,
		},
		{
			name:  "bun",
			reset: func() { ClearBunAll() },
			install: func() {
				db := new(bun.DB)
				mu.Lock()
				Bun = db
				bunDefaultInstance = "cache"
				mu.Unlock()
				bunInstances.putFirst("cache", db)
				bunInstances.putFirst("other", new(bun.DB))
			},
			defaultSet: func() bool { return GetBun() != nil },
			namedSet:   func(name string) bool { return GetBun(name) != nil },
			clearOne:   ClearBun,
			clearAll:   ClearBunAll,
		},
		{
			name:  "clickhouse",
			reset: func() { ClearClickHouseAll() },
			install: func() {
				db := new(ch.DB)
				mu.Lock()
				ClickHouse = db
				Clickhouse = db
				clickHouseDefaultInstance = "cache"
				mu.Unlock()
				clickHouseInstances.putFirst("cache", db)
				clickHouseInstances.putFirst("other", new(ch.DB))
			},
			defaultSet: func() bool { return GetClickHouse() != nil },
			namedSet:   func(name string) bool { return GetClickHouse(name) != nil },
			clearOne:   ClearClickHouse,
			clearAll:   ClearClickHouseAll,
		},
		{
			name:  "elastic",
			reset: func() { ClearElasticAll() },
			install: func() {
				client := new(elasticsearch.Client)
				mu.Lock()
				Elastic = client
				elasticDefaultInstance = "cache"
				mu.Unlock()
				elasticInstances.putFirst("cache", client)
				elasticInstances.putFirst("other", new(elasticsearch.Client))
			},
			defaultSet: func() bool { return GetElastic() != nil },
			namedSet:   func(name string) bool { return GetElastic(name) != nil },
			clearOne:   ClearElastic,
			clearAll:   ClearElasticAll,
		},
		{
			name:  "mongo",
			reset: func() { ClearMongoAll() },
			install: func() {
				client := new(mongo.Client)
				mu.Lock()
				Mongo = client
				mongoDBName = "app"
				mongoDefaultInstance = "cache"
				mu.Unlock()
				mongoInstances.putFirst("cache", mongoInstance{client: client, db: "app"})
				mongoInstances.putFirst("other", mongoInstance{client: new(mongo.Client), db: "other"})
			},
			defaultSet: func() bool { return GetMongo() != nil },
			namedSet:   func(name string) bool { return GetMongo(name) != nil },
			clearOne:   ClearMongo,
			clearAll:   ClearMongoAll,
		},
		{
			name:  "mqtt",
			reset: func() { ClearMQTTAll() },
			install: func() {
				client := mqtt.NewClient(mqtt.NewClientOptions().AddBroker("tcp://127.0.0.1:1"))
				mu.Lock()
				MQTT = client
				mqttDefaultInstance = "cache"
				mu.Unlock()
				mqttInstances.putFirst("cache", mqttInstance{client: client, clientID: "cache"})
				mqttInstances.putFirst("other", mqttInstance{
					client:   mqtt.NewClient(mqtt.NewClientOptions().AddBroker("tcp://127.0.0.1:1")),
					clientID: "other",
				})
			},
			defaultSet: func() bool { return GetMQTT() != nil },
			namedSet:   func(name string) bool { return GetMQTT(name) != nil },
			clearOne:   ClearMQTT,
			clearAll:   ClearMQTTAll,
		},
		{
			name:  "nats",
			reset: func() { ClearNATSAll() },
			install: func() {
				conn := new(nats.Conn)
				mu.Lock()
				NATS = conn
				Nats = conn
				natsDefaultInstance = "cache"
				mu.Unlock()
				natsInstances.putFirst("cache", conn)
				natsInstances.putFirst("other", new(nats.Conn))
			},
			defaultSet: func() bool { return GetNATS() != nil },
			namedSet:   func(name string) bool { return GetNATS(name) != nil },
			clearOne:   ClearNATS,
			clearAll:   ClearNATSAll,
		},
		{
			name:  "redis",
			reset: func() { ClearRedisAll() },
			install: func() {
				client := new(redis.Client)
				mu.Lock()
				Redis = client
				redisDefaultInstance = "cache"
				mu.Unlock()
				redisInstances.putFirst("cache", client)
				redisInstances.putFirst("other", new(redis.Client))
			},
			defaultSet: func() bool { return GetRedis() != nil },
			namedSet:   func(name string) bool { return GetRedis(name) != nil },
			clearOne:   ClearRedis,
			clearAll:   ClearRedisAll,
		},
		{
			name:  "ristretto",
			reset: func() { ClearRistrettoAll() },
			install: func() {
				cache := new(ristretto.Cache[string, any])
				mu.Lock()
				Ristretto = cache
				ristrettoDefaultInstance = "cache"
				mu.Unlock()
				ristrettoInstances.putFirst("cache", cache)
				ristrettoInstances.putFirst("other", new(ristretto.Cache[string, any]))
			},
			defaultSet: func() bool { return GetRistretto() != nil },
			namedSet:   func(name string) bool { return GetRistretto(name) != nil },
			clearOne:   ClearRistretto,
			clearAll:   ClearRistrettoAll,
		},
		{
			name:  "s3",
			reset: func() { ClearS3All() },
			install: func() {
				client := new(s3.Client)
				mu.Lock()
				S3 = client
				s3Bucket = "cache"
				s3DefaultInstance = "cache"
				mu.Unlock()
				s3Instances.putFirst("cache", s3Instance{client: client, bucket: "cache"})
				s3Instances.putFirst("other", s3Instance{client: new(s3.Client), bucket: "other"})
			},
			defaultSet: func() bool { return GetS3() != nil },
			namedSet:   func(name string) bool { return GetS3(name) != nil },
			clearOne:   ClearS3,
			clearAll:   ClearS3All,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.reset()
			t.Cleanup(tc.reset)
			tc.install()

			if !tc.defaultSet() || !tc.namedSet("cache") || !tc.namedSet("other") {
				t.Fatal("install did not register the default and named instances")
			}

			tc.clearOne("missing")
			if !tc.defaultSet() || !tc.namedSet("cache") || !tc.namedSet("other") {
				t.Fatal("ClearX(missing) must be a no-op")
			}

			tc.clearOne("")
			if tc.defaultSet() {
				t.Fatal("ClearX(\"\") must clear only the default reference")
			}

			if !tc.namedSet("cache") || !tc.namedSet("other") {
				t.Fatal("ClearX(\"\") must not clear named instances")
			}

			tc.install()

			tc.clearOne("other")
			if tc.namedSet("other") {
				t.Fatal("ClearX(other) left the named instance registered")
			}

			if !tc.defaultSet() || !tc.namedSet("cache") {
				t.Fatal("ClearX(other) must not clear the default or another instance")
			}

			tc.clearOne("cache")
			if tc.namedSet("cache") {
				t.Fatal("ClearX(cache) left the named instance registered")
			}

			if tc.defaultSet() {
				t.Fatal("ClearX(cache) must clear the default alias reference")
			}

			tc.install()
			tc.clearAll()
			if tc.defaultSet() || tc.namedSet("cache") || tc.namedSet("other") {
				t.Fatal("ClearXAll must clear every reference")
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
