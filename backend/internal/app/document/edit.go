package document

import (
	"context"
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

// Delete soft-deletes a document (recoverable in the DB). Owner-only.
func (s *Service) Delete(ctx context.Context, userID, docID string) error {
	if _, err := s.authorizeDoc(ctx, userID, docID, PermWrite); err != nil {
		return err
	}
	return s.store.SoftDeleteDocument(ctx, docID)
}
