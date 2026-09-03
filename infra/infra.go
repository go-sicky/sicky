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
)

// DefaultInitTimeoutSec bounds dial/ping/connect calls that previously had
// no deadline (Bun, Redis, MQTT, NATS, S3). Sub-configs may override it
// with their own Timeout field where one exists.
const DefaultInitTimeoutSec = 5

// mu guards all package-level singletons below. Writers are Init*
// (first-wins) and Clear* (shutdown); readers must go through the Get*
// helpers — reading the exported globals directly races with Init/Clear.
var mu sync.RWMutex

func GetBadger() *badger.DB {
	mu.RLock()
	defer mu.RUnlock()

	return Badger
}

func GetBun() *bun.DB {
	mu.RLock()
	defer mu.RUnlock()

	return Bun
}

func GetClickhouse() *ch.DB {
	mu.RLock()
	defer mu.RUnlock()

	return Clickhouse
}

func GetElastic() *elasticsearch.Client {
	mu.RLock()
	defer mu.RUnlock()

	return Elastic
}

func GetMongo() *mongo.Client {
	mu.RLock()
	defer mu.RUnlock()

	return Mongo
}

func GetMQTT() mqtt.Client {
	mu.RLock()
	defer mu.RUnlock()

	return MQTT
}

func GetNats() *nats.Conn {
	mu.RLock()
	defer mu.RUnlock()

	return Nats
}

func GetRedis() *redis.Client {
	mu.RLock()
	defer mu.RUnlock()

	return Redis
}

func GetRistretto() *ristretto.Cache[string, any] {
	mu.RLock()
	defer mu.RUnlock()

	return Ristretto
}

func GetS3() *s3.Client {
	mu.RLock()
	defer mu.RUnlock()

	return S3
}

// Clear* resets a singleton after Close/Disconnect so a later Run() builds
// a fresh connection instead of reusing a closed one.

func ClearBadger() {
	mu.Lock()
	defer mu.Unlock()
	Badger = nil
}

func ClearBun() {
	mu.Lock()
	defer mu.Unlock()
	Bun = nil
}

func ClearClickhouse() {
	mu.Lock()
	defer mu.Unlock()
	Clickhouse = nil
}

func ClearElastic() {
	mu.Lock()
	defer mu.Unlock()
	Elastic = nil
}

func ClearMongo() {
	mu.Lock()
	defer mu.Unlock()
	Mongo = nil
	mongoDBName = ""
}

func ClearMQTT() {
	mu.Lock()
	defer mu.Unlock()
	MQTT = nil
}

func ClearNats() {
	mu.Lock()
	defer mu.Unlock()
	Nats = nil
}

func ClearRedis() {
	mu.Lock()
	defer mu.Unlock()
	Redis = nil
}

func ClearRistretto() {
	mu.Lock()
	defer mu.Unlock()
	Ristretto = nil
}

func ClearS3() {
	mu.Lock()
	defer mu.Unlock()
	S3 = nil
	s3Bucket = ""
}

// redactDSN strips credentials (userinfo and common password query keys)
// from DSN/URI strings before they are written to logs. Clickhouse DSN and
// Mongo URI both embed user:password, so they must never be logged raw.
func redactDSN(raw string) string {
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
