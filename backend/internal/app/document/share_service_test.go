package document_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/adapter/store"
	"github.com/zulkhair/pustaka/backend/internal/app/document"
	"github.com/zulkhair/pustaka/backend/internal/domain"
	"github.com/zulkhair/pustaka/backend/internal/testsupport"
)

func mustVerifiedUser(t *testing.T, st *store.Store, username, email string) domain.User {
	t.Helper()
	u := mustUnverifiedUser(t, st, username, email)
	require.NoError(t, st.SetUserEmailVerified(context.Background(), u.ID))
	u.EmailVerified = true
	return u
}

func mustUnverifiedUser(t *testing.T, st *store.Store, username, email string) domain.User {
	t.Helper()
	u, err := st.CreateUser(context.Background(), domain.CreateUserParams{
		ID: uuid.NewString(), Username: username, Email: email,
		PasswordHash: "x", Role: domain.RoleUser,
	})
	require.NoError(t, err)
	return u
}

func TestShareDocumentFlow(t *testing.T) {
	st, cleanup := testsupport.NewTestStore(t)
	defer cleanup()
	ctx := context.Background()
	svc := document.New(st, nil)

	owner := mustVerifiedUser(t, st, "owner", "owner@e.com")
	sharee := mustVerifiedUser(t, st, "sharee", "sharee@e.com")
	unverified := mustUnverifiedUser(t, st, "unv", "unv@e.com")

	doc, err := st.CreateDocument(ctx, domain.CreateDocumentParams{
		ID: uuid.NewString(), UserID: owner.ID, Title: "D", Mode: "text",
	})
	require.NoError(t, err)

	// happy path: share with verified user, editor downgraded to viewer
	share, err := svc.ShareDocument(ctx, owner.ID, doc.ID, document.ShareInput{
		Email: "sharee@e.com", Permission: domain.PermissionEditor,
	})
	require.NoError(t, err)
	require.Equal(t, domain.PermissionViewer, share.Permission)
	require.Equal(t, sharee.ID, share.SharedWithUserID)

	// list shares
	views, err := svc.ListShares(ctx, owner.ID, doc.ID)
	require.NoError(t, err)
	require.Len(t, views, 1)
	require.Equal(t, "sharee@e.com", views[0].Email)

	// nonexistent email -> generic ErrValidation
	_, err = svc.ShareDocument(ctx, owner.ID, doc.ID, document.ShareInput{Email: "ghost@e.com"})
	require.ErrorIs(t, err, domain.ErrValidation)

	// unverified -> same generic ErrValidation
	_, err = svc.ShareDocument(ctx, owner.ID, doc.ID, document.ShareInput{Email: "unv@e.com"})
	require.ErrorIs(t, err, domain.ErrValidation)
	_ = unverified

	// non-owner cannot share (sharee has a share -> Forbidden on write)
	_, err = svc.ShareDocument(ctx, sharee.ID, doc.ID, document.ShareInput{Email: "owner@e.com"})
	require.ErrorIs(t, err, domain.ErrForbidden)

	// shared-with-me view
	owned, shared, err := svc.ListDocumentsWithShared(ctx, sharee.ID)
	require.NoError(t, err)
	require.Empty(t, owned)
	require.Len(t, shared, 1)
	require.Equal(t, doc.ID, shared[0].ID)

	ownerOwned, ownerShared, err := svc.ListDocumentsWithShared(ctx, owner.ID)
	require.NoError(t, err)
	require.Len(t, ownerOwned, 1)
	require.Empty(t, ownerShared)

	// revoke -> sharee loses access
	require.NoError(t, svc.RevokeShare(ctx, owner.ID, doc.ID, sharee.ID))
	_, shared, err = svc.ListDocumentsWithShared(ctx, sharee.ID)
	require.NoError(t, err)
	require.Empty(t, shared)
}
