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
 * @file swagger.go
 * @package fiber
 * @author Dr.NP <np@herewe.tech>
 * @since 12/07/2023
 */

package fiber

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/swagger"
)

type Swagger struct {
	pageTitle    string
	validatorURL string
}

func NewSwagger(title, url string) *Swagger {
	h := &Swagger{
		pageTitle:    title,
		validatorURL: url,
	}

	return h
}

func (h *Swagger) Register(app *fiber.App) {
	cfg := swagger.ConfigDefault
	if h.validatorURL != "" {
		cfg.ValidatorUrl = h.validatorURL
	} else {
		cfg.ValidatorUrl = "localhost"
	}

	if h.pageTitle != "" {
		cfg.Title = h.pageTitle
	} else {
		cfg.Title = "Sicky.Swagger.UI"
	}

	app.All("/docs/*", swagger.New(cfg))
}

func (h *Swagger) Name() string {
	return "sicky.swagger"
}

func (h *Swagger) Type() string {
	return "http"
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
