// Code generated by protoc-gen-go-http. DO NOT EDIT.
// versions:
// - protoc-gen-go-http v2.7.2
// - protoc             (unknown)
// source: pbuf-registry/v1/registry.proto

package v1

import (
	context "context"
	http "github.com/go-kratos/kratos/v2/transport/http"
	binding "github.com/go-kratos/kratos/v2/transport/http/binding"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the kratos package it is being compiled against.
var _ = new(context.Context)
var _ = binding.EncodeURL

const _ = http.SupportPackageIsVersion1

const OperationRegistryDeleteModule = "/pbufregistry.v1.Registry/DeleteModule"
const OperationRegistryDeleteModuleTag = "/pbufregistry.v1.Registry/DeleteModuleTag"
const OperationRegistryGetModule = "/pbufregistry.v1.Registry/GetModule"
const OperationRegistryGetModuleDependencies = "/pbufregistry.v1.Registry/GetModuleDependencies"
const OperationRegistryListModules = "/pbufregistry.v1.Registry/ListModules"
const OperationRegistryPullModule = "/pbufregistry.v1.Registry/PullModule"
const OperationRegistryPushModule = "/pbufregistry.v1.Registry/PushModule"
const OperationRegistryRegisterModule = "/pbufregistry.v1.Registry/RegisterModule"
const OperationRegistryRegisterToken = "/pbufregistry.v1.Registry/RegisterToken"
const OperationRegistryRevokeToken = "/pbufregistry.v1.Registry/RevokeToken"

type RegistryHTTPServer interface {
	// DeleteModule Delete a module by name
	DeleteModule(context.Context, *DeleteModuleRequest) (*DeleteModuleResponse, error)
	// DeleteModuleTag Delete a specific module tag
	DeleteModuleTag(context.Context, *DeleteModuleTagRequest) (*DeleteModuleTagResponse, error)
	// GetModule Get a module by name
	GetModule(context.Context, *GetModuleRequest) (*Module, error)
	// GetModuleDependencies Get Module Dependencies
	GetModuleDependencies(context.Context, *GetModuleDependenciesRequest) (*GetModuleDependenciesResponse, error)
	// ListModules List all registered modules
	ListModules(context.Context, *ListModulesRequest) (*ListModulesResponse, error)
	// PullModule Pull a module tag
	PullModule(context.Context, *PullModuleRequest) (*PullModuleResponse, error)
	// PushModule Push a module
	PushModule(context.Context, *PushModuleRequest) (*Module, error)
	// RegisterModule Register a module
	RegisterModule(context.Context, *RegisterModuleRequest) (*Module, error)
	// RegisterToken Register authorization token
	RegisterToken(context.Context, *RegisterTokenRequest) (*RegisterTokenResponse, error)
	// RevokeToken Revoke authorization token
	RevokeToken(context.Context, *RevokeTokenRequest) (*RevokeTokenResponse, error)
}

func RegisterRegistryHTTPServer(s *http.Server, srv RegistryHTTPServer) {
	r := s.Route("/")
	r.GET("/v1/modules", _Registry_ListModules0_HTTP_Handler(srv))
	r.POST("/v1/modules/get", _Registry_GetModule0_HTTP_Handler(srv))
	r.POST("/v1/modules", _Registry_RegisterModule0_HTTP_Handler(srv))
	r.POST("/v1/modules/pull", _Registry_PullModule0_HTTP_Handler(srv))
	r.POST("/v1/modules/push", _Registry_PushModule0_HTTP_Handler(srv))
	r.POST("/v1/modules/delete", _Registry_DeleteModule0_HTTP_Handler(srv))
	r.POST("/v1/modules/tags/delete", _Registry_DeleteModuleTag0_HTTP_Handler(srv))
	r.POST("/v1/modules/dependencies", _Registry_GetModuleDependencies0_HTTP_Handler(srv))
	r.POST("/v1/tokens/register", _Registry_RegisterToken0_HTTP_Handler(srv))
	r.POST("/v1/tokens/revoke", _Registry_RevokeToken0_HTTP_Handler(srv))
}

func _Registry_ListModules0_HTTP_Handler(srv RegistryHTTPServer) func(ctx http.Context) error {
	return func(ctx http.Context) error {
		var in ListModulesRequest
		if err := ctx.BindQuery(&in); err != nil {
			return err
		}
		http.SetOperation(ctx, OperationRegistryListModules)
		h := ctx.Middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
			return srv.ListModules(ctx, req.(*ListModulesRequest))
		})
		out, err := h(ctx, &in)
		if err != nil {
			return err
		}
		reply := out.(*ListModulesResponse)
		return ctx.Result(200, reply)
	}
}

func _Registry_GetModule0_HTTP_Handler(srv RegistryHTTPServer) func(ctx http.Context) error {
	return func(ctx http.Context) error {
		var in GetModuleRequest
		if err := ctx.Bind(&in); err != nil {
			return err
		}
		if err := ctx.BindQuery(&in); err != nil {
			return err
		}
		http.SetOperation(ctx, OperationRegistryGetModule)
		h := ctx.Middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
			return srv.GetModule(ctx, req.(*GetModuleRequest))
		})
		out, err := h(ctx, &in)
		if err != nil {
			return err
		}
		reply := out.(*Module)
		return ctx.Result(200, reply)
	}
}

func _Registry_RegisterModule0_HTTP_Handler(srv RegistryHTTPServer) func(ctx http.Context) error {
	return func(ctx http.Context) error {
		var in RegisterModuleRequest
		if err := ctx.Bind(&in); err != nil {
			return err
		}
		if err := ctx.BindQuery(&in); err != nil {
			return err
		}
		http.SetOperation(ctx, OperationRegistryRegisterModule)
		h := ctx.Middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
			return srv.RegisterModule(ctx, req.(*RegisterModuleRequest))
		})
		out, err := h(ctx, &in)
		if err != nil {
			return err
		}
		reply := out.(*Module)
		return ctx.Result(200, reply)
	}
}

func _Registry_PullModule0_HTTP_Handler(srv RegistryHTTPServer) func(ctx http.Context) error {
	return func(ctx http.Context) error {
		var in PullModuleRequest
		if err := ctx.Bind(&in); err != nil {
			return err
		}
		if err := ctx.BindQuery(&in); err != nil {
			return err
		}
		http.SetOperation(ctx, OperationRegistryPullModule)
		h := ctx.Middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
			return srv.PullModule(ctx, req.(*PullModuleRequest))
		})
		out, err := h(ctx, &in)
		if err != nil {
			return err
		}
		reply := out.(*PullModuleResponse)
		return ctx.Result(200, reply)
	}
}

func _Registry_PushModule0_HTTP_Handler(srv RegistryHTTPServer) func(ctx http.Context) error {
	return func(ctx http.Context) error {
		var in PushModuleRequest
		if err := ctx.Bind(&in); err != nil {
			return err
		}
		if err := ctx.BindQuery(&in); err != nil {
			return err
		}
		http.SetOperation(ctx, OperationRegistryPushModule)
		h := ctx.Middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
			return srv.PushModule(ctx, req.(*PushModuleRequest))
		})
		out, err := h(ctx, &in)
		if err != nil {
			return err
		}
		reply := out.(*Module)
		return ctx.Result(200, reply)
	}
}

func _Registry_DeleteModule0_HTTP_Handler(srv RegistryHTTPServer) func(ctx http.Context) error {
	return func(ctx http.Context) error {
		var in DeleteModuleRequest
		if err := ctx.Bind(&in); err != nil {
			return err
		}
		if err := ctx.BindQuery(&in); err != nil {
			return err
		}
		http.SetOperation(ctx, OperationRegistryDeleteModule)
		h := ctx.Middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
			return srv.DeleteModule(ctx, req.(*DeleteModuleRequest))
		})
		out, err := h(ctx, &in)
		if err != nil {
			return err
		}
		reply := out.(*DeleteModuleResponse)
		return ctx.Result(200, reply)
	}
}

func _Registry_DeleteModuleTag0_HTTP_Handler(srv RegistryHTTPServer) func(ctx http.Context) error {
	return func(ctx http.Context) error {
		var in DeleteModuleTagRequest
		if err := ctx.Bind(&in); err != nil {
			return err
		}
		if err := ctx.BindQuery(&in); err != nil {
			return err
		}
		http.SetOperation(ctx, OperationRegistryDeleteModuleTag)
		h := ctx.Middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
			return srv.DeleteModuleTag(ctx, req.(*DeleteModuleTagRequest))
		})
		out, err := h(ctx, &in)
		if err != nil {
			return err
		}
		reply := out.(*DeleteModuleTagResponse)
		return ctx.Result(200, reply)
	}
}

func _Registry_GetModuleDependencies0_HTTP_Handler(srv RegistryHTTPServer) func(ctx http.Context) error {
	return func(ctx http.Context) error {
		var in GetModuleDependenciesRequest
		if err := ctx.Bind(&in); err != nil {
			return err
		}
		if err := ctx.BindQuery(&in); err != nil {
			return err
		}
		http.SetOperation(ctx, OperationRegistryGetModuleDependencies)
		h := ctx.Middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
			return srv.GetModuleDependencies(ctx, req.(*GetModuleDependenciesRequest))
		})
		out, err := h(ctx, &in)
		if err != nil {
			return err
		}
		reply := out.(*GetModuleDependenciesResponse)
		return ctx.Result(200, reply)
	}
}

func _Registry_RegisterToken0_HTTP_Handler(srv RegistryHTTPServer) func(ctx http.Context) error {
	return func(ctx http.Context) error {
		var in RegisterTokenRequest
		if err := ctx.Bind(&in); err != nil {
			return err
		}
		if err := ctx.BindQuery(&in); err != nil {
			return err
		}
		http.SetOperation(ctx, OperationRegistryRegisterToken)
		h := ctx.Middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
			return srv.RegisterToken(ctx, req.(*RegisterTokenRequest))
		})
		out, err := h(ctx, &in)
		if err != nil {
			return err
		}
		reply := out.(*RegisterTokenResponse)
		return ctx.Result(200, reply)
	}
}

func _Registry_RevokeToken0_HTTP_Handler(srv RegistryHTTPServer) func(ctx http.Context) error {
	return func(ctx http.Context) error {
		var in RevokeTokenRequest
		if err := ctx.Bind(&in); err != nil {
			return err
		}
		if err := ctx.BindQuery(&in); err != nil {
			return err
		}
		http.SetOperation(ctx, OperationRegistryRevokeToken)
		h := ctx.Middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
			return srv.RevokeToken(ctx, req.(*RevokeTokenRequest))
		})
		out, err := h(ctx, &in)
		if err != nil {
			return err
		}
		reply := out.(*RevokeTokenResponse)
		return ctx.Result(200, reply)
	}
}

type RegistryHTTPClient interface {
	DeleteModule(ctx context.Context, req *DeleteModuleRequest, opts ...http.CallOption) (rsp *DeleteModuleResponse, err error)
	DeleteModuleTag(ctx context.Context, req *DeleteModuleTagRequest, opts ...http.CallOption) (rsp *DeleteModuleTagResponse, err error)
	GetModule(ctx context.Context, req *GetModuleRequest, opts ...http.CallOption) (rsp *Module, err error)
	GetModuleDependencies(ctx context.Context, req *GetModuleDependenciesRequest, opts ...http.CallOption) (rsp *GetModuleDependenciesResponse, err error)
	ListModules(ctx context.Context, req *ListModulesRequest, opts ...http.CallOption) (rsp *ListModulesResponse, err error)
	PullModule(ctx context.Context, req *PullModuleRequest, opts ...http.CallOption) (rsp *PullModuleResponse, err error)
	PushModule(ctx context.Context, req *PushModuleRequest, opts ...http.CallOption) (rsp *Module, err error)
	RegisterModule(ctx context.Context, req *RegisterModuleRequest, opts ...http.CallOption) (rsp *Module, err error)
	RegisterToken(ctx context.Context, req *RegisterTokenRequest, opts ...http.CallOption) (rsp *RegisterTokenResponse, err error)
	RevokeToken(ctx context.Context, req *RevokeTokenRequest, opts ...http.CallOption) (rsp *RevokeTokenResponse, err error)
}

type RegistryHTTPClientImpl struct {
	cc *http.Client
}

func NewRegistryHTTPClient(client *http.Client) RegistryHTTPClient {
	return &RegistryHTTPClientImpl{client}
}

func (c *RegistryHTTPClientImpl) DeleteModule(ctx context.Context, in *DeleteModuleRequest, opts ...http.CallOption) (*DeleteModuleResponse, error) {
	var out DeleteModuleResponse
	pattern := "/v1/modules/delete"
	path := binding.EncodeURL(pattern, in, false)
	opts = append(opts, http.Operation(OperationRegistryDeleteModule))
	opts = append(opts, http.PathTemplate(pattern))
	err := c.cc.Invoke(ctx, "POST", path, in, &out, opts...)
	if err != nil {
		return nil, err
	}
	return &out, err
}

func (c *RegistryHTTPClientImpl) DeleteModuleTag(ctx context.Context, in *DeleteModuleTagRequest, opts ...http.CallOption) (*DeleteModuleTagResponse, error) {
	var out DeleteModuleTagResponse
	pattern := "/v1/modules/tags/delete"
	path := binding.EncodeURL(pattern, in, false)
	opts = append(opts, http.Operation(OperationRegistryDeleteModuleTag))
	opts = append(opts, http.PathTemplate(pattern))
	err := c.cc.Invoke(ctx, "POST", path, in, &out, opts...)
	if err != nil {
		return nil, err
	}
	return &out, err
}

func (c *RegistryHTTPClientImpl) GetModule(ctx context.Context, in *GetModuleRequest, opts ...http.CallOption) (*Module, error) {
	var out Module
	pattern := "/v1/modules/get"
	path := binding.EncodeURL(pattern, in, false)
	opts = append(opts, http.Operation(OperationRegistryGetModule))
	opts = append(opts, http.PathTemplate(pattern))
	err := c.cc.Invoke(ctx, "POST", path, in, &out, opts...)
	if err != nil {
		return nil, err
	}
	return &out, err
}

func (c *RegistryHTTPClientImpl) GetModuleDependencies(ctx context.Context, in *GetModuleDependenciesRequest, opts ...http.CallOption) (*GetModuleDependenciesResponse, error) {
	var out GetModuleDependenciesResponse
	pattern := "/v1/modules/dependencies"
	path := binding.EncodeURL(pattern, in, false)
	opts = append(opts, http.Operation(OperationRegistryGetModuleDependencies))
	opts = append(opts, http.PathTemplate(pattern))
	err := c.cc.Invoke(ctx, "POST", path, in, &out, opts...)
	if err != nil {
		return nil, err
	}
	return &out, err
}

func (c *RegistryHTTPClientImpl) ListModules(ctx context.Context, in *ListModulesRequest, opts ...http.CallOption) (*ListModulesResponse, error) {
	var out ListModulesResponse
	pattern := "/v1/modules"
	path := binding.EncodeURL(pattern, in, true)
	opts = append(opts, http.Operation(OperationRegistryListModules))
	opts = append(opts, http.PathTemplate(pattern))
	err := c.cc.Invoke(ctx, "GET", path, nil, &out, opts...)
	if err != nil {
		return nil, err
	}
	return &out, err
}

func (c *RegistryHTTPClientImpl) PullModule(ctx context.Context, in *PullModuleRequest, opts ...http.CallOption) (*PullModuleResponse, error) {
	var out PullModuleResponse
	pattern := "/v1/modules/pull"
	path := binding.EncodeURL(pattern, in, false)
	opts = append(opts, http.Operation(OperationRegistryPullModule))
	opts = append(opts, http.PathTemplate(pattern))
	err := c.cc.Invoke(ctx, "POST", path, in, &out, opts...)
	if err != nil {
		return nil, err
	}
	return &out, err
}

func (c *RegistryHTTPClientImpl) PushModule(ctx context.Context, in *PushModuleRequest, opts ...http.CallOption) (*Module, error) {
	var out Module
	pattern := "/v1/modules/push"
	path := binding.EncodeURL(pattern, in, false)
	opts = append(opts, http.Operation(OperationRegistryPushModule))
	opts = append(opts, http.PathTemplate(pattern))
	err := c.cc.Invoke(ctx, "POST", path, in, &out, opts...)
	if err != nil {
		return nil, err
	}
	return &out, err
}

func (c *RegistryHTTPClientImpl) RegisterModule(ctx context.Context, in *RegisterModuleRequest, opts ...http.CallOption) (*Module, error) {
	var out Module
	pattern := "/v1/modules"
	path := binding.EncodeURL(pattern, in, false)
	opts = append(opts, http.Operation(OperationRegistryRegisterModule))
	opts = append(opts, http.PathTemplate(pattern))
	err := c.cc.Invoke(ctx, "POST", path, in, &out, opts...)
	if err != nil {
		return nil, err
	}
	return &out, err
}

func (c *RegistryHTTPClientImpl) RegisterToken(ctx context.Context, in *RegisterTokenRequest, opts ...http.CallOption) (*RegisterTokenResponse, error) {
	var out RegisterTokenResponse
	pattern := "/v1/tokens/register"
	path := binding.EncodeURL(pattern, in, false)
	opts = append(opts, http.Operation(OperationRegistryRegisterToken))
	opts = append(opts, http.PathTemplate(pattern))
	err := c.cc.Invoke(ctx, "POST", path, in, &out, opts...)
	if err != nil {
		return nil, err
	}
	return &out, err
}

func (c *RegistryHTTPClientImpl) RevokeToken(ctx context.Context, in *RevokeTokenRequest, opts ...http.CallOption) (*RevokeTokenResponse, error) {
	var out RevokeTokenResponse
	pattern := "/v1/tokens/revoke"
	path := binding.EncodeURL(pattern, in, false)
	opts = append(opts, http.Operation(OperationRegistryRevokeToken))
	opts = append(opts, http.PathTemplate(pattern))
	err := c.cc.Invoke(ctx, "POST", path, in, &out, opts...)
	if err != nil {
		return nil, err
	}
	return &out, err
}
