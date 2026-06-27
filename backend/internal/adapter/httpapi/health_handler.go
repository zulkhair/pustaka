package httpapi

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Pinger is the minimal surface the health check needs (satisfied by *pgxpool.Pool).
type Pinger interface {
	Ping(ctx context.Context) error
}

// HealthHandler always returns 200; data.db is "up" when the DB ping succeeds, else "down".
func HealthHandler(p Pinger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
		defer cancel()
		db := "up"
		if p == nil || p.Ping(ctx) != nil {
			db = "down"
		}
		return OK(c, fiber.Map{"db": db})
	}
}
