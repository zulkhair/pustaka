package ai_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/adapter/ai"
	"github.com/zulkhair/pustaka/backend/internal/domain"
)

func TestMockDefaultsAndRecording(t *testing.T) {
	var c domain.AIClient = ai.NewMock()
	md, err := c.Transcribe(context.Background(), []byte("img"))
	require.NoError(t, err)
	require.NotEmpty(t, md)
	out, err := c.Transform(context.Background(), "ocr text", domain.Template{OutputFormat: domain.FormatMarkdown})
	require.NoError(t, err)
	require.NotEmpty(t, out)

	m := c.(*ai.Mock)
	require.Equal(t, 1, m.TranscribeCalls)
	require.Equal(t, 1, m.TransformCalls)
	require.Equal(t, "ocr text", m.LastTransformText)
}

func TestMockOverrides(t *testing.T) {
	m := ai.NewMock()
	m.TranscribeFn = func([]byte) (string, error) { return "CUSTOM", nil }
	got, err := m.Transcribe(context.Background(), nil)
	require.NoError(t, err)
	require.Equal(t, "CUSTOM", got)
}
