/**
 * @file infra.go
 * @package infra
 * @author Dr.NP <np@herewe.tech>
 * @since 09/04/2026
 */

package infra

import (
	"net/url"
	"strings"
	"sync"

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

	"github.com/go-sicky/sicky/metrics"
)

// DefaultInitTimeoutSec bounds dial/ping/connect calls that previously had
// no deadline (Bun, Redis, MQTT, NATS, S3). Sub-configs may override it
// with their own Timeout field where one exists.
const DefaultInitTimeoutSec = 5

// mu guards all package-level singletons below. Writers are Init*
// (first-wins) and Clear* (shutdown); readers must go through the Get*
// helpers — reading the exported globals directly races with Init/Clear.
var mu sync.RWMutex

// GetBadger returns the badger.
func GetBadger() *badger.DB {
	mu.RLock()
	defer mu.RUnlock()

	return Badger
}

// GetBun returns the bun.
func GetBun() *bun.DB {
	mu.RLock()
	defer mu.RUnlock()

	return Bun
}

// GetClickHouse returns the clickhouse.
func GetClickHouse() *ch.DB {
	mu.RLock()
	defer mu.RUnlock()

	return ClickHouse
}

// Deprecated: use GetClickHouse.
func GetClickhouse() *ch.DB {
	return GetClickHouse()
}

// GetElastic returns the elastic.
func GetElastic() *elasticsearch.Client {
	mu.RLock()
	defer mu.RUnlock()

	return Elastic
}

// GetMongo returns the mongo.
func GetMongo() *mongo.Client {
	mu.RLock()
	defer mu.RUnlock()

	return Mongo
}

// GetMQTT returns the mqtt.
func GetMQTT() mqtt.Client {
	mu.RLock()
	defer mu.RUnlock()

	return MQTT
}

// GetNATS returns the nats.
func GetNATS() *nats.Conn {
	mu.RLock()
	defer mu.RUnlock()

	return NATS
}

// Deprecated: use GetNATS.
func GetNats() *nats.Conn {
	return GetNATS()
}

// GetRedis returns the redis.
func GetRedis() *redis.Client {
	mu.RLock()
	defer mu.RUnlock()

	return Redis
}

// GetRistretto returns the ristretto.
func GetRistretto() *ristretto.Cache[string, any] {
	mu.RLock()
	defer mu.RUnlock()

	return Ristretto
}

// GetS3 returns the s3.
func GetS3() *s3.Client {
	mu.RLock()
	defer mu.RUnlock()

	return S3
}

// Clear* resets a singleton after Close/Disconnect so a later Run() builds
// a fresh connection instead of reusing a closed one.

// ClearBadger is part of the public API.
func ClearBadger() {
	mu.Lock()
	defer mu.Unlock()
	Badger = nil
	metrics.SetInfraUp("badger", false)
}

// ClearBun is part of the public API.
func ClearBun() {
	mu.Lock()
	defer mu.Unlock()
	Bun = nil
	metrics.SetInfraUp("bun", false)
}

// ClearClickHouse is part of the public API.
func ClearClickHouse() {
	mu.Lock()
	defer mu.Unlock()
	ClickHouse = nil
	Clickhouse = nil
	metrics.SetInfraUp("clickhouse", false)
}

// Deprecated: use ClearClickHouse.
func ClearClickhouse() {
	ClearClickHouse()
}

// ClearElastic is part of the public API.
func ClearElastic() {
	mu.Lock()
	defer mu.Unlock()
	Elastic = nil
	metrics.SetInfraUp("elastic", false)
}

// ClearMongo is part of the public API.
func ClearMongo() {
	mu.Lock()
	defer mu.Unlock()
	Mongo = nil
	mongoDBName = ""
	metrics.SetInfraUp("mongo", false)
}

// ClearMQTT is part of the public API.
func ClearMQTT() {
	mu.Lock()
	defer mu.Unlock()
	MQTT = nil
	metrics.SetInfraUp("mqtt", false)
}

// ClearNATS is part of the public API.
func ClearNATS() {
	mu.Lock()
	defer mu.Unlock()
	NATS = nil
	Nats = nil
	metrics.SetInfraUp("nats", false)
}

// Deprecated: use ClearNATS.
func ClearNats() {
	ClearNATS()
}

// ClearRedis is part of the public API.
func ClearRedis() {
	mu.Lock()
	defer mu.Unlock()
	Redis = nil
	metrics.SetInfraUp("redis", false)
}

// ClearRistretto is part of the public API.
func ClearRistretto() {
	mu.Lock()
	defer mu.Unlock()
	Ristretto = nil
	metrics.SetInfraUp("ristretto", false)
}

// ClearS3 is part of the public API.
func ClearS3() {
	mu.Lock()
	defer mu.Unlock()
	S3 = nil
	s3Bucket = ""
	metrics.SetInfraUp("s3", false)
}

// RedactDSN strips credentials (userinfo and common password query keys)
// from DSN/URI strings before they are written to logs or error strings.
// Clickhouse DSN and Mongo URI both embed user:password, so they must
// never be logged raw. Callers outside the infra package (broker,
// registry) use this exported form.
func RedactDSN(raw string) string {
	if raw == "" {
		return ""
	}

	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "REDACTED"
	}

	u.User = nil

	q := u.Query()
	changed := false
	for _, k := range []string{"password", "pass", "passwd", "pwd", "secret", "token", "api_key", "apikey", "access_key", "secret_key", "session_token", "auth"} {
		if _, ok := q[k]; ok {
			q.Set(k, "REDACTED")
			changed = true
		}
	}

	if changed {
		u.RawQuery = q.Encode()
	}

	s := u.String()
	if strings.Contains(s, "://:") {
		return "REDACTED"
	}

	return s
}

// redactDSN is the package-internal alias kept for existing call sites.
func redactDSN(raw string) string {
	return RedactDSN(raw)
}
