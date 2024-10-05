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
 * @file config.go
 * @package grpc
 * @author Dr.NP <np@herewe.tech>
 * @since 11/21/2023
 */

package grpc

import "time"

const (
	DefaultNetwork = "tcp"
	DefaultAddr    = "127.0.0.1:9991"
)

type Config struct {
	Name              string        `json:"name" yaml:"name" mapstructure:"name"`
	Network           string        `json:"network" yaml:"network" mapstructure:"network"`
	Addr              string        `json:"addr" yaml:"addr" mapstructure:"addr"`
	Timeout           time.Duration `json:"timeout" yaml:"timeout" mapstructure:"timeout"`
	MaxHeaderListSize uint32        `json:"max_header_list_size" yaml:"max_header_list_size" mapstructure:"max_header_list_size"`
	MaxMsgSize        int           `json:"max_msg_size" yaml:"max_msg_size" mapstructure:"max_msg_size"`
	ReadBufferSize    int           `json:"read_buffer_size" yaml:"read_buffer_size" mapstructure:"read_buffer_size"`
	WriteBufferSize   int           `json:"write_buffer_size" yaml:"write_buffer_size" mapstructure:"write_buffer_size"`
}

func DefaultConfig(name string) *Config {
	return &Config{
		Name:    name,
		Network: DefaultNetwork,
		Addr:    DefaultAddr,
	}
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
