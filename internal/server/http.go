package server

import (
	"github.com/go-kratos/kratos/v2/log"
	kratosmiddleware "github.com/go-kratos/kratos/v2/middleware"
	kratosHttp "github.com/go-kratos/kratos/v2/transport/http"
	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/config"
	"github.com/pbufio/pbuf-registry/internal/data"
	"github.com/pbufio/pbuf-registry/internal/middleware"
)

// NewHTTPServer new a HTTP server.
func NewHTTPServer(cfg *config.Server,
	registryServer *RegistryServer,
	metadataServer *MetadataServer,
	usersServer *UsersServer,
	userRepo data.UserRepository,
	aclRepo data.ACLRepository,
	logger log.Logger,
) *kratosHttp.Server {
	logHelper := log.NewHelper(logger)

	authMiddleware, err := middleware.CreateAuthMiddleware(&cfg.HTTP.Auth, userRepo, logger)
	if err != nil {
		logHelper.Fatalf("failed to create auth middleware: %v", err)
	}

	serverMiddlewares := []kratosmiddleware.Middleware{authMiddleware.NewAuthMiddleware()}
	if cfg.HTTP.Auth.Enabled && cfg.HTTP.Auth.Type == "acl" {
		if aclRepo == nil {
			logHelper.Fatalf("ACL repository is required when auth type is 'acl'")
		}
		serverMiddlewares = append(serverMiddlewares, middleware.NewAuthorizationMiddleware(aclRepo, logger))
	}

	opts := []kratosHttp.ServerOption{
		kratosHttp.Address(cfg.HTTP.Addr),
		kratosHttp.Timeout(cfg.HTTP.Timeout),
		kratosHttp.Middleware(serverMiddlewares...),
	}

	srv := kratosHttp.NewServer(opts...)

	v1.RegisterRegistryHTTPServer(srv, registryServer)
	v1.RegisterMetadataServiceHTTPServer(srv, metadataServer)
	v1.RegisterUserServiceHTTPServer(srv, usersServer)

	return srv
}
