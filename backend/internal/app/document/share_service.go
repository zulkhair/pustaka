package document

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/zulkhair/pustaka/backend/internal/domain"
)

type ShareInput struct {
	Email      string
	Permission string
}

type ShareView struct {
	UserID     string
	Username   string
	Email      string
	Permission string
	CreatedAt  time.Time
}

// ShareDocument grants a viewer share on docID to the user identified by
// in.Email. The caller must be the document owner (PermWrite). The target must
// be an existing, email-verified user; otherwise a GENERIC ErrValidation is
// returned (no enumeration — same error for missing / unverified / self).
// v1 always issues a viewer share regardless of the requested permission.
func (s *Service) ShareDocument(ctx context.Context, ownerID, docID string, in ShareInput) (domain.DocumentShare, error) {
	doc, err := s.authorizeDoc(ctx, ownerID, docID, PermWrite)
	if err != nil {
		return domain.DocumentShare{}, err
	}

	email := strings.TrimSpace(strings.ToLower(in.Email))
	if email == "" {
		return domain.DocumentShare{}, domain.ErrValidation
	}

	target, err := s.store.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.DocumentShare{}, domain.ErrValidation // generic: do not leak
		}
		return domain.DocumentShare{}, err
	}
	if !target.EmailVerified || target.ID == doc.UserID {
		return domain.DocumentShare{}, domain.ErrValidation // unverified or self -> generic
	}

	return s.store.CreateShare(ctx, domain.CreateShareParams{
		ID:               uuid.NewString(),
		DocumentID:       docID,
		SharedWithUserID: target.ID,
		Permission:       domain.PermissionViewer, // v1: always viewer
	})
}

// ListShares returns the shares on a document (owner only), each enriched with
// the target user's display fields.
func (s *Service) ListShares(ctx context.Context, ownerID, docID string) ([]ShareView, error) {
	if _, err := s.authorizeDoc(ctx, ownerID, docID, PermWrite); err != nil {
		return nil, err
	}
	shares, err := s.store.ListSharesForDocument(ctx, docID)
	if err != nil {
		return nil, err
	}
	views := make([]ShareView, 0, len(shares))
	for _, sh := range shares {
		u, err := s.store.GetUserByID(ctx, sh.SharedWithUserID)
		if err != nil {
			return nil, err
		}
		views = append(views, ShareView{
			UserID:     u.ID,
			Username:   u.Username,
			Email:      u.Email,
			Permission: sh.Permission,
			CreatedAt:  sh.CreatedAt,
		})
	}
	return views, nil
}

// RevokeShare deletes the share for targetUserID on docID (owner only). It is
// idempotent: revoking a non-existent share is a no-op success. Access is cut
// immediately because authorizeDoc consults GetShare on the next request.
func (s *Service) RevokeShare(ctx context.Context, ownerID, docID, targetUserID string) error {
	if _, err := s.authorizeDoc(ctx, ownerID, docID, PermWrite); err != nil {
		return err
	}
	return s.store.DeleteShare(ctx, docID, targetUserID)
}

// ListDocumentsWithShared returns the user's owned documents and the documents
// shared with them, as two separate slices.
func (s *Service) ListDocumentsWithShared(ctx context.Context, userID string) (owned, shared []domain.Document, err error) {
	owned, err = s.store.ListDocumentsByUser(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	shared, err = s.store.ListDocumentsSharedWith(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	return owned, shared, nil
}
