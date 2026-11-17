package migrations

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
)

func TestMigrateWithError_ValidationAndCancelledContext(t *testing.T) {
	err := MigrateWithError(context.Background(), nil)
	require.ErrorContains(t, err, "database handle is nil")

	db, err := sql.Open("pgx", "postgres://pbuf:pbuf@127.0.0.1:1/pbuf_registry?sslmode=disable")
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = MigrateWithError(ctx, db)
	require.Error(t, err)
}
