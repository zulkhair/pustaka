package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLogoutHandler_RevokesThenRefresh401(t *testing.T) {
	ta := newAuthTestApp(t)
	seedVerifiedUser(t, ta.store, "yuri", "yuri@example.com", "yuripassword1")

	_, loginBody := doJSONBody(t, ta, http.MethodPost, "/api/auth/login",
		map[string]string{"identifier": "yuri@example.com", "password": "yuripassword1"})
	rt := loginBody["data"].(map[string]any)["refreshToken"].(string)

	resp, _ := doJSONBody(t, ta, http.MethodPost, "/api/auth/logout",
		map[string]string{"refreshToken": rt})
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp2, _ := doJSONBody(t, ta, http.MethodPost, "/api/auth/refresh",
		map[string]string{"refreshToken": rt})
	require.Equal(t, http.StatusUnauthorized, resp2.StatusCode)
}

func TestLogoutHandler_UnknownToken_200(t *testing.T) {
	ta := newAuthTestApp(t)
	resp, _ := doJSONBody(t, ta, http.MethodPost, "/api/auth/logout",
		map[string]string{"refreshToken": "never-issued"})
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestLogoutHandler_BadBody_400(t *testing.T) {
	ta := newAuthTestApp(t)
	resp := doRaw(t, ta, http.MethodPost, "/api/auth/logout", "}{")
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
