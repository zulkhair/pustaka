package httpapi

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/zulkhair/pustaka/backend/internal/adapter/httpapi/middleware"
)

// RouterDeps holds the dependencies needed to wire the HTTP routes.
type RouterDeps struct {
	Auth      *AuthHandler
	Pinger    Pinger
	JWTSecret string
}

const (
	authRateMax    = 10
	authRateWindow = time.Minute
)

// Mount registers all /api routes on the given Fiber app.
func Mount(app *fiber.App, deps RouterDeps) {
	api := app.Group("/api")

	rl := func() fiber.Handler { return middleware.RateLimit(authRateMax, authRateWindow) }

	authGrp := api.Group("/auth")
	authGrp.Post("/register", rl(), deps.Auth.Register)
	authGrp.Post("/verify-email", rl(), deps.Auth.VerifyEmail)
	authGrp.Post("/resend-verification", rl(), deps.Auth.ResendVerification)
	authGrp.Post("/login", rl(), deps.Auth.Login)
	authGrp.Post("/refresh", rl(), deps.Auth.Refresh)
	authGrp.Post("/logout", rl(), deps.Auth.Logout)
	authGrp.Get("/me", middleware.RequireAuth(deps.JWTSecret), deps.Auth.Me)

	api.Get("/health", HealthHandler(deps.Pinger))
}
