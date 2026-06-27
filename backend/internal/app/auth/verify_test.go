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
	"github.com/zulkhair/pustaka/backend/internal/pkg/hash"
)

func verifyTestCfg() config.Config {
	return config.Config{
		BcryptCost:  4,
		CodeTTL:     15 * time.Minute,
		MaxAttempts: 5,
		AccessTTL:   15 * time.Minute,
		RefreshTTL:  720 * time.Hour,
		JWTSecret:   "test-secret",
	}
}

func registerAlice(t *testing.T, store domain.Store, mock *mail.MockMailer, svc *auth.Service) (domain.User, string) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, svc.Register(ctx, auth.RegisterInput{
		Username: "alice", Email: "alice@example.com", Password: "supersecret",
	}))
	u, err := store.GetUserByEmail(ctx, "alice@example.com")
	require.NoError(t, err)
	return u, mock.LastCode
}

func TestVerifyEmailWrongCodeIncrementsAndFails(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	mock := mail.NewMockMailer()
	svc := auth.New(store, mock, verifyTestCfg())
	ctx := context.Background()
	u, _ := registerAlice(t, store, mock, svc)

	_, err := svc.VerifyEmail(ctx, auth.VerifyInput{Email: "alice@example.com", Code: "000000"})
	require.ErrorIs(t, err, domain.ErrInvalidCode)

	ev, err := store.GetActiveEmailVerification(ctx, u.ID)
	require.NoError(t, err)
	require.Equal(t, 1, ev.Attempts)
}

func TestVerifyEmailCorrectCodeIssuesTokens(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	mock := mail.NewMockMailer()
	svc := auth.New(store, mock, verifyTestCfg())
	ctx := context.Background()
	u, code := registerAlice(t, store, mock, svc)

	tokens, err := svc.VerifyEmail(ctx, auth.VerifyInput{Email: "alice@example.com", Code: code})
	require.NoError(t, err)
	require.NotEmpty(t, tokens.AccessToken)
	require.NotEmpty(t, tokens.RefreshToken)
	require.Equal(t, 900, tokens.ExpiresIn)

	verified, err := store.GetUserByID(ctx, u.ID)
	require.NoError(t, err)
	require.True(t, verified.EmailVerified)

	sess, err := store.GetSessionByTokenHash(ctx, hash.HashRefreshToken(tokens.RefreshToken))
	require.NoError(t, err)
	require.Equal(t, u.ID, sess.UserID)
}
