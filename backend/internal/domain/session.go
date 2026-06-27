package domain

import "time"

type Session struct {
	ID               string
	UserID           string
	RefreshTokenHash string
	ExpiresAt        time.Time
	CreatedAt        time.Time
	RevokedAt        *time.Time
}

type CreateSessionParams struct {
	ID               string
	UserID           string
	RefreshTokenHash string
	ExpiresAt        time.Time
}
