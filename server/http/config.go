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
 * @package http
 * @author Dr.NP <np@herewe.tech>
 * @since 11/21/2023
 */

package http

const (
	DefaultNetwork = "tcp"
	DefaultAddr    = ":9990"
)

type Config struct {
	Name             string `json:"name" yaml:"name" mapstructure:"name"`
	Network          string `json:"network" yaml:"network" mapstructure:"network"`
	Addr             string `json:"addr" yaml:"addr" mapstructure:"addr"`
	StrictRouting    bool   `json:"strict_routing" yaml:"strict_routing" mapstructure:"strict_routing"`
	CaseSensitive    bool   `json:"case_sensitive" yaml:"case_sensitive" mapstructure:"case_sensitive"`
	Etag             bool   `json:"etag" yaml:"etag" mapstructure:"etag"`
	BodyLimit        int    `json:"body_limit" yaml:"body_limit" mapstructure:"body_limit"`
	Concurrency      int    `json:"concurrency" yaml:"concurrency" mapstructure:"concurrency"`
	ReadBufferSize   int    `json:"read_buffer_size" yaml:"read_buffer_size" mapstructure:"read_buffer_size"`
	WriteBufferSize  int    `json:"write_buffer_size" yaml:"write_buffer_size" mapstructure:"write_buffer_size"`
	DisableKeepAlive bool   `json:"disable_keep_alive" yaml:"disable_keep_alive" mapstructure:"disable_keep_alive"`
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
