package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/adapter/httpapi"
	"github.com/zulkhair/pustaka/backend/internal/config"
)

func TestVersionGetPutDownload(t *testing.T) {
	dir := t.TempDir()
	apk := filepath.Join(dir, "pustaka.apk")
	require.NoError(t, os.WriteFile(apk, []byte("APKDATA"), 0o644))

	h := httpapi.NewVersionHandler(config.Config{AppVersion: "1.0.0", APKPath: apk})
	app := fiber.New()
	app.Get("/api/version", h.Get)
	app.Put("/api/version", h.Put)
	app.Get("/api/version/download", h.Download)

	// GET
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/version", nil), -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// PUT updates version
	putReq := httptest.NewRequest(http.MethodPut, "/api/version", strings.NewReader(`{"version":"2.0.0"}`))
	putReq.Header.Set("Content-Type", "application/json")
	putResp, err := app.Test(putReq, -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, putResp.StatusCode)

	// download streams the apk bytes
	dlResp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/version/download", nil), -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, dlResp.StatusCode)
}

func TestVersionDownloadMissing404(t *testing.T) {
	h := httpapi.NewVersionHandler(config.Config{AppVersion: "1.0.0", APKPath: ""})
	app := fiber.New()
	app.Get("/api/version/download", h.Download)
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/version/download", nil), -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}
