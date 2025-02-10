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
 * @package interactive
 * @author Dr.NP <np@herewe.tech>
 * @since 08/13/2024
 */

package interactive

const (
	DefaultStartupInfo = "\n @Sicky application\n"
	DefaultPrompt      = "SICKY> "
	DefaultPromptColor = "green"
	DefaultStopCommand = "exit"
)

type Config struct {
	StartupInfo           string `json:"startup_info" yaml:"startup_info" mapstructure:"startup_info"`
	Prompt                string `json:"prompt" yaml:"prompt" mapstructure:"prompt"`
	PromptColor           string `json:"prompt_color" yaml:"prompt_color" mapstructure:"prompt_color"`
	StopCommand           string `json:"stop_command" yaml:"stop_command" mapstructure:"stop_command"`
	DisableWrappers       bool   `json:"disable_wrappers" yaml:"disable_wrappers" mapstructure:"disable_wrappers"`
	DisableJobs           bool   `json:"disable_jobs" yaml:"disable_jobs" mapstructure:"disable_jobs"`
	DisableServerRegister bool   `json:"disable_server_register" yaml:"disable_server_register" mapstructure:"disable_server_register"`
}

func DefaultConfig() *Config {
	return &Config{}
}

func (c *Config) Ensure() *Config {
	if c == nil {
		c = DefaultConfig()
	}

	if c.StartupInfo == "" {
		c.StartupInfo = DefaultStartupInfo
	}

	if c.Prompt == "" {
		c.Prompt = DefaultPrompt
	}

	if c.PromptColor == "" {
		c.PromptColor = DefaultPromptColor
	}

	if c.StopCommand == "" {
		c.StopCommand = DefaultStopCommand
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
