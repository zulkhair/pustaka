package httpapi

import (
	"io"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/zulkhair/pustaka/backend/internal/app/document"
	"github.com/zulkhair/pustaka/backend/internal/app/ocr"
	"github.com/zulkhair/pustaka/backend/internal/domain"
)

type PageHandler struct {
	docSvc *document.Service
	ocrSvc *ocr.Service
	blob   domain.BlobStore
}

func NewPageHandler(docSvc *document.Service, ocrSvc *ocr.Service, blob domain.BlobStore) *PageHandler {
	return &PageHandler{docSvc: docSvc, ocrSvc: ocrSvc, blob: blob}
}

type addPageDTO struct {
	PageNumber int    `json:"pageNumber"`
	OCRText    string `json:"ocrText"`
}

func (h *PageHandler) AddPage(c *fiber.Ctx) error {
	uid, ok := localUserID(c)
	if !ok {
		return Fail(c, fiber.StatusUnauthorized, "unauthorized")
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return Fail(c, fiber.StatusBadRequest, "missing file")
	}
	f, err := fileHeader.Open()
	if err != nil {
		return Fail(c, fiber.StatusBadRequest, "cannot open file")
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(f)
	if err != nil {
		return Fail(c, fiber.StatusBadRequest, "cannot read file")
	}

	res, err := h.docSvc.AddPage(c.Context(), uid, c.Params("id"), data, h.ocrSvc)
	if err != nil {
		return mapDocError(c, err)
	}
	return OK(c, addPageDTO{PageNumber: res.Page.PageNumber, OCRText: res.OCRText})
}

// findPage returns the owner-scoped page matching the URL params, or an error.
func (h *PageHandler) findPage(c *fiber.Ctx, uid string) (domain.Page, error) {
	n, err := strconv.Atoi(c.Params("n"))
	if err != nil {
		return domain.Page{}, domain.ErrValidation
	}
	detail, err := h.docSvc.Get(c.Context(), uid, c.Params("id"))
	if err != nil {
		return domain.Page{}, err
	}
	for _, pv := range detail.Pages {
		if pv.Page.PageNumber == n {
			return pv.Page, nil
		}
	}
	return domain.Page{}, domain.ErrNotFound
}

func (h *PageHandler) serveBlob(c *fiber.Ctx, path *string) error {
	if path == nil {
		return Fail(c, fiber.StatusNotFound, "no image")
	}
	data, err := h.blob.Get(*path)
	if err != nil {
		return mapDocError(c, err)
	}
	c.Set("Content-Type", "image/jpeg")
	return c.Send(data)
}

func (h *PageHandler) Image(c *fiber.Ctx) error {
	uid, ok := localUserID(c)
	if !ok {
		return Fail(c, fiber.StatusUnauthorized, "unauthorized")
	}
	page, err := h.findPage(c, uid)
	if err != nil {
		return mapDocError(c, err)
	}
	return h.serveBlob(c, page.ImagePath)
}

func (h *PageHandler) Thumb(c *fiber.Ctx) error {
	uid, ok := localUserID(c)
	if !ok {
		return Fail(c, fiber.StatusUnauthorized, "unauthorized")
	}
	page, err := h.findPage(c, uid)
	if err != nil {
		return mapDocError(c, err)
	}
	return h.serveBlob(c, page.ThumbPath)
}

func (h *PageHandler) RerunOCR(c *fiber.Ctx) error {
	uid, ok := localUserID(c)
	if !ok {
		return Fail(c, fiber.StatusUnauthorized, "unauthorized")
	}
	n, err := strconv.Atoi(c.Params("n"))
	if err != nil {
		return Fail(c, fiber.StatusBadRequest, "bad page number")
	}
	res, err := h.ocrSvc.Rerun(c.Context(), uid, c.Params("id"), n)
	if err != nil {
		return mapDocError(c, err)
	}
	return OK(c, addPageDTO{PageNumber: n, OCRText: res.Text})
}
