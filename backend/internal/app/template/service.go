package template

import (
	"context"

	"github.com/zulkhair/pustaka/backend/internal/domain"
)

type Service struct {
	store domain.Store
}

func New(store domain.Store) *Service { return &Service{store: store} }

// List returns all available templates. In v1 these are the global built-ins;
// per-user templates are a post-v1 addition.
func (s *Service) List(ctx context.Context) ([]domain.Template, error) {
	return s.store.ListTemplates(ctx)
}
