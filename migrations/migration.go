package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

//go:embed *.sql
var embedMigrations embed.FS

// Migrate runs all pending database migrations serialized by a PostgreSQL session advisory lock.
func Migrate(db *sql.DB) {
	if err := MigrateWithError(context.Background(), db); err != nil {
		panic(err)
	}
}

// MigrateWithError runs all pending database migrations serialized by a PostgreSQL session advisory lock.
func MigrateWithError(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}

	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("failed to create postgres session locker: %w", err)
	}

	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		db,
		embedMigrations,
		goose.WithSessionLocker(locker),
	)
	if err != nil {
		return fmt.Errorf("failed to create migration provider: %w", err)
	}

	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("failed to apply database migrations: %w", err)
	}
	return nil
}
