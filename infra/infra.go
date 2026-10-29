/**
 * @file infra.go
 * @package infra
 * @author Dr.NP <np@herewe.tech>
 * @since 09/04/2026
 */

package infra

import (
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
	"github.com/go-sicky/sicky/utils"
)

// DefaultInitTimeoutSec bounds dial/ping/connect calls that previously had
// no deadline (Bun, Redis, MQTT, NATS, S3). Sub-configs may override it
// with their own Timeout field where one exists.
const DefaultInitTimeoutSec = 5

// mu guards all package-level singletons below. Writers are Init*
// (first-wins) and Clear* (shutdown); readers must go through the Get*
// helpers — reading the exported globals directly races with Init/Clear.
var mu sync.RWMutex

// GetBadger returns the badger. With a name it returns the named instance.
func GetBadger(name ...string) *badger.DB {
	if n := firstInstanceName(name); n != "" {
		if instance, ok := badgerInstances.get(n); ok {
			return instance.db
		}

		return nil
	}

	mu.RLock()
	defer mu.RUnlock()

	return Badger
}

// GetBadgerInstances returns a snapshot of the named badger instances.
func GetBadgerInstances() map[string]*badger.DB {
	snapshot := badgerInstances.snapshot()
	out := make(map[string]*badger.DB, len(snapshot))
	for name, instance := range snapshot {
		out[name] = instance.db
	}

	return out
}

// GetBun returns the bun. With a name it returns the named instance.
func GetBun(name ...string) *bun.DB {
	if n := firstInstanceName(name); n != "" {
		if instance, ok := bunInstances.get(n); ok {
			return instance
		}

		return nil
	}

	mu.RLock()
	defer mu.RUnlock()

	return Bun
}

// GetBunInstances returns a snapshot of the named bun instances.
func GetBunInstances() map[string]*bun.DB {
	return bunInstances.snapshot()
}

// GetClickHouse returns the clickhouse. With a name it returns the named instance.
func GetClickHouse(name ...string) *ch.DB {
	if n := firstInstanceName(name); n != "" {
		if instance, ok := clickHouseInstances.get(n); ok {
			return instance
		}

		return nil
	}

	mu.RLock()
	defer mu.RUnlock()

	return ClickHouse
}

// Deprecated: use GetClickHouse.
func GetClickhouse(name ...string) *ch.DB {
	return GetClickHouse(name...)
}

// GetClickHouseInstances returns a snapshot of the named clickhouse instances.
func GetClickHouseInstances() map[string]*ch.DB {
	return clickHouseInstances.snapshot()
}

// GetElastic returns the elastic. With a name it returns the named instance.
func GetElastic(name ...string) *elasticsearch.Client {
	if n := firstInstanceName(name); n != "" {
		if instance, ok := elasticInstances.get(n); ok {
			return instance
		}

		return nil
	}

	mu.RLock()
	defer mu.RUnlock()

	return Elastic
}

// GetElasticInstances returns a snapshot of the named elastic instances.
func GetElasticInstances() map[string]*elasticsearch.Client {
	return elasticInstances.snapshot()
}

// GetMongo returns the mongo. With a name it returns the named instance.
func GetMongo(name ...string) *mongo.Client {
	if n := firstInstanceName(name); n != "" {
		if instance, ok := mongoInstances.get(n); ok {
			return instance.client
		}

		return nil
	}

	mu.RLock()
	defer mu.RUnlock()

	return Mongo
}

// GetMongoInstances returns a snapshot of the named mongo instances.
func GetMongoInstances() map[string]*mongo.Client {
	snapshot := mongoInstances.snapshot()
	out := make(map[string]*mongo.Client, len(snapshot))
	for name, instance := range snapshot {
		out[name] = instance.client
	}

	return out
}

// GetMQTT returns the mqtt. With a name it returns the named instance.
func GetMQTT(name ...string) mqtt.Client {
	if n := firstInstanceName(name); n != "" {
		if instance, ok := mqttInstances.get(n); ok {
			return instance.client
		}

		return nil
	}

	mu.RLock()
	defer mu.RUnlock()

	return MQTT
}

// GetMQTTInstances returns a snapshot of the named mqtt instances.
func GetMQTTInstances() map[string]mqtt.Client {
	snapshot := mqttInstances.snapshot()
	out := make(map[string]mqtt.Client, len(snapshot))
	for name, instance := range snapshot {
		out[name] = instance.client
	}

	return out
}

// GetNATS returns the nats. With a name it returns the named instance.
func GetNATS(name ...string) *nats.Conn {
	if n := firstInstanceName(name); n != "" {
		if instance, ok := natsInstances.get(n); ok {
			return instance
		}

		return nil
	}

	mu.RLock()
	defer mu.RUnlock()

	return NATS
}

// Deprecated: use GetNATS.
func GetNats(name ...string) *nats.Conn {
	return GetNATS(name...)
}

// GetNATSInstances returns a snapshot of the named nats instances.
func GetNATSInstances() map[string]*nats.Conn {
	return natsInstances.snapshot()
}

// GetRedis returns the redis. With a name it returns the named instance.
func GetRedis(name ...string) *redis.Client {
	if n := firstInstanceName(name); n != "" {
		if instance, ok := redisInstances.get(n); ok {
			return instance
		}

		return nil
	}

	mu.RLock()
	defer mu.RUnlock()

	return Redis
}

// GetRedisInstances returns a snapshot of the named redis instances.
func GetRedisInstances() map[string]*redis.Client {
	return redisInstances.snapshot()
}

// GetRistretto returns the ristretto. With a name it returns the named instance.
func GetRistretto(name ...string) *ristretto.Cache[string, any] {
	if n := firstInstanceName(name); n != "" {
		if instance, ok := ristrettoInstances.get(n); ok {
			return instance
		}

		return nil
	}

	mu.RLock()
	defer mu.RUnlock()

	return Ristretto
}

// GetRistrettoInstances returns a snapshot of the named ristretto instances.
func GetRistrettoInstances() map[string]*ristretto.Cache[string, any] {
	return ristrettoInstances.snapshot()
}

// GetS3 returns the s3. With a name it returns the named instance. An empty
// name returns the shared singleton; "default" is not a reserved alias.
func GetS3(name ...string) *s3.Client {
	if n := firstInstanceName(name); n != "" {
		if instance, ok := s3Instances.get(n); ok {
			return instance.client
		}

		return nil
	}

	mu.RLock()
	defer mu.RUnlock()

	return S3
}

// GetS3Instances returns a snapshot of the named s3 instances.
func GetS3Instances() map[string]*s3.Client {
	snapshot := s3Instances.snapshot()
	out := make(map[string]*s3.Client, len(snapshot))
	for name, instance := range snapshot {
		out[name] = instance.client
	}

	return out
}

// Clear* resets references so a later Run() builds fresh connections instead
// of reusing a closed one. Clear* does not close clients; ShutdownX owns the
// close-and-clear path.

// ClearBadger clears the shared badger. With a name it clears that named
// instance instead; the default alias reference is dropped when they match.
func ClearBadger(names ...string) {
	if n := firstInstanceName(names); n != "" {
		clearBadgerInstance(n)

		return
	}

	mu.Lock()
	Badger = nil
	badgerDefaultInstance = ""
	mu.Unlock()

	metrics.SetInfraUp("badger", false)
}

// ClearBadgerAll clears the shared badger and every named instance.
func ClearBadgerAll() {
	mu.Lock()
	Badger = nil
	badgerDefaultInstance = ""
	mu.Unlock()

	for name := range badgerInstances.clear() {
		metrics.SetInfraUp("badger:"+name, false)
	}

	metrics.SetInfraUp("badger", false)
}

func clearBadgerInstance(name string) {
	mu.Lock()
	isDefault := badgerDefaultInstance == name
	if isDefault {
		Badger = nil
		badgerDefaultInstance = ""
	}
	mu.Unlock()

	if _, ok := badgerInstances.remove(name); ok {
		metrics.SetInfraUp("badger:"+name, false)
	}

	if isDefault {
		metrics.SetInfraUp("badger", false)
	}
}

// ClearBun clears the shared bun. With a name it clears that named instance
// instead; the default alias reference is dropped when they match.
func ClearBun(names ...string) {
	if n := firstInstanceName(names); n != "" {
		clearBunInstance(n)

		return
	}

	mu.Lock()
	Bun = nil
	bunDefaultInstance = ""
	mu.Unlock()

	metrics.SetInfraUp("bun", false)
}

// ClearBunAll clears the shared bun and every named instance.
func ClearBunAll() {
	mu.Lock()
	Bun = nil
	bunDefaultInstance = ""
	mu.Unlock()

	for name := range bunInstances.clear() {
		metrics.SetInfraUp("bun:"+name, false)
	}

	metrics.SetInfraUp("bun", false)
}

func clearBunInstance(name string) {
	mu.Lock()
	isDefault := bunDefaultInstance == name
	if isDefault {
		Bun = nil
		bunDefaultInstance = ""
	}
	mu.Unlock()

	if _, ok := bunInstances.remove(name); ok {
		metrics.SetInfraUp("bun:"+name, false)
	}

	if isDefault {
		metrics.SetInfraUp("bun", false)
	}
}

// ClearClickHouse clears the shared clickhouse. With a name it clears that
// named instance instead; the default alias reference is dropped when they
// match.
func ClearClickHouse(names ...string) {
	if n := firstInstanceName(names); n != "" {
		clearClickHouseInstance(n)

		return
	}

	mu.Lock()
	ClickHouse = nil
	Clickhouse = nil
	clickHouseDefaultInstance = ""
	mu.Unlock()

	metrics.SetInfraUp("clickhouse", false)
}

// Deprecated: use ClearClickHouse.
func ClearClickhouse(names ...string) {
	ClearClickHouse(names...)
}

// ClearClickHouseAll clears the shared clickhouse and every named instance.
func ClearClickHouseAll() {
	mu.Lock()
	ClickHouse = nil
	Clickhouse = nil
	clickHouseDefaultInstance = ""
	mu.Unlock()

	for name := range clickHouseInstances.clear() {
		metrics.SetInfraUp("clickhouse:"+name, false)
	}

	metrics.SetInfraUp("clickhouse", false)
}

func clearClickHouseInstance(name string) {
	mu.Lock()
	isDefault := clickHouseDefaultInstance == name
	if isDefault {
		ClickHouse = nil
		Clickhouse = nil
		clickHouseDefaultInstance = ""
	}
	mu.Unlock()

	if _, ok := clickHouseInstances.remove(name); ok {
		metrics.SetInfraUp("clickhouse:"+name, false)
	}

	if isDefault {
		metrics.SetInfraUp("clickhouse", false)
	}
}

// ClearElastic clears the shared elastic. With a name it clears that named
// instance instead; the default alias reference is dropped when they match.
func ClearElastic(names ...string) {
	if n := firstInstanceName(names); n != "" {
		clearElasticInstance(n)

		return
	}

	mu.Lock()
	Elastic = nil
	elasticDefaultInstance = ""
	mu.Unlock()

	metrics.SetInfraUp("elastic", false)
}

// ClearElasticAll clears the shared elastic and every named instance.
func ClearElasticAll() {
	mu.Lock()
	Elastic = nil
	elasticDefaultInstance = ""
	mu.Unlock()

	for name := range elasticInstances.clear() {
		metrics.SetInfraUp("elastic:"+name, false)
	}

	metrics.SetInfraUp("elastic", false)
}

func clearElasticInstance(name string) {
	mu.Lock()
	isDefault := elasticDefaultInstance == name
	if isDefault {
		Elastic = nil
		elasticDefaultInstance = ""
	}
	mu.Unlock()

	if _, ok := elasticInstances.remove(name); ok {
		metrics.SetInfraUp("elastic:"+name, false)
	}

	if isDefault {
		metrics.SetInfraUp("elastic", false)
	}
}

// ClearMongo clears the shared mongo. With a name it clears that named
// instance instead; the default alias reference is dropped when they match.
func ClearMongo(names ...string) {
	if n := firstInstanceName(names); n != "" {
		clearMongoInstance(n)

		return
	}

	mu.Lock()
	Mongo = nil
	mongoDBName = ""
	mongoDefaultInstance = ""
	mu.Unlock()

	metrics.SetInfraUp("mongo", false)
}

// ClearMongoAll clears the shared mongo and every named instance.
func ClearMongoAll() {
	mu.Lock()
	Mongo = nil
	mongoDBName = ""
	mongoDefaultInstance = ""
	mu.Unlock()

	for name := range mongoInstances.clear() {
		metrics.SetInfraUp("mongo:"+name, false)
	}

	metrics.SetInfraUp("mongo", false)
}

func clearMongoInstance(name string) {
	mu.Lock()
	isDefault := mongoDefaultInstance == name
	if isDefault {
		Mongo = nil
		mongoDBName = ""
		mongoDefaultInstance = ""
	}
	mu.Unlock()

	if _, ok := mongoInstances.remove(name); ok {
		metrics.SetInfraUp("mongo:"+name, false)
	}

	if isDefault {
		metrics.SetInfraUp("mongo", false)
	}
}

// ClearMQTT clears the shared mqtt. With a name it clears that named
// instance instead; the default alias reference is dropped when they match.
func ClearMQTT(names ...string) {
	if n := firstInstanceName(names); n != "" {
		clearMQTTInstance(n)

		return
	}

	mu.Lock()
	MQTT = nil
	mqttDefaultInstance = ""
	mu.Unlock()

	metrics.SetInfraUp("mqtt", false)
}

// ClearMQTTAll clears the shared mqtt and every named instance.
func ClearMQTTAll() {
	mu.Lock()
	MQTT = nil
	mqttDefaultInstance = ""
	mu.Unlock()

	for name := range mqttInstances.clear() {
		metrics.SetInfraUp("mqtt:"+name, false)
	}

	metrics.SetInfraUp("mqtt", false)
}

func clearMQTTInstance(name string) {
	mu.Lock()
	isDefault := mqttDefaultInstance == name
	if isDefault {
		MQTT = nil
		mqttDefaultInstance = ""
	}
	mu.Unlock()

	if _, ok := mqttInstances.remove(name); ok {
		metrics.SetInfraUp("mqtt:"+name, false)
	}

	if isDefault {
		metrics.SetInfraUp("mqtt", false)
	}
}

// ClearNATS clears the shared nats. With a name it clears that named
// instance instead; the default alias reference is dropped when they match.
func ClearNATS(names ...string) {
	if n := firstInstanceName(names); n != "" {
		clearNATSInstance(n)

		return
	}

	mu.Lock()
	NATS = nil
	Nats = nil
	natsDefaultInstance = ""
	mu.Unlock()

	metrics.SetInfraUp("nats", false)
}

// Deprecated: use ClearNATS.
func ClearNats(names ...string) {
	ClearNATS(names...)
}

// ClearNATSAll clears the shared nats and every named instance.
func ClearNATSAll() {
	mu.Lock()
	NATS = nil
	Nats = nil
	natsDefaultInstance = ""
	mu.Unlock()

	for name := range natsInstances.clear() {
		metrics.SetInfraUp("nats:"+name, false)
	}

	metrics.SetInfraUp("nats", false)
}

func clearNATSInstance(name string) {
	mu.Lock()
	isDefault := natsDefaultInstance == name
	if isDefault {
		NATS = nil
		Nats = nil
		natsDefaultInstance = ""
	}
	mu.Unlock()

	if _, ok := natsInstances.remove(name); ok {
		metrics.SetInfraUp("nats:"+name, false)
	}

	if isDefault {
		metrics.SetInfraUp("nats", false)
	}
}

// ClearRedis clears the shared redis. With a name it clears that named
// instance instead; the default alias reference is dropped when they match.
func ClearRedis(names ...string) {
	if n := firstInstanceName(names); n != "" {
		clearRedisInstance(n)

		return
	}

	mu.Lock()
	Redis = nil
	redisDefaultInstance = ""
	mu.Unlock()

	metrics.SetInfraUp("redis", false)
}

// ClearRedisAll clears the shared redis and every named instance.
func ClearRedisAll() {
	mu.Lock()
	Redis = nil
	redisDefaultInstance = ""
	mu.Unlock()

	for name := range redisInstances.clear() {
		metrics.SetInfraUp("redis:"+name, false)
	}

	metrics.SetInfraUp("redis", false)
}

func clearRedisInstance(name string) {
	mu.Lock()
	isDefault := redisDefaultInstance == name
	if isDefault {
		Redis = nil
		redisDefaultInstance = ""
	}
	mu.Unlock()

	if _, ok := redisInstances.remove(name); ok {
		metrics.SetInfraUp("redis:"+name, false)
	}

	if isDefault {
		metrics.SetInfraUp("redis", false)
	}
}

// ClearRistretto clears the shared ristretto. With a name it clears that
// named instance instead.
func ClearRistretto(names ...string) {
	if n := firstInstanceName(names); n != "" {
		clearRistrettoInstance(n)

		return
	}

	mu.Lock()
	Ristretto = nil
	ristrettoDefaultInstance = ""
	mu.Unlock()

	metrics.SetInfraUp("ristretto", false)
}

// ClearRistrettoAll clears the shared ristretto and every named instance.
func ClearRistrettoAll() {
	mu.Lock()
	Ristretto = nil
	ristrettoDefaultInstance = ""
	mu.Unlock()

	for name := range ristrettoInstances.clear() {
		metrics.SetInfraUp("ristretto:"+name, false)
	}

	metrics.SetInfraUp("ristretto", false)
}

func clearRistrettoInstance(name string) {
	mu.Lock()
	isDefault := ristrettoDefaultInstance == name
	if isDefault {
		Ristretto = nil
		ristrettoDefaultInstance = ""
	}
	mu.Unlock()

	if _, ok := ristrettoInstances.remove(name); ok {
		metrics.SetInfraUp("ristretto:"+name, false)
	}

	if isDefault {
		metrics.SetInfraUp("ristretto", false)
	}
}

// ClearS3 clears the shared s3. With a name it clears that named instance
// instead; the default alias reference is dropped when they match.
func ClearS3(names ...string) {
	if n := firstInstanceName(names); n != "" {
		clearS3Instance(n)

		return
	}

	mu.Lock()
	S3 = nil
	s3Bucket = ""
	s3DefaultInstance = ""
	mu.Unlock()

	metrics.SetInfraUp("s3", false)
}

// ClearS3All clears the shared s3 and every named instance.
func ClearS3All() {
	mu.Lock()
	S3 = nil
	s3Bucket = ""
	s3DefaultInstance = ""
	mu.Unlock()

	for name := range s3Instances.clear() {
		metrics.SetInfraUp("s3:"+name, false)
	}

	metrics.SetInfraUp("s3", false)
}

func clearS3Instance(name string) {
	mu.Lock()
	isDefault := s3DefaultInstance == name
	if isDefault {
		S3 = nil
		s3Bucket = ""
		s3DefaultInstance = ""
	}
	mu.Unlock()

	if _, ok := s3Instances.remove(name); ok {
		metrics.SetInfraUp("s3:"+name, false)
	}

	if isDefault {
		metrics.SetInfraUp("s3", false)
	}
}

// RedactDSN strips credentials (userinfo and common password query keys)
// from DSN/URI strings before they are written to logs or error strings.
// Clickhouse DSN and Mongo URI both embed user:password, so they must
// never be logged raw. Callers outside the infra package (broker,
// registry) use this exported form.
//
// It delegates to utils.RedactDSN, the single implementation. The copy that
// used to live here is gone: together with tracer.RedactDSN there were two
// independent implementations, and the permissive one failed open.
func RedactDSN(raw string) string {
	return utils.RedactDSN(raw)
}

// redactDSN is the package-internal alias kept for existing call sites.
func redactDSN(raw string) string {
	return RedactDSN(raw)
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
