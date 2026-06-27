package document_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/adapter/blob"
	"github.com/zulkhair/pustaka/backend/internal/app/document"
	"github.com/zulkhair/pustaka/backend/internal/domain"
	"github.com/zulkhair/pustaka/backend/internal/testsupport"
)

func newUser(t *testing.T, st interface {
	CreateUser(context.Context, domain.CreateUserParams) (domain.User, error)
}) string {
	t.Helper()
	u, err := st.CreateUser(context.Background(), domain.CreateUserParams{
		ID: uuid.NewString(), Username: "u-" + uuid.NewString()[:8],
		Email: uuid.NewString()[:8] + "@e.com", PasswordHash: "x", Role: domain.RoleUser,
	})
	require.NoError(t, err)
	return u.ID
}

func TestCreateAndList(t *testing.T) {
	st, cleanup := testsupport.NewTestStore(t)
	defer cleanup()
	svc := document.New(st, blob.NewMemory())
	ctx := context.Background()
	uid := newUser(t, st)

	doc, err := svc.Create(ctx, uid, "My Doc", domain.ModePhoto)
	require.NoError(t, err)
	require.Equal(t, "My Doc", doc.Title)
	require.Equal(t, domain.ModePhoto, doc.Mode)

	docs, err := svc.List(ctx, uid)
	require.NoError(t, err)
	require.Len(t, docs, 1)
}

func TestCreateValidatesModeAndTitle(t *testing.T) {
	st, cleanup := testsupport.NewTestStore(t)
	defer cleanup()
	svc := document.New(st, blob.NewMemory())
	ctx := context.Background()
	uid := newUser(t, st)

	_, err := svc.Create(ctx, uid, "", domain.ModePhoto)
	require.ErrorIs(t, err, domain.ErrValidation)
	_, err = svc.Create(ctx, uid, "T", "bogus")
	require.ErrorIs(t, err, domain.ErrValidation)
}

func TestGetOwnerScoped(t *testing.T) {
	st, cleanup := testsupport.NewTestStore(t)
	defer cleanup()
	svc := document.New(st, blob.NewMemory())
	ctx := context.Background()
	owner := newUser(t, st)
	other := newUser(t, st)

	doc, err := svc.Create(ctx, owner, "Owned", domain.ModeText)
	require.NoError(t, err)

	detail, err := svc.Get(ctx, owner, doc.ID)
	require.NoError(t, err)
	require.Equal(t, doc.ID, detail.Document.ID)
	require.Empty(t, detail.Pages)

	// other user cannot read it -> NotFound (no existence leak)
	_, err = svc.Get(ctx, other, doc.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)

	// missing doc -> NotFound
	_, err = svc.Get(ctx, owner, uuid.NewString())
	require.ErrorIs(t, err, domain.ErrNotFound)
}
