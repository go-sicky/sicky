/**
 * @file instances.go
 * @package infra
 * @author Dr.NP <np@herewe.tech>
 * @since 10/29/2026
 */

package infra

import (
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
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

// lifecycleMu serializes Init* and Shutdown* across all components. Run
// already calls them sequentially; this also protects direct callers from
// registering a handle while a concurrent shutdown is detaching the registry.
// Readers (Get*) never take it.
var lifecycleMu sync.Mutex

// instanceSet is the per-component registry of named instances. The shared
// singleton stays in its own package-level variable; this registry only holds
// the extra, named connections.
type instanceSet[T any] struct {
	mu sync.RWMutex
	m  map[string]T
}

func newInstanceSet[T any]() *instanceSet[T] {
	return &instanceSet[T]{m: make(map[string]T)}
}

// get returns the named instance, if any.
func (s *instanceSet[T]) get(name string) (T, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	v, ok := s.m[name]

	return v, ok
}

// putFirst stores value under name unless the name already exists. It returns
// the value that is registered and whether value was stored: a false second
// return means an existing instance won and the caller should close/drop the
// duplicate it built.
func (s *instanceSet[T]) putFirst(name string, value T) (T, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.m[name]; ok {
		return existing, false
	}

	s.m[name] = value

	return value, true
}

// snapshot returns a copy of the registry. The handles themselves are shared
// with the registry; callers must not close them without clearing first.
func (s *instanceSet[T]) snapshot() map[string]T {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make(map[string]T, len(s.m))
	maps.Copy(out, s.m)

	return out
}

// remove deletes one named instance and returns the value that was registered.
func (s *instanceSet[T]) remove(name string) (T, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	value, ok := s.m[name]
	if ok {
		delete(s.m, name)
	}

	return value, ok
}

// clear removes every named instance and returns the removed values.
func (s *instanceSet[T]) clear() map[string]T {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := s.m
	s.m = make(map[string]T)

	return out
}

// names returns the registered names in sorted order.
func (s *instanceSet[T]) names() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]string, 0, len(s.m))
	for name := range s.m {
		out = append(out, name)
	}

	slices.Sort(out)

	return out
}

// instanceNamePattern keeps names safe for configuration keys, health
// component names and Prometheus label values. Dots are forbidden because
// viper treats them as key separators; colons are forbidden because health
// and metric names use them to join component and instance.
var instanceNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

// Instance configuration sentinels. They are shared by all components so
// callers can match the class of failure without knowing which backend they
// configured.
var (
	ErrInstanceNameEmpty       = errors.New("infra: instance name is empty")
	ErrInstanceNameInvalid     = errors.New("infra: instance name is invalid")
	ErrInstanceNameDuplicate   = errors.New("infra: instance name is duplicated")
	ErrInstanceNested          = errors.New("infra: instance config cannot contain instances/default_instance/no_default")
	ErrInstanceNotFound        = errors.New("infra: instance not found")
	ErrInstanceDecode          = errors.New("infra: instance config cannot be decoded")
	ErrInstancesEmpty          = errors.New("infra: no_default is set but no instances are configured")
	ErrDefaultInstanceConflict = errors.New("infra: no_default and default_instance are mutually exclusive")
	ErrDefaultInstanceNotFound = errors.New("infra: default_instance does not name a configured instance")
)

// normalizeInstanceName lowercases and trims a configured or requested name.
// viper lowercases configuration keys, so instance names must be normalized
// before they are compared with GetX(name).
func normalizeInstanceName(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// validateInstanceName rejects names that cannot be represented safely in a
// configuration path, a health component name or a metric label.
func validateInstanceName(name string) error {
	if name == "" {
		return ErrInstanceNameEmpty
	}

	if !instanceNamePattern.MatchString(name) {
		return fmt.Errorf("%w: %q", ErrInstanceNameInvalid, name)
	}

	return nil
}

// firstInstanceName normalizes the optional name argument used by GetX,
// ClearX and PingX. Only the first element is read; an absent or blank
// argument means "the shared singleton".
func firstInstanceName(names []string) string {
	if len(names) == 0 {
		return ""
	}

	return normalizeInstanceName(names[0])
}

// normalizeInstanceNames normalizes every key and returns the names in sorted
// order. A nil config or a duplicate normalized name is a configuration
// error rather than a silently dropped instance.
func normalizeInstanceNames[C any](raw map[string]*C) (normalized map[string]*C, names []string, err error) {
	normalized = make(map[string]*C, len(raw))
	names = make([]string, 0, len(raw))

	for rawName, cfg := range raw {
		if cfg == nil {
			return nil, nil, fmt.Errorf("%w: %q", ErrInstanceDecode, rawName)
		}

		name := normalizeInstanceName(rawName)
		if verr := validateInstanceName(name); verr != nil {
			return nil, nil, fmt.Errorf("%w: %q", verr, rawName)
		}

		if _, ok := normalized[name]; ok {
			return nil, nil, fmt.Errorf("%w: %s", ErrInstanceNameDuplicate, name)
		}

		normalized[name] = cfg
		names = append(names, name)
	}

	slices.Sort(names)

	return normalized, names, nil
}

// defaultSelection is the resolved shared-singleton choice for one component.
type defaultSelection struct {
	// UseTopLevel reports that the block's own fields configure the shared
	// singleton.
	UseTopLevel bool
	// Instance is the named instance that becomes the shared singleton.
	Instance string
	// Ignored is the default_instance value that was ignored because the
	// top-level config is complete (or because the component has no
	// required identity, as Ristretto does).
	Ignored string
}

// resolveDefaultSelection applies the framework-wide selection rules:
//
//  1. no_default suppresses the singleton entirely.
//  2. A complete top-level config is the singleton; default_instance is
//     ignored.
//  3. Otherwise a named default_instance is used when present.
//  4. Otherwise the first instance in sorted order is used.
//  5. With no instances, the top-level identity error (if any) is returned.
func resolveDefaultSelection(
	noDefault bool,
	defaultInstance string,
	instanceNames []string,
	topLevelComplete bool,
	identityErr error,
) (defaultSelection, error) {
	if noDefault {
		if normalizeInstanceName(defaultInstance) != "" {
			return defaultSelection{}, ErrDefaultInstanceConflict
		}

		if len(instanceNames) == 0 {
			return defaultSelection{}, ErrInstancesEmpty
		}

		return defaultSelection{}, nil
	}

	explicit := normalizeInstanceName(defaultInstance)
	if explicit != "" {
		if topLevelComplete {
			return defaultSelection{UseTopLevel: true, Ignored: explicit}, nil
		}

		if err := validateInstanceName(explicit); err != nil {
			return defaultSelection{}, fmt.Errorf("default_instance: %w", err)
		}

		if !slices.Contains(instanceNames, explicit) {
			return defaultSelection{}, fmt.Errorf("%w: %s", ErrDefaultInstanceNotFound, explicit)
		}

		return defaultSelection{Instance: explicit}, nil
	}

	if topLevelComplete {
		return defaultSelection{UseTopLevel: true}, nil
	}

	switch {
	case len(instanceNames) == 1:
		return defaultSelection{Instance: instanceNames[0]}, nil
	case len(instanceNames) > 1:
		return defaultSelection{Instance: instanceNames[0]}, nil
	default:
		if identityErr != nil {
			return defaultSelection{}, identityErr
		}

		return defaultSelection{}, nil
	}
}

// badgerInstance carries the resolved directory with the open handle. Badger
// locks that directory, so the registry must remember it to detect a new
// instance that would collide with an already-open one.
type badgerInstance struct {
	db   *badger.DB
	path string
}

// s3Instance carries the per-instance bucket used by health checks.
type s3Instance struct {
	client *s3.Client
	bucket string
}

// mqttInstance carries the effective client_id used at connect time. The id
// may be generated when the config leaves it empty, so the registry is the
// only place that still knows which broker session this client occupies.
type mqttInstance struct {
	client   mqtt.Client
	clientID string
}

// mongoInstance carries the effective database resolved from MongoConfig.DB
// or the URI path. GetMongoDBOf reads it for named clients.
type mongoInstance struct {
	client *mongo.Client
	db     string
}

// Named-instance registries. The singleton globals (S3, Redis, ...) remain in
// infra.go and their component files.
var (
	badgerInstances     = newInstanceSet[badgerInstance]()
	bunInstances        = newInstanceSet[*bun.DB]()
	clickHouseInstances = newInstanceSet[*ch.DB]()
	elasticInstances    = newInstanceSet[*elasticsearch.Client]()
	mongoInstances      = newInstanceSet[mongoInstance]()
	mqttInstances       = newInstanceSet[mqttInstance]()
	natsInstances       = newInstanceSet[*nats.Conn]()
	redisInstances      = newInstanceSet[*redis.Client]()
	ristrettoInstances  = newInstanceSet[*ristretto.Cache[string, any]]()
	s3Instances         = newInstanceSet[s3Instance]()
)

// instanceLabel returns the metrics label for a named instance's operation
// hooks. A named default keeps the component label so existing per-component
// dashboards survive default_instance; the component:name series still exists
// for init/up/health.
func instanceLabel(component, name string, selection defaultSelection) string {
	if !selection.UseTopLevel && selection.Instance == name {
		return component
	}

	return component + ":" + name
}

// defaultNeedsClose reports whether the shared singleton must be closed
// separately from the named instances. When it aliases a named instance, that
// loop already closes it; closing it again would be a double close.
func defaultNeedsClose[T any](aliasName string, instances map[string]T) bool {
	if aliasName == "" {
		return true
	}

	_, ok := instances[aliasName]

	return !ok
}

// Default-alias names record which named instance is also the shared
// singleton. They are guarded by mu and are never set for Ristretto: its
// top-level config is always the singleton.
var (
	badgerDefaultInstance     string
	bunDefaultInstance        string
	clickHouseDefaultInstance string
	elasticDefaultInstance    string
	mongoDefaultInstance      string
	mqttDefaultInstance       string
	natsDefaultInstance       string
	redisDefaultInstance      string
	ristrettoDefaultInstance  string
	s3DefaultInstance         string
)

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
