package main

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/jackc/pgx/v5/pgxpool"
	v1 "github.com/pbufio/pbuf-registry/gen/v1"
	"github.com/pbufio/pbuf-registry/internal/config"
	"github.com/pbufio/pbuf-registry/internal/data"
	"github.com/pbufio/pbuf-registry/internal/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

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

	listener, err := net.Listen("tcp", fmt.Sprintf(config.Cfg.Server.HTTP.Addr))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	err = grpcServer.Serve(listener)
	if err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
