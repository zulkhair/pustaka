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

type VerifyReq struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

type TokensDTO struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int    `json:"expiresIn"`
}

func (h *AuthHandler) VerifyEmail(c *fiber.Ctx) error {
	var req VerifyReq
	if err := c.BodyParser(&req); err != nil {
		return Fail(c, fiber.StatusBadRequest, "invalid request body")
	}
	tokens, err := h.svc.VerifyEmail(c.Context(), auth.VerifyInput{
		Email: req.Email,
		Code:  req.Code,
	})
	if err != nil {
		return mapAuthError(c, err)
	}
	return OK(c, TokensDTO{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    tokens.ExpiresIn,
	})
}

type ResendReq struct {
	Email string `json:"email"`
}

func (h *AuthHandler) ResendVerification(c *fiber.Ctx) error {
	var req ResendReq
	if err := c.BodyParser(&req); err != nil {
		return Fail(c, fiber.StatusBadRequest, "invalid request body")
	}
	if req.Email == "" {
		return Fail(c, fiber.StatusBadRequest, "email is required")
	}
	if err := h.svc.ResendVerification(c.Context(), req.Email); err != nil {
		return mapAuthError(c, err)
	}
	return OK(c, nil) // always the same generic 200 (no cooldown signal leaks)
}

type LoginReq struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req LoginReq
	if err := c.BodyParser(&req); err != nil {
		return Fail(c, fiber.StatusBadRequest, "invalid request body")
	}
	if req.Identifier == "" || req.Password == "" {
		return Fail(c, fiber.StatusBadRequest, "identifier and password are required")
	}
	tok, err := h.svc.Login(c.Context(), auth.LoginInput{
		Identifier: req.Identifier,
		Password:   req.Password,
	})
	if err != nil {
		return mapAuthError(c, err)
	}
	return OK(c, TokensDTO{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		ExpiresIn:    tok.ExpiresIn,
	})
}

type RefreshReq struct {
	RefreshToken string `json:"refreshToken"`
}

func (h *AuthHandler) Refresh(c *fiber.Ctx) error {
	var req RefreshReq
	if err := c.BodyParser(&req); err != nil {
		return Fail(c, fiber.StatusBadRequest, "invalid request body")
	}
	if req.RefreshToken == "" {
		return Fail(c, fiber.StatusBadRequest, "refreshToken is required")
	}
	tok, err := h.svc.Refresh(c.Context(), req.RefreshToken)
	if err != nil {
		return mapAuthError(c, err)
	}
	return OK(c, TokensDTO{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		ExpiresIn:    tok.ExpiresIn,
	})
}

type LogoutReq struct {
	RefreshToken string `json:"refreshToken"`
}

func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	var req LogoutReq
	if err := c.BodyParser(&req); err != nil {
		return Fail(c, fiber.StatusBadRequest, "invalid request body")
	}
	if req.RefreshToken == "" {
		return Fail(c, fiber.StatusBadRequest, "refreshToken is required")
	}
	if err := h.svc.Logout(c.Context(), req.RefreshToken); err != nil {
		return mapAuthError(c, err)
	}
	return OK(c, nil)
}
