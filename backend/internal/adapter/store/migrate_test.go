package store_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/zulkhair/pustaka/backend/internal/adapter/store"
)

func startPostgres(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	ctr, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("pustaka"),
		postgres.WithUsername("pustaka"),
		postgres.WithPassword("pustaka"),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("5432/tcp").WithStartupTimeout(60*time.Second)),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	conn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	return conn
}

func TestMigrationsApplySchema(t *testing.T) {
	connString := startPostgres(t)

	// Drive the real embedded-FS runner.
	require.NoError(t, store.RunMigrations(connString))

	db, err := sql.Open("pgx", connString)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	for _, tbl := range []string{"web_user", "email_verification", "session"} {
		var got string
		err := db.QueryRow(
			`SELECT table_name FROM information_schema.tables WHERE table_name = $1`, tbl,
		).Scan(&got)
		require.NoError(t, err, "table %s should exist", tbl)
		require.Equal(t, tbl, got)
	}

	cols := map[string][]string{
		"web_user":           {"id", "username", "email", "password_hash", "role", "email_verified", "created_at"},
		"email_verification": {"id", "user_id", "code_hash", "expires_at", "attempts", "consumed_at", "created_at"},
		"session":            {"id", "user_id", "refresh_token_hash", "expires_at", "created_at", "revoked_at"},
	}
	for tbl, want := range cols {
		for _, col := range want {
			var n int
			err := db.QueryRow(
				`SELECT count(*) FROM information_schema.columns WHERE table_name = $1 AND column_name = $2`,
				tbl, col,
			).Scan(&n)
			require.NoError(t, err)
			require.Equal(t, 1, n, "column %s.%s should exist", tbl, col)
		}
	}

	var checkCount int
	require.NoError(t, db.QueryRow(
		`SELECT count(*) FROM information_schema.check_constraints
		 WHERE constraint_schema = 'public' AND check_clause ILIKE '%role%'`,
	).Scan(&checkCount))
	require.GreaterOrEqual(t, checkCount, 1, "role CHECK constraint should exist")

	for _, idx := range []string{"idx_email_verification_user", "idx_session_user", "idx_session_token"} {
		var name string
		require.NoError(t, db.QueryRow(
			`SELECT indexname FROM pg_indexes WHERE indexname = $1`, idx,
		).Scan(&name), "index %s should exist", idx)
		require.Equal(t, idx, name)
	}
}
