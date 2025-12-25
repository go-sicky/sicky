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

	"github.com/go-sicky/sicky/utils"
	"github.com/google/uuid"
)

type PoolEvent struct {
	Changed bool
}

var currentPool *Pool

type Pool struct {
	Services map[string]*Service `json:"services" yaml:"services"`
	Notify   chan PoolEvent

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
	ServiceMame      string             `json:"service_name" yaml:"service_name"`
	AdvertiseAddress string             `json:"advertise_address" yaml:"advertise_address"`
	ManagerPort      int                `json:"manager_port" yaml:"manager_port"`
	Address          string             `json:"address" yaml:"address"`
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
	ID               string    `json:"id" yaml:"id"`
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
	currentPool = &Pool{
		Services: make(map[string]*Service),
		Notify:   make(chan PoolEvent, 1),
	}

	return currentPool
}

func (p *Pool) RegisterService(svc *Service) {
	if p == nil {
		return
	}

	p.Lock()
	defer p.Unlock()

	p.Services[svc.Service] = svc
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
	if p.Services[ins.ServiceMame] == nil {
		// Service not exists
	} else {
		if p.Services[ins.ServiceMame].Instances == nil {
			p.Services[ins.ServiceMame].Instances = make(map[uuid.UUID]*Instance)
		}

		// Register
		p.Services[ins.ServiceMame].Instances[ins.ID] = ins
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
