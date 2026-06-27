package domain_test

import (
	"testing"

	"github.com/zulkhair/pustaka/backend/internal/domain"
)

func TestSentinelErrorsNonNilAndDistinct(t *testing.T) {
	errs := map[string]error{
		"ErrNotFound":           domain.ErrNotFound,
		"ErrConflict":           domain.ErrConflict,
		"ErrInvalidCredentials": domain.ErrInvalidCredentials,
		"ErrEmailNotVerified":   domain.ErrEmailNotVerified,
		"ErrInvalidCode":        domain.ErrInvalidCode,
		"ErrCodeExpired":        domain.ErrCodeExpired,
		"ErrTooManyAttempts":    domain.ErrTooManyAttempts,
		"ErrValidation":         domain.ErrValidation,
		"ErrResendCooldown":     domain.ErrResendCooldown,
		"ErrUnauthorized":       domain.ErrUnauthorized,
		"ErrForbidden":          domain.ErrForbidden,
	}
	for name, err := range errs {
		if err == nil {
			t.Fatalf("%s is nil", name)
		}
	}
	seen := map[error]string{}
	for name, err := range errs {
		if prev, ok := seen[err]; ok {
			t.Fatalf("%s and %s are the same error value", name, prev)
		}
		seen[err] = name
	}
}
