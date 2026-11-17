package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/pbufio/pbuf-registry/internal/config"
	"github.com/stretchr/testify/assert"
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
}
