package server

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/pbufio/pbuf-registry/internal/config"
	"github.com/pbufio/pbuf-registry/internal/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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

	logger := log.NewStdLogger(io.Discard)
	httpSrv := NewHTTPServer(cfg, &RegistryServer{}, &MetadataServer{}, &UsersServer{}, &DriftServer{}, nil, nil, logger)
	debugSrv := NewDebugServer(cfg, logger)

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

	t.Run("debug server serves healthz and blocks DefaultServeMux", func(t *testing.T) {
		okRec := httptest.NewRecorder()
		okReq := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		debugSrv.ServeHTTP(okRec, okReq)
		require.Equal(t, http.StatusOK, okRec.Code)

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
}
