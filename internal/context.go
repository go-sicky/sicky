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
 * @file context.go
 * @package internal
 * @author Dr.NP <np@herewe.tech>
 * @since 09/18/2024
 */

package internal

import (
	"context"
	"time"

	"github.com/go-sicky/sicky/broker"
	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/registry"
	"github.com/go-sicky/sicky/tracer"
	"github.com/go-sicky/sicky/utils"
	"github.com/google/uuid"
)

type Context struct {
	Base context.Context

	ID        uuid.UUID
	AppName   string
	Version   string
	Branch    string
	Metadata  utils.Metadata
	StartTime time.Time

	DefaultBroker   broker.Broker
	DefaultRegistry registry.Registry
	DefaultTracer   tracer.Tracer
	DefaultLogger   logger.GeneralLogger
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
