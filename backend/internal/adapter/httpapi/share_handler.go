package httpapi

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/zulkhair/pustaka/backend/internal/app/document"
)

type ShareReq struct {
	Email      string `json:"email"`
	Permission string `json:"permission"`
}

type ShareDTO struct {
	UserID     string    `json:"userId"`
	Username   string    `json:"username"`
	Email      string    `json:"email"`
	Permission string    `json:"permission"`
	CreatedAt  time.Time `json:"createdAt"`
}

// CreateShare grants a viewer share on a document to a verified user (owner only).
func (h *DocHandler) CreateShare(c *fiber.Ctx) error {
	userID, _ := localUserID(c)
	docID := c.Params("id")

	var req ShareReq
	if err := c.BodyParser(&req); err != nil {
		return Fail(c, fiber.StatusBadRequest, "invalid request body")
	}
	share, err := h.svc.ShareDocument(c.Context(), userID, docID, document.ShareInput{
		Email:      req.Email,
		Permission: req.Permission,
	})
	if err != nil {
		return mapDocError(c, err)
	}
	return OK(c, fiber.Map{
		"documentId": share.DocumentID,
		"userId":     share.SharedWithUserID,
		"permission": share.Permission,
		"createdAt":  share.CreatedAt,
	})
}

// ListShares returns the document's shares (owner only).
func (h *DocHandler) ListShares(c *fiber.Ctx) error {
	userID, _ := localUserID(c)
	docID := c.Params("id")

	views, err := h.svc.ListShares(c.Context(), userID, docID)
	if err != nil {
		return mapDocError(c, err)
	}
	out := make([]ShareDTO, 0, len(views))
	for _, v := range views {
		out = append(out, ShareDTO{
			UserID:     v.UserID,
			Username:   v.Username,
			Email:      v.Email,
			Permission: v.Permission,
			CreatedAt:  v.CreatedAt,
		})
	}
	return OK(c, out)
}

// RevokeShare removes a share (owner only). Idempotent.
func (h *DocHandler) RevokeShare(c *fiber.Ctx) error {
	userID, _ := localUserID(c)
	docID := c.Params("id")
	target := c.Params("userId")

	if err := h.svc.RevokeShare(c.Context(), userID, docID, target); err != nil {
		return mapDocError(c, err)
	}
	return OK(c, fiber.Map{"revoked": true})
}
