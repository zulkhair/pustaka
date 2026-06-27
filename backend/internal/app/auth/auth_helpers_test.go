package auth_test

import (
	"testing"

	"github.com/zulkhair/pustaka/backend/internal/adapter/store"
	"github.com/zulkhair/pustaka/backend/internal/testsupport"
)

// newTestStore forwards to the shared harness so every auth test uses the same
// testcontainers-backed store (no ad-hoc setup).
func newTestStore(t *testing.T) (*store.Store, func()) {
	return testsupport.NewTestStore(t)
}
