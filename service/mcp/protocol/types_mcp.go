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
 * @file types_mcp.go
 * @package protocol
 * @author Dr.NP <np@herewe.tech>
 * @since 07/06/2026
 */

package protocol

const (
	// MethodInitialize is a protocol constant.
	MethodInitialize = "initialize"
	// MethodInitialized is a protocol constant.
	MethodInitialized = "notifications/initialized"
	// MethodToolsList is a protocol constant.
	MethodToolsList = "tools/list"
	// MethodToolsCall is a protocol constant.
	MethodToolsCall = "tools/call"
	// MethodResourcesList is a protocol constant.
	MethodResourcesList = "resources/list"
	// MethodResourcesRead is a protocol constant.
	MethodResourcesRead = "resources/read"
	// MethodPromptsList is a protocol constant.
	MethodPromptsList = "prompts/list"
	// MethodPromptsGet is a protocol constant.
	MethodPromptsGet = "prompts/get"
	// MethodPing is a protocol constant.
	MethodPing = "ping"

	// ProtocolVersion is a protocol constant.
	ProtocolVersion = "2024-11-05"
)

const (
	// ContentTypeText is a protocol constant.
	ContentTypeText = "text"
	// ContentTypeImage is a protocol constant.
	ContentTypeImage = "image"
	// ContentTypeResource is a protocol constant.
	ContentTypeResource = "resource"

	// RoleUser is a protocol constant.
	RoleUser = "user"
	// RoleAssistant is a protocol constant.
	RoleAssistant = "assistant"
)

// ToolsCapability is a protocol component.
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourcesCapability is a protocol component.
type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

// PromptsCapability is a protocol component.
type PromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ServerCapabilities is a protocol component.
type ServerCapabilities struct {
	Tools     *ToolsCapability     `json:"tools,omitempty"`
	Resources *ResourcesCapability `json:"resources,omitempty"`
	Prompts   *PromptsCapability   `json:"prompts,omitempty"`
}

// ImplementationInfo is a protocol component.
type ImplementationInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeParams is a protocol component.
type InitializeParams struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ClientCapabilities `json:"capabilities"`
	ClientInfo      ImplementationInfo `json:"clientInfo"`
}

// ClientCapabilities is a protocol component.
type ClientCapabilities struct {
	Roots    *struct{} `json:"roots,omitempty"`
	Sampling *struct{} `json:"sampling,omitempty"`
}

// InitializeResult is a protocol component.
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ImplementationInfo `json:"serverInfo"`
}

// Tool is a protocol component.
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	InputSchema InputSchema `json:"inputSchema"`
}

// InputSchema is a protocol component.
type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties,omitempty"`
	Required   []string            `json:"required,omitempty"`
}

// Property is a protocol component.
type Property struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

// ToolsListParams is a protocol component.
type ToolsListParams struct {
	Cursor *string `json:"cursor,omitempty"`
}

// ToolsListResult is a protocol component.
type ToolsListResult struct {
	Tools      []Tool  `json:"tools"`
	NextCursor *string `json:"nextCursor,omitempty"`
}

// ToolsCallParams is a protocol component.
type ToolsCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// ToolsCallResult is a protocol component.
type ToolsCallResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock is a protocol component.
type ContentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"`
	URI      string `json:"uri,omitempty"`
}

// Resource is a protocol component.
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ResourcesListParams is a protocol component.
type ResourcesListParams struct {
	Cursor *string `json:"cursor,omitempty"`
}

// ResourcesListResult is a protocol component.
type ResourcesListResult struct {
	Resources  []Resource `json:"resources"`
	NextCursor *string    `json:"nextCursor,omitempty"`
}

// ResourcesReadParams is a protocol component.
type ResourcesReadParams struct {
	URI string `json:"uri"`
}

// ResourcesReadResult is a protocol component.
type ResourcesReadResult struct {
	Contents []ResourceContent `json:"contents"`
}

// ResourceContent is a protocol component.
type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"`
}

// Prompt is a protocol component.
type Prompt struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

// PromptArgument is a protocol component.
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// PromptsListParams is a protocol component.
type PromptsListParams struct {
	Cursor *string `json:"cursor,omitempty"`
}

// PromptsListResult is a protocol component.
type PromptsListResult struct {
	Prompts    []Prompt `json:"prompts"`
	NextCursor *string  `json:"nextCursor,omitempty"`
}

// PromptsGetParams is a protocol component.
type PromptsGetParams struct {
	Name      string            `json:"name"`
	Arguments map[string]string `json:"arguments,omitempty"`
}

// PromptsGetResult is a protocol component.
type PromptsGetResult struct {
	Description string          `json:"description,omitempty"`
	Messages    []PromptMessage `json:"messages"`
}

// PromptMessage is a protocol component.
type PromptMessage struct {
	Role    string        `json:"role"`
	Content PromptContent `json:"content"`
}

// PromptContent is a protocol component.
type PromptContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

// PingResult is a protocol component.
type PingResult struct {
	Message string `json:"message"`
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
