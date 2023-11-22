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
 * @file config.go
 * @package sicky
 * @author Dr.NP <np@herewe.tech>
 * @since 11/21/2023
 */

package sicky

import (
	cgrpc "github.com/go-sicky/sicky/client/grpc"
	sgrpc "github.com/go-sicky/sicky/server/grpc"
	shttp "github.com/go-sicky/sicky/server/http"
	swebsocket "github.com/go-sicky/sicky/server/websocket"
)

type ConfigService struct {
	Name    string `json:"name" yaml:"name" mapstructure:"name"`
	Version string `json:"version" yaml:"version" mapstructure:"version"`
}

type ConfigGlobal struct {
	Sicky struct {
		Service ConfigService `json:"service" yaml:"service" mapstructure:"service"`
		Servers struct {
			HTTP      map[string]shttp.Config      `json:"http" yaml:"http" mapstructure:"http"`
			GRPC      map[string]sgrpc.Config      `json:"grpc" yaml:"grpc" mapstructure:"grpc"`
			Websocket map[string]swebsocket.Config `json:"websocket" yaml:"websocket" mapstructure:"websocket"`
		} `json:"servers" yaml:"servers" mapstructure:"servers"`
		Clients struct {
			GRPC map[string]cgrpc.Config `json:"grpc" yaml:"grpc" mapstructure:"grpc"`
		} `json:"clients" yaml:"clients" mapstructure:"clients"`
		Debug bool `json:"debug" yaml:"debug" mapstructure:"debug"`
	} `json:"sicky" yaml:"sicky" mapstructure:"sicky"`
	App interface{} `json:"app" yaml:"app" mapstructure:"app"`
}

//var defaultConfig = map[string]interface{}{}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
