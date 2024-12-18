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
	"net"
	"sync"

	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/utils"
)

type PoolEvent struct {
	Changed bool
}

var (
	Pool     = make(map[string]*Service)
	PoolChan = make(chan PoolEvent)
	poolLock sync.RWMutex
)

// Service definition
type Service struct {
	Service   string          `json:"service" yaml:"service"`
	Instances map[string]*Ins `json:"instances" yaml:"instances"`
}

// Service instance
type Ins struct {
	ID       string         `json:"id" yaml:"id"`
	Service  string         `json:"service" yaml:"service"`
	Addr     net.Addr       `json:"addr" yaml:"addr"`
	Metadata utils.Metadata `json:"metadata" yaml:"metadata"`
}

func RegisterInstance(ins *Ins) {
	poolLock.Lock()
	if Pool[ins.Service] == nil {
		Pool[ins.Service] = &Service{
			Service:   ins.Service,
			Instances: make(map[string]*Ins),
		}
	}

	// Check status
	if Pool[ins.Service].Instances[ins.ID] == nil {
		Pool[ins.Service].Instances[ins.ID] = ins
	}

	poolLock.Unlock()
}

func GetInstances(service string) map[string]*Ins {
	poolLock.Lock()
	defer poolLock.Unlock()

	s, ok := Pool[service]
	if ok && s.Instances != nil {
		return s.Instances
	}

	return nil
}

func PurgeInstances() {
	poolLock.Lock()
	for service, svc := range Pool {
		if service != svc.Service {
			delete(Pool, service)
		}

		for id, ins := range svc.Instances {
			if ins.ID != id {
				delete(Pool[service].Instances, id)
			}

			// Check instance
			exists := false
			for _, rg := range registries {
				if rg.CheckInstance(ins.ID) {
					exists = true
					break
				}
			}

			if !exists {
				// Remove instance
				delete(Pool[service].Instances, id)
			}
		}

		if len(svc.Instances) == 0 {
			delete(Pool, service)
		}
	}

	select {
	case PoolChan <- PoolEvent{Changed: true}:
	default:
		// Just ignore
	}

	poolLock.Unlock()
	logger.Logger.Trace(
		"registry pool purged",
	)
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
