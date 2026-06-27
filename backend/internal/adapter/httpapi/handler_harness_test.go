package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/adapter/httpapi"
	"github.com/zulkhair/pustaka/backend/internal/adapter/mail"
	"github.com/zulkhair/pustaka/backend/internal/adapter/store"
	"github.com/zulkhair/pustaka/backend/internal/app/auth"
	"github.com/zulkhair/pustaka/backend/internal/config"
	"github.com/zulkhair/pustaka/backend/internal/domain"
	"github.com/zulkhair/pustaka/backend/internal/pkg/hash"
	"github.com/zulkhair/pustaka/backend/internal/testsupport"
)

const handlerTestSecret = "handler-secret-0123456789"

type testApp struct {
	app    *fiber.App
	store  *store.Store
	mailer *mail.MockMailer
}

// newAuthTestApp builds a LOCAL minimal Fiber app over a testcontainers-backed
// store + MockMailer, mounting ONLY the auth POST routes directly (no BuildApp,
// no Mount/RouterDeps, no RateLimit, no middleware import).
func newAuthTestApp(t *testing.T) *testApp {
	t.Helper()
	st, cleanup := testsupport.NewTestStore(t)
	t.Cleanup(cleanup)

	mailer := mail.NewMockMailer()
	cfg := config.Config{
		JWTSecret:      handlerTestSecret,
		AccessTTL:      15 * time.Minute,
		RefreshTTL:     720 * time.Hour,
		BcryptCost:     4,
		CodeTTL:        15 * time.Minute,
		MaxAttempts:    5,
		ResendCooldown: 60 * time.Second,
	}
	h := httpapi.NewAuthHandler(auth.New(st, mailer, cfg))

	app := fiber.New()
	app.Post("/api/auth/register", h.Register)
	app.Post("/api/auth/verify-email", h.VerifyEmail)
	app.Post("/api/auth/resend-verification", h.ResendVerification)
	app.Post("/api/auth/login", h.Login)
	// Tasks 16/17 each append their own mount line (refresh/logout) when implemented.
	return &testApp{app: app, store: st, mailer: mailer}
}

func doJSON(t *testing.T, ta *testApp, method, path string, body any) *http.Response {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	resp, err := ta.app.Test(req, -1)
	require.NoError(t, err)
	return resp
}

func doJSONBody(t *testing.T, ta *testApp, method, path string, body any) (*http.Response, map[string]any) {
	t.Helper()
	resp := doJSON(t, ta, method, path, body)
	b, _ := io.ReadAll(resp.Body)
	var env map[string]any
	require.NoError(t, json.Unmarshal(b, &env), "body: %s", string(b))
	return resp, env
}

func doRaw(t *testing.T, ta *testApp, method, path, raw string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	resp, err := ta.app.Test(req, -1)
	require.NoError(t, err)
	return resp
}

func seedUnverifiedUser(t *testing.T, st *store.Store, username, email string) domain.User {
	t.Helper()
	ctx := context.Background()
	u, err := st.CreateUser(ctx, domain.CreateUserParams{
		ID: uuid.NewString(), Username: username, Email: email,
		PasswordHash: "x", Role: domain.RoleUser,
	})
	require.NoError(t, err)
	_, err = st.CreateEmailVerification(ctx, domain.CreateEmailVerificationParams{
		ID: uuid.NewString(), UserID: u.ID, CodeHash: "seed-hash",
		ExpiresAt: time.Now().Add(15 * time.Minute),
	})
	require.NoError(t, err)
	return u
}

func seedUnverifiedUserWithPassword(t *testing.T, st *store.Store, username, email, pw string) domain.User {
	t.Helper()
	ctx := context.Background()
	ph, err := hash.HashPassword(pw, 4)
	require.NoError(t, err)
	u, err := st.CreateUser(ctx, domain.CreateUserParams{
		ID: uuid.NewString(), Username: username, Email: email,
		PasswordHash: ph, Role: domain.RoleUser,
	})
	require.NoError(t, err)
	return u
}

func seedVerifiedUser(t *testing.T, st *store.Store, username, email, pw string) domain.User {
	t.Helper()
	ctx := context.Background()
	u := seedUnverifiedUserWithPassword(t, st, username, email, pw)
	require.NoError(t, st.SetUserEmailVerified(ctx, u.ID))
	u.EmailVerified = true
	return u
}
