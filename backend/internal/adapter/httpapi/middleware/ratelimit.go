package middleware

import (
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

type rlEntry struct {
	count       int
	windowStart time.Time
}

// RateLimit is an in-memory, mutex-guarded fixed-window limiter keyed by IP+Path.
// The window is injectable so tests can use a short duration.
func RateLimit(max int, window time.Duration) fiber.Handler {
	var mu sync.Mutex
	entries := make(map[string]*rlEntry)

	return func(c *fiber.Ctx) error {
		key := c.IP() + ":" + c.Path()
		now := time.Now()

		mu.Lock()
		// Opportunistically prune stale windows to bound memory growth.
		for k, e := range entries {
			if now.Sub(e.windowStart) >= window {
				delete(entries, k)
			}
		}
		e, ok := entries[key]
		if !ok || now.Sub(e.windowStart) >= window {
			e = &rlEntry{count: 0, windowStart: now}
			entries[key] = e
		}
		e.count++
		over := e.count > max
		mu.Unlock()

		if over {
			return fail(c, fiber.StatusTooManyRequests, "too many requests")
		}
		return c.Next()
	}
}
