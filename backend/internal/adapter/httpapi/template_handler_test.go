package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/adapter/httpapi"
	"github.com/zulkhair/pustaka/backend/internal/domain"
)

func TestTemplateList(t *testing.T) {
	ta := newDocTestApp(t)
	uid := seedDocUser(t, ta)
	h := httpapi.NewTemplateHandler(ta.tmplSvc)
	ta.app.Group("/api", authMW(uid)).Get("/templates", h.List)

	code, env := getJSON(t, ta, "/api/templates")
	require.Equal(t, http.StatusOK, code)
	var list []struct {
		ID           string `json:"id"`
		Scope        string `json:"scope"`
		OutputFormat string `json:"outputFormat"`
	}
	decodeData(t, env, &list)
	require.GreaterOrEqual(t, len(list), 2)

	byID := map[string]string{}
	for _, tm := range list {
		byID[tm.ID] = tm.Scope + "/" + tm.OutputFormat
	}
	require.Equal(t, domain.ScopeDocument+"/"+domain.FormatMarkdown, byID[domain.TemplateIDCleanMarkdown])
	require.Equal(t, domain.ScopePage+"/"+domain.FormatJSON, byID[domain.TemplateIDStructuredJSON])
}
