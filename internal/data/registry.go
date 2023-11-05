package data

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type RegistryRepository interface {
	RegisterModule(ctx context.Context, moduleName string) error
}

type registryRepository struct {
	pool *pgxpool.Pool
}

func NewRegistryRepository(pool *pgxpool.Pool) RegistryRepository {
	return &registryRepository{
		pool: pool,
	}
}

func (r *registryRepository) RegisterModule(ctx context.Context, moduleName string) error {
	// Insert module
	_, err := r.pool.Exec(ctx,
		"INSERT INTO modules (name) VALUES ($1) ON CONFLICT (name) DO NOTHING",
		moduleName)
	if err != nil {
		return fmt.Errorf("could not insert module into database: %w", err)
	}

	return nil
}
