package domain

import "time"

type EmailVerification struct {
	ID         string
	UserID     string
	CodeHash   string
	ExpiresAt  time.Time
	Attempts   int
	ConsumedAt *time.Time
	CreatedAt  time.Time
}

type CreateEmailVerificationParams struct {
	ID        string
	UserID    string
	CodeHash  string
	ExpiresAt time.Time
}
