package document_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/app/document"
	"github.com/zulkhair/pustaka/backend/internal/domain"
	"github.com/zulkhair/pustaka/backend/internal/testsupport"
)

func TestRenameAndDeleteAuthz(t *testing.T) {
	st, cleanup := testsupport.NewTestStore(t)
	defer cleanup()
	ctx := context.Background()
	svc := document.New(st, nil)

	owner := newUser(t, st)
	sharee := newUser(t, st)
	stranger := newUser(t, st)

	doc, err := st.CreateDocument(ctx, domain.CreateDocumentParams{
		ID: uuid.NewString(), UserID: owner, Title: "Old", Mode: domain.ModeText,
	})
	require.NoError(t, err)
	_, err = st.CreateShare(ctx, domain.CreateShareParams{
		ID: uuid.NewString(), DocumentID: doc.ID, SharedWithUserID: sharee, Permission: domain.PermissionViewer,
	})
	require.NoError(t, err)

	// owner renames
	d, err := svc.Rename(ctx, owner, doc.ID, "Renamed")
	require.NoError(t, err)
	require.Equal(t, "Renamed", d.Title)
	// empty title -> validation
	_, err = svc.Rename(ctx, owner, doc.ID, "   ")
	require.ErrorIs(t, err, domain.ErrValidation)
	// sharee cannot write -> forbidden
	err = svc.Delete(ctx, sharee, doc.ID)
	require.ErrorIs(t, err, domain.ErrForbidden)
	// stranger -> not found
	_, err = svc.Rename(ctx, stranger, doc.ID, "x")
	require.ErrorIs(t, err, domain.ErrNotFound)

	// owner deletes -> subsequent read is not found
	require.NoError(t, svc.Delete(ctx, owner, doc.ID))
	_, err = svc.Get(ctx, owner, doc.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
}
