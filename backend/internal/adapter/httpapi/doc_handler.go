package httpapi

import (
	"fmt"

	"github.com/gofiber/fiber/v2"

	"github.com/zulkhair/pustaka/backend/internal/app/document"
	"github.com/zulkhair/pustaka/backend/internal/domain"
)

type DocHandler struct {
	svc *document.Service
}

func NewDocHandler(svc *document.Service) *DocHandler { return &DocHandler{svc: svc} }

type createDocReq struct {
	Title string `json:"title"`
	Mode  string `json:"mode"`
}

type docDTO struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Mode      string `json:"mode"`
	Status    string `json:"status"`
	PageCount int    `json:"pageCount"`
	CreatedAt string `json:"createdAt"`
	ThumbURL  string `json:"thumbUrl,omitempty"`
}

func (h *DocHandler) Create(c *fiber.Ctx) error {
	uid, ok := localUserID(c)
	if !ok {
		return Fail(c, fiber.StatusUnauthorized, "unauthorized")
	}
	var req createDocReq
	if err := c.BodyParser(&req); err != nil {
		return Fail(c, fiber.StatusBadRequest, "invalid body")
	}
	doc, err := h.svc.Create(c.Context(), uid, req.Title, req.Mode)
	if err != nil {
		return mapDocError(c, err)
	}
	return OK(c, docDTO{
		ID: doc.ID, Title: doc.Title, Mode: doc.Mode, Status: doc.Status,
		PageCount: doc.PageCount, CreatedAt: doc.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

func (h *DocHandler) List(c *fiber.Ctx) error {
	uid, ok := localUserID(c)
	if !ok {
		return Fail(c, fiber.StatusUnauthorized, "unauthorized")
	}
	owned, shared, err := h.svc.ListDocumentsWithShared(c.Context(), uid)
	if err != nil {
		return mapDocError(c, err)
	}
	return OK(c, fiber.Map{
		"owned":  toDocDTOs(owned),
		"shared": toDocDTOs(shared),
	})
}

// toDocDTOs builds the inline docDTO list, reused for both the owned and shared arrays.
func toDocDTOs(docs []domain.Document) []docDTO {
	out := make([]docDTO, 0, len(docs))
	for _, d := range docs {
		dto := docDTO{
			ID: d.ID, Title: d.Title, Mode: d.Mode, Status: d.Status,
			PageCount: d.PageCount, CreatedAt: d.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		if d.PageCount > 0 && d.Mode == "photo" {
			dto.ThumbURL = fmt.Sprintf("/api/documents/%s/pages/1/thumb", d.ID)
		}
		out = append(out, dto)
	}
	return out
}

type renameDocReq struct {
	Title string `json:"title"`
}

func (h *DocHandler) Rename(c *fiber.Ctx) error {
	uid, ok := localUserID(c)
	if !ok {
		return Fail(c, fiber.StatusUnauthorized, "unauthorized")
	}
	var req renameDocReq
	if err := c.BodyParser(&req); err != nil {
		return Fail(c, fiber.StatusBadRequest, "invalid body")
	}
	doc, err := h.svc.Rename(c.Context(), uid, c.Params("id"), req.Title)
	if err != nil {
		return mapDocError(c, err)
	}
	return OK(c, docDTO{
		ID: doc.ID, Title: doc.Title, Mode: doc.Mode, Status: doc.Status,
		PageCount: doc.PageCount, CreatedAt: doc.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

func (h *DocHandler) Delete(c *fiber.Ctx) error {
	uid, ok := localUserID(c)
	if !ok {
		return Fail(c, fiber.StatusUnauthorized, "unauthorized")
	}
	if err := h.svc.Delete(c.Context(), uid, c.Params("id")); err != nil {
		return mapDocError(c, err)
	}
	return OK(c, nil)
}

type pageDTO struct {
	PageNumber int    `json:"pageNumber"`
	Status     string `json:"status"`
	OCRText    string `json:"ocrText"`
	OCRStatus  string `json:"ocrStatus"`
	ImageURL   string `json:"imageUrl,omitempty"`
	ThumbURL   string `json:"thumbUrl,omitempty"`
}

type outputDTO struct {
	ID         string `json:"id"`
	TemplateID string `json:"templateId"`
	Content    string `json:"content"`
	Status     string `json:"status"`
	CreatedAt  string `json:"createdAt"`
}

type docDetailDTO struct {
	Document docDTO      `json:"document"`
	Pages    []pageDTO   `json:"pages"`
	Outputs  []outputDTO `json:"outputs"`
}

func (h *DocHandler) Get(c *fiber.Ctx) error {
	uid, ok := localUserID(c)
	if !ok {
		return Fail(c, fiber.StatusUnauthorized, "unauthorized")
	}
	detail, err := h.svc.Get(c.Context(), uid, c.Params("id"))
	if err != nil {
		return mapDocError(c, err)
	}
	d := detail.Document
	dto := docDetailDTO{
		Document: docDTO{
			ID: d.ID, Title: d.Title, Mode: d.Mode, Status: d.Status,
			PageCount: d.PageCount, CreatedAt: d.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		},
		Pages:   make([]pageDTO, 0, len(detail.Pages)),
		Outputs: make([]outputDTO, 0, len(detail.Outputs)),
	}
	for _, pv := range detail.Pages {
		pd := pageDTO{
			PageNumber: pv.Page.PageNumber, Status: pv.Page.Status,
			OCRText: pv.OCRText, OCRStatus: pv.OCRStatus,
		}
		if pv.Page.ImagePath != nil {
			pd.ImageURL = fmt.Sprintf("/api/documents/%s/pages/%d/image", d.ID, pv.Page.PageNumber)
		}
		if pv.Page.ThumbPath != nil {
			pd.ThumbURL = fmt.Sprintf("/api/documents/%s/pages/%d/thumb", d.ID, pv.Page.PageNumber)
		}
		dto.Pages = append(dto.Pages, pd)
	}
	for _, o := range detail.Outputs {
		dto.Outputs = append(dto.Outputs, outputDTO{
			ID: o.ID, TemplateID: o.TemplateID, Content: o.Content, Status: o.Status,
			CreatedAt: o.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	return OK(c, dto)
}
