package testsupport_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/domain"
	"github.com/zulkhair/pustaka/backend/internal/testsupport"
)

func TestNewTestStoreRoundtrip(t *testing.T) {
	st, cleanup := testsupport.NewTestStore(t)
	defer cleanup()
	ctx := context.Background()

	u, err := st.CreateUser(ctx, domain.CreateUserParams{
		ID: uuid.NewString(), Username: "smoke", Email: "smoke@example.com",
		PasswordHash: "x", Role: domain.RoleUser,
	})
	require.NoError(t, err)

	got, err := st.GetUserByEmail(ctx, "smoke@example.com")
	require.NoError(t, err)
	require.Equal(t, u.ID, got.ID)

	_, err = st.CreateEmailVerification(ctx, domain.CreateEmailVerificationParams{
		ID: uuid.NewString(), UserID: u.ID, CodeHash: "h",
		ExpiresAt: time.Now().Add(15 * time.Minute),
	})
	require.NoError(t, err)

	testsupport.BackdateVerification(t, st.Pool(), u.ID, time.Now().Add(-10*time.Minute))

	ev, err := st.GetActiveEmailVerification(ctx, u.ID)
	require.NoError(t, err)
	require.WithinDuration(t, time.Now().Add(-10*time.Minute), ev.CreatedAt, time.Minute)
}
