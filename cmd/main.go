package main

import (
	"context"
	"os"
	"sync"

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

type appRunner interface {
	Run() error
}

type Launcher struct {
	config                  *config.Config
	loadConfig              func() *config.Config
	buildMainApp            func() (appRunner, error)
	buildDebugApp           func() (appRunner, error)
	buildCompactionDaemon   func() (background.Daemon, error)
	buildProtoParsingDaemon func() (background.Daemon, error)
}

func (l *Launcher) getConfig() *config.Config {
	if l.config != nil {
		return l.config
	}
	if l.loadConfig != nil {
		l.config = l.loadConfig()
	}
	return l.config
}

func main() {
	os.Exit(run())
}

func run() int {
	logger := log.DefaultLogger
	logHelper := log.NewHelper(logger)

	var (
		cfgOnce sync.Once
		poolMu  sync.Mutex
		pool    *pgxpool.Pool
	)
	loadCfg := func() *config.Config {
		cfgOnce.Do(func() {
			config.NewLoader().MustLoad()
		})
		return config.Cfg
	}
	getPool := func() (*pgxpool.Pool, error) {
		poolMu.Lock()
		defer poolMu.Unlock()
		if pool != nil {
			return pool, nil
		}
		cfg := loadCfg()
		p, err := pgxpool.New(context.Background(), cfg.Data.Database.DSN)
		if err != nil {
			return nil, err
		}
		pool = p
		return pool, nil
	}
	defer func() {
		poolMu.Lock()
		p := pool
		poolMu.Unlock()
		if p != nil {
			p.Close()
		}
	}()

	launcher := &Launcher{
		loadConfig: loadCfg,
		buildMainApp: func() (appRunner, error) {
			cfg := loadCfg()
			p, err := getPool()
			if err != nil {
				return nil, err
			}
			registryRepository := data.NewRegistryRepository(p, logger)
			metadataRepository := data.NewMetadataRepository(p, logger)
			userRepository := data.NewUserRepository(p, logger)
			aclRepository := data.NewACLRepository(p, logger)
			driftRepository := data.NewDriftRepository(p, logger)
			registryServer := server.NewRegistryServer(registryRepository, metadataRepository, logger)
			metadataServer := server.NewMetadataServer(registryRepository, metadataRepository, logger)
			usersServer := server.NewUsersServer(userRepository, aclRepository, logger)
			driftServer := server.NewDriftServer(driftRepository, logger)

			return kratos.New(
				kratos.ID(id),
				kratos.Name(Name),
				kratos.Version(Version),
				kratos.Metadata(map[string]string{}),
				kratos.Logger(logger),
				kratos.Server(
					server.NewGRPCServer(&cfg.Server, registryServer, metadataServer, usersServer, driftServer, userRepository, aclRepository, logger),
					server.NewHTTPServer(&cfg.Server, registryServer, metadataServer, usersServer, driftServer, userRepository, aclRepository, logger),
					server.NewDebugServer(&cfg.Server, logger),
				),
			), nil
		},
		buildDebugApp: func() (appRunner, error) {
			cfg := loadCfg()
			return kratos.New(
				kratos.ID(id),
				kratos.Name(Name),
				kratos.Version(Version),
				kratos.Metadata(map[string]string{}),
				kratos.Logger(logger),
				kratos.Server(
					server.NewDebugServer(&cfg.Server, logger),
				),
			), nil
		},
		buildCompactionDaemon: func() (background.Daemon, error) {
			p, err := getPool()
			if err != nil {
				return nil, err
			}
			registryRepository := data.NewRegistryRepository(p, logger)
			return background.NewCompactionDaemon(registryRepository, logger), nil
		},
		buildProtoParsingDaemon: func() (background.Daemon, error) {
			cfg := loadCfg()
			p, err := getPool()
			if err != nil {
				return nil, err
			}
			driftNotifier, err := background.NewWebhookDriftNotifier(cfg.Daemons.ProtoParsing.Webhook, logger)
			if err != nil {
				return nil, err
			}
			metadataRepository := data.NewMetadataRepository(p, logger)
			driftRepository := data.NewDriftRepository(p, logger)
			return background.NewProtoParsingDaemon(metadataRepository, driftRepository, logger, driftNotifier), nil
		},
	}

	if err := CreateRootCommand(launcher).Execute(); err != nil {
		logHelper.Errorf("failed to run application: %v", err)
		return 1
	}
	return 0
}
