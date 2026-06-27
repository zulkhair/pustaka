package httpapi_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/adapter/httpapi"
	"github.com/zulkhair/pustaka/backend/internal/adapter/mail"
	"github.com/zulkhair/pustaka/backend/internal/app/auth"
	"github.com/zulkhair/pustaka/backend/internal/config"
	"github.com/zulkhair/pustaka/backend/internal/testsupport"
)

func newMeHandler(t *testing.T) (*httpapi.AuthHandler, *testApp) {
	t.Helper()
	st, cleanup := testsupport.NewTestStore(t)
	t.Cleanup(cleanup)
	cfg := config.Config{BcryptCost: 4, JWTSecret: "me-secret", AccessTTL: 15 * time.Minute}
	h := httpapi.NewAuthHandler(auth.New(st, mail.NewMockMailer(), cfg))
	return h, &testApp{store: st}
}

func TestMeHandler_NoUserID_401(t *testing.T) {
	h, _ := newMeHandler(t)
	app := fiber.New()
	app.Get("/api/auth/me", h.Me) // no middleware: c.Locals("userID") is empty
	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestMeHandler_WithUserID_200(t *testing.T) {
	h, ta := newMeHandler(t)
	u := seedVerifiedUser(t, ta.store, "nadia", "nadia@example.com", "nadiapassword1")

	app := fiber.New()
	// Tiny inline middleware stands in for RequireAuth: it sets the principal id
	// so the Me handler test does not depend on the real JWT/RequireAuth (Task 19).
	app.Get("/api/auth/me", func(c *fiber.Ctx) error {
		c.Locals("userID", u.ID)
		return c.Next()
	}, h.Me)

	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	require.Contains(t, string(body), "nadia")
	require.Contains(t, string(body), "nadia@example.com")
	require.Contains(t, string(body), `"emailVerified":true`)
}
