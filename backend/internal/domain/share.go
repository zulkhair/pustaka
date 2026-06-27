package domain

import "time"

const (
	PermissionViewer = "viewer"
	PermissionEditor = "editor" // reserved; v1 never issues editor shares
)

type DocumentShare struct {
	ID               string
	DocumentID       string
	SharedWithUserID string
	Permission       string
	CreatedAt        time.Time
}

type CreateShareParams struct {
	ID               string
	DocumentID       string
	SharedWithUserID string
	Permission       string
}
