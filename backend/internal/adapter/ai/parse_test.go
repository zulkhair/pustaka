package ai

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/domain"
)

func TestParseTransformOutputMarkdownPassthrough(t *testing.T) {
	out, err := parseTransformOutput("# Title\n\nbody", domain.FormatMarkdown)
	require.NoError(t, err)
	require.Equal(t, "# Title\n\nbody", out)
}

func TestParseTransformOutputJSONStripsFences(t *testing.T) {
	raw := "```json\n{\"name\": \"Acme\"}\n```"
	out, err := parseTransformOutput(raw, domain.FormatJSON)
	require.NoError(t, err)
	require.JSONEq(t, `{"name":"Acme"}`, out)
}

func TestParseTransformOutputJSONInvalid(t *testing.T) {
	_, err := parseTransformOutput("not json at all", domain.FormatJSON)
	require.ErrorIs(t, err, domain.ErrSchemaInvalid)
}

func TestBuildTransformPromptIncludesPromptAndText(t *testing.T) {
	tmpl := domain.Template{Prompt: "Extract fields", OutputFormat: domain.FormatJSON, JSONSchema: ptr(`{"type":"object"}`)}
	p := buildTransformPrompt(tmpl, "OCR TEXT HERE")
	require.Contains(t, p, "Extract fields")
	require.Contains(t, p, "OCR TEXT HERE")
	require.Contains(t, p, `{"type":"object"}`)
}

func ptr(s string) *string { return &s }
