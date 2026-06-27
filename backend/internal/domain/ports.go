package domain

import "context"

type Mailer interface {
	SendVerificationCode(ctx context.Context, toEmail, code string) error
}

type Store interface {
	ExecTx(ctx context.Context, fn func(Store) error) error

	CreateUser(ctx context.Context, p CreateUserParams) (User, error)
	GetUserByEmail(ctx context.Context, email string) (User, error)
	GetUserByUsername(ctx context.Context, username string) (User, error)
	GetUserByID(ctx context.Context, id string) (User, error)
	SetUserEmailVerified(ctx context.Context, id string) error

	CreateEmailVerification(ctx context.Context, p CreateEmailVerificationParams) (EmailVerification, error)
	GetActiveEmailVerification(ctx context.Context, userID string) (EmailVerification, error)
	IncrementVerificationAttempts(ctx context.Context, id string) (int, error)
	ConsumeEmailVerification(ctx context.Context, id string) error
	DeleteEmailVerificationsByUser(ctx context.Context, userID string) error

	CreateSession(ctx context.Context, p CreateSessionParams) (Session, error)
	GetSessionByTokenHash(ctx context.Context, hash string) (Session, error)
	RevokeSession(ctx context.Context, id string) error
	RevokeAllUserSessions(ctx context.Context, userID string) error
}
