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

func TestSetThumbnailAuthzAndValidation(t *testing.T) {
	st, cleanup := testsupport.NewTestStore(t)
	defer cleanup()
	ctx := context.Background()
	svc := document.New(st, nil)

	owner := newUser(t, st)
	sharee := newUser(t, st)
	stranger := newUser(t, st)

	doc, err := st.CreateDocument(ctx, domain.CreateDocumentParams{
		ID: uuid.NewString(), UserID: owner, Title: "Doc", Mode: domain.ModePhoto,
	})
	require.NoError(t, err)
	_, err = st.CreateShare(ctx, domain.CreateShareParams{
		ID: uuid.NewString(), DocumentID: doc.ID, SharedWithUserID: sharee, Permission: domain.PermissionViewer,
	})
	require.NoError(t, err)

	thumb := "blob/thumb.jpg"
	// page 1 has an image+thumb; page 2 has neither (e.g. failed capture)
	_, err = st.CreatePage(ctx, domain.CreatePageParams{
		ID: uuid.NewString(), DocumentID: doc.ID, PageNumber: 1,
		ImagePath: &thumb, ThumbPath: &thumb, Status: domain.StatusDone,
	})
	require.NoError(t, err)
	_, err = st.CreatePage(ctx, domain.CreatePageParams{
		ID: uuid.NewString(), DocumentID: doc.ID, PageNumber: 2, Status: domain.StatusFailed,
	})
	require.NoError(t, err)

	// owner sets a valid page
	d, err := svc.SetThumbnail(ctx, owner, doc.ID, 1)
	require.NoError(t, err)
	require.Equal(t, 1, d.ThumbPage)

	// non-existent page -> validation
	_, err = svc.SetThumbnail(ctx, owner, doc.ID, 99)
	require.ErrorIs(t, err, domain.ErrValidation)
	// page without a thumbnail -> validation
	_, err = svc.SetThumbnail(ctx, owner, doc.ID, 2)
	require.ErrorIs(t, err, domain.ErrValidation)
	// sharee (read-only) -> forbidden
	_, err = svc.SetThumbnail(ctx, sharee, doc.ID, 1)
	require.ErrorIs(t, err, domain.ErrForbidden)
	// stranger -> not found
	_, err = svc.SetThumbnail(ctx, stranger, doc.ID, 1)
	require.ErrorIs(t, err, domain.ErrNotFound)
}
