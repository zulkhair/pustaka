package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/adapter/httpapi"
	"github.com/zulkhair/pustaka/backend/internal/domain"
)

// mountDocRoutes wires the document routes onto the harness app behind authMW(uid).
func mountDocRoutes(ta *docTestApp, uid string) {
	h := httpapi.NewDocHandler(ta.docSvc)
	g := ta.app.Group("/api", authMW(uid))
	g.Post("/documents", h.Create)
	g.Get("/documents", h.List)
	g.Get("/documents/:id", h.Get)
}

func TestDocCreateAndGet(t *testing.T) {
	ta := newDocTestApp(t)
	uid := seedDocUser(t, ta)
	mountDocRoutes(ta, uid)

	code, env := postJSON(t, ta, "/api/documents", map[string]string{"title": "Doc A", "mode": "photo"})
	require.Equal(t, http.StatusOK, code)
	var created struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Mode  string `json:"mode"`
	}
	decodeData(t, env, &created)
	require.NotEmpty(t, created.ID)
	require.Equal(t, "Doc A", created.Title)
	require.Equal(t, domain.ModePhoto, created.Mode)

	code, env = getJSON(t, ta, "/api/documents/"+created.ID)
	require.Equal(t, http.StatusOK, code)

	code, env = getJSON(t, ta, "/api/documents")
	require.Equal(t, http.StatusOK, code)
	var list struct {
		Owned  []map[string]any `json:"owned"`
		Shared []map[string]any `json:"shared"`
	}
	decodeData(t, env, &list)
	require.Len(t, list.Owned, 1)
	require.Empty(t, list.Shared)
}

func TestDocCreateValidation(t *testing.T) {
	ta := newDocTestApp(t)
	uid := seedDocUser(t, ta)
	mountDocRoutes(ta, uid)

	code, _ := postJSON(t, ta, "/api/documents", map[string]string{"title": "", "mode": "photo"})
	require.Equal(t, http.StatusBadRequest, code)
	code, _ = postJSON(t, ta, "/api/documents", map[string]string{"title": "X", "mode": "bogus"})
	require.Equal(t, http.StatusBadRequest, code)
}

func TestDocGetForeignReturns404(t *testing.T) {
	ta := newDocTestApp(t)
	owner := seedDocUser(t, ta)
	mountDocRoutes(ta, owner)
	code, env := postJSON(t, ta, "/api/documents", map[string]string{"title": "Owned", "mode": "text"})
	require.Equal(t, http.StatusOK, code)
	var created struct {
		ID string `json:"id"`
	}
	decodeData(t, env, &created)

	// rebuild app as a different user
	ta2 := newDocTestApp(t)
	other := seedDocUser(t, ta2)
	mountDocRoutes(ta2, other)
	code, _ = getJSON(t, ta2, "/api/documents/"+created.ID)
	require.Equal(t, http.StatusNotFound, code)
}
