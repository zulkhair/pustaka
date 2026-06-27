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

func TestServiceAuthz(t *testing.T) {
	st, cleanup := testsupport.NewTestStore(t)
	defer cleanup()
	ctx := context.Background()

	svc := document.New(st, nil) // blob not used by the Get/AddPage-guard paths

	owner := newUser(t, st)
	sharee := newUser(t, st)
	stranger := newUser(t, st)

	doc, err := st.CreateDocument(ctx, domain.CreateDocumentParams{
		ID: uuid.NewString(), UserID: owner, Title: "D", Mode: "text",
	})
	require.NoError(t, err)
	_, err = st.CreateShare(ctx, domain.CreateShareParams{
		ID: uuid.NewString(), DocumentID: doc.ID,
		SharedWithUserID: sharee, Permission: domain.PermissionViewer,
	})
	require.NoError(t, err)

	// owner read OK
	_, err = svc.Get(ctx, owner, doc.ID)
	require.NoError(t, err)
	// sharee read OK
	_, err = svc.Get(ctx, sharee, doc.ID)
	require.NoError(t, err)
	// sharee write (AddPage) -> Forbidden; authorizeDoc(PermWrite) short-circuits
	// before any blob/OCR work, so the nil ocrRunner is never reached.
	_, err = svc.AddPage(ctx, sharee, doc.ID, []byte("img"), nil)
	require.ErrorIs(t, err, domain.ErrForbidden)
	// stranger read -> NotFound
	_, err = svc.Get(ctx, stranger, doc.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
}
