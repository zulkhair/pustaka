package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/app/auth"
	"github.com/zulkhair/pustaka/backend/internal/domain"
	"github.com/zulkhair/pustaka/backend/internal/pkg/hash"
)

func TestRefresh_Valid_RotatesAndRevokesOld(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	svc := newLoginService(t, store)
	ctx := context.Background()
	seedVerifiedUserSvc(t, store, "rita", "rita@example.com", "ritapassword1")

	first, err := svc.Login(ctx, auth.LoginInput{Identifier: "rita@example.com", Password: "ritapassword1"})
	require.NoError(t, err)

	second, err := svc.Refresh(ctx, first.RefreshToken)
	require.NoError(t, err)
	require.NotEmpty(t, second.AccessToken)
	require.NotEqual(t, first.RefreshToken, second.RefreshToken)

	old, err := store.GetSessionByTokenHash(ctx, hash.HashRefreshToken(first.RefreshToken))
	require.NoError(t, err)
	require.NotNil(t, old.RevokedAt, "old session must be revoked after rotation")

	fresh, err := store.GetSessionByTokenHash(ctx, hash.HashRefreshToken(second.RefreshToken))
	require.NoError(t, err)
	require.Nil(t, fresh.RevokedAt)
}

func TestRefresh_ReuseAfterRotation_RevokesAllAndUnauthorized(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	svc := newLoginService(t, store)
	ctx := context.Background()
	seedVerifiedUserSvc(t, store, "sam", "sam@example.com", "sampassword1")

	first, err := svc.Login(ctx, auth.LoginInput{Identifier: "sam@example.com", Password: "sampassword1"})
	require.NoError(t, err)
	second, err := svc.Refresh(ctx, first.RefreshToken)
	require.NoError(t, err)

	// Replaying the old (now-revoked) token is theft: reject AND kill the live session.
	_, err = svc.Refresh(ctx, first.RefreshToken)
	require.ErrorIs(t, err, domain.ErrUnauthorized)

	live, err := store.GetSessionByTokenHash(ctx, hash.HashRefreshToken(second.RefreshToken))
	require.NoError(t, err)
	require.NotNil(t, live.RevokedAt, "replay must revoke the live session too (theft response)")
}

func TestRefresh_UnknownToken_Unauthorized(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	svc := newLoginService(t, store)

	_, err := svc.Refresh(context.Background(), "this-token-was-never-issued")
	require.ErrorIs(t, err, domain.ErrUnauthorized)
}

func TestRefresh_Expired_Unauthorized(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	svc := newLoginService(t, store)
	ctx := context.Background()
	u := seedVerifiedUserSvc(t, store, "tina", "tina@example.com", "tinapassword1")

	raw := "expired-refresh-token-value"
	_, err := store.CreateSession(ctx, domain.CreateSessionParams{
		ID:               uuid.NewString(),
		UserID:           u.ID,
		RefreshTokenHash: hash.HashRefreshToken(raw),
		ExpiresAt:        time.Now().Add(-time.Minute), // already expired
	})
	require.NoError(t, err)

	_, err = svc.Refresh(ctx, raw)
	require.ErrorIs(t, err, domain.ErrUnauthorized)
}
