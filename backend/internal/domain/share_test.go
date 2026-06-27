package domain_test

import (
	"testing"

	"github.com/zulkhair/pustaka/backend/internal/domain"
)

func TestSharePermissionConstants(t *testing.T) {
	if domain.PermissionViewer == "" || domain.PermissionEditor == "" {
		t.Fatal("permission constants must be non-empty")
	}
	if domain.PermissionViewer == domain.PermissionEditor {
		t.Fatal("viewer and editor must differ")
	}
}

func TestDocumentShareZeroValue(t *testing.T) {
	var s domain.DocumentShare
	_ = domain.CreateShareParams{
		ID:               "x",
		DocumentID:       "d",
		SharedWithUserID: "u",
		Permission:       domain.PermissionViewer,
	}
	if s.ID != "" {
		t.Fatal("zero value should have empty ID")
	}
}
