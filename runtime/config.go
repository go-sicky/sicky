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
 * @package runtime
 * @author Dr.NP <np@herewe.tech>
 * @since 09/18/2024
 */

package runtime

import (
	"net/url"
	"strings"

	"github.com/go-sicky/sicky/logger"
	"github.com/spf13/viper"
	_ "github.com/spf13/viper/remote"
)

var (
	Config *viper.Viper
)

func LoadConfig() error {
	cfg := viper.New()
	Config = cfg

	cfg.SetConfigType(configType)

	// Try config source
	u, err := url.Parse(configLoc)
	if err == nil && u != nil && u.Scheme != "" && u.Path != "" {
		// Remote config source
		remote := strings.ToLower(u.Scheme)
		err = cfg.AddRemoteProvider(remote, u.Host, u.Path)
		if err != nil {
			logger.Logger.Fatal("Add remote config source failed", "error", err.Error())
		}

		err = cfg.ReadRemoteConfig()
	} else {
		// Local file
		cfg.SetConfigName(configLoc)
		cfg.AddConfigPath("/etc")
		cfg.AddConfigPath("/etc/" + AppName)
		cfg.AddConfigPath("$HOME/." + AppName)
		cfg.AddConfigPath(".")

		err = cfg.ReadInConfig()
	}

	if err != nil {
		logger.Logger.Fatal("Read config failed", "error", err.Error())
	}

	logger.Logger.Info("Config read", "location", configLoc)

	// Read config from environment variables
	cfg.SetEnvPrefix(strings.ToUpper(AppName))
	cfg.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	cfg.AutomaticEnv()

	return err
}

func WatchConfig() error {
	return nil
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
