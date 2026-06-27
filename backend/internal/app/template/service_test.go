package template_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/app/template"
	"github.com/zulkhair/pustaka/backend/internal/domain"
	"github.com/zulkhair/pustaka/backend/internal/testsupport"
)

func TestListReturnsBuiltins(t *testing.T) {
	st, cleanup := testsupport.NewTestStore(t)
	defer cleanup()
	svc := template.New(st)

	tmpls, err := svc.List(context.Background())
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(tmpls), 2)

	ids := map[string]bool{}
	for _, tm := range tmpls {
		ids[tm.ID] = true
		require.True(t, tm.IsBuiltin)
	}
	require.True(t, ids[domain.TemplateIDCleanMarkdown])
	require.True(t, ids[domain.TemplateIDStructuredJSON])
}
