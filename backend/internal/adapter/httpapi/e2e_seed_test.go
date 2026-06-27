package httpapi_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/zulkhair/pustaka/backend/internal/adapter/store"
	"github.com/zulkhair/pustaka/backend/internal/domain"
	"github.com/zulkhair/pustaka/backend/internal/pkg/hash"
)

// seedAdminForTest mirrors cmd/seed: hash the password, create a pre-verified admin.
func seedAdminForTest(t *testing.T, st *store.Store, username, email, pw string, cost int) error {
	t.Helper()
	ctx := context.Background()
	ph, err := hash.HashPassword(pw, cost)
	if err != nil {
		return err
	}
	u, err := st.CreateUser(ctx, domain.CreateUserParams{
		ID: uuid.NewString(), Username: username, Email: email,
		PasswordHash: ph, Role: domain.RoleAdmin,
	})
	if err != nil {
		return err
	}
	return st.SetUserEmailVerified(ctx, u.ID)
}
