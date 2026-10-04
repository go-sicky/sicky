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
 * @file pool.go
 * @package registry
 * @author Dr.NP <np@herewe.tech>
 * @since 09/22/2024
 */

package registry

import (
	"sync"

	"github.com/google/uuid"

	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/utils"
)

// PoolEvent is a registry component.
type PoolEvent struct {
	Changed bool
}

var (
	currentPool *Pool
	poolLock    sync.RWMutex
)

// Pool definition

// Pool is a registry component.
type Pool struct {
	Services map[string]*Service `json:"services" yaml:"services"`
	Notify   chan PoolEvent      `json:"-"        yaml:"-"`

	sync.RWMutex
}

// Service definition.
type Service struct {
	Service   string                  `json:"service"   yaml:"service"`
	Kind      string                  `json:"kind"      yaml:"kind"`
	Self      bool                    `json:"self"      yaml:"self"`
	Tags      []string                `json:"tags"      yaml:"tags"`
	Metadata  utils.Metadata          `json:"metadata"  yaml:"metadata"`
	Instances map[uuid.UUID]*Instance `json:"instances" yaml:"instances"`
}

// Service instance.
type Instance struct {
	ID               uuid.UUID          `json:"id"                yaml:"id"`
	ServiceName      string             `json:"service_name"      yaml:"service_name"`
	Type             string             `json:"type"              yaml:"type"`
	AdvertiseAddress string             `json:"advertise_address" yaml:"advertise_address"`
	ManagerPort      int                `json:"manager_port"      yaml:"manager_port"`
	ManagerAddress   string             `json:"manager_address"   yaml:"manager_address"`
	Tags             []string           `json:"tags"              yaml:"tags"`
	Metadata         utils.Metadata     `json:"metadata"          yaml:"metadata"`
	Weight           int                `json:"weight"            yaml:"weight"`
	Status           int                `json:"status"            yaml:"status"`
	CheckEntryPoint  string             `json:"check_entry_point" yaml:"check_entry_point"`
	TTL              int                `json:"ttl"               yaml:"ttl"`
	Servers          map[string]*Server `json:"servers"           yaml:"servers"`
	Topics           map[string]*Topic  `json:"topics"            yaml:"topics"`
}

// Server is a registry component.
type Server struct {
	ID               uuid.UUID `json:"id"                yaml:"id"`
	InstanceID       uuid.UUID `json:"instance_id"       yaml:"instance_id"`
	Type             string    `json:"type"              yaml:"type"`
	Name             string    `json:"name"              yaml:"name"`
	AdvertiseAddress string    `json:"advertise_address" yaml:"advertise_address"`
	Port             int       `json:"port"              yaml:"port"`
}

// Topic is a registry component.
type Topic struct {
	Instance *Instance `json:"instance" yaml:"instance"`
	Name     string    `json:"name"     yaml:"name"`
	Type     string    `json:"type"     yaml:"type"`
	Group    string    `json:"group"    yaml:"group"`
}

// Init pool.
func InitPool() *Pool {
	poolLock.Lock()
	defer poolLock.Unlock()

	if currentPool != nil {
		// Idempotent: a second Run() must not swap the Notify channel out
		// from under watchers that captured it via NotifyChan() (the gRPC
		// client resolver holds one for the process lifetime), while the
		// previous run's services must not leak into the new one.
		//
		// The pool's own mutex is required here too. Every other writer
		// (PurgePool) and every reader (GetPool, and the exported Pool
		// methods, which take only p.RWMutex) guards Services with it, so
		// resetting the map without it raced any in-flight reader holding
		// p.RLock() — reachable because InitPool hands out the live pool.
		currentPool.Lock()
		currentPool.Services = make(map[string]*Service)
		currentPool.Unlock()

		return currentPool
	}

	currentPool = &Pool{
		Services: make(map[string]*Service),
		Notify:   make(chan PoolEvent, 1),
	}

	return currentPool
}

// Deprecated: creates a pool disconnected from the global discovery state.
// Prefer InitPool, which installs the global pool and preserves the stable
// NotifyChan contract relied upon by watchers (e.g. the gRPC client).
func NewPool() *Pool {
	return &Pool{
		Services: make(map[string]*Service),
		Notify:   make(chan PoolEvent, 1),
	}
}

// GetPool returns a snapshot copy of the discovery pool.
func GetPool() *Pool {
	poolLock.RLock()
	defer poolLock.RUnlock()

	if currentPool == nil {
		return nil
	}

	// Return a deep-copy snapshot so readers (e.g. /services handler)
	// never race with pool writers.
	currentPool.RLock()
	defer currentPool.RUnlock()

	return currentPool.clone()
}

// Deprecated: replacing the pool swaps the Notify channel out from under
// existing watchers. Prefer PurgePool, which merges in place and keeps the
// channel (and pool pointer) stable.
func SetPool(p *Pool) {
	poolLock.Lock()
	defer poolLock.Unlock()

	currentPool = p
}

// NotifyChan returns the pool update notification channel, or nil if the
// pool is not initialized yet. The channel is stable across PurgePool calls
// (the pool pointer and its Notify channel are preserved), so holders never
// need to re-subscribe on purge; only InitPool/SetPool replace it.
func NotifyChan() <-chan PoolEvent {
	poolLock.RLock()
	defer poolLock.RUnlock()

	if currentPool == nil {
		return nil
	}

	return currentPool.Notify
}

// RegisterService registers a service in the pool.
func (p *Pool) RegisterService(svc *Service) {
	if p == nil {
		return
	}

	p.Lock()
	defer p.Unlock()

	// Deep copy, like every other pool write. Storing the caller's pointer
	// let a caller that reused its Service — or the Instance values inside
	// it — mutate what GetService, GetInstances and GetPool handed out,
	// concurrently with readers holding p.RLock().
	p.Services[svc.Service] = cloneService(svc)
	logger.Debug("Service registered", "service", svc.Service)
}

// GetService looks up a service by name.
// The result is a deep copy: mutating it never races with pool writers.
func (p *Pool) GetService(service string) *Service {
	if p == nil {
		return nil
	}

	p.RLock()
	defer p.RUnlock()

	return cloneService(p.Services[service])
}

// RegisterInstance registers an instance.
// RegisterInstance puts an instance into the pool directly.
//
// The registration is temporary: PurgePool replaces the whole pool from
// the backend's Load() result, and an entry that is not part of that
// result is dropped at the next discovery refresh. The instance is stored
// as a deep copy.
func (p *Pool) RegisterInstance(ins *Instance) {
	if ins == nil {
		return
	}

	p.Lock()
	defer p.Unlock()

	// Check service
	if p.Services[ins.ServiceName] == nil {
		// Service not exists
		logger.Warn("Try to register instance to non exist service", "service", ins.ServiceName, "instance", ins.ID.String())
	} else {
		if p.Services[ins.ServiceName].Instances == nil {
			p.Services[ins.ServiceName].Instances = make(map[uuid.UUID]*Instance)
		}

		// Register
		p.Services[ins.ServiceName].Instances[ins.ID] = cloneInstance(ins)
		logger.Debug("Instance registered", "service", ins.ServiceName, "instance", ins.ID.String())
	}
}

// GetInstance looks up an instance.
// The result is a deep copy: mutating it never races with pool writers.
func (p *Pool) GetInstance(service string, id uuid.UUID) *Instance {
	p.RLock()
	defer p.RUnlock()

	if p.Services[service] == nil {
		return nil
	}

	return cloneInstance(p.Services[service].Instances[id])
}

// UnregisterInstance removes an instance.
func (p *Pool) UnregisterInstance(service string, id uuid.UUID) {
	p.Lock()
	defer p.Unlock()

	if p.Services[service] == nil {
		return
	}

	delete(p.Services[service].Instances, id)
	logger.Debug("Instance unregistered", "service", service, "instance", id.String())
}

// GetInstances lists instances of a service.
func (p *Pool) GetInstances(service string) map[uuid.UUID]*Instance {
	p.RLock()
	defer p.RUnlock()

	s, ok := p.Services[service]
	if ok && s.Instances != nil {
		out := make(map[uuid.UUID]*Instance, len(s.Instances))
		for id, in := range s.Instances {
			out[id] = cloneInstance(in)
		}

		return out
	}

	return nil
}

/* {{{ [Helpers]. */
// RegisterInstance registers an instance directly in the global pool.
// The entry is temporary - the next PurgePool replaces the pool - and is
// stored as a deep copy (see Pool.RegisterInstance).
func RegisterInstance(ins *Instance) {
	poolLock.Lock()
	defer poolLock.Unlock()

	if currentPool == nil {
		return
	}

	currentPool.RegisterInstance(ins)
}

// GetInstance looks up an instance.
func GetInstance(service string, id uuid.UUID) *Instance {
	poolLock.RLock()
	defer poolLock.RUnlock()

	if currentPool == nil {
		return nil
	}

	return currentPool.GetInstance(service, id)
}

// UnregisterInstance removes an instance.
func UnregisterInstance(service string, id uuid.UUID) {
	poolLock.Lock()
	defer poolLock.Unlock()

	if currentPool == nil {
		return
	}

	currentPool.UnregisterInstance(service, id)
}

// GetInstances lists instances of a service.
func GetInstances(service string) map[uuid.UUID]*Instance {
	poolLock.RLock()
	defer poolLock.RUnlock()

	if currentPool == nil {
		return nil
	}

	return currentPool.GetInstances(service)
}

// RegisterService registers a service in the pool.
func RegisterService(svc *Service) {
	poolLock.Lock()
	defer poolLock.Unlock()

	if currentPool == nil {
		return
	}

	currentPool.RegisterService(svc)
}

// GetService looks up a service by name.
func GetService(service string) *Service {
	poolLock.RLock()
	defer poolLock.RUnlock()

	if currentPool == nil {
		return nil
	}

	return currentPool.GetService(service)
}

/* }}} */

// PurgePool replaces the pool contents with the given instances.
//
// It is a replace, not a merge: entries registered through
// RegisterInstance that are not part of ins disappear at the next purge,
// and every instance is stored as a deep copy so callers may keep and
// mutate their own without racing the readers (GetPool/GetInstances
// return copies of these).
func PurgePool(ins []*Instance) {
	// Build the replacement map before taking poolLock. cloneInstance
	// allocates a fresh Instance plus its Servers and Topics maps for
	// every entry, so holding the package-global write lock across the
	// rebuild blocks every pool reader (GetPool, GetInstance,
	// GetInstances, GetService) for the whole pass: a 1k-instance
	// cluster turns one discovery refresh into ~10k allocations during
	// which discovery is unavailable. Building outside the lock is safe
	// because ins is caller-owned (see the deep-copy note below) and
	// nothing below reads or writes currentPool until the swap.
	services := make(map[string]*Service, len(ins))
	for _, in := range ins {
		if in == nil {
			continue
		}

		svc := services[in.ServiceName]
		if svc == nil {
			svc = &Service{
				Service:   in.ServiceName,
				Instances: make(map[uuid.UUID]*Instance),
			}

			services[in.ServiceName] = svc
		}

		if svc.Instances == nil {
			svc.Instances = make(map[uuid.UUID]*Instance)
		}

		// Deep copy: the caller owns its Instance (it may reuse it for
		// the next registration) and the pool hands out copies of ours.
		svc.Instances[in.ID] = cloneInstance(in)
	}

	// Swap in place so the *Pool pointer (and its Notify channel) stays
	// stable for existing holders that already captured it.
	poolLock.Lock()
	defer poolLock.Unlock()

	if currentPool == nil {
		currentPool = NewPool()
	}

	currentPool.Lock()
	currentPool.Services = services
	notify := currentPool.Notify
	currentPool.Unlock()

	select {
	case notify <- PoolEvent{Changed: true}:
	default:
		// No listener or buffer full: never block the purge path.
	}
}

// clone deep-copies the pool for race-free snapshots.
// Caller must hold at least p.RLock().
func (p *Pool) clone() *Pool {
	if p == nil {
		return nil
	}

	out := &Pool{
		Services: make(map[string]*Service, len(p.Services)),
	}

	for name, svc := range p.Services {
		if svc == nil {
			continue
		}

		out.Services[name] = cloneService(svc)
	}

	return out
}

func cloneMetadata(md utils.Metadata) utils.Metadata {
	if md == nil {
		return nil
	}

	return md.Copy()
}

func cloneService(svc *Service) *Service {
	if svc == nil {
		return nil
	}

	dup := &Service{
		Service:   svc.Service,
		Kind:      svc.Kind,
		Self:      svc.Self,
		Tags:      append([]string(nil), svc.Tags...),
		Metadata:  cloneMetadata(svc.Metadata),
		Instances: make(map[uuid.UUID]*Instance, len(svc.Instances)),
	}

	for id, in := range svc.Instances {
		dup.Instances[id] = cloneInstance(in)
	}

	return dup
}

func cloneInstance(in *Instance) *Instance {
	if in == nil {
		return nil
	}

	out := &Instance{
		ID:               in.ID,
		ServiceName:      in.ServiceName,
		Type:             in.Type,
		AdvertiseAddress: in.AdvertiseAddress,
		ManagerPort:      in.ManagerPort,
		ManagerAddress:   in.ManagerAddress,
		Tags:             append([]string(nil), in.Tags...),
		Metadata:         cloneMetadata(in.Metadata),
		Weight:           in.Weight,
		Status:           in.Status,
		CheckEntryPoint:  in.CheckEntryPoint,
		TTL:              in.TTL,
		Servers:          make(map[string]*Server, len(in.Servers)),
		Topics:           make(map[string]*Topic, len(in.Topics)),
	}

	for name, srv := range in.Servers {
		if srv == nil {
			continue
		}

		dup := *srv
		out.Servers[name] = &dup
	}

	for name, tp := range in.Topics {
		if tp == nil {
			continue
		}

		dup := *tp
		// No back-reference: Instance->Topics->Instance would form a JSON
		// cycle (the source instance itself is a cyclic in-memory graph)
		// that breaks /services and every registry backend.
		dup.Instance = nil
		out.Topics[name] = &dup
	}

	return out
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
