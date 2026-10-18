package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	kratosHttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/google/uuid"
	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/middleware"
	"github.com/pbufio/pbuf-registry/internal/mocks"
	"github.com/pbufio/pbuf-registry/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type stubServer struct {
	v1.UnimplementedRegistryServer
	v1.UnimplementedMetadataServiceServer
	v1.UnimplementedDriftServiceServer
	fail bool
}

func (s *stubServer) ListModules(_ context.Context, _ *v1.ListModulesRequest) (*v1.ListModulesResponse, error) {
	if s.fail {
		return nil, errors.New("backend failure")
	}
	return &v1.ListModulesResponse{
		Modules: []*v1.Module{{Id: "1", Name: "org/mod", Tags: []string{"v1.0.0"}}},
	}, nil
}

func (s *stubServer) GetModule(_ context.Context, req *v1.GetModuleRequest) (*v1.Module, error) {
	return &v1.Module{Id: "1", Name: req.Name, Tags: []string{"v1.0.0"}}, nil
}

func (s *stubServer) GetModuleDependencies(_ context.Context, _ *v1.GetModuleDependenciesRequest) (*v1.GetModuleDependenciesResponse, error) {
	return &v1.GetModuleDependenciesResponse{
		Dependencies: []*v1.Dependency{{Name: "org/dep", Tag: "v0.1.0", DependencyType: "direct"}},
	}, nil
}

func (s *stubServer) PullModule(_ context.Context, req *v1.PullModuleRequest) (*v1.PullModuleResponse, error) {
	return &v1.PullModuleResponse{
		Module:     &v1.Module{Name: req.Name},
		Protofiles: []*v1.ProtoFile{{Filename: "hello.proto", Content: "syntax = \"proto3\";"}},
	}, nil
}

func (s *stubServer) GetMetadata(_ context.Context, _ *v1.GetMetadataRequest) (*v1.GetMetadataResponse, error) {
	return &v1.GetMetadataResponse{
		Packages: []*v1.Package{{Name: "hello.v1"}},
	}, nil
}

func (s *stubServer) ListDriftEvents(_ context.Context, req *v1.ListDriftEventsRequest) (*v1.ListDriftEventsResponse, error) {
	return &v1.ListDriftEventsResponse{
		Events: []*v1.DriftEvent{{Id: "ev-1", ModuleName: "org/mod", Acknowledged: !req.UnacknowledgedOnly}},
	}, nil
}

func (s *stubServer) GetModuleDriftEvents(_ context.Context, req *v1.GetModuleDriftEventsRequest) (*v1.GetModuleDriftEventsResponse, error) {
	return &v1.GetModuleDriftEventsResponse{
		Events: []*v1.DriftEvent{{Id: "ev-1", ModuleName: req.ModuleName}},
	}, nil
}

func (s *stubServer) GetModuleDependencyDriftStatus(_ context.Context, _ *v1.GetModuleDependencyDriftStatusRequest) (*v1.GetModuleDependencyDriftStatusResponse, error) {
	return &v1.GetModuleDependencyDriftStatusResponse{
		Statuses: []*v1.DependencyDriftStatus{{DependencyName: "org/dep", CurrentTag: "v0.1.0", TargetTag: "v0.2.0"}},
	}, nil
}

func TestServeStdioAndHTTP(t *testing.T) {
	stub := &stubServer{}
	httpSrv := kratosHttp.NewServer()
	RegisterHTTPServer(httpSrv, stub, stub, stub)

	requests := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"ping"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"list_modules","arguments":{"page_size":10}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"get_module","arguments":{"name":"org/mod"}}}`,
		`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"get_module_dependencies","arguments":{"name":"org/mod","resolve_transitive":true}}}`,
		`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"get_module_metadata","arguments":{"name":"org/mod","tag":"v1.0.0"}}}`,
		`{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"get_proto_files","arguments":{"name":"org/mod","tag":"v1.0.0"}}}`,
		`{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"list_drift_events"}}`,
		`{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"get_module_drift_events","arguments":{"module_name":"org/mod"}}}`,
		`{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"get_dependency_drift_status","arguments":{"module_name":"org/mod"}}}`,
		`{"jsonrpc":"2.0","id":12,"method":"tools/call","params":{"name":"unknown_tool"}}`,
		`{"jsonrpc":"2.0","id":13,"method":"tools/call","params":{"name":"list_modules","arguments":{"page_size":"not-an-int"}}}`,
		`{"jsonrpc":"2.0","id":14,"method":"unknown/method"}`,
		`not-valid-json`,
	}

	for _, reqBody := range requests {
		req := httptest.NewRequest(http.MethodPost, "/v1/mcp", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		httpSrv.ServeHTTP(rec, req)
		if strings.Contains(reqBody, "notifications/initialized") {
			assert.Equal(t, http.StatusAccepted, rec.Code)
		} else {
			assert.Equal(t, http.StatusOK, rec.Code)
		}
	}

	conn := &inProcessConn{registry: stub, metadata: stub, drift: stub}
	srv := NewServer(
		v1.NewRegistryClient(conn),
		v1.NewMetadataServiceClient(conn),
		v1.NewDriftServiceClient(conn),
	)

	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize"}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_modules"}}`,
		`{"jsonrpc":"2.0","id":null,"method":"ping"}`,
		`{}`,
	}, "\n") + "\n"

	var out bytes.Buffer
	require.NoError(t, srv.ServeStdio(context.Background(), strings.NewReader(in), &out))

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	require.Len(t, lines, 4)
	assert.Contains(t, lines[0], `"protocolVersion":"2024-11-05"`)
	assert.Contains(t, lines[1], "org/mod")
	assert.Contains(t, lines[2], `"id":null,"result":{}`)
	assert.Contains(t, lines[3], `"code":-32600`)

	stub.fail = true
	out.Reset()
	require.NoError(t, srv.ServeStdio(context.Background(), strings.NewReader("{\"jsonrpc\":\"2.0\",\"id\":3,\"method\":\"tools/call\",\"params\":{\"name\":\"list_modules\"}}\n"), &out))
	assert.Contains(t, out.String(), `"isError":true`)
}

func TestHTTPServerAuthAndACL(t *testing.T) {
	userRepo := mocks.NewUserRepository(t)
	aclRepo := mocks.NewACLRepository(t)

	aliceID := uuid.New()
	alice := &model.User{ID: aliceID, Name: "alice", Type: model.UserTypeUser, IsActive: true}
	userRepo.On("GetUserByToken", mock.Anything, "alice-token").Return(alice, nil).Twice()
	aclRepo.On("CheckPermission", mock.Anything, aliceID, "allowed/mod", model.PermissionRead).Return(true, nil).Once()
	aclRepo.On("CheckPermission", mock.Anything, aliceID, "secret/mod", model.PermissionRead).Return(false, nil).Once()

	authMw := middleware.NewACLAuth("admin-token", userRepo, log.DefaultLogger)
	authzMw := middleware.NewAuthorizationMiddleware(aclRepo, log.DefaultLogger)

	httpSrv := kratosHttp.NewServer(
		kratosHttp.Middleware(authMw.NewAuthMiddleware(), authzMw),
	)
	stub := &stubServer{}
	RegisterHTTPServer(httpSrv, stub, stub, stub)

	// 1. Unauthenticated request -> 401
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	rec := httptest.NewRecorder()
	httpSrv.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	// 2. Authenticated user calling allowed module -> 200 OK
	req = httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_module","arguments":{"name":"allowed/mod"}}}`))
	req.Header.Set("Authorization", "Bearer alice-token")
	rec = httptest.NewRecorder()
	httpSrv.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "allowed/mod")

	// 3. Authenticated user calling forbidden module -> 403 Forbidden
	req = httptest.NewRequest(http.MethodPost, "/v1/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_module","arguments":{"name":"secret/mod"}}}`))
	req.Header.Set("Authorization", "Bearer alice-token")
	rec = httptest.NewRecorder()
	httpSrv.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)

	// 4. Verify tools/list returns all 8 tools
	req = httptest.NewRequest(http.MethodPost, "/v1/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":4,"method":"tools/list"}`))
	req.Header.Set("Authorization", "Bearer admin-token")
	rec = httptest.NewRecorder()
	httpSrv.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	var listResp struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &listResp))
	assert.Len(t, listResp.Result.Tools, 8)

	// 5. Reject oversized (>1MB) HTTP body -> 400 Bad Request
	req = httptest.NewRequest(http.MethodPost, "/v1/mcp", strings.NewReader(strings.Repeat("x", maxHTTPBodyBytes+10)))
	req.Header.Set("Authorization", "Bearer admin-token")
	rec = httptest.NewRecorder()
	httpSrv.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
