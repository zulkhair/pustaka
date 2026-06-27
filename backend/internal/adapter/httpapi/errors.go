package httpapi

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/zulkhair/pustaka/backend/internal/domain"
)

// mapAuthError translates a domain sentinel into an HTTP status + generic message.
// Defined ONCE; every handler calls it. ErrResendCooldown is intentionally absent —
// it is internal-only and swallowed by the resend handler into a generic 200.
func mapAuthError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, domain.ErrConflict):
		return Fail(c, fiber.StatusConflict, "conflict")
	case errors.Is(err, domain.ErrInvalidCredentials),
		errors.Is(err, domain.ErrEmailNotVerified),
		errors.Is(err, domain.ErrUnauthorized):
		return Fail(c, fiber.StatusUnauthorized, "unauthorized")
	case errors.Is(err, domain.ErrForbidden):
		return Fail(c, fiber.StatusForbidden, "forbidden")
	case errors.Is(err, domain.ErrNotFound):
		return Fail(c, fiber.StatusNotFound, "not found")
	case errors.Is(err, domain.ErrInvalidCode),
		errors.Is(err, domain.ErrCodeExpired),
		errors.Is(err, domain.ErrValidation):
		return Fail(c, fiber.StatusBadRequest, "bad request")
	case errors.Is(err, domain.ErrTooManyAttempts):
		return Fail(c, fiber.StatusTooManyRequests, "too many attempts")
	default:
		return Fail(c, fiber.StatusInternalServerError, "internal error")
	}
}
