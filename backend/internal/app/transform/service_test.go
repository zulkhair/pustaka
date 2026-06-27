package transform_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/adapter/ai"
	"github.com/zulkhair/pustaka/backend/internal/app/transform"
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

// seedDocWithOCR creates a doc with two OCR'd pages and returns its id.
func seedDocWithOCR(t *testing.T, st domain.Store, uid string) string {
	t.Helper()
	ctx := context.Background()
	doc, err := st.CreateDocument(ctx, domain.CreateDocumentParams{ID: uuid.NewString(), UserID: uid, Title: "T", Mode: domain.ModePhoto})
	require.NoError(t, err)
	for i := 1; i <= 2; i++ {
		p, err := st.CreatePage(ctx, domain.CreatePageParams{ID: uuid.NewString(), DocumentID: doc.ID, PageNumber: i, Status: domain.StatusDone})
		require.NoError(t, err)
		_, err = st.CreateOCRResult(ctx, domain.CreateOCRResultParams{ID: uuid.NewString(), PageID: p.ID, Model: "glm-ocr", Text: "page text", Status: domain.StatusDone})
		require.NoError(t, err)
	}
	return doc.ID
}

func TestTransformDocumentScope(t *testing.T) {
	st, cleanup := testsupport.NewTestStore(t)
	defer cleanup()
	mock := ai.NewMock()
	mock.TransformFn = func(text string, tmpl domain.Template) (string, error) { return "# combined", nil }
	svc := transform.New(st, mock)
	ctx := context.Background()
	uid := mkUser(t, st)
	docID := seedDocWithOCR(t, st, uid)

	out, err := svc.Run(ctx, uid, docID, domain.TemplateIDCleanMarkdown)
	require.NoError(t, err)
	require.Equal(t, "# combined", out.Content)
	require.Equal(t, domain.StatusDone, out.Status)
	require.Equal(t, 1, mock.TransformCalls, "document scope runs Transform once")
}

func TestTransformPageScope(t *testing.T) {
	st, cleanup := testsupport.NewTestStore(t)
	defer cleanup()
	mock := ai.NewMock()
	mock.TransformFn = func(text string, tmpl domain.Template) (string, error) { return `{"k":"v"}`, nil }
	svc := transform.New(st, mock)
	ctx := context.Background()
	uid := mkUser(t, st)
	docID := seedDocWithOCR(t, st, uid)

	out, err := svc.Run(ctx, uid, docID, domain.TemplateIDStructuredJSON)
	require.NoError(t, err)
	require.Equal(t, 2, mock.TransformCalls, "page scope runs Transform per page")

	// content is a JSON array of per-page entries
	var entries []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(out.Content), &entries))
	require.Len(t, entries, 2)
	require.Contains(t, entries[0], "page_number")
	require.Contains(t, entries[0], "result")
}

func TestTransformOwnerScoped(t *testing.T) {
	st, cleanup := testsupport.NewTestStore(t)
	defer cleanup()
	svc := transform.New(st, ai.NewMock())
	ctx := context.Background()
	owner := mkUser(t, st)
	other := mkUser(t, st)
	docID := seedDocWithOCR(t, st, owner)

	_, err := svc.Run(ctx, other, docID, domain.TemplateIDCleanMarkdown)
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestTransformNoOCRFails(t *testing.T) {
	st, cleanup := testsupport.NewTestStore(t)
	defer cleanup()
	svc := transform.New(st, ai.NewMock())
	ctx := context.Background()
	uid := mkUser(t, st)
	doc, err := st.CreateDocument(ctx, domain.CreateDocumentParams{ID: uuid.NewString(), UserID: uid, Title: "Empty", Mode: domain.ModePhoto})
	require.NoError(t, err)

	_, err = svc.Run(ctx, uid, doc.ID, domain.TemplateIDCleanMarkdown)
	require.ErrorIs(t, err, domain.ErrValidation)
}

func TestGetOutputOwnerScoped(t *testing.T) {
	st, cleanup := testsupport.NewTestStore(t)
	defer cleanup()
	svc := transform.New(st, ai.NewMock())
	ctx := context.Background()
	owner := mkUser(t, st)
	other := mkUser(t, st)
	docID := seedDocWithOCR(t, st, owner)
	out, err := svc.Run(ctx, owner, docID, domain.TemplateIDCleanMarkdown)
	require.NoError(t, err)

	got, err := svc.GetOutput(ctx, owner, out.ID)
	require.NoError(t, err)
	require.Equal(t, out.ID, got.ID)

	_, err = svc.GetOutput(ctx, other, out.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
}
