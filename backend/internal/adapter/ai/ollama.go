package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/zulkhair/pustaka/backend/internal/config"
	"github.com/zulkhair/pustaka/backend/internal/domain"
)

const ocrNumPredict = 4096

// Ollama implements domain.AIClient against an Ollama server (GLM-OCR for
// transcription via /api/generate, qwen2.5 for transform via /api/chat).
type Ollama struct {
	BaseURL        string
	OCRModel       string
	TransformModel string
	client         *http.Client
}

func NewOllama(cfg config.Config) *Ollama {
	return &Ollama{
		BaseURL:        cfg.OllamaHost,
		OCRModel:       cfg.OCRModel,
		TransformModel: cfg.TransformModel,
		client:         &http.Client{Timeout: 5 * time.Minute},
	}
}

var _ domain.AIClient = (*Ollama)(nil)

type generateReq struct {
	Model   string         `json:"model"`
	Prompt  string         `json:"prompt"`
	Images  []string       `json:"images,omitempty"`
	Stream  bool           `json:"stream"`
	Options map[string]any `json:"options,omitempty"`
}

type generateResp struct {
	Response string `json:"response"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatReq struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
	Format   string        `json:"format,omitempty"`
}

type chatResp struct {
	Message chatMessage `json:"message"`
}

func (o *Ollama) postJSON(ctx context.Context, path string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("ai: marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("ai: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.client.Do(req)
	if err != nil {
		return fmt.Errorf("ai: do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ai: ollama status %d: %s", resp.StatusCode, string(raw))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("ai: decode response: %w", err)
	}
	return nil
}

func (o *Ollama) Transcribe(ctx context.Context, imageBytes []byte) (string, error) {
	payload := generateReq{
		Model:   o.OCRModel,
		Prompt:  "Transcribe this page to clean Markdown. Output only the transcription.",
		Images:  []string{base64.StdEncoding.EncodeToString(imageBytes)},
		Stream:  false,
		Options: map[string]any{"num_predict": ocrNumPredict},
	}
	var out generateResp
	if err := o.postJSON(ctx, "/api/generate", payload, &out); err != nil {
		return "", err
	}
	return out.Response, nil
}

func (o *Ollama) Transform(ctx context.Context, ocrText string, tmpl domain.Template) (string, error) {
	req := chatReq{
		Model:    o.TransformModel,
		Messages: []chatMessage{{Role: "user", Content: buildTransformPrompt(tmpl, ocrText)}},
		Stream:   false,
	}
	if tmpl.OutputFormat == domain.FormatJSON {
		req.Format = "json"
	}
	var out chatResp
	if err := o.postJSON(ctx, "/api/chat", req, &out); err != nil {
		return "", err
	}
	return parseTransformOutput(out.Message.Content, tmpl.OutputFormat)
}
