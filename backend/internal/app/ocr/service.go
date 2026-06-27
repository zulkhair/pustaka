package ocr

import (
	"context"

	"github.com/google/uuid"

	"github.com/zulkhair/pustaka/backend/internal/app/document"
	"github.com/zulkhair/pustaka/backend/internal/domain"
)

type Service struct {
	store domain.Store
	ai    domain.AIClient
	blob  domain.BlobStore
	authz document.Authorizer
}

func New(store domain.Store, ai domain.AIClient, blob domain.BlobStore, authz document.Authorizer) *Service {
	return &Service{store: store, ai: ai, blob: blob, authz: authz}
}

// Run transcribes the given page image and stores an ocr_result row. It
// satisfies document.OCRRunner.
func (s *Service) Run(ctx context.Context, page domain.Page, imageBytes []byte) (domain.OCRResult, error) {
	text, err := s.ai.Transcribe(ctx, imageBytes)
	if err != nil {
		// Persist a failed marker so the page shows a retryable failure.
		_, _ = s.store.CreateOCRResult(ctx, domain.CreateOCRResultParams{
			ID: uuid.NewString(), PageID: page.ID, Model: "", Text: "", Status: domain.StatusFailed,
		})
		return domain.OCRResult{}, err
	}
	res, err := s.store.CreateOCRResult(ctx, domain.CreateOCRResultParams{
		ID: uuid.NewString(), PageID: page.ID, Model: "glm-ocr", Text: text, Status: domain.StatusDone,
	})
	if err != nil {
		return domain.OCRResult{}, err
	}
	return res, nil
}

// Rerun re-OCRs a stored page image. Owner-only (write) via the shared
// authorizer. A page without a stored image (text mode) yields ErrNotFound.
func (s *Service) Rerun(ctx context.Context, userID, docID string, pageNumber int) (domain.OCRResult, error) {
	if _, err := s.authz.AuthorizeDoc(ctx, userID, docID, document.PermWrite); err != nil {
		return domain.OCRResult{}, err
	}
	page, err := s.store.GetPageByNumber(ctx, docID, pageNumber)
	if err != nil {
		return domain.OCRResult{}, err
	}
	if page.ImagePath == nil {
		return domain.OCRResult{}, domain.ErrNotFound
	}
	imageBytes, err := s.blob.Get(*page.ImagePath)
	if err != nil {
		return domain.OCRResult{}, err
	}
	return s.Run(ctx, page, imageBytes)
}
