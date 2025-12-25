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
 * @file nats.go
 * @package infra
 * @author Dr.NP <np@herewe.tech>
 * @since 12/21/2025
 */

package infra

import (
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
)

var MQTT mqtt.Client

type MQTTConfig struct {
	Broker   string `json:"broker" yaml:"broker" mapstructure:"broker"`
	ClientID string `json:"client_id" yaml:"client_id" mapstructure:"client_id"`
}

func InitMQTT(cfg *MQTTConfig) (mqtt.Client, error) {
	if cfg == nil {
		return nil, nil
	}

	if cfg.ClientID == "" {
		cfg.ClientID = "sicky::" + uuid.NewString()
	}

	opts := mqtt.NewClientOptions().AddBroker(cfg.Broker)
	opts.SetClientID(cfg.ClientID)
	client := mqtt.NewClient(opts)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	MQTT = client

	return client, nil
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
