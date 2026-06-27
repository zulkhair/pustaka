package auth_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/domain"
)

func TestMe_ReturnsUser(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	svc := newLoginService(t, store)
	u := seedVerifiedUserSvc(t, store, "mia", "mia@example.com", "miapassword1")

	got, err := svc.Me(context.Background(), u.ID)
	require.NoError(t, err)
	require.Equal(t, u.ID, got.ID)
	require.Equal(t, "mia", got.Username)
	require.Equal(t, "mia@example.com", got.Email)
	require.True(t, got.EmailVerified)
}

func TestMe_UnknownUser_NotFound(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	svc := newLoginService(t, store)

	_, err := svc.Me(context.Background(), "no-such-id")
	require.ErrorIs(t, err, domain.ErrNotFound)
}
