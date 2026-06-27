package httpapi_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

type envResp struct {
	Status  int             `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func e2ePost(t *testing.T, app interface {
	Test(*http.Request, ...int) (*http.Response, error)
}, path string, body any, bearer string) (int, envResp) {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest("POST", path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	b, _ := io.ReadAll(resp.Body)
	var e envResp
	require.NoError(t, json.Unmarshal(b, &e), "body: %s", string(b))
	return resp.StatusCode, e
}

func TestAuthFlow_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e in short mode")
	}
	ta := newTestApp(t) // full wired app via BuildApp (Task 21 harness)
	app := ta.app
	mockMail := ta.mailer

	const email = "e2e@example.com"

	// 1. register (generic success, no tokens yet)
	code, _ := e2ePost(t, app, "/api/auth/register",
		map[string]string{"username": "e2euser", "email": email, "password": "hunter2pass"}, "")
	require.Equal(t, 200, code)

	// 2. read the verification code from the mock mailer
	vcode, ok := mockMail.CodeFor(email)
	require.True(t, ok, "mock mailer should have captured a code")
	require.Len(t, vcode, 6)

	// 3. verify-email -> tokens
	code, env := e2ePost(t, app, "/api/auth/verify-email",
		map[string]string{"email": email, "code": vcode}, "")
	require.Equal(t, 200, code)
	var verifyTokens struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ExpiresIn    int    `json:"expiresIn"`
	}
	require.NoError(t, json.Unmarshal(env.Data, &verifyTokens))
	require.NotEmpty(t, verifyTokens.AccessToken)
	require.NotEmpty(t, verifyTokens.RefreshToken)

	// 4. login -> tokens
	code, env = e2ePost(t, app, "/api/auth/login",
		map[string]string{"identifier": email, "password": "hunter2pass"}, "")
	require.Equal(t, 200, code)
	var loginTokens struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
	}
	require.NoError(t, json.Unmarshal(env.Data, &loginTokens))
	require.NotEmpty(t, loginTokens.AccessToken)
	require.NotEmpty(t, loginTokens.RefreshToken)

	// 5. GET /auth/me with the access token
	meReq := httptest.NewRequest("GET", "/api/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+loginTokens.AccessToken)
	meResp, err := app.Test(meReq, -1)
	require.NoError(t, err)
	require.Equal(t, 200, meResp.StatusCode)
	meBody, _ := io.ReadAll(meResp.Body)
	require.Contains(t, string(meBody), "e2euser")
	require.Contains(t, string(meBody), email)

	// 6. refresh -> new tokens, old refresh token is rotated out
	code, env = e2ePost(t, app, "/api/auth/refresh",
		map[string]string{"refreshToken": loginTokens.RefreshToken}, "")
	require.Equal(t, 200, code)
	var refreshed struct {
		RefreshToken string `json:"refreshToken"`
	}
	require.NoError(t, json.Unmarshal(env.Data, &refreshed))
	require.NotEmpty(t, refreshed.RefreshToken)
	require.NotEqual(t, loginTokens.RefreshToken, refreshed.RefreshToken)

	// 7. logout the rotated refresh token
	code, _ = e2ePost(t, app, "/api/auth/logout",
		map[string]string{"refreshToken": refreshed.RefreshToken}, "")
	require.Equal(t, 200, code)

	// 8. refresh again with the logged-out token must fail (401)
	code, _ = e2ePost(t, app, "/api/auth/refresh",
		map[string]string{"refreshToken": refreshed.RefreshToken}, "")
	require.Equal(t, 401, code)
}

func TestSeededAdminCanLogin_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e in short mode")
	}
	ta := newTestApp(t) // full wired app via BuildApp (Task 21 harness)

	// Seed an admin (pre-verified) directly via the store, mirroring cmd/seed.
	// Cost 4 matches the harness's BcryptCost; bcrypt verifies regardless of cost.
	require.NoError(t, seedAdminForTest(t, ta.store, "admin", "admin@pustaka.local", "admin123", 4))

	code, _ := e2ePost(t, ta.app, "/api/auth/login",
		map[string]string{"identifier": "admin", "password": "admin123"}, "")
	require.Equal(t, 200, code, "seeded admin must be able to log in")
}
