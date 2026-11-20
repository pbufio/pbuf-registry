package server

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/config"
	"github.com/pbufio/pbuf-registry/internal/mocks"
	"github.com/pbufio/pbuf-registry/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestServersDoNotExposeDefaultServeMux(t *testing.T) {
	const leakedRoute = "/__cve_2026_6993_default_mux_probe__"
	http.DefaultServeMux.HandleFunc(leakedRoute, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("leaked-default-mux"))
	})

	t.Setenv("SERVER_STATIC_TOKEN", "super-secret-admin-token")
	cfg := &config.Server{}
	cfg.HTTP.Addr = "127.0.0.1:0"
	cfg.HTTP.Timeout = time.Second
	cfg.HTTP.Auth.Enabled = true
	cfg.HTTP.Auth.Type = "static-token"
	cfg.Debug.Addr = "127.0.0.1:0"
	cfg.Debug.Timeout = time.Second

	var dbUnhealthy atomic.Bool
	logger := log.NewStdLogger(io.Discard)
	httpSrv := NewHTTPServer(cfg, &RegistryServer{}, &MetadataServer{}, &UsersServer{}, &DriftServer{}, nil, nil, logger)
	debugSrv := NewDebugServer(cfg, logger, func(context.Context) error {
		if dbUnhealthy.Load() {
			return errors.New("postgres down")
		}
		return nil
	})

	t.Run("http server returns 404 and 405 without hitting DefaultServeMux", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, leakedRoute, nil)
		httpSrv.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.NotContains(t, rec.Body.String(), "leaked-default-mux")

		methodRec := httptest.NewRecorder()
		methodReq := httptest.NewRequest(http.MethodPatch, "/v1/modules", nil)
		httpSrv.ServeHTTP(methodRec, methodReq)
		assert.Equal(t, http.StatusMethodNotAllowed, methodRec.Code)
	})

	t.Run("debug server serves healthz, readyz with DB check, and blocks DefaultServeMux", func(t *testing.T) {
		okRec := httptest.NewRecorder()
		okReq := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		debugSrv.ServeHTTP(okRec, okReq)
		require.Equal(t, http.StatusOK, okRec.Code)

		readyRec := httptest.NewRecorder()
		readyReq := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		debugSrv.ServeHTTP(readyRec, readyReq)
		require.Equal(t, http.StatusOK, readyRec.Code)

		dbUnhealthy.Store(true)
		defer dbUnhealthy.Store(false)

		unreadyRec := httptest.NewRecorder()
		unreadyReq := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		debugSrv.ServeHTTP(unreadyRec, unreadyReq)
		require.Equal(t, http.StatusServiceUnavailable, unreadyRec.Code)
		assert.Contains(t, unreadyRec.Body.String(), "unavailable")

		// liveness /healthz stays 200 even while DB is down
		liveRec := httptest.NewRecorder()
		debugSrv.ServeHTTP(liveRec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		require.Equal(t, http.StatusOK, liveRec.Code)

		postReadyRec := httptest.NewRecorder()
		debugSrv.ServeHTTP(postReadyRec, httptest.NewRequest(http.MethodPost, "/readyz", nil))
		assert.Equal(t, http.StatusMethodNotAllowed, postReadyRec.Code)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, leakedRoute, nil)
		debugSrv.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.NotContains(t, rec.Body.String(), "leaked-default-mux")
	})

	t.Run("http server caps request body size before auth and routes slash module_name on RevokePermission", func(t *testing.T) {
		oversized := bytes.NewReader(make([]byte, maxHTTPRequestBytes+1024))
		pushRec := httptest.NewRecorder()
		pushReq := httptest.NewRequest(http.MethodPost, "/v1/modules/push", oversized)
		pushReq.Header.Set("Content-Type", "application/json")
		httpSrv.ServeHTTP(pushRec, pushReq)
		assert.GreaterOrEqual(t, pushRec.Code, 400)
		// Ensure reader was cut off right after maxHTTPRequestBytes rather than reading entire buffer
		assert.Greater(t, oversized.Len(), 512)

		aclRepo := mocks.NewACLRepository(t)
		userID := uuid.New()
		aclRepo.On("RevokePermission", mock.Anything, userID, "myorg/payment-service").Return(nil).Once()
		aclRepo.On("RevokePermission", mock.Anything, userID, "*").Return(nil).Once()
		usersSrv := NewUsersServer(mocks.NewUserRepository(t), aclRepo, logger)
		authedHTTP := NewHTTPServer(cfg, &RegistryServer{}, &MetadataServer{}, usersSrv, &DriftServer{}, nil, nil, logger)

		delRec := httptest.NewRecorder()
		delReq := httptest.NewRequest(http.MethodDelete, "/v1/users/"+userID.String()+"/permissions/myorg/payment-service", nil)
		delReq.Header.Set("Authorization", "bearer super-secret-admin-token")
		authedHTTP.ServeHTTP(delRec, delReq)
		assert.Equal(t, http.StatusOK, delRec.Code)

		starRec := httptest.NewRecorder()
		starReq := httptest.NewRequest(http.MethodDelete, "/v1/users/"+userID.String()+"/permissions/*", nil)
		starReq.Header.Set("Authorization", "bearer super-secret-admin-token")
		authedHTTP.ServeHTTP(starRec, starReq)
		assert.Equal(t, http.StatusOK, starRec.Code)
	})

	t.Run("http and grpc servers recover handler panics and enforce RBAC when auth.type is token", func(t *testing.T) {
		panicRepo := mocks.NewRegistryRepository(t)
		panicRepo.On("GetModuleWithDrafts", mock.Anything, "boom/mod", false).Run(func(mock.Arguments) {
			panic("simulated handler panic")
		}).Return(nil, nil).Times(2)

		regSrv := NewRegistryServer(panicRepo, nil, logger)
		panicSrv := NewHTTPServer(cfg, regSrv, &MetadataServer{}, &UsersServer{}, &DriftServer{}, nil, nil, logger)
		panicRec := httptest.NewRecorder()
		panicReq := httptest.NewRequest(http.MethodPost, "/v1/modules/get", bytes.NewBufferString(`{"name":"boom/mod"}`))
		panicReq.Header.Set("Content-Type", "application/json")
		panicReq.Header.Set("Authorization", "Bearer super-secret-admin-token")
		panicSrv.ServeHTTP(panicRec, panicReq)
		assert.Equal(t, http.StatusInternalServerError, panicRec.Code)

		userID := uuid.New()
		userRepo := mocks.NewUserRepository(t)
		userRepo.On("GetUserByToken", mock.Anything, "pbuf_user_limited").
			Return(&model.User{ID: userID, Name: "limited", Type: model.UserTypeUser, IsActive: true}, nil).Times(2)
		aclRepo := mocks.NewACLRepository(t)
		aclRepo.On("CheckPermission", mock.Anything, userID, "forbidden/mod", model.PermissionRead).
			Return(false, nil).Times(2)

		tokenCfg := *cfg
		tokenCfg.HTTP.Auth.Type = "token"
		tokenCfg.GRPC.Addr = "127.0.0.1:0"
		tokenCfg.GRPC.Timeout = time.Second
		tokenCfg.GRPC.Auth.Enabled = true
		tokenCfg.GRPC.Auth.Type = "token"

		tokenHTTP := NewHTTPServer(&tokenCfg, regSrv, &MetadataServer{}, &UsersServer{}, &DriftServer{}, userRepo, aclRepo, logger)
		forbidRec := httptest.NewRecorder()
		forbidReq := httptest.NewRequest(http.MethodPost, "/v1/modules/get", bytes.NewBufferString(`{"name":"forbidden/mod"}`))
		forbidReq.Header.Set("Content-Type", "application/json")
		forbidReq.Header.Set("Authorization", "Bearer pbuf_user_limited")
		tokenHTTP.ServeHTTP(forbidRec, forbidReq)
		assert.Equal(t, http.StatusForbidden, forbidRec.Code)

		tokenGRPC := NewGRPCServer(&tokenCfg, regSrv, &MetadataServer{}, &UsersServer{}, &DriftServer{}, userRepo, aclRepo, logger)
		ep, err := tokenGRPC.Endpoint()
		require.NoError(t, err)
		go func() {
			_ = tokenGRPC.Start(context.Background())
		}()
		t.Cleanup(func() {
			_ = tokenGRPC.Stop(context.Background())
		})

		conn, err := grpc.NewClient(ep.Host, grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })
		grpcClient := v1.NewRegistryClient(conn)

		userCtx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer pbuf_user_limited"))
		_, err = grpcClient.GetModule(userCtx, &v1.GetModuleRequest{Name: "forbidden/mod"})
		assert.Equal(t, codes.PermissionDenied, status.Code(err))

		adminCtx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer super-secret-admin-token"))
		_, err = grpcClient.GetModule(adminCtx, &v1.GetModuleRequest{Name: "boom/mod"})
		assert.Equal(t, codes.Internal, status.Code(err))
	})
}
