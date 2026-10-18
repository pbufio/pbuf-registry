package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	kerrors "github.com/go-kratos/kratos/v2/errors"
	kratosHttp "github.com/go-kratos/kratos/v2/transport/http"
	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const (
	defaultProtocolVersion = "2024-11-05"
	maxScannerBuffer       = 10 * 1024 * 1024
	maxHTTPBodyBytes       = 1 << 20
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolSpec struct {
	name        string
	description string
	schema      map[string]any
	newReq      func() proto.Message
	call        func(ctx context.Context, req proto.Message) (proto.Message, error)
}

// Server implements a Model Context Protocol (MCP) server for pbuf-registry.
type Server struct {
	tools []toolSpec
}

// NewServer creates a new MCP server backed by the provided gRPC service clients.
func NewServer(registry v1.RegistryClient, metadata v1.MetadataServiceClient, drift v1.DriftServiceClient) *Server {
	return &Server{
		tools: []toolSpec{
			{
				name:        "list_modules",
				description: "List registered protobuf modules in the registry with optional pagination.",
				schema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"page_size":  map[string]any{"type": "integer", "description": "Maximum number of modules to return (1-1000)."},
						"page_token": map[string]any{"type": "string", "description": "Pagination token from a previous list_modules call."},
					},
				},
				newReq: func() proto.Message { return &v1.ListModulesRequest{} },
				call: func(ctx context.Context, req proto.Message) (proto.Message, error) {
					return registry.ListModules(ctx, req.(*v1.ListModulesRequest))
				},
			},
			{
				name:        "get_module",
				description: "Get a protobuf module by name, including its tags, draft tags, and packages.",
				schema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name":               map[string]any{"type": "string", "description": "Module name (e.g. pbufio/pbuf-registry)."},
						"include_draft_tags": map[string]any{"type": "boolean", "description": "Include draft tags in the response."},
					},
					"required": []string{"name"},
				},
				newReq: func() proto.Message { return &v1.GetModuleRequest{} },
				call: func(ctx context.Context, req proto.Message) (proto.Message, error) {
					return registry.GetModule(ctx, req.(*v1.GetModuleRequest))
				},
			},
			{
				name:        "get_module_dependencies",
				description: "Get direct or transitive dependencies for a protobuf module and optional tag.",
				schema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name":               map[string]any{"type": "string", "description": "Module name."},
						"tag":                map[string]any{"type": "string", "description": "Module tag (defaults to latest tag when omitted)."},
						"resolve_transitive": map[string]any{"type": "boolean", "description": "Resolve transitive dependencies."},
					},
					"required": []string{"name"},
				},
				newReq: func() proto.Message { return &v1.GetModuleDependenciesRequest{} },
				call: func(ctx context.Context, req proto.Message) (proto.Message, error) {
					return registry.GetModuleDependencies(ctx, req.(*v1.GetModuleDependenciesRequest))
				},
			},
			{
				name:        "get_module_metadata",
				description: "Get parsed protobuf schema metadata (packages, messages, fields, enums, services, and RPC methods) for a module and tag.",
				schema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{"type": "string", "description": "Module name."},
						"tag":  map[string]any{"type": "string", "description": "Module tag."},
					},
					"required": []string{"name", "tag"},
				},
				newReq: func() proto.Message { return &v1.GetMetadataRequest{} },
				call: func(ctx context.Context, req proto.Message) (proto.Message, error) {
					return metadata.GetMetadata(ctx, req.(*v1.GetMetadataRequest))
				},
			},
			{
				name:        "get_proto_files",
				description: "Pull raw .proto file contents for a module and tag.",
				schema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{"type": "string", "description": "Module name."},
						"tag":  map[string]any{"type": "string", "description": "Module tag."},
					},
					"required": []string{"name", "tag"},
				},
				newReq: func() proto.Message { return &v1.PullModuleRequest{} },
				call: func(ctx context.Context, req proto.Message) (proto.Message, error) {
					return registry.PullModule(ctx, req.(*v1.PullModuleRequest))
				},
			},
			{
				name:        "list_drift_events",
				description: "List schema drift and breaking-change events across the registry.",
				schema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"unacknowledged_only": map[string]any{"type": "boolean", "description": "Only return unacknowledged drift events (default: true)."},
					},
				},
				newReq: func() proto.Message { return &v1.ListDriftEventsRequest{UnacknowledgedOnly: true} },
				call: func(ctx context.Context, req proto.Message) (proto.Message, error) {
					return drift.ListDriftEvents(ctx, req.(*v1.ListDriftEventsRequest))
				},
			},
			{
				name:        "get_module_drift_events",
				description: "Get schema drift and breaking-change events for a specific module, optionally filtered by tag.",
				schema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"module_name": map[string]any{"type": "string", "description": "Module name."},
						"tag_name":    map[string]any{"type": "string", "description": "Optional tag name filter."},
					},
					"required": []string{"module_name"},
				},
				newReq: func() proto.Message { return &v1.GetModuleDriftEventsRequest{} },
				call: func(ctx context.Context, req proto.Message) (proto.Message, error) {
					return drift.GetModuleDriftEvents(ctx, req.(*v1.GetModuleDriftEventsRequest))
				},
			},
			{
				name:        "get_dependency_drift_status",
				description: "Get dependency update and breaking-change drift status for a specific module.",
				schema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"module_name": map[string]any{"type": "string", "description": "Module name."},
						"tag_name":    map[string]any{"type": "string", "description": "Optional module tag to evaluate (defaults to latest stable tag)."},
					},
					"required": []string{"module_name"},
				},
				newReq: func() proto.Message { return &v1.GetModuleDependencyDriftStatusRequest{} },
				call: func(ctx context.Context, req proto.Message) (proto.Message, error) {
					return drift.GetModuleDependencyDriftStatus(ctx, req.(*v1.GetModuleDependencyDriftStatusRequest))
				},
			},
		},
	}
}

// ServeStdio runs the MCP server over newline-delimited JSON-RPC 2.0 on in/out.
func (s *Server) ServeStdio(ctx context.Context, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), maxScannerBuffer)

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		respBytes, hasReply, _ := s.handleMessage(ctx, line)
		if !hasReply {
			continue
		}
		if _, err := out.Write(append(respBytes, '\n')); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func (s *Server) handleMessage(ctx context.Context, raw []byte) ([]byte, bool, error) {
	var req rpcRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		resp, _ := json.Marshal(rpcResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage("null"),
			Error:   &rpcError{Code: -32700, Message: "parse error"},
		})
		return resp, true, nil
	}

	if req.JSONRPC != "2.0" || req.Method == "" {
		id := req.ID
		if len(id) == 0 {
			id = json.RawMessage("null")
		}
		resp, _ := json.Marshal(rpcResponse{
			JSONRPC: "2.0",
			ID:      id,
			Error:   &rpcError{Code: -32600, Message: "invalid request"},
		})
		return resp, true, nil
	}

	if len(req.ID) == 0 || strings.HasPrefix(req.Method, "notifications/") {
		return nil, false, nil
	}

	var (
		result  any
		rpcErr  *rpcError
		authErr error
	)

	switch req.Method {
	case "initialize":
		var params struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &params)
		version := defaultProtocolVersion
		switch params.ProtocolVersion {
		case "2024-11-05", "2025-03-26", "2025-06-18":
			version = params.ProtocolVersion
		}
		result = map[string]any{
			"protocolVersion": version,
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    "pbuf-registry",
				"version": "1.0.0",
			},
		}
	case "ping":
		result = map[string]any{}
	case "tools/list":
		defs := make([]map[string]any, 0, len(s.tools))
		for _, t := range s.tools {
			defs = append(defs, map[string]any{
				"name":        t.name,
				"description": t.description,
				"inputSchema": t.schema,
			})
		}
		result = map[string]any{"tools": defs}
	case "tools/call":
		result, rpcErr, authErr = s.callTool(ctx, req.Params)
		if authErr != nil {
			return nil, false, authErr
		}
	default:
		rpcErr = &rpcError{Code: -32601, Message: "method not found: " + req.Method}
	}

	resp, err := json.Marshal(rpcResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
		Error:   rpcErr,
	})
	return resp, true, err
}

func (s *Server) callTool(ctx context.Context, rawParams json.RawMessage) (any, *rpcError, error) {
	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(rawParams, &call); err != nil || call.Name == "" {
		return nil, &rpcError{Code: -32602, Message: "invalid tool call parameters"}, nil
	}

	for _, tool := range s.tools {
		if tool.name != call.Name {
			continue
		}
		reqMsg := tool.newReq()
		if len(call.Arguments) > 0 && string(call.Arguments) != "null" {
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(call.Arguments, reqMsg); err != nil {
				return nil, &rpcError{Code: -32602, Message: fmt.Sprintf("invalid arguments for %s: %v", call.Name, err)}, nil
			}
		}

		respMsg, err := tool.call(ctx, reqMsg)
		if err != nil {
			if se := kerrors.FromError(err); se != nil && (se.Code == http.StatusUnauthorized || se.Code == http.StatusForbidden) {
				return nil, nil, err
			}
			return map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": err.Error()},
				},
				"isError": true,
			}, nil, nil
		}

		marshalled, err := (protojson.MarshalOptions{Multiline: true, Indent: "  "}).Marshal(respMsg)
		if err != nil {
			return nil, &rpcError{Code: -32603, Message: err.Error()}, nil
		}
		return map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": string(marshalled)},
			},
		}, nil, nil
	}

	return nil, &rpcError{Code: -32602, Message: "unknown tool: " + call.Name}, nil
}

type httpCtxKeyType struct{}

// RegisterHTTPServer mounts the MCP Streamable HTTP endpoint at POST /v1/mcp and POST /mcp.
func RegisterHTTPServer(
	srv *kratosHttp.Server,
	registry v1.RegistryHTTPServer,
	metadata v1.MetadataServiceHTTPServer,
	drift v1.DriftServiceHTTPServer,
) {
	conn := &inProcessConn{
		registry: registry,
		metadata: metadata,
		drift:    drift,
	}
	mcpSrv := NewServer(
		v1.NewRegistryClient(conn),
		v1.NewMetadataServiceClient(conn),
		v1.NewDriftServiceClient(conn),
	)

	handler := func(ctx kratosHttp.Context) error {
		body, err := io.ReadAll(io.LimitReader(ctx.Request().Body, maxHTTPBodyBytes+1))
		if err != nil {
			return err
		}
		if len(body) > maxHTTPBodyBytes {
			return kerrors.BadRequest("PAYLOAD_TOO_LARGE", "request body exceeds 1MB limit")
		}

		// Authenticate top-level MCP session/discovery calls via Kratos middleware.
		var authedCtx context.Context
		kratosHttp.SetOperation(ctx, "/pbufregistry.v1.MCP/Handle")
		h := ctx.Middleware(func(c context.Context, _ any) (any, error) {
			authedCtx = context.WithValue(c, httpCtxKeyType{}, ctx)
			return nil, nil
		})
		if _, err := h(ctx, nil); err != nil {
			return err
		}

		respBytes, hasReply, err := mcpSrv.handleMessage(authedCtx, body)
		if err != nil {
			return err
		}
		if !hasReply {
			ctx.Response().WriteHeader(http.StatusAccepted)
			return nil
		}
		return ctx.Blob(http.StatusOK, "application/json", respBytes)
	}

	r := srv.Route("/")
	r.POST("/v1/mcp", handler)
	r.POST("/mcp", handler)
}

type inProcessConn struct {
	registry v1.RegistryHTTPServer
	metadata v1.MetadataServiceHTTPServer
	drift    v1.DriftServiceHTTPServer
}

func (c *inProcessConn) Invoke(ctx context.Context, method string, args any, reply any, _ ...grpc.CallOption) error {
	var (
		out any
		err error
	)
	if httpCtx, ok := ctx.Value(httpCtxKeyType{}).(kratosHttp.Context); ok {
		kratosHttp.SetOperation(httpCtx, method)
		h := httpCtx.Middleware(func(mCtx context.Context, req any) (any, error) {
			return c.dispatch(mCtx, method, req)
		})
		out, err = h(ctx, args)
	} else {
		out, err = c.dispatch(ctx, method, args)
	}
	if err != nil {
		return err
	}
	proto.Reset(reply.(proto.Message))
	proto.Merge(reply.(proto.Message), out.(proto.Message))
	return nil
}

func (c *inProcessConn) NewStream(context.Context, *grpc.StreamDesc, string, ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, errors.New("streaming not supported")
}

func (c *inProcessConn) dispatch(ctx context.Context, method string, args any) (any, error) {
	switch method {
	case v1.OperationRegistryListModules:
		return c.registry.ListModules(ctx, args.(*v1.ListModulesRequest))
	case v1.OperationRegistryGetModule:
		return c.registry.GetModule(ctx, args.(*v1.GetModuleRequest))
	case v1.OperationRegistryPullModule:
		return c.registry.PullModule(ctx, args.(*v1.PullModuleRequest))
	case v1.OperationRegistryGetModuleDependencies:
		return c.registry.GetModuleDependencies(ctx, args.(*v1.GetModuleDependenciesRequest))
	case v1.OperationMetadataServiceGetMetadata:
		return c.metadata.GetMetadata(ctx, args.(*v1.GetMetadataRequest))
	case v1.OperationDriftServiceListDriftEvents:
		return c.drift.ListDriftEvents(ctx, args.(*v1.ListDriftEventsRequest))
	case v1.OperationDriftServiceGetModuleDriftEvents:
		return c.drift.GetModuleDriftEvents(ctx, args.(*v1.GetModuleDriftEventsRequest))
	case v1.OperationDriftServiceGetModuleDependencyDriftStatus:
		return c.drift.GetModuleDependencyDriftStatus(ctx, args.(*v1.GetModuleDependencyDriftStatusRequest))
	default:
		return nil, fmt.Errorf("unsupported method: %s", method)
	}
}
