package main

import (
	"context"
	"log"
	"net"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/jackc/pgx/v5/pgxpool"
	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/config"
	"github.com/pbufio/pbuf-registry/internal/data"
	"github.com/pbufio/pbuf-registry/internal/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func startGRPCServer(address string, grpcServer *grpc.Server) error {
	listen, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	return grpcServer.Serve(listen)
}

func startHTTPServer(address string, grpcServer *server.RegistryServer) error {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	mux := runtime.NewServeMux()
	err := v1.RegisterRegistryHandlerServer(ctx, mux, grpcServer)
	if err != nil {
		return err
	}

	return http.ListenAndServe(address, mux)
}

func startDebugServer(address string) error {
	// add /healthz endpoint
	mux := runtime.NewServeMux()

	err := mux.HandlePath("GET", "/healthz", func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		w.WriteHeader(http.StatusOK)
	})

	if err != nil {
		log.Fatalf("failed to register debug server: %v", err)
	}

	return http.ListenAndServe(address, mux)
}

func main() {
	config.NewLoader().MustLoad()

	grpcServer := grpc.NewServer()

	pool, err := pgxpool.New(context.Background(), config.Cfg.Data.Database.DSN)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	registryRepository := data.NewRegistryRepository(pool)
	registryServer := server.NewRegistryServer(registryRepository)
	v1.RegisterRegistryServer(grpcServer, registryServer)
	reflection.Register(grpcServer)

	go func() {
		err := startGRPCServer(config.Cfg.Server.GRPC.Addr, grpcServer)
		if err != nil {
			log.Fatalf("failed to start grpc server: %v", err)
		}
	}()

	go func() {
		err = startHTTPServer(config.Cfg.Server.HTTP.Addr, registryServer)
		if err != nil {
			log.Fatalf("failed to start http server: %v", err)
		}
	}()

	err = startDebugServer(config.Cfg.Server.Debug.Addr)
	if err != nil {
		log.Fatalf("failed to start debug server: %v", err)
	}
}
