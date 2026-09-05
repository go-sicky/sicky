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
 * @file service.go
 * @package service
 * @author Dr.NP <np@herewe.tech>
 * @since 08/01/2024
 */

package service

import (
	"context"
	"maps"
	"sync"

	"github.com/google/uuid"

	"github.com/go-sicky/sicky/broker"
	"github.com/go-sicky/sicky/job"
	"github.com/go-sicky/sicky/registry"
	"github.com/go-sicky/sicky/server"
	"github.com/go-sicky/sicky/tracer"
)

// Service is a service component.
type Service interface {
	// Get context
	Context() context.Context
	// Service options
	Options() *Options
	// Stringify
	String() string
	// Start service
	Start() []error
	// Stop service
	Stop() []error

	// Subordinates
	Servers(srvs ...server.Server) []server.Server
	Brokers(brks ...broker.Broker) []broker.Broker
	Jobs(jobs ...job.Job) []job.Job
	Registries(rgts ...registry.Registry) []registry.Registry
	Tracers(trcs ...tracer.Tracer) []tracer.Tracer
}

var (
	services       = make(map[uuid.UUID]Service)
	defaultService Service
	svcMu          sync.RWMutex
)

// Set registers service instances; the first one becomes the default.
func Set(svcs ...Service) {
	svcMu.Lock()
	defer svcMu.Unlock()

	for _, svc := range svcs {
		if svc == nil || svc.Options() == nil {
			continue
		}

		if _, exists := services[svc.Options().ID]; exists {
			continue
		}

		services[svc.Options().ID] = svc
		if defaultService == nil {
			defaultService = svc
		}
	}
}

// Get looks up a service instance by ID.
func Get(id uuid.UUID) Service {
	svcMu.RLock()
	defer svcMu.RUnlock()

	return services[id]
}

// Default returns the default service instance.
func Default() Service {
	svcMu.RLock()
	defer svcMu.RUnlock()

	return defaultService
}

// Services returns a copy of the service registry.
func Services() map[uuid.UUID]Service {
	svcMu.RLock()
	defer svcMu.RUnlock()

	out := make(map[uuid.UUID]Service, len(services))
	maps.Copy(out, services)

	return out
}

// Clear resets the global registry. Intended for tests and restart flows.
func Clear() {
	svcMu.Lock()
	defer svcMu.Unlock()

	services = make(map[uuid.UUID]Service)
	defaultService = nil
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
