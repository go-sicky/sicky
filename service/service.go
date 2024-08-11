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
 * @file service.go
 * @package service
 * @author Dr.NP <np@herewe.tech>
 * @since 08/01/2024
 */

package service

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/server"
)

type Service interface {
	// Get context
	Context() context.Context
	// Service options
	Options() *Options
	// Stringify
	String() string
	// Register servers
	Servers(...server.Server) Service
	// Start service
	Start() error
	// Stop service
	Stop() error
}

var (
	Instance Service
)

func Run() error {
	var (
		err, gerr error
	)

	if Instance == nil {
		logger.Fatal("No service initialized")
	}

	logger.InfoContext(
		Instance.Context(),
		"Startring service",
		"service", Instance.String(),
		"id", Instance.Options().ID,
		"name", Instance.Options().Name,
		"version", Instance.Options().Version,
		"branch", Instance.Options().Branch,
	)

	// Start service
	err = Instance.Start()
	if err != nil {
		logger.ErrorContext(
			Instance.Context(),
			"Service start failed",
			"error", err.Error(),
		)

		return err
	}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGHUP, syscall.SIGABRT)
	select {
	case <-ch:
	case <-Instance.Context().Done():
	}

	logger.InfoContext(
		Instance.Context(),
		"Stopping service",
		"service", Instance.String(),
		"id", Instance.Options().ID,
		"name", Instance.Options().Name,
		"version", Instance.Options().Version,
		"branch", Instance.Options().Branch,
	)

	// Stop services
	err = Instance.Stop()
	if err != nil {
		logger.ErrorContext(
			Instance.Context(),
			"Service stop failed",
			"error", err.Error(),
		)

		return err
	}

	return gerr
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
