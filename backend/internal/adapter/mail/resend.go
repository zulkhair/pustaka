package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/zulkhair/pustaka/backend/internal/config"
	"github.com/zulkhair/pustaka/backend/internal/domain"
)

const defaultResendBaseURL = "https://api.resend.com"

type ResendMailer struct {
	APIKey  string
	From    string
	BaseURL string
	client  *http.Client
}

func NewResendMailer(cfg config.Config) *ResendMailer {
	return &ResendMailer{
		APIKey:  cfg.ResendAPIKey,
		From:    cfg.MailFrom,
		BaseURL: defaultResendBaseURL,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

var _ domain.Mailer = (*ResendMailer)(nil)

type resendEmailReq struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Subject string `json:"subject"`
	HTML    string `json:"html"`
	Text    string `json:"text"`
}

func (m *ResendMailer) SendVerificationCode(ctx context.Context, toEmail, code string) error {
	payload := resendEmailReq{
		From:    m.From,
		To:      toEmail,
		Subject: "Your Pustaka verification code",
		HTML:    fmt.Sprintf("<p>Your Pustaka verification code is <strong>%s</strong>. It expires in 15 minutes.</p>", code),
		Text:    fmt.Sprintf("Your Pustaka verification code is %s. It expires in 15 minutes.", code),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal resend payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.BaseURL+"/emails", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build resend request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+m.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("send resend request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("resend returned status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
