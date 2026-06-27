package httpapi

import (
	"github.com/gofiber/fiber/v2"

	"github.com/zulkhair/pustaka/backend/internal/app/template"
)

type TemplateHandler struct {
	svc *template.Service
}

func NewTemplateHandler(svc *template.Service) *TemplateHandler { return &TemplateHandler{svc: svc} }

type templateDTO struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Scope        string `json:"scope"`
	OutputFormat string `json:"outputFormat"`
	DocTypeHint  string `json:"docTypeHint"`
	IsBuiltin    bool   `json:"isBuiltin"`
}

func (h *TemplateHandler) List(c *fiber.Ctx) error {
	tmpls, err := h.svc.List(c.Context())
	if err != nil {
		return mapDocError(c, err)
	}
	out := make([]templateDTO, 0, len(tmpls))
	for _, t := range tmpls {
		out = append(out, templateDTO{
			ID: t.ID, Name: t.Name, Scope: t.Scope, OutputFormat: t.OutputFormat,
			DocTypeHint: t.DocTypeHint, IsBuiltin: t.IsBuiltin,
		})
	}
	return OK(c, out)
}
