package ai_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/adapter/ai"
	"github.com/zulkhair/pustaka/backend/internal/config"
	"github.com/zulkhair/pustaka/backend/internal/domain"
)

func TestOllamaTranscribeCallsGenerate(t *testing.T) {
	var gotPath string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		_, _ = w.Write([]byte(`{"response":"# transcribed"}`))
	}))
	defer srv.Close()

	c := ai.NewOllama(config.Config{OllamaHost: srv.URL, OCRModel: "glm-ocr"})
	out, err := c.Transcribe(context.Background(), []byte("img-bytes"))
	require.NoError(t, err)
	require.Equal(t, "# transcribed", out)
	require.Equal(t, "/api/generate", gotPath)
	require.Equal(t, "glm-ocr", body["model"])
	require.Equal(t, false, body["stream"])
}

func TestOllamaTransformCallsChatWithJSONFormat(t *testing.T) {
	var gotPath string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"{\"name\":\"Acme\"}"}}`))
	}))
	defer srv.Close()

	c := ai.NewOllama(config.Config{OllamaHost: srv.URL, TransformModel: "qwen2.5:14b-instruct"})
	out, err := c.Transform(context.Background(), "ocr text",
		domain.Template{OutputFormat: domain.FormatJSON, Prompt: "extract"})
	require.NoError(t, err)
	require.JSONEq(t, `{"name":"Acme"}`, out)
	require.Equal(t, "/api/chat", gotPath)
	require.Equal(t, "qwen2.5:14b-instruct", body["model"])
	require.Equal(t, "json", body["format"])
}

func TestOllamaTransformMarkdownNoFormat(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"# clean"}}`))
	}))
	defer srv.Close()

	c := ai.NewOllama(config.Config{OllamaHost: srv.URL, TransformModel: "qwen2.5:14b-instruct"})
	out, err := c.Transform(context.Background(), "ocr", domain.Template{OutputFormat: domain.FormatMarkdown})
	require.NoError(t, err)
	require.Equal(t, "# clean", out)
	_, hasFormat := body["format"]
	require.False(t, hasFormat, "markdown transform must not set format=json")
}

func TestOllamaErrorsOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := ai.NewOllama(config.Config{OllamaHost: srv.URL, OCRModel: "glm-ocr"})
	_, err := c.Transcribe(context.Background(), []byte("x"))
	require.Error(t, err)
}
