package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRefreshHandler_Valid_200NewTokens(t *testing.T) {
	ta := newAuthTestApp(t)
	seedVerifiedUser(t, ta.store, "uma", "uma@example.com", "umapassword1")

	_, loginBody := doJSONBody(t, ta, http.MethodPost, "/api/auth/login",
		map[string]string{"identifier": "uma@example.com", "password": "umapassword1"})
	first := loginBody["data"].(map[string]any)["refreshToken"].(string)

	resp, body := doJSONBody(t, ta, http.MethodPost, "/api/auth/refresh",
		map[string]string{"refreshToken": first})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	second := body["data"].(map[string]any)["refreshToken"].(string)
	require.NotEqual(t, first, second)
}

func TestRefreshHandler_ReuseAfterRotation_401(t *testing.T) {
	ta := newAuthTestApp(t)
	seedVerifiedUser(t, ta.store, "vic", "vic@example.com", "vicpassword1")

	_, loginBody := doJSONBody(t, ta, http.MethodPost, "/api/auth/login",
		map[string]string{"identifier": "vic@example.com", "password": "vicpassword1"})
	first := loginBody["data"].(map[string]any)["refreshToken"].(string)

	resp1, _ := doJSONBody(t, ta, http.MethodPost, "/api/auth/refresh",
		map[string]string{"refreshToken": first})
	require.Equal(t, http.StatusOK, resp1.StatusCode)

	resp2, _ := doJSONBody(t, ta, http.MethodPost, "/api/auth/refresh",
		map[string]string{"refreshToken": first})
	require.Equal(t, http.StatusUnauthorized, resp2.StatusCode)
}

func TestRefreshHandler_BadBody_400(t *testing.T) {
	ta := newAuthTestApp(t)
	resp := doRaw(t, ta, http.MethodPost, "/api/auth/refresh", "{bad")
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
