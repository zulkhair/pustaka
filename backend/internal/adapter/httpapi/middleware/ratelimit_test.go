package middleware_test

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/adapter/httpapi/middleware"
)

func newRateLimitApp(path string, max int, window time.Duration) *fiber.App {
	app := fiber.New()
	app.Get(path, middleware.RateLimit(max, window), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})
	return app
}

func doGet(t *testing.T, app *fiber.App, path string) int {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	// Fixed remote addr so c.IP() is stable across calls.
	req.RemoteAddr = "10.0.0.9:1234"
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	return resp.StatusCode
}

func TestRateLimit_UnderLimitPasses(t *testing.T) {
	app := newRateLimitApp("/rl-under", 3, time.Minute)
	require.Equal(t, 200, doGet(t, app, "/rl-under"))
	require.Equal(t, 200, doGet(t, app, "/rl-under"))
	require.Equal(t, 200, doGet(t, app, "/rl-under"))
}

func TestRateLimit_OverLimitReturns429(t *testing.T) {
	app := newRateLimitApp("/rl-over", 2, time.Minute)
	require.Equal(t, 200, doGet(t, app, "/rl-over"))
	require.Equal(t, 200, doGet(t, app, "/rl-over"))
	require.Equal(t, 429, doGet(t, app, "/rl-over"))
}

func TestRateLimit_ResetsAfterWindow(t *testing.T) {
	app := newRateLimitApp("/rl-reset", 1, 50*time.Millisecond)
	require.Equal(t, 200, doGet(t, app, "/rl-reset"))
	require.Equal(t, 429, doGet(t, app, "/rl-reset"))
	time.Sleep(80 * time.Millisecond)
	require.Equal(t, 200, doGet(t, app, "/rl-reset"))
}
