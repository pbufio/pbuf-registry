package main

import (
	"context"
	"database/sql"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pbufio/pbuf-registry/internal/config"
	"github.com/pbufio/pbuf-registry/migrations"
)

const migrationTimeout = 3 * time.Minute

func main() {
	os.Exit(runMigrations())
}

func runMigrations() int {
	logHelper := log.NewHelper(log.DefaultLogger)
	config.NewLoader().MustLoad()

	db, err := sql.Open("pgx", config.Cfg.Data.Database.DSN)
	if err != nil {
		logHelper.Errorf("failed to open database connection: %v", err)
		return 1
	}
	defer func() { _ = db.Close() }()

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ctx, cancel := context.WithTimeout(sigCtx, migrationTimeout)
	defer cancel()

	if err := migrations.MigrateWithError(ctx, db); err != nil {
		logHelper.Errorf("database migrations failed: %v", err)
		return 1
	}
	return 0
}
