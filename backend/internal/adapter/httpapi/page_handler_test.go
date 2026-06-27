package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/adapter/httpapi"
)

func mountPageRoutes(ta *docTestApp, uid string) {
	dh := httpapi.NewDocHandler(ta.docSvc)
	ph := httpapi.NewPageHandler(ta.docSvc, ta.ocrSvc, ta.blob)
	g := ta.app.Group("/api", authMW(uid))
	g.Post("/documents", dh.Create)
	g.Post("/documents/:id/pages", ph.AddPage)
	g.Get("/documents/:id/pages/:n/image", ph.Image)
	g.Get("/documents/:id/pages/:n/thumb", ph.Thumb)
	g.Post("/documents/:id/pages/:n/ocr", ph.RerunOCR)
}

func TestAddPagePhotoThenServeImage(t *testing.T) {
	ta := newDocTestApp(t)
	uid := seedDocUser(t, ta)
	mountPageRoutes(ta, uid)
	ta.ai.TranscribeFn = func([]byte) (string, error) { return "# ocr text", nil }

	code, env := postJSON(t, ta, "/api/documents", map[string]string{"title": "P", "mode": "photo"})
	require.Equal(t, http.StatusOK, code)
	var created struct {
		ID string `json:"id"`
	}
	decodeData(t, env, &created)

	resp := doMultipart(t, ta, "/api/documents/"+created.ID+"/pages", uid, []byte("img-bytes"))
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// image is served back
	imgResp, err := ta.app.Test(httptest.NewRequest(http.MethodGet, "/api/documents/"+created.ID+"/pages/1/image", nil), -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, imgResp.StatusCode)
	require.Equal(t, "image/jpeg", imgResp.Header.Get("Content-Type"))
}

func TestAddPageTextModeNoImage(t *testing.T) {
	ta := newDocTestApp(t)
	uid := seedDocUser(t, ta)
	mountPageRoutes(ta, uid)
	ta.ai.TranscribeFn = func([]byte) (string, error) { return "transcribed", nil }

	code, env := postJSON(t, ta, "/api/documents", map[string]string{"title": "T", "mode": "text"})
	require.Equal(t, http.StatusOK, code)
	var created struct {
		ID string `json:"id"`
	}
	decodeData(t, env, &created)

	resp := doMultipart(t, ta, "/api/documents/"+created.ID+"/pages", uid, []byte("img"))
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// image NOT available in text mode -> 404
	imgResp, err := ta.app.Test(httptest.NewRequest(http.MethodGet, "/api/documents/"+created.ID+"/pages/1/image", nil), -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, imgResp.StatusCode)
}

func TestRerunOCR(t *testing.T) {
	ta := newDocTestApp(t)
	uid := seedDocUser(t, ta)
	mountPageRoutes(ta, uid)
	ta.ai.TranscribeFn = func([]byte) (string, error) { return "v1", nil }

	code, env := postJSON(t, ta, "/api/documents", map[string]string{"title": "P", "mode": "photo"})
	require.Equal(t, http.StatusOK, code)
	var created struct {
		ID string `json:"id"`
	}
	decodeData(t, env, &created)
	_ = doMultipart(t, ta, "/api/documents/"+created.ID+"/pages", uid, []byte("img"))

	ta.ai.TranscribeFn = func([]byte) (string, error) { return "v2", nil }
	code, env = postJSON(t, ta, "/api/documents/"+created.ID+"/pages/1/ocr", nil)
	require.Equal(t, http.StatusOK, code)
	var out struct {
		OCRText string `json:"ocrText"`
	}
	decodeData(t, env, &out)
	require.Equal(t, "v2", out.OCRText)
}
