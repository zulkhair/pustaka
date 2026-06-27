package httpapi_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/adapter/httpapi"
	"github.com/zulkhair/pustaka/backend/internal/app/auth"
	"github.com/zulkhair/pustaka/backend/internal/config"
	"github.com/zulkhair/pustaka/backend/internal/domain"
	"github.com/zulkhair/pustaka/backend/internal/pkg/jwt"
)

const routerSecret = "router-secret-abcdefgh"

// fakePinger lets us flip DB up/down without Postgres.
type fakePinger struct{ err error }

func (f fakePinger) Ping(ctx context.Context) error { return f.err }

// meStore is a minimal domain.Store; only GetUserByID is used by /auth/me.
type meStore struct {
	domain.Store // embed nil interface; unused methods will panic if called (they aren't)
	user         domain.User
	err          error
}

func (m meStore) GetUserByID(ctx context.Context, id string) (domain.User, error) {
	return m.user, m.err
}

func buildRouterApp(t *testing.T, p httpapi.Pinger, store domain.Store) *fiber.App {
	t.Helper()
	cfg := config.Config{JWTSecret: routerSecret, AccessTTL: 15 * time.Minute}
	svc := auth.New(store, nil, cfg)
	app := fiber.New()
	httpapi.Mount(app, httpapi.RouterDeps{
		Auth:      httpapi.NewAuthHandler(svc),
		Pinger:    p,
		JWTSecret: routerSecret,
	})
	return app
}

func TestHealth_ReportsDBUp(t *testing.T) {
	app := buildRouterApp(t, fakePinger{err: nil}, meStore{})
	req := httptest.NewRequest("GET", "/api/health", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var env struct {
		Status int `json:"status"`
		Data   struct {
			DB string `json:"db"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &env))
	require.Equal(t, 0, env.Status)
	require.Equal(t, "up", env.Data.DB)
}

func TestHealth_ReportsDBDownStill200(t *testing.T) {
	app := buildRouterApp(t, fakePinger{err: context.DeadlineExceeded}, meStore{})
	req := httptest.NewRequest("GET", "/api/health", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	require.Contains(t, string(body), `"db":"down"`)
}

func TestAuthMe_NoToken_401(t *testing.T) {
	app := buildRouterApp(t, fakePinger{}, meStore{})
	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 401, resp.StatusCode)
}

func TestAuthMe_ValidToken_200(t *testing.T) {
	store := meStore{user: domain.User{
		ID: "u-9", Username: "alice", Email: "alice@example.com",
		Role: domain.RoleUser, EmailVerified: true,
	}}
	app := buildRouterApp(t, fakePinger{}, store)

	token, err := jwt.GenerateAccess("u-9", domain.RoleUser, routerSecret, 15*time.Minute)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	require.Contains(t, string(body), "alice")
	require.Contains(t, string(body), "alice@example.com")
}
