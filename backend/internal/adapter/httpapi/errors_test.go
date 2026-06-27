package httpapi

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/domain"
)

func TestMapAuthError(t *testing.T) {
	cases := []struct {
		err  error
		code int
	}{
		{domain.ErrConflict, 409},
		{domain.ErrInvalidCredentials, 401},
		{domain.ErrEmailNotVerified, 401},
		{domain.ErrUnauthorized, 401},
		{domain.ErrForbidden, 403},
		{domain.ErrNotFound, 404},
		{domain.ErrInvalidCode, 400},
		{domain.ErrCodeExpired, 400},
		{domain.ErrValidation, 400},
		{domain.ErrTooManyAttempts, 429},
		{errSentinelOther, 500},
	}
	for _, tc := range cases {
		app := fiber.New()
		app.Get("/x", func(c *fiber.Ctx) error { return mapAuthError(c, tc.err) })
		resp, err := app.Test(httptest.NewRequest("GET", "/x", nil), -1)
		require.NoError(t, err)
		require.Equal(t, tc.code, resp.StatusCode, "err=%v", tc.err)
	}
}

var errSentinelOther = fiber.NewError(0, "other") // any non-mapped error -> 500
