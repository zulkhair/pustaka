package httpapi

import (
	"errors"
	"net/http"

	"github.com/gofiber/fiber/v2"

	"github.com/zulkhair/pustaka/backend/internal/domain"
)

// mapDocError maps document-layer domain errors to HTTP responses via Fail.
func mapDocError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return Fail(c, http.StatusNotFound, "not found")
	case errors.Is(err, domain.ErrValidation),
		errors.Is(err, domain.ErrUnsupportedFormat),
		errors.Is(err, domain.ErrSchemaInvalid):
		return Fail(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrForbidden):
		return Fail(c, http.StatusForbidden, "forbidden")
	default:
		return Fail(c, http.StatusInternalServerError, "internal error")
	}
}

// MapDocErrorForTest exposes mapDocError to the external test package.
func MapDocErrorForTest(c *fiber.Ctx, err error) error { return mapDocError(c, err) }

// localUserID reads the authenticated principal id set by RequireAuth.
func localUserID(c *fiber.Ctx) (string, bool) {
	id, ok := c.Locals("userID").(string)
	if !ok || id == "" {
		return "", false
	}
	return id, true
}
