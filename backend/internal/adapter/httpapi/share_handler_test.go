package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShareEndpoints(t *testing.T) {
	ta := newTestApp(t)

	owner := registerVerifiedUser(t, ta, "owner", "owner@e.com", "hunter2pass")
	sharee := registerVerifiedUser(t, ta, "sharee", "sharee@e.com", "hunter2pass")

	docID := createDocument(t, ta, owner.access, "My Doc")

	// owner shares with the verified sharee
	resp := doAuthedJSON(t, ta, http.MethodPost, "/api/documents/"+docID+"/shares", owner.access,
		map[string]string{"email": "sharee@e.com", "permission": "viewer"})
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// list shares (owner)
	resp = doAuthedJSON(t, ta, http.MethodGet, "/api/documents/"+docID+"/shares", owner.access, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// share with unknown email -> generic 400
	resp = doAuthedJSON(t, ta, http.MethodPost, "/api/documents/"+docID+"/shares", owner.access,
		map[string]string{"email": "ghost@e.com", "permission": "viewer"})
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// non-owner sharee tries to share -> 403 (visible via the share, but write-denied)
	resp = doAuthedJSON(t, ta, http.MethodPost, "/api/documents/"+docID+"/shares", sharee.access,
		map[string]string{"email": "owner@e.com", "permission": "viewer"})
	require.Equal(t, http.StatusForbidden, resp.StatusCode)

	// revoke
	resp = doAuthedJSON(t, ta, http.MethodDelete,
		"/api/documents/"+docID+"/shares/"+sharee.id, owner.access, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}
