package server

import (
	"net/http"

	"github.com/go-kratos/kratos/v2/log"
	kratosHttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/pbufio/pbuf-registry/internal/config"
)

func NewDebugServer(cfg *config.Server, logger log.Logger) *kratosHttp.Server {
	logHelper := log.NewHelper(logger)

	opts := []kratosHttp.ServerOption{
		kratosHttp.Address(cfg.Debug.Addr),
		kratosHttp.Timeout(cfg.Debug.Timeout),
	}

	srv := kratosHttp.NewServer(opts...)

	// add healthz endpoint to debug server
	srv.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		status := http.StatusOK

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")

		w.WriteHeader(status)
		_, err := w.Write([]byte("{}\n"))
		if err != nil {
			logHelper.Errorf("failed to write response: %v", err)
			return
		}
	})

	return srv
}
