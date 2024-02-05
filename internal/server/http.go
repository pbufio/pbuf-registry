package server

import (
	"github.com/go-kratos/kratos/v2/log"
	kratosHttp "github.com/go-kratos/kratos/v2/transport/http"
	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/config"
	"github.com/pbufio/pbuf-registry/internal/middleware"
)

// NewHTTPServer new a HTTP server.
func NewHTTPServer(cfg *config.Server,
	registryServer *RegistryServer,
	metadataServer *MetadataServer,
	tokenServer *TokenServer,
	logger log.Logger,
) *kratosHttp.Server {
	logHelper := log.NewHelper(logger)

	authMiddleware, err := middleware.CreateAuthMiddleware(&cfg.GRPC.Auth, logger)
	if err != nil {
		logHelper.Fatalf("failed to create auth middleware: %v", err)
	}

	opts := []kratosHttp.ServerOption{
		kratosHttp.Address(cfg.HTTP.Addr),
		kratosHttp.Timeout(cfg.HTTP.Timeout),
		kratosHttp.Middleware(
			authMiddleware.NewAuthMiddleware(),
		),
	}

	srv := kratosHttp.NewServer(opts...)

	v1.RegisterRegistryHTTPServer(srv, registryServer)
	v1.RegisterMetadataServiceHTTPServer(srv, metadataServer)
	v1.RegisterTokenServiceHTTPServer(srv, tokenServer)

	return srv
}
