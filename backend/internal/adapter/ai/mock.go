package ai

import (
	"context"

	"github.com/zulkhair/pustaka/backend/internal/domain"
)

// Mock is a deterministic domain.AIClient for tests (no GPU/Ollama).
type Mock struct {
	TranscribeFn      func(imageBytes []byte) (string, error)
	TransformFn       func(ocrText string, tmpl domain.Template) (string, error)
	TranscribeCalls   int
	TransformCalls    int
	LastTransformText string
}

func NewMock() *Mock { return &Mock{} }

var _ domain.AIClient = (*Mock)(nil)

func (m *Mock) Transcribe(_ context.Context, imageBytes []byte) (string, error) {
	m.TranscribeCalls++
	if m.TranscribeFn != nil {
		return m.TranscribeFn(imageBytes)
	}
	return "# mock ocr text", nil
}

func (m *Mock) Transform(_ context.Context, ocrText string, tmpl domain.Template) (string, error) {
	m.TransformCalls++
	m.LastTransformText = ocrText
	if m.TransformFn != nil {
		return m.TransformFn(ocrText, tmpl)
	}
	if tmpl.OutputFormat == domain.FormatJSON {
		return `{"mock":"output"}`, nil
	}
	return "# mock transformed", nil
}
