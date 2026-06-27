package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/zulkhair/pustaka/backend/internal/domain"
	"github.com/zulkhair/pustaka/backend/internal/pkg/jwt"
)

// fail writes the standard {status, message, data} error envelope. Defined
// locally so this package does not import httpapi (which imports middleware —
// importing it back would create an import cycle).
func fail(c *fiber.Ctx, code int, msg string) error {
	return c.Status(code).JSON(fiber.Map{"status": 1, "message": msg, "data": nil})
}

// RequireAuth validates a Bearer access JWT and stores the principal in c.Locals.
func RequireAuth(secret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authz := c.Get("Authorization")
		const prefix = "Bearer "
		if len(authz) <= len(prefix) || !strings.EqualFold(authz[:len(prefix)], prefix) {
			return fail(c, fiber.StatusUnauthorized, "missing or malformed authorization header")
		}
		token := strings.TrimSpace(authz[len(prefix):])
		claims, err := jwt.ParseAccess(token, secret)
		if err != nil {
			return fail(c, fiber.StatusUnauthorized, "invalid or expired token")
		}
		c.Locals("userID", claims.UserID)
		c.Locals("role", claims.Role)
		return c.Next()
	}
}

// RequireAdmin allows the request only when the principal's role is admin.
// Must run after RequireAuth so c.Locals("role") is populated.
func RequireAdmin() fiber.Handler {
	return func(c *fiber.Ctx) error {
		role, _ := c.Locals("role").(string)
		if role != domain.RoleAdmin {
			return fail(c, fiber.StatusForbidden, "admin access required")
		}
		return c.Next()
	}
}
