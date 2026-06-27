package domain

import "time"

const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

type User struct {
	ID            string
	Username      string
	Email         string
	PasswordHash  string
	Role          string
	EmailVerified bool
	CreatedAt     time.Time
}

type CreateUserParams struct {
	ID           string
	Username     string
	Email        string
	PasswordHash string
	Role         string
}
