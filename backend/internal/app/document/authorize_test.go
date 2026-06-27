package document

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/domain"
)

type authFakeStore struct {
	domain.Store // embedded; unused methods panic if called
	docs         map[string]domain.Document
	shares       map[string]map[string]domain.DocumentShare // docID -> userID -> share
}

func (f authFakeStore) GetDocument(_ context.Context, id string) (domain.Document, error) {
	d, ok := f.docs[id]
	if !ok {
		return domain.Document{}, domain.ErrNotFound
	}
	return d, nil
}

func (f authFakeStore) GetShare(_ context.Context, docID, userID string) (domain.DocumentShare, error) {
	if m, ok := f.shares[docID]; ok {
		if s, ok := m[userID]; ok {
			return s, nil
		}
	}
	return domain.DocumentShare{}, domain.ErrNotFound
}

func newAuthService(fs authFakeStore) *Service {
	return New(fs, nil)
}

func TestAuthorizeDoc(t *testing.T) {
	ctx := context.Background()
	owner, sharee, stranger := "owner-1", "sharee-1", "stranger-1"
	doc := domain.Document{ID: "doc-1", UserID: owner}

	fs := authFakeStore{
		docs: map[string]domain.Document{doc.ID: doc},
		shares: map[string]map[string]domain.DocumentShare{
			doc.ID: {sharee: {ID: "s1", DocumentID: doc.ID, SharedWithUserID: sharee, Permission: domain.PermissionViewer}},
		},
	}
	svc := newAuthService(fs)

	// owner: read + write OK
	_, err := svc.authorizeDoc(ctx, owner, doc.ID, PermRead)
	require.NoError(t, err)
	_, err = svc.authorizeDoc(ctx, owner, doc.ID, PermWrite)
	require.NoError(t, err)

	// sharee: read OK, write Forbidden
	got, err := svc.authorizeDoc(ctx, sharee, doc.ID, PermRead)
	require.NoError(t, err)
	require.Equal(t, doc.ID, got.ID)
	_, err = svc.authorizeDoc(ctx, sharee, doc.ID, PermWrite)
	require.ErrorIs(t, err, domain.ErrForbidden)

	// stranger: read NotFound (owner-isolation), write NotFound
	_, err = svc.authorizeDoc(ctx, stranger, doc.ID, PermRead)
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = svc.authorizeDoc(ctx, stranger, doc.ID, PermWrite)
	require.ErrorIs(t, err, domain.ErrNotFound)

	// missing document: NotFound
	_, err = svc.authorizeDoc(ctx, owner, "nope", PermRead)
	require.ErrorIs(t, err, domain.ErrNotFound)
}
