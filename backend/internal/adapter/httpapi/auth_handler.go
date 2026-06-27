package httpapi

import (
	"github.com/gofiber/fiber/v2"

	"github.com/zulkhair/pustaka/backend/internal/app/auth"
)

type AuthHandler struct {
	svc *auth.Service
}

func NewAuthHandler(svc *auth.Service) *AuthHandler {
	return &AuthHandler{svc: svc}
}

type RegisterReq struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req RegisterReq
	if err := c.BodyParser(&req); err != nil {
		return Fail(c, fiber.StatusBadRequest, "invalid request body")
	}
	if err := h.svc.Register(c.Context(), auth.RegisterInput{
		Username: req.Username,
		Email:    req.Email,
		Password: req.Password,
	}); err != nil {
		return mapAuthError(c, err)
	}
	return OK(c, nil)
}
