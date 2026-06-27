package httpapi

import (
	"github.com/gofiber/fiber/v2"

	"github.com/zulkhair/pustaka/backend/internal/app/transform"
)

type OutputHandler struct {
	svc *transform.Service
}

func NewOutputHandler(svc *transform.Service) *OutputHandler { return &OutputHandler{svc: svc} }

func (h *OutputHandler) Get(c *fiber.Ctx) error {
	uid, ok := localUserID(c)
	if !ok {
		return Fail(c, fiber.StatusUnauthorized, "unauthorized")
	}
	out, err := h.svc.GetOutput(c.Context(), uid, c.Params("id"))
	if err != nil {
		return mapDocError(c, err)
	}
	return OK(c, toFullOutputDTO(out))
}
