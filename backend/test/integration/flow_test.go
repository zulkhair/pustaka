//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/domain"
)

// TestFullFlow drives the real server over HTTP: seed+login -> me -> create doc
// -> add page (OCR) -> serve image -> templates -> transform -> get output ->
// refresh -> logout -> refresh-again-401. AI is the fake Ollama unless
// OLLAMA_REAL=1 (then assertions loosen to 200 + non-empty).
func TestFullFlow(t *testing.T) {
	env := startServer(t)
	const (
		email = "int@example.com"
		pw    = "hunter2pass"
	)
	env.seedVerifiedUser("intuser", email, pw)

	// login
	code, e := env.postJSON("/api/auth/login", "", map[string]string{"identifier": email, "password": pw})
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, 0, e.Status)
	var tokens struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
	}
	env.decode(e, &tokens)
	require.NotEmpty(t, tokens.AccessToken)
	require.NotEmpty(t, tokens.RefreshToken)
	access := tokens.AccessToken

	// me
	code, e = env.getJSON("/api/auth/me", access)
	require.Equal(t, http.StatusOK, code)
	var me struct {
		Username string `json:"username"`
		Email    string `json:"email"`
	}
	env.decode(e, &me)
	require.Equal(t, "intuser", me.Username)
	require.Equal(t, email, me.Email)

	// version is open
	code, _ = env.getJSON("/api/version", "")
	require.Equal(t, http.StatusOK, code)

	// create a photo document
	code, e = env.postJSON("/api/documents", access, map[string]string{"title": "Scan", "mode": "photo"})
	require.Equal(t, http.StatusOK, code)
	var doc struct {
		ID string `json:"id"`
	}
	env.decode(e, &doc)
	require.NotEmpty(t, doc.ID)

	// add a page (multipart) -> OCR runs through the real Ollama adapter
	code, e = env.postMultipart("/api/documents/"+doc.ID+"/pages", access, syntheticJPEG(t))
	require.Equal(t, http.StatusOK, code)
	var page struct {
		PageNumber int    `json:"pageNumber"`
		OCRText    string `json:"ocrText"`
	}
	env.decode(e, &page)
	require.Equal(t, 1, page.PageNumber)
	if env.real {
		require.NotEmpty(t, page.OCRText)
	} else {
		require.Equal(t, mockOCRText, page.OCRText)
	}

	// the stored image is served back as JPEG
	imgResp, _ := env.do(http.MethodGet, "/api/documents/"+doc.ID+"/pages/1/image", access, "", nil)
	require.Equal(t, http.StatusOK, imgResp.StatusCode)
	require.Equal(t, "image/jpeg", imgResp.Header.Get("Content-Type"))

	// templates lists the built-ins
	code, e = env.getJSON("/api/templates", access)
	require.Equal(t, http.StatusOK, code)
	var tmpls []struct {
		ID string `json:"id"`
	}
	env.decode(e, &tmpls)
	require.GreaterOrEqual(t, len(tmpls), 2)

	// transform (document scope, Clean Markdown)
	code, e = env.postJSON("/api/documents/"+doc.ID+"/transform", access,
		map[string]string{"template_id": domain.TemplateIDCleanMarkdown})
	require.Equal(t, http.StatusOK, code)
	var out struct {
		ID      string `json:"id"`
		Content string `json:"content"`
	}
	env.decode(e, &out)
	require.NotEmpty(t, out.ID)
	if env.real {
		require.NotEmpty(t, out.Content)
	} else {
		require.Equal(t, mockTransformText, out.Content)
	}

	// fetch the output
	code, _ = env.getJSON("/api/outputs/"+out.ID, access)
	require.Equal(t, http.StatusOK, code)

	// refresh rotates the token
	code, e = env.postJSON("/api/auth/refresh", "", map[string]string{"refreshToken": tokens.RefreshToken})
	require.Equal(t, http.StatusOK, code)
	var refreshed struct {
		RefreshToken string `json:"refreshToken"`
	}
	env.decode(e, &refreshed)
	require.NotEqual(t, tokens.RefreshToken, refreshed.RefreshToken)

	// logout, then the logged-out token can no longer refresh
	code, _ = env.postJSON("/api/auth/logout", "", map[string]string{"refreshToken": refreshed.RefreshToken})
	require.Equal(t, http.StatusOK, code)

	code, _ = env.postJSON("/api/auth/refresh", "", map[string]string{"refreshToken": refreshed.RefreshToken})
	require.Equal(t, http.StatusUnauthorized, code)
}

func TestForeignDocumentIsHidden(t *testing.T) {
	env := startServer(t)
	env.seedVerifiedUser("owner", "owner@example.com", "ownerpass1")

	// owner logs in + creates a doc
	_, e := env.postJSON("/api/auth/login", "", map[string]string{"identifier": "owner@example.com", "password": "ownerpass1"})
	var ot struct {
		AccessToken string `json:"accessToken"`
	}
	env.decode(e, &ot)
	_, e = env.postJSON("/api/documents", ot.AccessToken, map[string]string{"title": "Secret", "mode": "text"})
	var doc struct {
		ID string `json:"id"`
	}
	env.decode(e, &doc)
	require.NotEmpty(t, doc.ID)

	// a different user cannot see it -> 404 (no existence leak)
	env.seedVerifiedUser("intruder", "intruder@example.com", "intruderpw1")
	_, e2 := env.postJSON("/api/auth/login", "", map[string]string{"identifier": "intruder@example.com", "password": "intruderpw1"})
	var it struct {
		AccessToken string `json:"accessToken"`
	}
	env.decode(e2, &it)
	code, _ := env.getJSON("/api/documents/"+doc.ID, it.AccessToken)
	require.Equal(t, http.StatusNotFound, code)
}
