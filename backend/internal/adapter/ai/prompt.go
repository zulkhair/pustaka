package ai

import (
	"fmt"
	"strings"

	"github.com/zulkhair/pustaka/backend/internal/domain"
)

// buildTransformPrompt assembles the user message for the transform model from
// the template prompt, an optional JSON schema, and the OCR text.
func buildTransformPrompt(tmpl domain.Template, ocrText string) string {
	var b strings.Builder
	b.WriteString(tmpl.Prompt)
	b.WriteString("\n\n")
	if tmpl.OutputFormat == domain.FormatJSON && tmpl.JSONSchema != nil && *tmpl.JSONSchema != "" {
		fmt.Fprintf(&b, "Return ONLY JSON matching this schema:\n%s\n\n", *tmpl.JSONSchema)
	}
	b.WriteString("--- BEGIN OCR TEXT ---\n")
	b.WriteString(ocrText)
	b.WriteString("\n--- END OCR TEXT ---")
	return b.String()
}
