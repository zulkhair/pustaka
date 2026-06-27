package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoginHandler_Good_200WithTokens(t *testing.T) {
	ta := newAuthTestApp(t)
	seedVerifiedUser(t, ta.store, "eve", "eve@example.com", "longpassword1")

	resp, body := doJSONBody(t, ta, http.MethodPost, "/api/auth/login",
		map[string]string{"identifier": "eve@example.com", "password": "longpassword1"})
	require.Equal(t, http.StatusOK, resp.StatusCode)

	data := body["data"].(map[string]any)
	require.NotEmpty(t, data["accessToken"])
	require.NotEmpty(t, data["refreshToken"])
	require.Greater(t, data["expiresIn"].(float64), float64(0))
}

func TestLoginHandler_WrongPassword_401(t *testing.T) {
	ta := newAuthTestApp(t)
	seedVerifiedUser(t, ta.store, "frank", "frank@example.com", "correctpass1")

	resp, body := doJSONBody(t, ta, http.MethodPost, "/api/auth/login",
		map[string]string{"identifier": "frank@example.com", "password": "nope"})
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	require.Equal(t, wrongCredsMsg(t, ta), body["message"])
}

func TestLoginHandler_UnknownIdentifier_401_SameMessage(t *testing.T) {
	ta := newAuthTestApp(t)
	resp, body := doJSONBody(t, ta, http.MethodPost, "/api/auth/login",
		map[string]string{"identifier": "ghost@example.com", "password": "whatever"})
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	require.Equal(t, wrongCredsMsg(t, ta), body["message"]) // enumeration-safe: identical to wrong-password
}

func TestLoginHandler_Unverified_401(t *testing.T) {
	ta := newAuthTestApp(t)
	seedUnverifiedUserWithPassword(t, ta.store, "gina", "gina@example.com", "verifyme123")

	resp, _ := doJSONBody(t, ta, http.MethodPost, "/api/auth/login",
		map[string]string{"identifier": "gina@example.com", "password": "verifyme123"})
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// wrongCredsMsg captures the generic invalid-credentials message once so the
// two enumeration-safe assertions compare against the same source of truth.
func wrongCredsMsg(t *testing.T, ta *testApp) string {
	t.Helper()
	_, body := doJSONBody(t, ta, http.MethodPost, "/api/auth/login",
		map[string]string{"identifier": "zzz@example.com", "password": "x"})
	return body["message"].(string)
}
