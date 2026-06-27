package transform_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/app/document"
	"github.com/zulkhair/pustaka/backend/internal/app/transform"
	"github.com/zulkhair/pustaka/backend/internal/domain"
)

type fakeAuthz struct{ err error }

func (f fakeAuthz) AuthorizeDoc(_ context.Context, _, _ string, _ document.Permission) (domain.Document, error) {
	if f.err != nil {
		return domain.Document{}, f.err
	}
	return domain.Document{ID: "doc-1"}, nil
}

func TestTransformRunDeniedForSharee(t *testing.T) {
	// nil store + ai: if Run reaches them it panics, proving the guard short-circuits.
	svc := transform.New(nil, nil, fakeAuthz{err: domain.ErrForbidden})
	_, err := svc.Run(context.Background(), "sharee", "doc-1", "tmpl-1")
	require.ErrorIs(t, err, domain.ErrForbidden)
}
