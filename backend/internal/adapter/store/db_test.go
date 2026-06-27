package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func startPostgres(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("pustaka"),
		postgres.WithUsername("pustaka"),
		postgres.WithPassword("pustaka"),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("5432/tcp").WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	return dsn
}

func TestOpenPoolPings(t *testing.T) {
	dsn := startPostgres(t)
	ctx := context.Background()
	pool, err := OpenPool(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, pool.Ping(ctx))
}

func TestRunMigrationsAppliesEmbeddedFiles(t *testing.T) {
	dsn := startPostgres(t)
	ctx := context.Background()

	require.NoError(t, RunMigrations(dsn))

	pool, err := OpenPool(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	var exists bool
	err = pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'smoke')`,
	).Scan(&exists)
	require.NoError(t, err)
	require.True(t, exists, "smoke table should exist after RunMigrations")
}
