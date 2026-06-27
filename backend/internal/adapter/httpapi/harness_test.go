package httpapi_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/adapter/ai"
	"github.com/zulkhair/pustaka/backend/internal/adapter/blob"
	"github.com/zulkhair/pustaka/backend/internal/adapter/httpapi"
	"github.com/zulkhair/pustaka/backend/internal/adapter/mail"
	"github.com/zulkhair/pustaka/backend/internal/app/auth"
	"github.com/zulkhair/pustaka/backend/internal/app/document"
	"github.com/zulkhair/pustaka/backend/internal/app/ocr"
	"github.com/zulkhair/pustaka/backend/internal/app/template"
	"github.com/zulkhair/pustaka/backend/internal/app/transform"
	"github.com/zulkhair/pustaka/backend/internal/config"
	"github.com/zulkhair/pustaka/backend/internal/testsupport"
)

// newTestApp builds the FULL wired Fiber app via BuildApp (proxy config +
// recover/logger + Mount with RateLimit + RequireAuth). Reuses the testApp
// struct and helpers from handler_harness_test.go (same package).
func newTestApp(t *testing.T) *testApp {
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
	bs := blob.NewMemory()
	aimock := ai.NewMock()
	docSvc := document.New(st, bs)
	xfSvc := transform.New(st, aimock, docSvc)
	app := httpapi.BuildApp(httpapi.RouterDeps{
		Auth:      httpapi.NewAuthHandler(auth.New(st, mailer, cfg)),
		Pinger:    st.Pool(),
		JWTSecret: cfg.JWTSecret,
		Doc:       httpapi.NewDocHandler(docSvc),
		Page:      httpapi.NewPageHandler(docSvc, ocr.New(st, aimock, bs, docSvc), bs),
		Template:  httpapi.NewTemplateHandler(template.New(st)),
		Transform: httpapi.NewTransformHandler(xfSvc),
		Output:    httpapi.NewOutputHandler(xfSvc),
		Version:   httpapi.NewVersionHandler(cfg),
	})
	return &testApp{app: app, store: st, mailer: mailer, ai: aimock}
}

// TestRouter_FullApp_HealthAndMe exercises the fully wired app: open /health and
// the RequireAuth-protected /me (no token -> 401).
func TestRouter_FullApp_HealthAndMe(t *testing.T) {
	ta := newTestApp(t)

	resp := doJSON(t, ta, http.MethodGet, "/api/health", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	meResp := doJSON(t, ta, http.MethodGet, "/api/auth/me", nil)
	require.Equal(t, http.StatusUnauthorized, meResp.StatusCode)
}
