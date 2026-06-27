package httpapi

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

// BuildApp constructs the Fiber app with the Caddy-aware proxy config, recover +
// logger middleware, and all routes mounted. Used by the composition root, the
// shared test harness, and the end-to-end tests.
//
// ProxyHeader + EnableTrustedProxyCheck + TrustedProxies make c.IP() resolve to
// the real client via X-Forwarded-For only when the request comes from the
// loopback reverse proxy (Caddy), so per-IP rate-limiting sees real addresses.
func BuildApp(deps RouterDeps) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:                 "pustaka",
		DisableStartupMessage:   true,
		ProxyHeader:             fiber.HeaderXForwardedFor,
		EnableTrustedProxyCheck: true,
		TrustedProxies:          []string{"127.0.0.1", "::1"},
	})
	app.Use(recover.New())
	app.Use(logger.New())
	Mount(app, deps)
	return app
}
