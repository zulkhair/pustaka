package httpapi_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResendHandler_UniformGeneric200(t *testing.T) {
	ta := newAuthTestApp(t)

	// unknown email
	resp := doJSON(t, ta, http.MethodPost, "/api/auth/resend-verification",
		map[string]string{"email": "nobody@example.com"})
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// verified user
	v := seedUnverifiedUser(t, ta.store, "verif", "verif@example.com")
	require.NoError(t, ta.store.SetUserEmailVerified(context.Background(), v.ID))
	resp = doJSON(t, ta, http.MethodPost, "/api/auth/resend-verification",
		map[string]string{"email": "verif@example.com"})
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// fresh unverified user, immediate resend -> within cooldown, still 200 (silent)
	seedUnverifiedUser(t, ta.store, "fresh", "fresh@example.com")
	resp = doJSON(t, ta, http.MethodPost, "/api/auth/resend-verification",
		map[string]string{"email": "fresh@example.com"})
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestResendHandler_BadBody_400(t *testing.T) {
	ta := newAuthTestApp(t)
	resp := doRaw(t, ta, http.MethodPost, "/api/auth/resend-verification", "not-json")
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
