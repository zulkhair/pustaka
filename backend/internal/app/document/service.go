package document

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/zulkhair/pustaka/backend/internal/domain"
)

type Service struct {
	store domain.Store
	blob  domain.BlobStore
}

func New(store domain.Store, blob domain.BlobStore) *Service {
	return &Service{store: store, blob: blob}
}

type PageView struct {
	Page      domain.Page
	OCRText   string
	OCRStatus string
}

type DocumentDetail struct {
	Document domain.Document
	Pages    []PageView
	Outputs  []domain.Output
}

func (s *Service) Create(ctx context.Context, userID, title, mode string) (domain.Document, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return domain.Document{}, domain.ErrValidation
	}
	if mode != domain.ModePhoto && mode != domain.ModeText {
		return domain.Document{}, domain.ErrValidation
	}
	return s.store.CreateDocument(ctx, domain.CreateDocumentParams{
		ID: uuid.NewString(), UserID: userID, Title: title, Mode: mode,
	})
}

func (s *Service) List(ctx context.Context, userID string) ([]domain.Document, error) {
	return s.store.ListDocumentsByUser(ctx, userID)
}

func (s *Service) Get(ctx context.Context, userID, docID string) (DocumentDetail, error) {
	doc, err := s.authorizeDoc(ctx, userID, docID, PermRead)
	if err != nil {
		return DocumentDetail{}, err
	}

	pages, err := s.store.ListPagesByDocument(ctx, docID)
	if err != nil {
		return DocumentDetail{}, err
	}
	views := make([]PageView, 0, len(pages))
	for _, p := range pages {
		pv := PageView{Page: p}
		ocr, err := s.store.GetLatestOCRResult(ctx, p.ID)
		switch {
		case err == nil:
			pv.OCRText = ocr.Text
			pv.OCRStatus = ocr.Status
		case errors.Is(err, domain.ErrNotFound):
			pv.OCRStatus = domain.StatusPending
		default:
			return DocumentDetail{}, err
		}
		views = append(views, pv)
	}

	outputs, err := s.store.ListOutputsByDocument(ctx, docID)
	if err != nil {
		return DocumentDetail{}, err
	}
	return DocumentDetail{Document: doc, Pages: views, Outputs: outputs}, nil
}
