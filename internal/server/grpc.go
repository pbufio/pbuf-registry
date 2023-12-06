package server

import (
	"crypto/tls"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/config"
	"github.com/pbufio/pbuf-registry/internal/middleware"
)

// NewGRPCServer new a gRPC server.
func NewGRPCServer(cfg *config.Server,
	registryServer *RegistryServer,
	metadataServer *MetadataServer,
	logger log.Logger,
) *grpc.Server {
	logHelper := log.NewHelper(logger)

	tlsConfig, err := createTLSConfig(cfg, logHelper)
	if err != nil {
		logHelper.Fatalf("failed to create TLS config: %v", err)
	}

	authMiddleware, err := middleware.CreateAuthMiddleware(&cfg.GRPC.Auth, logger)
	if err != nil {
		logHelper.Fatalf("failed to create auth middleware: %v", err)
	}

	opts := []grpc.ServerOption{
		grpc.Address(cfg.GRPC.Addr),
		grpc.Timeout(cfg.GRPC.Timeout),
		grpc.TLSConfig(tlsConfig),
		grpc.Middleware(
			authMiddleware.NewAuthMiddleware(),
		),
	}

	grpcServer := grpc.NewServer(opts...)

	v1.RegisterRegistryServer(grpcServer, registryServer)
	v1.RegisterMetadataServiceServer(grpcServer, metadataServer)

	return grpcServer
}

// createTLSConfig creates a TLS config
func createTLSConfig(cfg *config.Server, logHelper *log.Helper) (*tls.Config, error) {
	logHelper.Infof("TLS Config: %v", cfg.GRPC.TLS)

	if !cfg.GRPC.TLS.Enabled {
		logHelper.Infof("TLS is disabled. Skipping TLS config creation.")
		return nil, nil
	}

	cert, err := tls.LoadX509KeyPair(cfg.GRPC.TLS.CertFile, cfg.GRPC.TLS.KeyFile)
	if err != nil {
		logHelper.Errorf("failed to load TLS key pair: %v", err)
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.NoClientCert,
	}, nil
}
