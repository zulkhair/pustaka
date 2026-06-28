package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/domain"
	"github.com/zulkhair/pustaka/backend/internal/testsupport"
)

func TestDocumentRenameAndSoftDelete(t *testing.T) {
	st, cleanup := testsupport.NewTestStore(t)
	defer cleanup()
	ctx := context.Background()
	uid := seedUser(t, st)

	doc, err := st.CreateDocument(ctx, domain.CreateDocumentParams{
		ID: uuid.NewString(), UserID: uid, Title: "Old", Mode: domain.ModeText,
	})
	require.NoError(t, err)

	// rename
	renamed, err := st.UpdateDocumentTitle(ctx, doc.ID, "New Title")
	require.NoError(t, err)
	require.Equal(t, "New Title", renamed.Title)
	got, err := st.GetDocument(ctx, doc.ID)
	require.NoError(t, err)
	require.Equal(t, "New Title", got.Title)

	// soft delete -> excluded from get + list
	require.NoError(t, st.SoftDeleteDocument(ctx, doc.ID))
	_, err = st.GetDocument(ctx, doc.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
	list, err := st.ListDocumentsByUser(ctx, uid)
	require.NoError(t, err)
	require.Empty(t, list)
}

func TestSetDocumentThumbPage(t *testing.T) {
	st, cleanup := testsupport.NewTestStore(t)
	defer cleanup()
	ctx := context.Background()
	uid := seedUser(t, st)

	doc, err := st.CreateDocument(ctx, domain.CreateDocumentParams{
		ID: uuid.NewString(), UserID: uid, Title: "Doc", Mode: domain.ModePhoto,
	})
	require.NoError(t, err)
	require.Equal(t, 1, doc.ThumbPage) // defaults to first page

	updated, err := st.SetDocumentThumbPage(ctx, doc.ID, 3)
	require.NoError(t, err)
	require.Equal(t, 3, updated.ThumbPage)

	got, err := st.GetDocument(ctx, doc.ID)
	require.NoError(t, err)
	require.Equal(t, 3, got.ThumbPage)
}
