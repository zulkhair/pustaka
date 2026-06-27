package hash

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"math/big"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword bcrypt-hashes a password at the given cost.
func HashPassword(pw string, cost int) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), cost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CheckPassword reports whether pw matches the bcrypt hash.
func CheckPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

// HashCode bcrypt-hashes a verification code. Codes are low-entropy, so bcrypt
// (not a fast hash) is used to resist offline brute force.
func HashCode(code string, cost int) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(code), cost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CheckCode reports whether code matches the bcrypt hash.
func CheckCode(hash, code string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(code)) == nil
}

// GenerateNumericCode returns an n-digit, zero-padded code from a CSPRNG.
func GenerateNumericCode(n int) (string, error) {
	var sb strings.Builder
	sb.Grow(n)
	for i := 0; i < n; i++ {
		d, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		sb.WriteByte(byte('0' + d.Int64()))
	}
	return sb.String(), nil
}

// HashRefreshToken returns the SHA-256 hex of a high-entropy refresh token.
func HashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// ConstantTimeEqualHex compares two hex strings in constant time.
func ConstantTimeEqualHex(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
