package domain

import "errors"

var (
	ErrNotFound           = errors.New("not found")
	ErrConflict           = errors.New("conflict")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailNotVerified   = errors.New("email not verified")
	ErrInvalidCode        = errors.New("invalid code")
	ErrCodeExpired        = errors.New("code expired")
	ErrTooManyAttempts    = errors.New("too many attempts")
	ErrValidation         = errors.New("validation failed")
	// ErrResendCooldown is internal-only: ResendVerification returns it so the
	// service layer can enforce the cooldown, but the handler swallows it into a
	// generic 200 (never surfaced to the client).
	ErrResendCooldown = errors.New("resend cooldown active")
	ErrUnauthorized   = errors.New("unauthorized")
	ErrForbidden      = errors.New("forbidden")

	ErrUnsupportedFormat = errors.New("unsupported output format")
	ErrSchemaInvalid     = errors.New("output failed schema validation")
)
