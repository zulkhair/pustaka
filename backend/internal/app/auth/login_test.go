package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/adapter/mail"
	"github.com/zulkhair/pustaka/backend/internal/app/auth"
	"github.com/zulkhair/pustaka/backend/internal/config"
	"github.com/zulkhair/pustaka/backend/internal/domain"
	"github.com/zulkhair/pustaka/backend/internal/pkg/hash"
)

func newLoginService(t *testing.T, store domain.Store) *auth.Service {
	t.Helper()
	cfg := config.Config{
		BcryptCost: 4,
		JWTSecret:  "test-secret",
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 720 * time.Hour,
	}
	return auth.New(store, mail.NewMockMailer(), cfg)
}

func seedVerifiedUserSvc(t *testing.T, store domain.Store, username, email, pw string) domain.User {
	t.Helper()
	ctx := context.Background()
	ph, err := hash.HashPassword(pw, 4)
	require.NoError(t, err)
	u, err := store.CreateUser(ctx, domain.CreateUserParams{
		ID: uuid.NewString(), Username: username, Email: email,
		PasswordHash: ph, Role: domain.RoleUser,
	})
	require.NoError(t, err)
	require.NoError(t, store.SetUserEmailVerified(ctx, u.ID))
	u.EmailVerified = true
	return u
}

func TestLogin_GoodCreds_IssuesTokensAndSession(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	svc := newLoginService(t, store)
	u := seedVerifiedUserSvc(t, store, "alice", "alice@example.com", "hunter2pw")

	tok, err := svc.Login(context.Background(), auth.LoginInput{
		Identifier: "alice@example.com", Password: "hunter2pw",
	})
	require.NoError(t, err)
	require.NotEmpty(t, tok.AccessToken)
	require.NotEmpty(t, tok.RefreshToken)
	require.Equal(t, int((15 * time.Minute).Seconds()), tok.ExpiresIn)

	sess, err := store.GetSessionByTokenHash(context.Background(), hash.HashRefreshToken(tok.RefreshToken))
	require.NoError(t, err)
	require.Equal(t, u.ID, sess.UserID)
}

func TestLogin_ByUsername_Works(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	svc := newLoginService(t, store)
	seedVerifiedUserSvc(t, store, "bob", "bob@example.com", "passwordpw")

	tok, err := svc.Login(context.Background(), auth.LoginInput{
		Identifier: "bob", Password: "passwordpw",
	})
	require.NoError(t, err)
	require.NotEmpty(t, tok.AccessToken)
}

func TestLogin_ByEmail_NormalizesCase(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	svc := newLoginService(t, store)
	seedVerifiedUserSvc(t, store, "cara", "cara@example.com", "carapassword1")

	tok, err := svc.Login(context.Background(), auth.LoginInput{
		Identifier: "  Cara@Example.com ", Password: "carapassword1",
	})
	require.NoError(t, err)
	require.NotEmpty(t, tok.AccessToken)
}

func TestLogin_WrongPassword_InvalidCredentials(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	svc := newLoginService(t, store)
	seedVerifiedUserSvc(t, store, "carol", "carol@example.com", "rightpassword")

	_, err := svc.Login(context.Background(), auth.LoginInput{
		Identifier: "carol@example.com", Password: "wrongpassword",
	})
	require.ErrorIs(t, err, domain.ErrInvalidCredentials)
}

func TestLogin_UnknownIdentifier_InvalidCredentials(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	svc := newLoginService(t, store)

	_, err := svc.Login(context.Background(), auth.LoginInput{
		Identifier: "ghost@example.com", Password: "whatever123",
	})
	require.ErrorIs(t, err, domain.ErrInvalidCredentials) // identical to wrong-password = enumeration-safe
}

func TestLogin_Unverified_EmailNotVerified(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	svc := newLoginService(t, store)
	ctx := context.Background()
	ph, err := hash.HashPassword("secretpass1", 4)
	require.NoError(t, err)
	_, err = store.CreateUser(ctx, domain.CreateUserParams{
		ID: uuid.NewString(), Username: "dan", Email: "dan@example.com",
		PasswordHash: ph, Role: domain.RoleUser,
	})
	require.NoError(t, err)

	_, err = svc.Login(ctx, auth.LoginInput{Identifier: "dan@example.com", Password: "secretpass1"})
	require.ErrorIs(t, err, domain.ErrEmailNotVerified)
}
