/*
 * The MIT License (MIT)
 *
 * Copyright (c) 2021 HereweTech Co.LTD
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

	"github.com/go-sicky/sicky/utils"
	"github.com/google/uuid"
)

var (
	Pool = make(map[string]*Service)
)

// Service definition
type Service struct {
	Name      string          `json:"name" yaml:"name"`
	Instances map[string]*Ins `json:"instances" yaml:"instances"`
}

// Service instance
type Ins struct {
	Name        string             `json:"name" yaml:"name"`
	ServiceName string             `json:"service_name" yaml:"service_name"`
	Registries  map[uuid.UUID]bool `json:"registries" yaml:"registries"`
	Addr        net.Addr           `json:"addr" yaml:"addr"`
	Metadata    utils.Metadata     `json:"metadata" yaml:"metadata"`
}

func RegisterInstance(ins *Ins, rg uuid.UUID) {
	if Pool[ins.ServiceName] == nil {
		Pool[ins.ServiceName] = &Service{
			Name:      ins.ServiceName,
			Instances: make(map[string]*Ins),
		}
	}

	// Check status
	if Pool[ins.ServiceName].Instances[ins.Name] == nil {
		ins.Registries = make(map[uuid.UUID]bool)
		Pool[ins.ServiceName].Instances[ins.Name] = ins
	}

	Pool[ins.ServiceName].Instances[ins.Name].Registries[rg] = true
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
