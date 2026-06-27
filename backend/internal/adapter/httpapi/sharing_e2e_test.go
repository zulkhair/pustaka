package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/domain"
)

func TestSharingFlow_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e in short mode")
	}
	ta := newTestApp(t)

	owner := registerVerifiedUser(t, ta, "owner", "owner@e.com", "hunter2pass")
	sharee := registerVerifiedUser(t, ta, "sharee", "sharee@e.com", "hunter2pass")
	stranger := registerVerifiedUser(t, ta, "stranger", "stranger@e.com", "hunter2pass")

	docID := createDocument(t, ta, owner.access, "Shared Doc")

	// stranger cannot see the doc at all (owner-isolation)
	require.Equal(t, http.StatusNotFound,
		doAuthedJSON(t, ta, http.MethodGet, "/api/documents/"+docID, stranger.access, nil).StatusCode)

	// owner shares with the verified sharee
	resp := doAuthedJSON(t, ta, http.MethodPost, "/api/documents/"+docID+"/shares", owner.access,
		map[string]string{"email": "sharee@e.com", "permission": "viewer"})
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// sharee can READ the document
	require.Equal(t, http.StatusOK,
		doAuthedJSON(t, ta, http.MethodGet, "/api/documents/"+docID, sharee.access, nil).StatusCode)

	// sharee can READ a shared document's OUTPUTS directly (document-scoped).
	outID := createOutput(t, ta, owner.id, docID)
	require.Equal(t, http.StatusOK,
		doAuthedJSON(t, ta, http.MethodGet, "/api/outputs/"+outID, owner.access, nil).StatusCode)
	require.Equal(t, http.StatusOK,
		doAuthedJSON(t, ta, http.MethodGet, "/api/outputs/"+outID, sharee.access, nil).StatusCode)
	require.Equal(t, http.StatusNotFound,
		doAuthedJSON(t, ta, http.MethodGet, "/api/outputs/"+outID, stranger.access, nil).StatusCode)

	// sharee CANNOT write: transform, rerun OCR, or reshare -> 403
	// authorizeDoc(PermWrite) rejects the sharee before any pipeline work.
	require.Equal(t, http.StatusForbidden,
		doAuthedJSON(t, ta, http.MethodPost, "/api/documents/"+docID+"/transform", sharee.access,
			map[string]string{"template_id": domain.TemplateIDCleanMarkdown}).StatusCode)
	require.Equal(t, http.StatusForbidden,
		doAuthedJSON(t, ta, http.MethodPost, "/api/documents/"+docID+"/pages/1/ocr", sharee.access, nil).StatusCode)
	require.Equal(t, http.StatusForbidden,
		doAuthedJSON(t, ta, http.MethodPost, "/api/documents/"+docID+"/shares", sharee.access,
			map[string]string{"email": "stranger@e.com", "permission": "viewer"}).StatusCode)

	// GET /documents shows the doc under "shared" for the sharee
	listResp, body := doAuthedJSONBody(t, ta, http.MethodGet, "/api/documents", sharee.access, nil)
	require.Equal(t, http.StatusOK, listResp.StatusCode)
	requireSharedContains(t, body, docID)

	// revoke -> sharee loses access immediately (doc + outputs)
	require.Equal(t, http.StatusOK,
		doAuthedJSON(t, ta, http.MethodDelete, "/api/documents/"+docID+"/shares/"+sharee.id, owner.access, nil).StatusCode)
	require.Equal(t, http.StatusNotFound,
		doAuthedJSON(t, ta, http.MethodGet, "/api/documents/"+docID, sharee.access, nil).StatusCode)
	require.Equal(t, http.StatusNotFound,
		doAuthedJSON(t, ta, http.MethodGet, "/api/outputs/"+outID, sharee.access, nil).StatusCode)
}
