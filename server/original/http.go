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
 * @file http.go
 * @package original
 * @author Dr.NP <np@herewe.tech>
 * @since 08/27/2025
 */

package original

import (
	"github.com/go-sicky/sicky/server"
)

/* {{{ [Server] */
type HTTPServer struct {
	// config        *Config
	// ctx           context.Context
	// options       *server.Options
	// running       bool
	// addr          net.Addr
	// advertiseAddr net.Addr
	// metadata      utils.Metadata

	// sync.RWMutex
	// wg sync.WaitGroup
}

// New HTTP server (net/http)
func New(opts *server.Options, cfg *Config) *HTTPServer {
	// opts = opts.Ensure()
	// cfg = cfg.Ensure()

	// var (
	// 	addr          net.Addr
	// 	advertiseAddr net.Addr
	// 	err           error
	// )

	return nil
}

/* }}} */

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
