package httpapi_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/adapter/httpapi"
	"github.com/zulkhair/pustaka/backend/internal/domain"
)

// seedOCRdDoc inserts a doc with one OCR'd page directly via the store.
func seedOCRdDoc(t *testing.T, ta *docTestApp, uid string) string {
	t.Helper()
	ctx := context.Background()
	doc, err := ta.store.CreateDocument(ctx, domain.CreateDocumentParams{ID: uuid.NewString(), UserID: uid, Title: "X", Mode: domain.ModePhoto})
	require.NoError(t, err)
	p, err := ta.store.CreatePage(ctx, domain.CreatePageParams{ID: uuid.NewString(), DocumentID: doc.ID, PageNumber: 1, Status: domain.StatusDone})
	require.NoError(t, err)
	_, err = ta.store.CreateOCRResult(ctx, domain.CreateOCRResultParams{ID: uuid.NewString(), PageID: p.ID, Model: "glm-ocr", Text: "page text", Status: domain.StatusDone})
	require.NoError(t, err)
	return doc.ID
}

func TestTransformRunAndGetOutput(t *testing.T) {
	ta := newDocTestApp(t)
	uid := seedDocUser(t, ta)
	th := httpapi.NewTransformHandler(ta.xfSvc)
	oh := httpapi.NewOutputHandler(ta.xfSvc)
	g := ta.app.Group("/api", authMW(uid))
	g.Post("/documents/:id/transform", th.Run)
	g.Get("/outputs/:id", oh.Get)
	ta.ai.TransformFn = func(string, domain.Template) (string, error) { return "# transformed", nil }

	docID := seedOCRdDoc(t, ta, uid)

	code, env := postJSON(t, ta, "/api/documents/"+docID+"/transform",
		map[string]string{"template_id": domain.TemplateIDCleanMarkdown})
	require.Equal(t, http.StatusOK, code)
	var out struct {
		ID      string `json:"id"`
		Content string `json:"content"`
	}
	decodeData(t, env, &out)
	require.NotEmpty(t, out.ID)
	require.Equal(t, "# transformed", out.Content)

	code, env = getJSON(t, ta, "/api/outputs/"+out.ID)
	require.Equal(t, http.StatusOK, code)
}

func TestTransformNoOCRReturns400(t *testing.T) {
	ta := newDocTestApp(t)
	uid := seedDocUser(t, ta)
	th := httpapi.NewTransformHandler(ta.xfSvc)
	ta.app.Group("/api", authMW(uid)).Post("/documents/:id/transform", th.Run)

	ctx := context.Background()
	doc, err := ta.store.CreateDocument(ctx, domain.CreateDocumentParams{ID: uuid.NewString(), UserID: uid, Title: "Empty", Mode: domain.ModePhoto})
	require.NoError(t, err)

	code, _ := postJSON(t, ta, "/api/documents/"+doc.ID+"/transform",
		map[string]string{"template_id": domain.TemplateIDCleanMarkdown})
	require.Equal(t, http.StatusBadRequest, code)
}

func TestGetOutputForeignReturns404(t *testing.T) {
	ta := newDocTestApp(t)
	uid := seedDocUser(t, ta)
	other := seedDocUser(t, ta)
	oh := httpapi.NewOutputHandler(ta.xfSvc)
	// mount as `other`
	ta.app.Group("/api", authMW(other)).Get("/outputs/:id", oh.Get)

	ctx := context.Background()
	docID := seedOCRdDoc(t, ta, uid)
	out, err := ta.xfSvc.Run(ctx, uid, docID, domain.TemplateIDCleanMarkdown)
	require.NoError(t, err)

	code, _ := getJSON(t, ta, "/api/outputs/"+out.ID)
	require.Equal(t, http.StatusNotFound, code)
}
