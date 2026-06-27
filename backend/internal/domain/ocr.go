package domain

import "time"

type OCRResult struct {
	ID        string
	PageID    string
	Model     string
	Text      string
	Status    string
	CreatedAt time.Time
}

type CreateOCRResultParams struct {
	ID     string
	PageID string
	Model  string
	Text   string
	Status string
}
