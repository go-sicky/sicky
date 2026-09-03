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

	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/utils"
	"github.com/google/uuid"
)

type PoolEvent struct {
	Changed bool
}

var (
	currentPool *Pool
	poolLock    sync.RWMutex
)

// Pool definition

type Pool struct {
	Services map[string]*Service `json:"services" yaml:"services"`
	Notify   chan PoolEvent      `json:"-" yaml:"-"`

	sync.RWMutex
}

// Service definition
type Service struct {
	Service   string                  `json:"service" yaml:"service"`
	Kind      string                  `json:"kind" yaml:"kind"`
	Self      bool                    `json:"self" yaml:"self"`
	Tags      []string                `json:"tags" yaml:"tags"`
	Metadata  utils.Metadata          `json:"metadata" yaml:"metadata"`
	Instances map[uuid.UUID]*Instance `json:"instances" yaml:"instances"`
}

// Service instance
type Instance struct {
	ID               uuid.UUID          `json:"id" yaml:"id"`
	ServiceName      string             `json:"service_name" yaml:"service_name"`
	Type             string             `json:"type" yaml:"type"`
	AdvertiseAddress string             `json:"advertise_address" yaml:"advertise_address"`
	ManagerPort      int                `json:"manager_port" yaml:"manager_port"`
	ManagerAddress   string             `json:"manager_address" yaml:"manager_address"`
	Tags             []string           `json:"tags" yaml:"tags"`
	Metadata         utils.Metadata     `json:"metadata" yaml:"metadata"`
	Weight           int                `json:"weight" yaml:"weight"`
	Status           int                `json:"status" yaml:"status"`
	CheckEntryPoint  string             `json:"check_entry_point" yaml:"check_entry_point"`
	TTL              int                `json:"ttl" yaml:"ttl"`
	Servers          map[string]*Server `json:"servers" yaml:"servers"`
	Topics           map[string]*Topic  `json:"topics" yaml:"topics"`
}

type Server struct {
	ID               uuid.UUID `json:"id" yaml:"id"`
	InstanceID       uuid.UUID `json:"instance_id" yaml:"instance_id"`
	Type             string    `json:"type" yaml:"type"`
	Name             string    `json:"name" yaml:"name"`
	AdvertiseAddress string    `json:"advertise_address" yaml:"advertise_address"`
	Port             int       `json:"port" yaml:"port"`
}

type Topic struct {
	Instance *Instance `json:"instance" yaml:"instance"`
	Name     string    `json:"name" yaml:"name"`
	Type     string    `json:"type" yaml:"type"`
	Group    string    `json:"group" yaml:"group"`
}

// Init pool
func InitPool() *Pool {
	poolLock.Lock()
	defer poolLock.Unlock()

	currentPool = &Pool{
		Services: make(map[string]*Service),
		Notify:   make(chan PoolEvent, 1),
	}

	return currentPool
}

func NewPool() *Pool {
	return &Pool{
		Services: make(map[string]*Service),
		Notify:   make(chan PoolEvent, 1),
	}
}

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

func SetPool(p *Pool) {
	poolLock.Lock()
	defer poolLock.Unlock()

	currentPool = p
}

func (p *Pool) RegisterService(svc *Service) {
	if p == nil {
		return
	}

	p.Lock()
	defer p.Unlock()

	p.Services[svc.Service] = svc
	logger.Debug("Service registered", "service", svc.Service)
}

func (p *Pool) GetService(service string) *Service {
	if p == nil {
		return nil
	}

	p.RLock()
	defer p.RUnlock()

	return p.Services[service]
}

func (p *Pool) RegisterInstance(ins *Instance) {
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
		p.Services[ins.ServiceName].Instances[ins.ID] = ins
		logger.Debug("Instance registered", "service", ins.ServiceName, "instance", ins.ID.String())
	}
}

func (p *Pool) GetInstance(service string, id uuid.UUID) *Instance {
	p.RLock()
	defer p.RUnlock()

	if p.Services[service] == nil {
		return nil
	}

	return p.Services[service].Instances[id]
}

func (p *Pool) UnregisterInstance(service string, id uuid.UUID) {
	p.Lock()
	defer p.Unlock()

	if p.Services[service] == nil {
		return
	}

	delete(p.Services[service].Instances, id)
	logger.Debug("Instance unregistered", "service", service, "instance", id.String())
}

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

/* {{{ [Helpers] */
func RegisterInstance(ins *Instance) {
	poolLock.Lock()
	defer poolLock.Unlock()

	if currentPool == nil {
		return
	}

	currentPool.RegisterInstance(ins)
}

func GetInstance(service string, id uuid.UUID) *Instance {
	poolLock.RLock()
	defer poolLock.RUnlock()

	if currentPool == nil {
		return nil
	}

	return currentPool.GetInstance(service, id)
}

func UnregisterInstance(service string, id uuid.UUID) {
	poolLock.Lock()
	defer poolLock.Unlock()

	if currentPool == nil {
		return
	}

	currentPool.UnregisterInstance(service, id)
}

func GetInstances(service string) map[uuid.UUID]*Instance {
	poolLock.RLock()
	defer poolLock.RUnlock()

	if currentPool == nil {
		return nil
	}

	return currentPool.GetInstances(service)
}

func RegisterService(svc *Service) {
	poolLock.Lock()
	defer poolLock.Unlock()

	if currentPool == nil {
		return
	}

	currentPool.RegisterService(svc)
}

func GetService(service string) *Service {
	poolLock.RLock()
	defer poolLock.RUnlock()

	if currentPool == nil {
		return nil
	}

	return currentPool.GetService(service)
}

/* }}} */

func PurgePool(ins []*Instance) {
	poolLock.Lock()
	defer poolLock.Unlock()

	if currentPool == nil {
		currentPool = NewPool()
	}

	// Rebuild the services map in place so the *Pool pointer (and its
	// Notify channel) stays stable for existing holders.
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
		svc.Instances[in.ID] = in
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
		out.Services[name] = dup
	}

	return out
}

func cloneMetadata(md utils.Metadata) utils.Metadata {
	if md == nil {
		return nil
	}

	return md.Copy()
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
		dup.Instance = out
		out.Topics[name] = &dup
	}

	return out
}

// func GetInstances(service string) map[string]*Instance {
// 	poolLock.Lock()
// 	defer poolLock.Unlock()

// 	s, ok := Pool[service]
// 	if ok && s.Instances != nil {
// 		return s.Instances
// 	}

// 	return nil
// }

// func PurgeInstances() {
// 	poolLock.Lock()
// 	defer poolLock.Unlock()

// 	for service, svc := range Pool {
// 		if service != svc.Service {
// 			delete(Pool, service)
// 		}

// 		for id, ins := range svc.Instances {
// 			if ins.ID != id {
// 				delete(Pool[service].Instances, id)
// 			}

// 			// Check instance
// 			exists := false
// 			for _, rg := range registries {
// 				if rg.CheckInstance(ins.ID) {
// 					exists = true
// 					break
// 				}
// 			}

// 			if !exists {
// 				// Remove instance
// 				delete(Pool[service].Instances, id)
// 			}
// 		}

// 		if len(svc.Instances) == 0 {
// 			delete(Pool, service)
// 		}
// 	}

// 	select {
// 	case PoolChan <- PoolEvent{Changed: true}:
// 	default:
// 		// Just ignore
// 	}

// 	logger.Logger.Debug(
// 		"registry pool purged",
// 	)
// }

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
