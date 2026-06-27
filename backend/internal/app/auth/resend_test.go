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
	"github.com/zulkhair/pustaka/backend/internal/testsupport"
)

func newResendService(t *testing.T, store domain.Store) (*auth.Service, *mail.MockMailer) {
	t.Helper()
	mailer := mail.NewMockMailer()
	cfg := config.Config{
		BcryptCost:     4,
		CodeTTL:        15 * time.Minute,
		ResendCooldown: 60 * time.Second,
	}
	return auth.New(store, mailer, cfg), mailer
}

func TestResendVerification_UnknownEmail_NoOp(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	svc, mailer := newResendService(t, store)

	err := svc.ResendVerification(context.Background(), "nobody@example.com")
	require.NoError(t, err)
	require.Len(t, mailer.Sends, 0, "no mail must be sent for unknown email")
}

func TestResendVerification_AlreadyVerified_NoOp(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	svc, mailer := newResendService(t, store)
	ctx := context.Background()

	u, err := store.CreateUser(ctx, domain.CreateUserParams{
		ID: uuid.NewString(), Username: "verified", Email: "verified@example.com",
		PasswordHash: "x", Role: domain.RoleUser,
	})
	require.NoError(t, err)
	require.NoError(t, store.SetUserEmailVerified(ctx, u.ID))

	err = svc.ResendVerification(ctx, "verified@example.com")
	require.NoError(t, err)
	require.Len(t, mailer.Sends, 0, "verified users must not get a resend")
}

func TestResendVerification_WithinCooldown_SilentNoOp(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	svc, mailer := newResendService(t, store)
	ctx := context.Background()

	u, err := store.CreateUser(ctx, domain.CreateUserParams{
		ID: uuid.NewString(), Username: "fresh", Email: "fresh@example.com",
		PasswordHash: "x", Role: domain.RoleUser,
	})
	require.NoError(t, err)
	_, err = store.CreateEmailVerification(ctx, domain.CreateEmailVerificationParams{
		ID: uuid.NewString(), UserID: u.ID, CodeHash: "h",
		ExpiresAt: time.Now().Add(15 * time.Minute),
	})
	require.NoError(t, err)

	// SILENT no-op: the service swallows the cooldown internally and returns nil.
	err = svc.ResendVerification(ctx, "fresh@example.com")
	require.NoError(t, err)
	require.Len(t, mailer.Sends, 0)
}

func TestResendVerification_AfterCooldown_SendsNewCode(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	svc, mailer := newResendService(t, store)
	ctx := context.Background()

	u, err := store.CreateUser(ctx, domain.CreateUserParams{
		ID: uuid.NewString(), Username: "stale", Email: "stale@example.com",
		PasswordHash: "x", Role: domain.RoleUser,
	})
	require.NoError(t, err)
	_, err = store.CreateEmailVerification(ctx, domain.CreateEmailVerificationParams{
		ID: uuid.NewString(), UserID: u.ID, CodeHash: "old",
		ExpiresAt: time.Now().Add(15 * time.Minute),
	})
	require.NoError(t, err)
	// Age the verification past the cooldown window via the shared harness helper.
	testsupport.BackdateVerification(t, store.Pool(), u.ID, time.Now().Add(-5*time.Minute))

	err = svc.ResendVerification(ctx, "stale@example.com")
	require.NoError(t, err)
	require.Len(t, mailer.Sends, 1)
	require.Equal(t, "stale@example.com", mailer.LastEmail)
	require.Len(t, mailer.LastCode, 6)
}
