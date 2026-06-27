package ocr_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/adapter/ai"
	"github.com/zulkhair/pustaka/backend/internal/adapter/blob"
	"github.com/zulkhair/pustaka/backend/internal/app/ocr"
	"github.com/zulkhair/pustaka/backend/internal/domain"
	"github.com/zulkhair/pustaka/backend/internal/testsupport"
)

func mkUser(t *testing.T, st interface {
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

func TestRunStoresOCRResult(t *testing.T) {
	st, cleanup := testsupport.NewTestStore(t)
	defer cleanup()
	mock := ai.NewMock()
	mock.TranscribeFn = func([]byte) (string, error) { return "# transcribed", nil }
	svc := ocr.New(st, mock, blob.NewMemory())
	ctx := context.Background()
	uid := mkUser(t, st)

	doc, err := st.CreateDocument(ctx, domain.CreateDocumentParams{ID: uuid.NewString(), UserID: uid, Title: "T", Mode: domain.ModePhoto})
	require.NoError(t, err)
	page, err := st.CreatePage(ctx, domain.CreatePageParams{ID: uuid.NewString(), DocumentID: doc.ID, PageNumber: 1, Status: domain.StatusProcessing})
	require.NoError(t, err)

	res, err := svc.Run(ctx, page, []byte("img"))
	require.NoError(t, err)
	require.Equal(t, "# transcribed", res.Text)
	require.Equal(t, domain.StatusDone, res.Status)

	latest, err := st.GetLatestOCRResult(ctx, page.ID)
	require.NoError(t, err)
	require.Equal(t, "# transcribed", latest.Text)
}

func TestRerunReadsImageFromBlob(t *testing.T) {
	st, cleanup := testsupport.NewTestStore(t)
	defer cleanup()
	bs := blob.NewMemory()
	mock := ai.NewMock()
	calls := 0
	mock.TranscribeFn = func([]byte) (string, error) { calls++; return "rerun-text", nil }
	svc := ocr.New(st, mock, bs)
	ctx := context.Background()
	uid := mkUser(t, st)

	doc, err := st.CreateDocument(ctx, domain.CreateDocumentParams{ID: uuid.NewString(), UserID: uid, Title: "T", Mode: domain.ModePhoto})
	require.NoError(t, err)
	path, err := bs.Put(uid, doc.ID, 1, []byte("stored-img"))
	require.NoError(t, err)
	_, err = st.CreatePage(ctx, domain.CreatePageParams{ID: uuid.NewString(), DocumentID: doc.ID, PageNumber: 1, ImagePath: &path, ThumbPath: &path, Status: domain.StatusDone})
	require.NoError(t, err)

	res, err := svc.Rerun(ctx, uid, doc.ID, 1)
	require.NoError(t, err)
	require.Equal(t, "rerun-text", res.Text)
	require.Equal(t, 1, calls)
}

func TestRerunOwnerScopedAndNoImage(t *testing.T) {
	st, cleanup := testsupport.NewTestStore(t)
	defer cleanup()
	svc := ocr.New(st, ai.NewMock(), blob.NewMemory())
	ctx := context.Background()
	owner := mkUser(t, st)
	other := mkUser(t, st)
	doc, err := st.CreateDocument(ctx, domain.CreateDocumentParams{ID: uuid.NewString(), UserID: owner, Title: "T", Mode: domain.ModeText})
	require.NoError(t, err)
	_, err = st.CreatePage(ctx, domain.CreatePageParams{ID: uuid.NewString(), DocumentID: doc.ID, PageNumber: 1, Status: domain.StatusDone})
	require.NoError(t, err)

	// foreign user -> NotFound
	_, err = svc.Rerun(ctx, other, doc.ID, 1)
	require.ErrorIs(t, err, domain.ErrNotFound)

	// owner, but text-mode page has no stored image -> NotFound
	_, err = svc.Rerun(ctx, owner, doc.ID, 1)
	require.ErrorIs(t, err, domain.ErrNotFound)
}
