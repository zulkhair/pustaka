package jwt_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	pjwt "github.com/zulkhair/pustaka/backend/internal/pkg/jwt"
)

func TestGenerateAndParseAccess(t *testing.T) {
	tok, err := pjwt.GenerateAccess("user-1", "admin", "secret", time.Minute)
	require.NoError(t, err)

	claims, err := pjwt.ParseAccess(tok, "secret")
	require.NoError(t, err)
	require.Equal(t, "user-1", claims.UserID)
	require.Equal(t, "admin", claims.Role)
}

func TestParseAccessRejectsExpired(t *testing.T) {
	tok, err := pjwt.GenerateAccess("user-1", "user", "secret", -time.Minute)
	require.NoError(t, err)

	_, err = pjwt.ParseAccess(tok, "secret")
	require.Error(t, err)
}

func TestParseAccessRejectsWrongSecret(t *testing.T) {
	tok, err := pjwt.GenerateAccess("user-1", "user", "secret", time.Minute)
	require.NoError(t, err)

	_, err = pjwt.ParseAccess(tok, "other-secret")
	require.Error(t, err)
}

func TestGenerateRefreshToken(t *testing.T) {
	seen := map[string]struct{}{}
	for i := 0; i < 100; i++ {
		tok, err := pjwt.GenerateRefreshToken()
		require.NoError(t, err)
		require.Len(t, tok, 43) // 32 bytes base64url-no-pad
		_, dup := seen[tok]
		require.False(t, dup, "refresh tokens must be unique")
		seen[tok] = struct{}{}
	}
}
