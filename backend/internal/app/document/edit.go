package document

import (
	"context"
	"errors"
	"strings"

	"github.com/zulkhair/pustaka/backend/internal/domain"
)

// Rename updates a document's title. Owner-only.
func (s *Service) Rename(ctx context.Context, userID, docID, title string) (domain.Document, error) {
	if _, err := s.authorizeDoc(ctx, userID, docID, PermWrite); err != nil {
		return domain.Document{}, err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return domain.Document{}, domain.ErrValidation
	}
	return s.store.UpdateDocumentTitle(ctx, docID, title)
}

// SetThumbnail picks which scanned page is the document cover. Owner-only; the
// page must exist and have a thumbnail (i.e. a photo page).
func (s *Service) SetThumbnail(ctx context.Context, userID, docID string, page int) (domain.Document, error) {
	if _, err := s.authorizeDoc(ctx, userID, docID, PermWrite); err != nil {
		return domain.Document{}, err
	}
	p, err := s.store.GetPageByNumber(ctx, docID, page)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Document{}, domain.ErrValidation // no such page in this doc
		}
		return domain.Document{}, err
	}
	if p.ThumbPath == nil {
		return domain.Document{}, domain.ErrValidation // page has no image to use as a cover
	}
	return s.store.SetDocumentThumbPage(ctx, docID, page)
}

// Delete soft-deletes a document (recoverable in the DB). Owner-only.
func (s *Service) Delete(ctx context.Context, userID, docID string) error {
	if _, err := s.authorizeDoc(ctx, userID, docID, PermWrite); err != nil {
		return err
	}
	return s.store.SoftDeleteDocument(ctx, docID)
}
