package mail_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/adapter/mail"
	"github.com/zulkhair/pustaka/backend/internal/domain"
)

func TestMockMailerRecordsSends(t *testing.T) {
	var m domain.Mailer = mail.NewMockMailer()
	require.NoError(t, m.SendVerificationCode(context.Background(), "a@example.com", "111111"))
	require.NoError(t, m.SendVerificationCode(context.Background(), "b@example.com", "222222"))

	mock := m.(*mail.MockMailer)
	require.Equal(t, "b@example.com", mock.LastEmail)
	require.Equal(t, "222222", mock.LastCode)
	require.Len(t, mock.Sends, 2)
	require.Equal(t, "a@example.com", mock.Sends[0].Email)
	require.Equal(t, "111111", mock.Sends[0].Code)

	code, ok := mock.CodeFor("a@example.com")
	require.True(t, ok)
	require.Equal(t, "111111", code)
	_, ok = mock.CodeFor("missing@example.com")
	require.False(t, ok)
}
