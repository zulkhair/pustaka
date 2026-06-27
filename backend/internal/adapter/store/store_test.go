package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/zulkhair/pustaka/backend/internal/adapter/store"
	"github.com/zulkhair/pustaka/backend/internal/domain"
)

func newStore(t *testing.T) *store.Store {
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
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	require.NoError(t, store.RunMigrations(dsn))

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return store.New(pool)
}

func TestCreateAndGetUserRoundtrip(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	want, err := s.CreateUser(ctx, domain.CreateUserParams{
		ID:           uuid.NewString(),
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
		Role:         domain.RoleUser,
	})
	require.NoError(t, err)

	got, err := s.GetUserByEmail(ctx, "alice@example.com")
	require.NoError(t, err)
	require.Equal(t, want.ID, got.ID)
	require.Equal(t, "alice", got.Username)
	require.Equal(t, domain.RoleUser, got.Role)
	require.False(t, got.EmailVerified)
}

func TestGetUserByEmailNotFound(t *testing.T) {
	s := newStore(t)
	_, err := s.GetUserByEmail(context.Background(), "nobody@example.com")
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestExecTxRollsBackOnError(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	id := uuid.NewString()

	sentinel := errors.New("boom")
	err := s.ExecTx(ctx, func(tx domain.Store) error {
		_, cerr := tx.CreateUser(ctx, domain.CreateUserParams{
			ID:           id,
			Username:     "bob",
			Email:        "bob@example.com",
			PasswordHash: "hash",
			Role:         domain.RoleUser,
		})
		require.NoError(t, cerr)
		return sentinel
	})
	require.ErrorIs(t, err, sentinel)

	_, err = s.GetUserByID(ctx, id)
	require.ErrorIs(t, err, domain.ErrNotFound)
}
