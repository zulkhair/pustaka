package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func setRequired(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://u:p@127.0.0.1:5434/db?sslmode=disable")
	t.Setenv("JWT_SECRET", "secret")
	t.Setenv("RESEND_API_KEY", "re_test")
	t.Setenv("MAIL_FROM", "Pustaka <no-reply@example.com>")
}

func TestLoadDefaults(t *testing.T) {
	setRequired(t)
	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "dev", cfg.AppEnv)
	require.Equal(t, ":8002", cfg.HTTPAddr)
	require.Equal(t, 15*time.Minute, cfg.AccessTTL)
	require.Equal(t, 720*time.Hour, cfg.RefreshTTL)
	require.Equal(t, 12, cfg.BcryptCost)
	require.Equal(t, 15*time.Minute, cfg.CodeTTL)
	require.Equal(t, 5, cfg.MaxAttempts)
	require.Equal(t, 60*time.Second, cfg.ResendCooldown)
}

func TestLoadOverrides(t *testing.T) {
	setRequired(t)
	t.Setenv("APP_ENV", "prod")
	t.Setenv("HTTP_ADDR", ":9000")
	t.Setenv("ACCESS_TTL", "5m")
	t.Setenv("REFRESH_TTL", "240h")
	t.Setenv("BCRYPT_COST", "10")
	t.Setenv("VERIFICATION_CODE_TTL", "10m")
	t.Setenv("VERIFICATION_MAX_ATTEMPTS", "3")
	t.Setenv("RESEND_COOLDOWN", "30s")
	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "prod", cfg.AppEnv)
	require.Equal(t, ":9000", cfg.HTTPAddr)
	require.Equal(t, 5*time.Minute, cfg.AccessTTL)
	require.Equal(t, 240*time.Hour, cfg.RefreshTTL)
	require.Equal(t, 10, cfg.BcryptCost)
	require.Equal(t, 10*time.Minute, cfg.CodeTTL)
	require.Equal(t, 3, cfg.MaxAttempts)
	require.Equal(t, 30*time.Second, cfg.ResendCooldown)
}

func TestLoadMissingRequired(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("RESEND_API_KEY", "")
	t.Setenv("MAIL_FROM", "")
	_, err := Load()
	require.Error(t, err)
}

func TestLoadBadDuration(t *testing.T) {
	setRequired(t)
	t.Setenv("ACCESS_TTL", "not-a-duration")
	_, err := Load()
	require.Error(t, err)
}
