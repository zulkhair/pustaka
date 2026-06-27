package httpapi

import "github.com/gofiber/fiber/v2"

type envelope struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// OK writes a 200 success envelope: {status:0, message:"ok", data}.
func OK(c *fiber.Ctx, data any) error {
	return c.Status(fiber.StatusOK).JSON(envelope{Status: 0, Message: "ok", Data: data})
}

// Fail writes an error envelope with the given HTTP status: {status:1, message, data:null}.
func Fail(c *fiber.Ctx, httpCode int, msg string) error {
	return c.Status(httpCode).JSON(envelope{Status: 1, Message: msg, Data: nil})
}
