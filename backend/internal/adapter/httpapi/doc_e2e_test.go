package httpapi_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/domain"
)

func TestDocumentFlow_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e in short mode")
	}
	ta := newTestApp(t)
	app := ta.app
	ta.ai.TranscribeFn = func([]byte) (string, error) { return "# page one text", nil }
	ta.ai.TransformFn = func(string, domain.Template) (string, error) { return "# clean doc", nil }

	const email = "doc-e2e@example.com"
	// register -> verify (reuse Plan-1 e2ePost + mock mailer)
	code, _ := e2ePost(t, app, "/api/auth/register",
		map[string]string{"username": "docuser", "email": email, "password": "hunter2pass"}, "")
	require.Equal(t, 200, code)
	vcode, ok := ta.mailer.CodeFor(email)
	require.True(t, ok)
	code, env := e2ePost(t, app, "/api/auth/verify-email", map[string]string{"email": email, "code": vcode}, "")
	require.Equal(t, 200, code)
	var tokens struct {
		AccessToken string `json:"accessToken"`
	}
	require.NoError(t, json.Unmarshal(env.Data, &tokens))
	access := tokens.AccessToken
	require.NotEmpty(t, access)

	// create a photo document
	code, env = e2ePost(t, app, "/api/documents", map[string]string{"title": "Scan", "mode": "photo"}, access)
	require.Equal(t, 200, code)
	var doc struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(env.Data, &doc))
	require.NotEmpty(t, doc.ID)

	// add a page (multipart) -> OCR runs via mock
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormFile("file", "p.jpg")
	_, _ = fw.Write([]byte("img-bytes"))
	_ = w.Close()
	pReq := httptest.NewRequest(http.MethodPost, "/api/documents/"+doc.ID+"/pages", &buf)
	pReq.Header.Set("Content-Type", w.FormDataContentType())
	pReq.Header.Set("Authorization", "Bearer "+access)
	pResp, err := app.Test(pReq, -1)
	require.NoError(t, err)
	require.Equal(t, 200, pResp.StatusCode)
	pBody, _ := io.ReadAll(pResp.Body)
	require.Contains(t, string(pBody), "page one text")

	// transform (document scope, Clean Markdown)
	code, env = e2ePost(t, app, "/api/documents/"+doc.ID+"/transform",
		map[string]string{"template_id": domain.TemplateIDCleanMarkdown}, access)
	require.Equal(t, 200, code)
	var out struct {
		ID      string `json:"id"`
		Content string `json:"content"`
	}
	require.NoError(t, json.Unmarshal(env.Data, &out))
	require.Equal(t, "# clean doc", out.Content)

	// fetch the output
	getReq := httptest.NewRequest(http.MethodGet, "/api/outputs/"+out.ID, nil)
	getReq.Header.Set("Authorization", "Bearer "+access)
	getResp, err := app.Test(getReq, -1)
	require.NoError(t, err)
	require.Equal(t, 200, getResp.StatusCode)
}
