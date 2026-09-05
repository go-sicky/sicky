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
 * @file export.go
 * @package mcp
 * @author Dr.NP <np@herewe.tech>
 * @since 07/06/2026
 */

package mcp

import "github.com/go-sicky/sicky/service/mcp/protocol"

type (
	// ServerCapabilities is a mcp component.
	ServerCapabilities = protocol.ServerCapabilities
	// ToolsCapability is a mcp component.
	ToolsCapability = protocol.ToolsCapability
	// ResourcesCapability is a mcp component.
	ResourcesCapability = protocol.ResourcesCapability
	// PromptsCapability is a mcp component.
	PromptsCapability = protocol.PromptsCapability
	// ImplementationInfo is a mcp component.
	ImplementationInfo = protocol.ImplementationInfo

	// Tool is a mcp component.
	Tool = protocol.Tool
	// InputSchema is a mcp component.
	InputSchema = protocol.InputSchema
	// Property is a mcp component.
	Property = protocol.Property
	// ContentBlock is a mcp component.
	ContentBlock = protocol.ContentBlock
	// Resource is a mcp component.
	Resource = protocol.Resource
	// ResourceContent is a mcp component.
	ResourceContent = protocol.ResourceContent
	// Prompt is a mcp component.
	Prompt = protocol.Prompt
	// PromptArgument is a mcp component.
	PromptArgument = protocol.PromptArgument
	// PromptMessage is a mcp component.
	PromptMessage = protocol.PromptMessage
	// PromptContent is a mcp component.
	PromptContent = protocol.PromptContent

	// ToolsListResult is a mcp component.
	ToolsListResult = protocol.ToolsListResult
	// ToolsCallParams is a mcp component.
	ToolsCallParams = protocol.ToolsCallParams
	// ToolsCallResult is a mcp component.
	ToolsCallResult = protocol.ToolsCallResult
	// ResourcesListResult is a mcp component.
	ResourcesListResult = protocol.ResourcesListResult
	// ResourcesReadResult is a mcp component.
	ResourcesReadResult = protocol.ResourcesReadResult
	// PromptsListResult is a mcp component.
	PromptsListResult = protocol.PromptsListResult
	// PromptsGetResult is a mcp component.
	PromptsGetResult = protocol.PromptsGetResult
)

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
