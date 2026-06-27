package httpapi

import (
	"github.com/gofiber/fiber/v2"

	"github.com/zulkhair/pustaka/backend/internal/app/transform"
	"github.com/zulkhair/pustaka/backend/internal/domain"
)

type TransformHandler struct {
	svc *transform.Service
}

func NewTransformHandler(svc *transform.Service) *TransformHandler { return &TransformHandler{svc: svc} }

type transformReq struct {
	TemplateID string `json:"template_id"`
}

type fullOutputDTO struct {
	ID         string `json:"id"`
	DocumentID string `json:"documentId"`
	TemplateID string `json:"templateId"`
	Content    string `json:"content"`
	Model      string `json:"model"`
	Status     string `json:"status"`
	CreatedAt  string `json:"createdAt"`
}

func toFullOutputDTO(o domain.Output) fullOutputDTO {
	return fullOutputDTO{
		ID: o.ID, DocumentID: o.DocumentID, TemplateID: o.TemplateID,
		Content: o.Content, Model: o.Model, Status: o.Status,
		CreatedAt: o.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func (h *TransformHandler) Run(c *fiber.Ctx) error {
	uid, ok := localUserID(c)
	if !ok {
		return Fail(c, fiber.StatusUnauthorized, "unauthorized")
	}
	var req transformReq
	if err := c.BodyParser(&req); err != nil {
		return Fail(c, fiber.StatusBadRequest, "invalid body")
	}
	if req.TemplateID == "" {
		return Fail(c, fiber.StatusBadRequest, "template_id required")
	}
	out, err := h.svc.Run(c.Context(), uid, c.Params("id"), req.TemplateID)
	if err != nil {
		return mapDocError(c, err)
	}
	return OK(c, toFullOutputDTO(out))
}
