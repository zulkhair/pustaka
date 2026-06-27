package document

import (
	"context"

	"github.com/google/uuid"

	"github.com/zulkhair/pustaka/backend/internal/domain"
)

// OCRRunner is the slice of the OCR use-case AddPage needs. The concrete
// *ocr.Service satisfies it; declaring it here avoids an app-layer import cycle.
type OCRRunner interface {
	Run(ctx context.Context, page domain.Page, imageBytes []byte) (domain.OCRResult, error)
}

type AddPageResult struct {
	Page    domain.Page
	OCRText string
}

func (s *Service) AddPage(ctx context.Context, userID, docID string, imageBytes []byte, ocrRunner OCRRunner) (AddPageResult, error) {
	doc, err := s.ownedDocument(ctx, userID, docID)
	if err != nil {
		return AddPageResult{}, err
	}

	pageNumber := doc.PageCount + 1

	create := domain.CreatePageParams{
		ID:         uuid.NewString(),
		DocumentID: doc.ID,
		PageNumber: pageNumber,
		Status:     domain.StatusProcessing,
	}

	if doc.Mode == domain.ModePhoto {
		imgPath, err := s.blob.Put(userID, doc.ID, pageNumber, imageBytes)
		if err != nil {
			return AddPageResult{}, err
		}
		thumbPath, err := s.blob.Thumbnail(userID, doc.ID, pageNumber, imageBytes)
		if err != nil {
			return AddPageResult{}, err
		}
		create.ImagePath = &imgPath
		create.ThumbPath = &thumbPath
	}

	page, err := s.store.CreatePage(ctx, create)
	if err != nil {
		return AddPageResult{}, err
	}

	ocr, err := ocrRunner.Run(ctx, page, imageBytes)
	if err != nil {
		_ = s.store.SetPageStatus(ctx, page.ID, domain.StatusFailed)
		return AddPageResult{}, err
	}

	if err := s.store.SetPageStatus(ctx, page.ID, domain.StatusDone); err != nil {
		return AddPageResult{}, err
	}
	page.Status = domain.StatusDone

	if _, err := s.store.IncrementDocumentPageCount(ctx, doc.ID); err != nil {
		return AddPageResult{}, err
	}

	return AddPageResult{Page: page, OCRText: ocr.Text}, nil
}
