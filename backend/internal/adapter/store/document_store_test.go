package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/domain"
	"github.com/zulkhair/pustaka/backend/internal/testsupport"
)

func seedUser(t *testing.T, st interface {
	CreateUser(context.Context, domain.CreateUserParams) (domain.User, error)
}) string {
	t.Helper()
	u, err := st.CreateUser(context.Background(), domain.CreateUserParams{
		ID: uuid.NewString(), Username: "owner-" + uuid.NewString()[:8],
		Email: uuid.NewString()[:8] + "@example.com", PasswordHash: "x", Role: domain.RoleUser,
	})
	require.NoError(t, err)
	return u.ID
}

func TestDocumentLifecycle(t *testing.T) {
	st, cleanup := testsupport.NewTestStore(t)
	defer cleanup()
	ctx := context.Background()
	uid := seedUser(t, st)

	doc, err := st.CreateDocument(ctx, domain.CreateDocumentParams{
		ID: uuid.NewString(), UserID: uid, Title: "Receipts", Mode: domain.ModePhoto,
	})
	require.NoError(t, err)
	require.Equal(t, domain.StatusPending, doc.Status)
	require.Equal(t, 0, doc.PageCount)

	docs, err := st.ListDocumentsByUser(ctx, uid)
	require.NoError(t, err)
	require.Len(t, docs, 1)

	n, err := st.IncrementDocumentPageCount(ctx, doc.ID)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	require.NoError(t, st.SetDocumentStatus(ctx, doc.ID, domain.StatusDone))
	got, err := st.GetDocument(ctx, doc.ID)
	require.NoError(t, err)
	require.Equal(t, domain.StatusDone, got.Status)
}

func TestPageAndOCR(t *testing.T) {
	st, cleanup := testsupport.NewTestStore(t)
	defer cleanup()
	ctx := context.Background()
	uid := seedUser(t, st)
	doc, err := st.CreateDocument(ctx, domain.CreateDocumentParams{
		ID: uuid.NewString(), UserID: uid, Title: "T", Mode: domain.ModePhoto,
	})
	require.NoError(t, err)

	img := "u/d/1.jpg"
	page, err := st.CreatePage(ctx, domain.CreatePageParams{
		ID: uuid.NewString(), DocumentID: doc.ID, PageNumber: 1,
		ImagePath: &img, ThumbPath: &img, Width: 800, Height: 600, Status: domain.StatusDone,
	})
	require.NoError(t, err)
	require.NotNil(t, page.ImagePath)
	require.Equal(t, "u/d/1.jpg", *page.ImagePath)

	got, err := st.GetPageByNumber(ctx, doc.ID, 1)
	require.NoError(t, err)
	require.Equal(t, page.ID, got.ID)

	require.NoError(t, st.ClearPageImage(ctx, page.ID))
	cleared, err := st.GetPageByNumber(ctx, doc.ID, 1)
	require.NoError(t, err)
	require.Nil(t, cleared.ImagePath)

	_, err = st.CreateOCRResult(ctx, domain.CreateOCRResultParams{
		ID: uuid.NewString(), PageID: page.ID, Model: "glm-ocr", Text: "# hi", Status: domain.StatusDone,
	})
	require.NoError(t, err)
	ocr, err := st.GetLatestOCRResult(ctx, page.ID)
	require.NoError(t, err)
	require.Equal(t, "# hi", ocr.Text)
}

func TestSeededTemplates(t *testing.T) {
	st, cleanup := testsupport.NewTestStore(t)
	defer cleanup()
	ctx := context.Background()

	tmpls, err := st.ListTemplates(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(tmpls), 2)

	md, err := st.GetTemplate(ctx, domain.TemplateIDCleanMarkdown)
	require.NoError(t, err)
	require.Equal(t, domain.ScopeDocument, md.Scope)
	require.Equal(t, domain.FormatMarkdown, md.OutputFormat)
	require.True(t, md.IsBuiltin)
	require.Nil(t, md.OwnerUserID)

	js, err := st.GetTemplate(ctx, domain.TemplateIDStructuredJSON)
	require.NoError(t, err)
	require.Equal(t, domain.ScopePage, js.Scope)
	require.Equal(t, domain.FormatJSON, js.OutputFormat)
	require.NotNil(t, js.JSONSchema)
}

func TestOutputRoundtrip(t *testing.T) {
	st, cleanup := testsupport.NewTestStore(t)
	defer cleanup()
	ctx := context.Background()
	uid := seedUser(t, st)
	doc, err := st.CreateDocument(ctx, domain.CreateDocumentParams{
		ID: uuid.NewString(), UserID: uid, Title: "T", Mode: domain.ModeText,
	})
	require.NoError(t, err)

	out, err := st.CreateOutput(ctx, domain.CreateOutputParams{
		ID: uuid.NewString(), UserID: uid, DocumentID: doc.ID,
		TemplateID: domain.TemplateIDCleanMarkdown, Content: "# out", Model: "qwen2.5", Status: domain.StatusDone,
	})
	require.NoError(t, err)

	got, err := st.GetOutput(ctx, out.ID)
	require.NoError(t, err)
	require.Equal(t, "# out", got.Content)

	list, err := st.ListOutputsByDocument(ctx, doc.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
}
