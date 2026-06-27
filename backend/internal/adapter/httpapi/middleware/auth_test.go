package middleware_test

import (
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/adapter/httpapi/middleware"
	"github.com/zulkhair/pustaka/backend/internal/domain"
	"github.com/zulkhair/pustaka/backend/internal/pkg/jwt"
)

const testSecret = "test-secret-0123456789"

func newProbeApp() *fiber.App {
	app := fiber.New()
	app.Get("/protected", middleware.RequireAuth(testSecret), func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"userID": c.Locals("userID"),
			"role":   c.Locals("role"),
		})
	})
	app.Get("/admin", middleware.RequireAuth(testSecret), middleware.RequireAdmin(), func(c *fiber.Ctx) error {
		return c.SendString("admin-ok")
	})
	return app
}

func TestRequireAuth_NoToken_401(t *testing.T) {
	app := newProbeApp()
	req := httptest.NewRequest("GET", "/protected", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 401, resp.StatusCode)
}

func TestRequireAuth_BadToken_401(t *testing.T) {
	app := newProbeApp()
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer not-a-jwt")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 401, resp.StatusCode)
}

func TestRequireAuth_GoodToken_SetsLocals(t *testing.T) {
	app := newProbeApp()
	token, err := jwt.GenerateAccess("user-123", domain.RoleUser, testSecret, 15*time.Minute)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	require.Contains(t, string(body), "user-123")
	require.Contains(t, string(body), domain.RoleUser)
}

func TestRequireAdmin_AdminAllowed(t *testing.T) {
	app := newProbeApp()
	token, err := jwt.GenerateAccess("admin-1", domain.RoleAdmin, testSecret, 15*time.Minute)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, "admin-ok", string(body))
}

func TestRequireAdmin_UserBlocked_403(t *testing.T) {
	app := newProbeApp()
	token, err := jwt.GenerateAccess("user-2", domain.RoleUser, testSecret, 15*time.Minute)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 403, resp.StatusCode)
}
