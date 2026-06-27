package document

import (
	"context"
	"errors"

	"github.com/zulkhair/pustaka/backend/internal/domain"
)

// Permission is the access level a caller needs for a document operation.
type Permission int

const (
	// PermRead is granted to the owner OR any user with an active share.
	PermRead Permission = iota
	// PermWrite is granted to the owner only.
	PermWrite
)

// AuthorizeDoc is the exported form of authorizeDoc so sibling app services
// (ocr, transform) enforce the SAME access rule without re-implementing it.
func (s *Service) AuthorizeDoc(ctx context.Context, userID, docID string, perm Permission) (domain.Document, error) {
	return s.authorizeDoc(ctx, userID, docID, perm)
}

// Authorizer is the access-control surface ocr/transform depend on (satisfied
// by *document.Service).
type Authorizer interface {
	AuthorizeDoc(ctx context.Context, userID, docID string, perm Permission) (domain.Document, error)
}

// authorizeDoc is the single source of truth for document access control.
// It loads the document and applies: read = owner OR active share; write = owner
// only. It returns the loaded document so callers avoid a second fetch.
//
//   - missing doc                     -> ErrNotFound
//   - read by non-owner with no share -> ErrNotFound (do not reveal existence)
//   - write by a non-owner sharee     -> ErrForbidden (visible, read-only)
//   - write by a non-owner stranger   -> ErrNotFound
func (s *Service) authorizeDoc(ctx context.Context, userID, docID string, perm Permission) (domain.Document, error) {
	doc, err := s.store.GetDocument(ctx, docID)
	if err != nil {
		return domain.Document{}, err // ErrNotFound bubbles up unchanged
	}

	if doc.UserID == userID {
		return doc, nil // owner: read and write both allowed
	}

	_, shareErr := s.store.GetShare(ctx, docID, userID)
	hasShare := shareErr == nil
	if shareErr != nil && !errors.Is(shareErr, domain.ErrNotFound) {
		return domain.Document{}, shareErr
	}

	switch perm {
	case PermRead:
		if hasShare {
			return doc, nil
		}
		return domain.Document{}, domain.ErrNotFound
	case PermWrite:
		if hasShare {
			return domain.Document{}, domain.ErrForbidden
		}
		return domain.Document{}, domain.ErrNotFound
	default:
		return domain.Document{}, domain.ErrForbidden
	}
}
