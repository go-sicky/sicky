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
 * @since 09/15/2024
 */

package grpc

const (
	// DefaultEndpoint is a grpc constant.
	DefaultEndpoint = "127.0.0.1:4317"
	// DefaultServiceName is a grpc constant.
	DefaultServiceName = "sicky"
	// DefaultServiceVersion is a grpc constant.
	DefaultServiceVersion = "latest"
	// DefaultSampleRate is a grpc constant.
	DefaultSampleRate = 1.0
)

// Config is a grpc component.
type Config struct {
	ServiceName    string            `json:"service_name"    mapstructure:"service_name"    yaml:"service_name"`
	ServiceVersion string            `json:"service_version" mapstructure:"service_version" yaml:"service_version"`
	Endpoint       string            `json:"endpoint"        mapstructure:"endpoint"        yaml:"endpoint"`
	Compress       bool              `json:"compress"        mapstructure:"compress"        yaml:"compress"`
	Timeout        int               `json:"timeout"         mapstructure:"timeout"         yaml:"timeout"`
	SampleRate     float64           `json:"sample_rate"     mapstructure:"sample_rate"     yaml:"sample_rate"`
	Insecure       bool              `json:"insecure"        mapstructure:"insecure"        yaml:"insecure"`
	Headers        map[string]string `json:"headers"         mapstructure:"headers"         yaml:"headers"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Endpoint:       DefaultEndpoint,
		Compress:       false,
		Timeout:        0,
		ServiceName:    DefaultServiceName,
		ServiceVersion: DefaultServiceVersion,
		SampleRate:     DefaultSampleRate,
		Insecure:       true,
	}
}

// Ensure fills zero-valued fields with defaults and returns the receiver (nil-safe).
func (c *Config) Ensure() *Config {
	if c == nil {
		c = DefaultConfig()
	}

	if c.Endpoint == "" {
		c.Endpoint = DefaultEndpoint
	}

	if c.ServiceName == "" {
		c.ServiceName = DefaultServiceName
	}

	if c.ServiceVersion == "" {
		c.ServiceVersion = DefaultServiceVersion
	}

	if c.SampleRate > 1.0 || c.SampleRate < 0.0 {
		c.SampleRate = DefaultSampleRate
	}

	return c
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
