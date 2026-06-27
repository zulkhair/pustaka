package store

import (
	"context"

	"github.com/zulkhair/pustaka/backend/internal/adapter/store/sqlc"
	"github.com/zulkhair/pustaka/backend/internal/domain"
)

func toShare(r sqlc.DocumentShare) domain.DocumentShare {
	return domain.DocumentShare{
		ID:               r.ID,
		DocumentID:       r.DocumentID,
		SharedWithUserID: r.SharedWithUserID,
		Permission:       r.Permission,
		CreatedAt:        r.CreatedAt.Time,
	}
}

func (s *Store) CreateShare(ctx context.Context, p domain.CreateShareParams) (domain.DocumentShare, error) {
	r, err := s.q.CreateShare(ctx, sqlc.CreateShareParams{
		ID:               p.ID,
		DocumentID:       p.DocumentID,
		SharedWithUserID: p.SharedWithUserID,
		Permission:       p.Permission,
	})
	if err != nil {
		return domain.DocumentShare{}, mapErr(err)
	}
	return toShare(r), nil
}

func (s *Store) ListSharesForDocument(ctx context.Context, documentID string) ([]domain.DocumentShare, error) {
	rows, err := s.q.ListSharesForDocument(ctx, documentID)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.DocumentShare, 0, len(rows))
	for _, r := range rows {
		out = append(out, toShare(r))
	}
	return out, nil
}

func (s *Store) GetShare(ctx context.Context, documentID, userID string) (domain.DocumentShare, error) {
	r, err := s.q.GetShare(ctx, sqlc.GetShareParams{DocumentID: documentID, SharedWithUserID: userID})
	if err != nil {
		return domain.DocumentShare{}, mapErr(err)
	}
	return toShare(r), nil
}

func (s *Store) DeleteShare(ctx context.Context, documentID, userID string) error {
	return mapErr(s.q.DeleteShare(ctx, sqlc.DeleteShareParams{DocumentID: documentID, SharedWithUserID: userID}))
}

func (s *Store) ListDocumentsSharedWith(ctx context.Context, userID string) ([]domain.Document, error) {
	rows, err := s.q.ListDocumentsSharedWith(ctx, userID)
	if err != nil {
		return nil, mapErr(err)
	}
	out := make([]domain.Document, 0, len(rows))
	for _, r := range rows {
		out = append(out, toDocument(r))
	}
	return out, nil
}
