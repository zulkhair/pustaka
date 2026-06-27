package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/domain"
)

type verifiedUser struct {
	id     string
	access string
}

// registerVerifiedUser runs register -> read MockMailer code -> verify -> /me,
// returning the user's id + access token. Reuses Plan 1's e2ePost.
func registerVerifiedUser(t *testing.T, ta *testApp, username, email, pw string) verifiedUser {
	t.Helper()
	code, _ := e2ePost(t, ta.app, "/api/auth/register",
		map[string]string{"username": username, "email": email, "password": pw}, "")
	require.Equal(t, 200, code)

	vcode, ok := ta.mailer.CodeFor(email)
	require.True(t, ok, "mock mailer should have a code for %s", email)

	code, env := e2ePost(t, ta.app, "/api/auth/verify-email",
		map[string]string{"email": email, "code": vcode}, "")
	require.Equal(t, 200, code)
	var tok struct {
		AccessToken string `json:"accessToken"`
	}
	require.NoError(t, json.Unmarshal(env.Data, &tok))
	require.NotEmpty(t, tok.AccessToken)

	_, body := doAuthedJSONBody(t, ta, http.MethodGet, "/api/auth/me", tok.AccessToken, nil)
	data := body["data"].(map[string]any)
	return verifiedUser{id: data["id"].(string), access: tok.AccessToken}
}

func createDocument(t *testing.T, ta *testApp, access, title string) string {
	t.Helper()
	_, body := doAuthedJSONBody(t, ta, http.MethodPost, "/api/documents", access,
		map[string]string{"title": title, "mode": "photo"})
	data := body["data"].(map[string]any)
	return data["id"].(string)
}

// createOutput seeds one output row for docID via the store the harness exposes.
func createOutput(t *testing.T, ta *testApp, ownerID, docID string) string {
	t.Helper()
	id := uuid.NewString()
	_, err := ta.store.CreateOutput(context.Background(), domain.CreateOutputParams{
		ID: id, UserID: ownerID, DocumentID: docID,
		TemplateID: domain.TemplateIDCleanMarkdown, Content: "x", Model: "test", Status: domain.StatusDone,
	})
	require.NoError(t, err)
	return id
}

func doAuthed(t *testing.T, ta *testApp, method, path, bearer string, body any) (*http.Response, []byte) {
	t.Helper()
	var r io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		r = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := ta.app.Test(req, -1)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp, b
}

func doAuthedJSON(t *testing.T, ta *testApp, method, path, bearer string, body any) *http.Response {
	t.Helper()
	resp, _ := doAuthed(t, ta, method, path, bearer, body)
	return resp
}

func doAuthedJSONBody(t *testing.T, ta *testApp, method, path, bearer string, body any) (*http.Response, map[string]any) {
	t.Helper()
	resp, b := doAuthed(t, ta, method, path, bearer, body)
	var m map[string]any
	if len(b) > 0 {
		require.NoError(t, json.Unmarshal(b, &m), "body: %s", string(b))
	}
	return resp, m
}

// requireSharedContains asserts GET /documents data.shared contains docID and data.owned is empty.
func requireSharedContains(t *testing.T, body map[string]any, docID string) {
	t.Helper()
	data := body["data"].(map[string]any)
	shared, _ := data["shared"].([]any)
	found := false
	for _, it := range shared {
		if m, ok := it.(map[string]any); ok && m["id"] == docID {
			found = true
		}
	}
	require.True(t, found, "document %s should appear under shared", docID)
	owned, _ := data["owned"].([]any)
	require.Empty(t, owned)
}
