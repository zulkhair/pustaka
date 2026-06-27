package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/adapter/ai"
	"github.com/zulkhair/pustaka/backend/internal/adapter/blob"
	"github.com/zulkhair/pustaka/backend/internal/adapter/store"
	"github.com/zulkhair/pustaka/backend/internal/app/document"
	"github.com/zulkhair/pustaka/backend/internal/app/ocr"
	"github.com/zulkhair/pustaka/backend/internal/app/template"
	"github.com/zulkhair/pustaka/backend/internal/app/transform"
	"github.com/zulkhair/pustaka/backend/internal/domain"
	"github.com/zulkhair/pustaka/backend/internal/testsupport"
)

type docTestApp struct {
	app     *fiber.App
	store   *store.Store
	blob    *blob.Memory
	ai      *ai.Mock
	docSvc  *document.Service
	ocrSvc  *ocr.Service
	tmplSvc *template.Service
	xfSvc   *transform.Service
}

func newDocTestApp(t *testing.T) *docTestApp {
	t.Helper()
	st, cleanup := testsupport.NewTestStore(t)
	t.Cleanup(cleanup)

	bs := blob.NewMemory()
	aimock := ai.NewMock()
	docSvc := document.New(st, bs)

	return &docTestApp{
		app:     fiber.New(),
		store:   st,
		blob:    bs,
		ai:      aimock,
		docSvc:  docSvc,
		ocrSvc:  ocr.New(st, aimock, bs, docSvc),
		tmplSvc: template.New(st),
		xfSvc:   transform.New(st, aimock, docSvc),
	}
}

// authMW stands in for RequireAuth: it sets the principal id in c.Locals.
func authMW(userID string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals("userID", userID)
		return c.Next()
	}
}

func seedDocUser(t *testing.T, ta *docTestApp) string {
	t.Helper()
	u, err := ta.store.CreateUser(context.Background(), domain.CreateUserParams{
		ID: uuid.NewString(), Username: "u-" + uuid.NewString()[:8],
		Email: uuid.NewString()[:8] + "@e.com", PasswordHash: "x", Role: domain.RoleUser,
	})
	require.NoError(t, err)
	return u.ID
}

func doMultipart(t *testing.T, ta *docTestApp, path, _ string, fileBytes []byte) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", "page.jpg")
	require.NoError(t, err)
	_, err = fw.Write(fileBytes)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := ta.app.Test(req, -1)
	require.NoError(t, err)
	return resp
}

type docEnv struct {
	Status  int             `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func postJSON(t *testing.T, ta *docTestApp, path string, body any) (int, docEnv) {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	return doDoc(t, ta, req)
}

func getJSON(t *testing.T, ta *docTestApp, path string) (int, docEnv) {
	t.Helper()
	return doDoc(t, ta, httptest.NewRequest(http.MethodGet, path, nil))
}

func doDoc(t *testing.T, ta *docTestApp, req *http.Request) (int, docEnv) {
	t.Helper()
	resp, err := ta.app.Test(req, -1)
	require.NoError(t, err)
	b, _ := io.ReadAll(resp.Body)
	var e docEnv
	if len(b) > 0 {
		require.NoError(t, json.Unmarshal(b, &e), "body: %s", string(b))
	}
	return resp.StatusCode, e
}

func decodeData(t *testing.T, e docEnv, out any) {
	t.Helper()
	require.NoError(t, json.Unmarshal(e.Data, out))
}
