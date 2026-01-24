package main

import (
	"context"
	"os"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pbufio/pbuf-registry/internal/background"
	"github.com/pbufio/pbuf-registry/internal/config"
	"github.com/pbufio/pbuf-registry/internal/data"
	"github.com/pbufio/pbuf-registry/internal/server"
)

// go build -ldflags "-X main.Version=x.y.z"
var (
	// Name is the name of the compiled software.
	Name string
	// Version is the version of the compiled software.
	Version string

	id, _ = os.Hostname()
)

type Launcher struct {
	config *config.Config

	mainApp  *kratos.App
	debugApp *kratos.App

	compactionDaemon   background.Daemon
	protoParsingDaemon background.Daemon
}

func main() {
	config.NewLoader().MustLoad()

	logger := log.DefaultLogger
	logHelper := log.NewHelper(logger)

	pool, err := pgxpool.New(context.Background(), config.Cfg.Data.Database.DSN)
	if err != nil {
		logHelper.Errorf("failed to connect to database: %v", err)
		return
	}
	defer pool.Close()

	registryRepository := data.NewRegistryRepository(pool, logger)
	metadataRepository := data.NewMetadataRepository(pool, logger)
	userRepository := data.NewUserRepository(pool, logger)
	aclRepository := data.NewACLRepository(pool, logger)
	registryServer := server.NewRegistryServer(registryRepository, metadataRepository, logger)
	metadataServer := server.NewMetadataServer(registryRepository, metadataRepository, logger)
	usersServer := server.NewUsersServer(userRepository, aclRepository, logger)

	app := kratos.New(
		kratos.ID(id),
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Metadata(map[string]string{}),
		kratos.Logger(logger),
		kratos.Server(
			server.NewGRPCServer(&config.Cfg.Server, registryServer, metadataServer, usersServer, userRepository, aclRepository, logger),
			server.NewHTTPServer(&config.Cfg.Server, registryServer, metadataServer, usersServer, userRepository, aclRepository, logger),
			server.NewDebugServer(&config.Cfg.Server, logger),
		),
	)

	debugApp := kratos.New(
		kratos.ID(id),
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Metadata(map[string]string{}),
		kratos.Logger(logger),
		kratos.Server(
			server.NewDebugServer(&config.Cfg.Server, logger),
		),
	)

	launcher := &Launcher{
		config: config.Cfg,

		mainApp:  app,
		debugApp: debugApp,

		compactionDaemon:   background.NewCompactionDaemon(registryRepository, logger),
		protoParsingDaemon: background.NewProtoParsingDaemon(metadataRepository, logger),
	}

	err = CreateRootCommand(launcher).Execute()
	if err != nil {
		logHelper.Errorf("failed to run application: %v", err)
	}
}
