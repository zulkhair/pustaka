package domain_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/domain"
)

func TestDocumentConstants(t *testing.T) {
	require.Equal(t, "photo", domain.ModePhoto)
	require.Equal(t, "text", domain.ModeText)
	require.Equal(t, "pending", domain.StatusPending)
	require.Equal(t, "processing", domain.StatusProcessing)
	require.Equal(t, "done", domain.StatusDone)
	require.Equal(t, "failed", domain.StatusFailed)
	require.Equal(t, "page", domain.ScopePage)
	require.Equal(t, "document", domain.ScopeDocument)
	require.Equal(t, "markdown", domain.FormatMarkdown)
	require.Equal(t, "json", domain.FormatJSON)
	require.Equal(t, "csv", domain.FormatCSV)
	require.Equal(t, "text", domain.FormatText)
	require.Equal(t, "00000000-0000-0000-0000-000000000001", domain.TemplateIDCleanMarkdown)
	require.Equal(t, "00000000-0000-0000-0000-000000000002", domain.TemplateIDStructuredJSON)
}

func TestDocumentEntitiesConstruct(t *testing.T) {
	img := "u/d/1.jpg"
	d := domain.Document{ID: "d1", UserID: "u1", Title: "T", Mode: domain.ModePhoto, Status: domain.StatusPending}
	p := domain.Page{ID: "p1", DocumentID: "d1", PageNumber: 1, ImagePath: &img}
	o := domain.OCRResult{ID: "o1", PageID: "p1", Model: "glm-ocr", Text: "# hi", Status: domain.StatusDone}
	tmpl := domain.Template{ID: "t1", Name: "n", Scope: domain.ScopeDocument, OutputFormat: domain.FormatMarkdown, IsBuiltin: true}
	out := domain.Output{ID: "out1", UserID: "u1", DocumentID: "d1", TemplateID: "t1", Content: "x", Status: domain.StatusDone}
	require.Equal(t, "u/d/1.jpg", *p.ImagePath)
	require.Equal(t, domain.ModePhoto, d.Mode)
	require.Equal(t, "glm-ocr", o.Model)
	require.True(t, tmpl.IsBuiltin)
	require.Equal(t, "d1", out.DocumentID)
}
