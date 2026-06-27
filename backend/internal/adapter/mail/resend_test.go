package mail_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/adapter/mail"
	"github.com/zulkhair/pustaka/backend/internal/config"
)

func TestResendMailerSendsCorrectRequest(t *testing.T) {
	var gotAuth, gotCT, gotMethod, gotPath string
	var body map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotCT = r.Header.Get("Content-Type")
		raw, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(raw, &body))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"abc"}`))
	}))
	defer srv.Close()

	m := mail.NewResendMailer(config.Config{ResendAPIKey: "re_test_key", MailFrom: "Pustaka <no-reply@pustaka.test>"})
	m.BaseURL = srv.URL

	require.NoError(t, m.SendVerificationCode(context.Background(), "user@example.com", "123456"))

	require.Equal(t, http.MethodPost, gotMethod)
	require.Equal(t, "/emails", gotPath)
	require.Equal(t, "Bearer re_test_key", gotAuth)
	require.Equal(t, "application/json", gotCT)
	require.Equal(t, "Pustaka <no-reply@pustaka.test>", body["from"])
	require.Equal(t, "user@example.com", body["to"])
	require.Contains(t, body["text"], "123456")
	require.Contains(t, body["html"], "123456")
	require.NotEmpty(t, body["subject"])
}

func TestResendMailerErrorsOnNon2xx(t *testing.T) {
	for _, code := range []int{http.StatusUnprocessableEntity, http.StatusInternalServerError} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
			_, _ = w.Write([]byte(`{"message":"boom"}`))
		}))
		m := mail.NewResendMailer(config.Config{ResendAPIKey: "re_test_key", MailFrom: "x@y.test"})
		m.BaseURL = srv.URL
		err := m.SendVerificationCode(context.Background(), "user@example.com", "123456")
		require.Error(t, err)
		srv.Close()
	}
}
