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
 * @package uptrace
 * @author Dr.NP <np@herewe.tech>
 * @since 12/30/2025
 */

package uptrace

const (
	DefaultServiceName    = "sicky"
	DefaultServiceVersion = "latest"
	DefaultSampleRate     = 1.0
)

type Config struct {
	DSN            string `json:"dsn" yaml:"dsn" mapstructure:"dsn"`
	ServiceName    string `json:"service_name" yaml:"service_name" mapstructure:"service_name"`
	ServiceVersion string `json:"service_version" yaml:"service_version" mapstructure:"service_version"`
	// SampleRate is deprecated: sampling is controlled server-side by
	// Uptrace. Kept for config compatibility; New() warns and ignores it.
	SampleRate float64 `json:"sample_rate" yaml:"sample_rate" mapstructure:"sample_rate"`
}

func DefaultConfig() *Config {
	return &Config{
		ServiceName:    DefaultServiceName,
		ServiceVersion: DefaultServiceVersion,
		SampleRate:     DefaultSampleRate,
	}
}

func (c *Config) Ensure() *Config {
	if c == nil {
		c = DefaultConfig()
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
