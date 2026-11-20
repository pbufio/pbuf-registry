package server

import (
	"context"
	"net/http"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	kratosHttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/pbufio/pbuf-registry/internal/config"
)

const readinessCheckTimeout = 2 * time.Second

func methodNotAllowedHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	})
}

func writeProbeJSON(w http.ResponseWriter, status int, body string, logHelper *log.Helper) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.WriteHeader(status)
	if _, err := w.Write([]byte(body)); err != nil {
		logHelper.Errorf("failed to write response: %v", err)
	}
}

func NewDebugServer(cfg *config.Server, logger log.Logger, readinessCheck func(context.Context) error) *kratosHttp.Server {
	logHelper := log.NewHelper(logger)

	opts := []kratosHttp.ServerOption{
		kratosHttp.Address(cfg.Debug.Addr),
		kratosHttp.Timeout(cfg.Debug.Timeout),
		kratosHttp.NotFoundHandler(http.NotFoundHandler()),
		kratosHttp.MethodNotAllowedHandler(methodNotAllowedHandler()),
	}

	srv := kratosHttp.NewServer(opts...)

	srv.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeProbeJSON(w, http.StatusOK, "{}\n", logHelper)
	})

	srv.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if readinessCheck != nil {
			ctx, cancel := context.WithTimeout(r.Context(), readinessCheckTimeout)
			defer cancel()
			if err := readinessCheck(ctx); err != nil {
				logHelper.Warnf("readiness check failed: %v", err)
				writeProbeJSON(w, http.StatusServiceUnavailable, "{\"status\":\"unavailable\"}\n", logHelper)
				return
			}
		}
		writeProbeJSON(w, http.StatusOK, "{}\n", logHelper)
	})

	return srv
}
