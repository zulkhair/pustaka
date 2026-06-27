package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/adapter/mail"
	"github.com/zulkhair/pustaka/backend/internal/app/auth"
	"github.com/zulkhair/pustaka/backend/internal/config"
	"github.com/zulkhair/pustaka/backend/internal/domain"
	"github.com/zulkhair/pustaka/backend/internal/testsupport"
)

func registerCfg() config.Config {
	return config.Config{BcryptCost: 4, CodeTTL: 15 * time.Minute}
}

func TestRegisterCreatesUnverifiedUserAndSendsCode(t *testing.T) {
	store, cleanup := testsupport.NewTestStore(t)
	defer cleanup()
	mock := mail.NewMockMailer()
	svc := auth.New(store, mock, registerCfg())
	ctx := context.Background()

	err := svc.Register(ctx, auth.RegisterInput{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "supersecret",
	})
	require.NoError(t, err)

	u, err := store.GetUserByEmail(ctx, "alice@example.com")
	require.NoError(t, err)
	require.False(t, u.EmailVerified)
	require.Equal(t, domain.RoleUser, u.Role)
	require.NotEqual(t, "supersecret", u.PasswordHash)

	ev, err := store.GetActiveEmailVerification(ctx, u.ID)
	require.NoError(t, err)
	require.NotEmpty(t, ev.CodeHash)

	require.Equal(t, "alice@example.com", mock.LastEmail)
	require.Len(t, mock.LastCode, 6)
}

func TestRegisterDuplicateEmailConflict(t *testing.T) {
	store, cleanup := testsupport.NewTestStore(t)
	defer cleanup()
	svc := auth.New(store, mail.NewMockMailer(), registerCfg())
	ctx := context.Background()

	in := auth.RegisterInput{Username: "bob", Email: "bob@example.com", Password: "supersecret"}
	require.NoError(t, svc.Register(ctx, in))

	dup := auth.RegisterInput{Username: "bob2", Email: "bob@example.com", Password: "supersecret"}
	err := svc.Register(ctx, dup)
	require.ErrorIs(t, err, domain.ErrConflict)
}

func TestRegisterValidationErrors(t *testing.T) {
	store, cleanup := testsupport.NewTestStore(t)
	defer cleanup()
	svc := auth.New(store, mail.NewMockMailer(), registerCfg())
	ctx := context.Background()

	cases := []auth.RegisterInput{
		{Username: "", Email: "x@example.com", Password: "supersecret"},
		{Username: "carol", Email: "not-an-email", Password: "supersecret"},
		{Username: "carol", Email: "carol@example.com", Password: "short"},
	}
	for _, in := range cases {
		err := svc.Register(ctx, in)
		require.ErrorIs(t, err, domain.ErrValidation)
	}
}
