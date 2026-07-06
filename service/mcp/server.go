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
 * @file server.go
 * @package mcp
 * @author Dr.NP <np@herewe.tech>
 * @since 07/06/2026
 */

package mcp

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/go-sicky/sicky/logger"
	"github.com/go-sicky/sicky/service/mcp/protocol"
)

/* {{{ [MCPServer] */

type MCPServer struct {
	info         protocol.ImplementationInfo
	capabilities protocol.ServerCapabilities
	handlers     []Handler
	transport    protocol.Transport
	initialized  bool
	log          logger.GeneralLogger
}

func NewMCPServer(info protocol.ImplementationInfo, caps protocol.ServerCapabilities) *MCPServer {
	return &MCPServer{
		info:         info,
		capabilities: caps,
		log:          logger.DefaultGeneralLogger,
	}
}

func (s *MCPServer) SetLogger(l logger.GeneralLogger) {
	s.log = l
}

func (s *MCPServer) Handle(hdls ...Handler) {
	s.handlers = append(s.handlers, hdls...)
}

func (s *MCPServer) Serve(ctx context.Context, transport protocol.Transport) error {
	s.transport = transport
	if err := transport.Start(); err != nil {
		return err
	}

	defer transport.Stop()

	s.log.InfoContext(ctx, "MCP server started",
		"name", s.info.Name,
		"version", s.info.Version,
	)

	for {
		select {
		case <-ctx.Done():
			s.log.InfoContext(ctx, "MCP server stopping")

			return ctx.Err()

		default:
		}

		data, err := transport.Read()
		if err != nil {
			s.log.ErrorContext(ctx, "MCP transport read error", slog.String("error", err.Error()))

			return err
		}

		s.handleMessage(ctx, data)
	}
}

func (s *MCPServer) handleMessage(ctx context.Context, data []byte) {
	if protocol.IsNotification(data) {
		notif, err := protocol.ParseNotification(data)
		if err != nil {
			s.log.ErrorContext(ctx, "MCP notification parse error", slog.String("error", err.Error()))

			return
		}

		s.dispatchNotification(ctx, notif)

		return
	}

	req, err := protocol.ParseRequest(data)
	if err != nil {
		s.log.ErrorContext(ctx, "MCP request parse error", slog.String("error", err.Error()))

		resp := protocol.NewErrorResponse(nil, protocol.ErrCodeParseError, protocol.ErrParseError.Error())
		s.writeResponse(resp)

		return
	}

	resp := s.dispatchRequest(ctx, req)
	if resp != nil {
		s.writeResponse(resp)
	}
}

func (s *MCPServer) writeResponse(resp *protocol.Response) {
	if resp == nil {
		return
	}

	raw, err := json.Marshal(resp)
	if err != nil {
		s.log.Error("MCP response marshal error", slog.String("error", err.Error()))

		return
	}

	if err := s.transport.Write(raw); err != nil {
		s.log.Error("MCP transport write error", slog.String("error", err.Error()))
	}
}

func (s *MCPServer) dispatchRequest(ctx context.Context, req *protocol.Request) *protocol.Response {
	switch req.Method {
	case protocol.MethodInitialize:
		return s.handleInitialize(ctx, req)

	case protocol.MethodPing:
		return s.handlePing(ctx, req)

	case protocol.MethodToolsList:
		return s.handleToolsList(ctx, req)

	case protocol.MethodToolsCall:
		return s.handleToolsCall(ctx, req)

	case protocol.MethodResourcesList:
		return s.handleResourcesList(ctx, req)

	case protocol.MethodResourcesRead:
		return s.handleResourcesRead(ctx, req)

	case protocol.MethodPromptsList:
		return s.handlePromptsList(ctx, req)

	case protocol.MethodPromptsGet:
		return s.handlePromptsGet(ctx, req)

	default:
		return protocol.NewErrorResponse(req.ID, protocol.ErrCodeMethodNotFound, protocol.ErrMethodNotFound.Error())
	}
}

func (s *MCPServer) dispatchNotification(ctx context.Context, notif *protocol.Notification) {
	switch notif.Method {
	case protocol.MethodInitialized:
		s.initialized = true
		s.log.InfoContext(ctx, "MCP client initialized")

	default:
		s.log.DebugContext(ctx, "MCP unknown notification", "method", notif.Method)
	}
}

func (s *MCPServer) handleInitialize(ctx context.Context, req *protocol.Request) *protocol.Response {
	var params protocol.InitializeParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return protocol.NewErrorResponse(req.ID, protocol.ErrCodeInvalidParams, protocol.ErrInvalidParams.Error())
	}

	s.log.InfoContext(ctx, "MCP initialize",
		"client", params.ClientInfo.Name,
		"client_version", params.ClientInfo.Version,
		"protocol_version", params.ProtocolVersion,
	)

	result := protocol.InitializeResult{
		ProtocolVersion: protocol.ProtocolVersion,
		Capabilities:    s.capabilities,
		ServerInfo:      s.info,
	}

	return protocol.NewResponse(req.ID, result)
}

func (s *MCPServer) handlePing(ctx context.Context, req *protocol.Request) *protocol.Response {
	return protocol.NewResponse(req.ID, protocol.PingResult{Message: "pong"})
}

func (s *MCPServer) handleToolsList(ctx context.Context, req *protocol.Request) *protocol.Response {
	tools := make([]protocol.Tool, 0)
	for _, h := range s.handlers {
		tools = append(tools, h.Tools()...)
	}

	return protocol.NewResponse(req.ID, protocol.ToolsListResult{Tools: tools})
}

func (s *MCPServer) handleToolsCall(ctx context.Context, req *protocol.Request) *protocol.Response {
	var params protocol.ToolsCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return protocol.NewErrorResponse(req.ID, protocol.ErrCodeInvalidParams, protocol.ErrInvalidParams.Error())
	}

	for _, h := range s.handlers {
		for _, t := range h.Tools() {
			if t.Name == params.Name {
				result, err := h.CallTool(params.Name, params.Arguments)
				if err != nil {
					return protocol.NewErrorResponseWithData(req.ID, protocol.ErrCodeInternalError, err.Error(), nil)
				}

				return protocol.NewResponse(req.ID, result)
			}
		}
	}

	return protocol.NewErrorResponse(req.ID, protocol.ErrCodeMethodNotFound, "tool not found: "+params.Name)
}

func (s *MCPServer) handleResourcesList(ctx context.Context, req *protocol.Request) *protocol.Response {
	resources := make([]protocol.Resource, 0)
	for _, h := range s.handlers {
		resources = append(resources, h.Resources()...)
	}

	return protocol.NewResponse(req.ID, protocol.ResourcesListResult{Resources: resources})
}

func (s *MCPServer) handleResourcesRead(ctx context.Context, req *protocol.Request) *protocol.Response {
	var params protocol.ResourcesReadParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return protocol.NewErrorResponse(req.ID, protocol.ErrCodeInvalidParams, protocol.ErrInvalidParams.Error())
	}

	for _, h := range s.handlers {
		for _, r := range h.Resources() {
			if r.URI == params.URI {
				result, err := h.ReadResource(params.URI)
				if err != nil {
					return protocol.NewErrorResponseWithData(req.ID, protocol.ErrCodeInternalError, err.Error(), nil)
				}

				return protocol.NewResponse(req.ID, result)
			}
		}
	}

	return protocol.NewErrorResponse(req.ID, protocol.ErrCodeMethodNotFound, "resource not found: "+params.URI)
}

func (s *MCPServer) handlePromptsList(ctx context.Context, req *protocol.Request) *protocol.Response {
	prompts := make([]protocol.Prompt, 0)
	for _, h := range s.handlers {
		prompts = append(prompts, h.Prompts()...)
	}

	return protocol.NewResponse(req.ID, protocol.PromptsListResult{Prompts: prompts})
}

func (s *MCPServer) handlePromptsGet(ctx context.Context, req *protocol.Request) *protocol.Response {
	var params protocol.PromptsGetParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return protocol.NewErrorResponse(req.ID, protocol.ErrCodeInvalidParams, protocol.ErrInvalidParams.Error())
	}

	for _, h := range s.handlers {
		for _, p := range h.Prompts() {
			if p.Name == params.Name {
				result, err := h.GetPrompt(params.Name, params.Arguments)
				if err != nil {
					return protocol.NewErrorResponseWithData(req.ID, protocol.ErrCodeInternalError, err.Error(), nil)
				}

				return protocol.NewResponse(req.ID, result)
			}
		}
	}

	return protocol.NewErrorResponse(req.ID, protocol.ErrCodeMethodNotFound, "prompt not found: "+params.Name)
}

/* }}} */

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
