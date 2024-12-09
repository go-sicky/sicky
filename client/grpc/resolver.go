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
 * @file resolver.go
 * @package grpc
 * @author Dr.NP <np@herewe.tech>
 * @since 12/04/2024
 */

package grpc

import (
	"fmt"

	"google.golang.org/grpc/resolver"
)

// Resolver
/* {{{ [sickyGRPCResolver] */
func sickyResolveNow(rno resolver.ResolveNowOptions) {
	fmt.Println("Update Resolver")
}

func sickyUpdateState(err error) {
	fmt.Println(err)
}

func sickyBuild(rt resolver.Target, rcc resolver.ClientConn, rbo resolver.BuildOptions) {
	fmt.Println("URL", rt.URL)
	fmt.Println("Endpoint", rt.Endpoint())
	fmt.Println("String", rt.String())
}

func sickyClose() {
	fmt.Println("Resolver closed")
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
