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
	"cmp"
	"context"
	"maps"
	"slices"
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

	// regSeq is the monotonic registration counter behind Ordered. It is
	// bumped once per accepted service, never on a rejected one, so the
	// sequence reflects the order services were actually registered in.
	regSeq uint64
	// regOrder maps an ID to its registration sequence. It lives beside
	// services so the two are always written together under svcMu.
	regOrder = make(map[uuid.UUID]uint64)
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

		// Record the registration order. Callers that need a dependency
		// between services depend on the order they call Set in, not on
		// map iteration, which Go randomizes per range.
		regSeq++
		regOrder[svc.Options().ID] = regSeq

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
//
// The map is unordered, so iterating it yields a different order on every
// pass; use Ordered when the sequence matters.
func Services() map[uuid.UUID]Service {
	svcMu.RLock()
	defer svcMu.RUnlock()

	out := make(map[uuid.UUID]Service, len(services))
	maps.Copy(out, services)

	return out
}

// Ordered returns the registered services in the order they were passed to
// Set. The lifecycle uses it so a service that was registered after one it
// depends on starts after it and stops before it; the registry keeps no
// explicit dependency edges, so registration order is the contract.
//
// Set is the only source of a sequence, so the order is exactly the order of
// the Set calls. A service that Set rejected (nil, or an already-registered
// ID) keeps the position of the registration that was accepted, and a
// Clear resets the sequence so a restart renumbers from the beginning.
func Ordered() []Service {
	svcMu.RLock()
	defer svcMu.RUnlock()

	out := make([]Service, 0, len(services))
	for _, svc := range services {
		out = append(out, svc)
	}

	slices.SortFunc(out, func(a, b Service) int {
		return cmp.Compare(regOrder[a.Options().ID], regOrder[b.Options().ID])
	})

	return out
}

// Clear resets the global registry. Intended for tests and restart flows.
func Clear() {
	svcMu.Lock()
	defer svcMu.Unlock()

	services = make(map[uuid.UUID]Service)
	defaultService = nil

	// Reset the registration sequence too: leaving it running would let a
	// service registered after a Clear sort ahead of one registered
	// before it, which is the opposite of what a restart expects.
	regOrder = make(map[uuid.UUID]uint64)
	regSeq = 0
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
