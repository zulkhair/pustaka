package hash_test

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/pkg/hash"
)

func TestHashPasswordRoundtrip(t *testing.T) {
	h, err := hash.HashPassword("s3cret", 4)
	require.NoError(t, err)
	require.NotEqual(t, "s3cret", h)
	require.True(t, hash.CheckPassword(h, "s3cret"))
	require.False(t, hash.CheckPassword(h, "wrong"))
}

func TestHashCodeRoundtrip(t *testing.T) {
	h, err := hash.HashCode("123456", 4)
	require.NoError(t, err)
	require.NotEqual(t, "123456", h)
	require.True(t, hash.CheckCode(h, "123456"))
	require.False(t, hash.CheckCode(h, "654321"))
}

func TestGenerateNumericCode(t *testing.T) {
	re := regexp.MustCompile(`^[0-9]{6}$`)
	seen := map[string]int{}
	for i := 0; i < 50; i++ {
		c, err := hash.GenerateNumericCode(6)
		require.NoError(t, err)
		require.Len(t, c, 6)
		require.True(t, re.MatchString(c), "code %q must be exactly 6 digits", c)
		seen[c]++
	}
	require.Greater(t, len(seen), 1, "codes should vary across calls")
}

func TestHashRefreshTokenDeterministic(t *testing.T) {
	got := hash.HashRefreshToken("token-abc")
	want := sha256.Sum256([]byte("token-abc"))
	require.Equal(t, hex.EncodeToString(want[:]), got)
	require.Equal(t, got, hash.HashRefreshToken("token-abc"))
	require.NotEqual(t, got, hash.HashRefreshToken("token-xyz"))
	require.Len(t, got, 64)
}

func TestConstantTimeEqualHex(t *testing.T) {
	a := hash.HashRefreshToken("same")
	require.True(t, hash.ConstantTimeEqualHex(a, a))
	require.False(t, hash.ConstantTimeEqualHex(a, hash.HashRefreshToken("diff")))
	require.False(t, hash.ConstantTimeEqualHex(a, "short"))
}
