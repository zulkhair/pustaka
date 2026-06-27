package testsupport

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/zulkhair/pustaka/backend/internal/adapter/store"
)

// NewTestStore boots an ephemeral Postgres, applies migrations, and returns a
// concrete *store.Store plus a cleanup func. Used by every DB-backed test.
func NewTestStore(t *testing.T) (*store.Store, func()) {
	t.Helper()
	ctx := context.Background()
	ctr, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("pustaka"),
		postgres.WithUsername("pustaka"),
		postgres.WithPassword("pustaka"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)
	require.NoError(t, err)

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, store.RunMigrations(dsn))

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)

	st := store.New(pool)
	cleanup := func() {
		pool.Close()
		_ = ctr.Terminate(ctx)
	}
	return st, cleanup
}

// BackdateVerification rewrites email_verification.created_at for a user, so
// resend-cooldown tests can simulate a stale verification row.
func BackdateVerification(t *testing.T, pool *pgxpool.Pool, userID string, ts time.Time) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`UPDATE email_verification SET created_at = $2 WHERE user_id = $1`, userID, ts)
	require.NoError(t, err)
}
