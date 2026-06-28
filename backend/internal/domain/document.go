package domain

import "time"

const (
	ModePhoto = "photo"
	ModeText  = "text"
)

const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusDone       = "done"
	StatusFailed     = "failed"
)

type Document struct {
	ID        string
	UserID    string
	Title     string
	Mode      string
	Status    string
	PageCount int
	ThumbPage int
	CreatedAt time.Time
}

type CreateDocumentParams struct {
	ID     string
	UserID string
	Title  string
	Mode   string
}
