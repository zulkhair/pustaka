package ai

import (
	"encoding/json"
	"strings"

	"github.com/zulkhair/pustaka/backend/internal/domain"
)

// stripCodeFence removes a leading ```lang and trailing ``` if present.
func stripCodeFence(s string) string {
	t := strings.TrimSpace(s)
	if !strings.HasPrefix(t, "```") {
		return t
	}
	t = strings.TrimPrefix(t, "```")
	if i := strings.IndexByte(t, '\n'); i >= 0 {
		t = t[i+1:] // drop the language tag line
	}
	t = strings.TrimSuffix(strings.TrimSpace(t), "```")
	return strings.TrimSpace(t)
}

// parseTransformOutput cleans raw model output per the template's output format.
// For JSON it strips code fences and validates the result is parseable JSON.
func parseTransformOutput(raw, format string) (string, error) {
	cleaned := strings.TrimSpace(raw)
	if format == domain.FormatJSON {
		cleaned = stripCodeFence(cleaned)
		var probe any
		if err := json.Unmarshal([]byte(cleaned), &probe); err != nil {
			return "", domain.ErrSchemaInvalid
		}
		canon, err := json.Marshal(probe)
		if err != nil {
			return "", domain.ErrSchemaInvalid
		}
		return string(canon), nil
	}
	return cleaned, nil
}
