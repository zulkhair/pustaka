package auth_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/app/auth"
	"github.com/zulkhair/pustaka/backend/internal/domain"
	"github.com/zulkhair/pustaka/backend/internal/pkg/hash"
)

func TestLogout_RevokesSession(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	svc := newLoginService(t, store)
	ctx := context.Background()
	seedVerifiedUserSvc(t, store, "walt", "walt@example.com", "waltpassword1")

	tok, err := svc.Login(ctx, auth.LoginInput{Identifier: "walt@example.com", Password: "waltpassword1"})
	require.NoError(t, err)

	require.NoError(t, svc.Logout(ctx, tok.RefreshToken))

	sess, err := store.GetSessionByTokenHash(ctx, hash.HashRefreshToken(tok.RefreshToken))
	require.NoError(t, err)
	require.NotNil(t, sess.RevokedAt, "session must be revoked after logout")
}

func TestLogout_ThenRefresh_Unauthorized(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	svc := newLoginService(t, store)
	ctx := context.Background()
	seedVerifiedUserSvc(t, store, "xena", "xena@example.com", "xenapassword1")

	tok, err := svc.Login(ctx, auth.LoginInput{Identifier: "xena@example.com", Password: "xenapassword1"})
	require.NoError(t, err)
	require.NoError(t, svc.Logout(ctx, tok.RefreshToken))

	_, err = svc.Refresh(ctx, tok.RefreshToken)
	require.ErrorIs(t, err, domain.ErrUnauthorized)
}

func TestLogout_UnknownToken_Idempotent(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	svc := newLoginService(t, store)

	err := svc.Logout(context.Background(), "never-issued-token")
	require.NoError(t, err) // idempotent: unknown token still succeeds
}
