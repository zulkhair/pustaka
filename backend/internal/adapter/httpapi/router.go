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

	Doc       *DocHandler
	Page      *PageHandler
	Template  *TemplateHandler
	Transform *TransformHandler
	Output    *OutputHandler
	Version   *VersionHandler
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

	// Mobile OTA: GET is open; PUT is admin-only.
	api.Get("/version", deps.Version.Get)
	api.Get("/version/download", deps.Version.Download)
	api.Put("/version", middleware.RequireAuth(deps.JWTSecret), middleware.RequireAdmin(), deps.Version.Put)

	auth := middleware.RequireAuth(deps.JWTSecret)

	docs := api.Group("/documents", auth)
	docs.Post("/", deps.Doc.Create)
	docs.Get("/", deps.Doc.List)
	docs.Get("/:id", deps.Doc.Get)
	docs.Patch("/:id", deps.Doc.Rename)
	docs.Patch("/:id/thumbnail", deps.Doc.SetThumbnail)
	docs.Delete("/:id", deps.Doc.Delete)
	docs.Post("/:id/pages", deps.Page.AddPage)
	docs.Get("/:id/pages/:n/image", deps.Page.Image)
	docs.Get("/:id/pages/:n/thumb", deps.Page.Thumb)
	docs.Post("/:id/pages/:n/ocr", deps.Page.RerunOCR)
	docs.Post("/:id/transform", deps.Transform.Run)
	docs.Post("/:id/shares", deps.Doc.CreateShare)
	docs.Get("/:id/shares", deps.Doc.ListShares)
	docs.Delete("/:id/shares/:userId", deps.Doc.RevokeShare)

	api.Get("/templates", auth, deps.Template.List)
	api.Get("/outputs/:id", auth, deps.Output.Get)
}
