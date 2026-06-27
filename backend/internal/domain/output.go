package domain

import "time"

type Output struct {
	ID         string
	UserID     string
	DocumentID string
	TemplateID string
	Content    string
	FilePath   *string
	Model      string
	Status     string
	CreatedAt  time.Time
}

type CreateOutputParams struct {
	ID         string
	UserID     string
	DocumentID string
	TemplateID string
	Content    string
	FilePath   *string
	Model      string
	Status     string
}
