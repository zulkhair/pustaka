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

func TestLoadDocumentDefaults(t *testing.T) {
	setRequired(t)
	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "glm-ocr", cfg.OCRModel)
	require.Equal(t, "qwen2.5:14b-instruct", cfg.TransformModel)
	require.Equal(t, "0.0.0", cfg.AppVersion)
	require.Equal(t, "", cfg.BlobDir)
	require.Equal(t, "", cfg.OllamaHost)
	require.Equal(t, "", cfg.APKPath)
}

func TestLoadDocumentOverrides(t *testing.T) {
	setRequired(t)
	t.Setenv("BLOB_DIR", "/data/blobs")
	t.Setenv("OLLAMA_HOST", "http://100.65.255.51:11434")
	t.Setenv("OCR_MODEL", "glm-ocr:latest")
	t.Setenv("TRANSFORM_MODEL", "qwen2.5:7b")
	t.Setenv("APP_VERSION", "1.2.3")
	t.Setenv("APK_PATH", "/srv/pustaka.apk")
	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "/data/blobs", cfg.BlobDir)
	require.Equal(t, "http://100.65.255.51:11434", cfg.OllamaHost)
	require.Equal(t, "glm-ocr:latest", cfg.OCRModel)
	require.Equal(t, "qwen2.5:7b", cfg.TransformModel)
	require.Equal(t, "1.2.3", cfg.AppVersion)
	require.Equal(t, "/srv/pustaka.apk", cfg.APKPath)
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
