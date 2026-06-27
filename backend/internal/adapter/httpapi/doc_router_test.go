package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/adapter/ai"
	"github.com/zulkhair/pustaka/backend/internal/adapter/blob"
	"github.com/zulkhair/pustaka/backend/internal/adapter/httpapi"
	"github.com/zulkhair/pustaka/backend/internal/app/document"
	"github.com/zulkhair/pustaka/backend/internal/app/ocr"
	"github.com/zulkhair/pustaka/backend/internal/app/template"
	"github.com/zulkhair/pustaka/backend/internal/app/transform"
	"github.com/zulkhair/pustaka/backend/internal/config"
	"github.com/zulkhair/pustaka/backend/internal/domain"
	pjwt "github.com/zulkhair/pustaka/backend/internal/pkg/jwt"
	"github.com/zulkhair/pustaka/backend/internal/testsupport"
)

func configForRouter() config.Config { return config.Config{AppVersion: "0.0.0"} }

func TestMountDocumentRoutes(t *testing.T) {
	st, cleanup := testsupport.NewTestStore(t)
	defer cleanup()
	bs := blob.NewMemory()
	aimock := ai.NewMock()

	const secret = "router-secret"
	app := httpapi.BuildApp(httpapi.RouterDeps{
		JWTSecret: secret,
		Pinger:    st.Pool(),
		Doc:       httpapi.NewDocHandler(document.New(st, bs)),
		Page:      httpapi.NewPageHandler(document.New(st, bs), ocr.New(st, aimock, bs), bs),
		Template:  httpapi.NewTemplateHandler(template.New(st)),
		Transform: httpapi.NewTransformHandler(transform.New(st, aimock)),
		Output:    httpapi.NewOutputHandler(transform.New(st, aimock)),
		Version:   httpapi.NewVersionHandler(configForRouter()),
	})

	// seed a verified user and mint a real access token
	u, err := st.CreateUser(context.Background(), domain.CreateUserParams{
		ID: uuid.NewString(), Username: "router", Email: "router@e.com", PasswordHash: "x", Role: domain.RoleUser,
	})
	require.NoError(t, err)
	token, err := pjwt.GenerateAccess(u.ID, domain.RoleUser, secret, time.Minute)
	require.NoError(t, err)

	// no token -> 401
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/documents", nil), -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	// with token -> 200
	req := httptest.NewRequest(http.MethodGet, "/api/documents", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// /api/version is open (no token) -> 200
	vResp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/version", nil), -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, vResp.StatusCode)

	// /api/templates behind RequireAuth -> 200 with token
	tReq := httptest.NewRequest(http.MethodGet, "/api/templates", nil)
	tReq.Header.Set("Authorization", "Bearer "+token)
	tResp, err := app.Test(tReq, -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, tResp.StatusCode)
}
