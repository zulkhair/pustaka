package mail

import (
	"context"

	"github.com/zulkhair/pustaka/backend/internal/domain"
)

type MockSend struct {
	Email string
	Code  string
}

type MockMailer struct {
	LastEmail string
	LastCode  string
	Sends     []MockSend
}

func NewMockMailer() *MockMailer {
	return &MockMailer{}
}

var _ domain.Mailer = (*MockMailer)(nil)

func (m *MockMailer) SendVerificationCode(_ context.Context, toEmail, code string) error {
	m.LastEmail = toEmail
	m.LastCode = code
	m.Sends = append(m.Sends, MockSend{Email: toEmail, Code: code})
	return nil
}

// CodeFor returns the most recent code captured for the given email, if any.
func (m *MockMailer) CodeFor(email string) (string, bool) {
	for i := len(m.Sends) - 1; i >= 0; i-- {
		if m.Sends[i].Email == email {
			return m.Sends[i].Code, true
		}
	}
	return "", false
}
