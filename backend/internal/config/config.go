package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppEnv         string
	HTTPAddr       string
	DatabaseURL    string
	JWTSecret      string
	AccessTTL      time.Duration
	RefreshTTL     time.Duration
	BcryptCost     int
	ResendAPIKey   string
	MailFrom       string
	CodeTTL        time.Duration
	MaxAttempts    int
	ResendCooldown time.Duration
	BlobDir        string
	OllamaHost     string
	OCRModel       string
	TransformModel string
	AppVersion     string
	APKPath        string
}

func Load() (Config, error) {
	var cfg Config
	var err error

	cfg.AppEnv = getDefault("APP_ENV", "dev")
	cfg.HTTPAddr = getDefault("HTTP_ADDR", ":8002")

	if cfg.DatabaseURL, err = required("DATABASE_URL"); err != nil {
		return Config{}, err
	}
	if cfg.JWTSecret, err = required("JWT_SECRET"); err != nil {
		return Config{}, err
	}
	if cfg.ResendAPIKey, err = required("RESEND_API_KEY"); err != nil {
		return Config{}, err
	}
	if cfg.MailFrom, err = required("MAIL_FROM"); err != nil {
		return Config{}, err
	}

	if cfg.AccessTTL, err = durationDefault("ACCESS_TTL", 15*time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.RefreshTTL, err = durationDefault("REFRESH_TTL", 720*time.Hour); err != nil {
		return Config{}, err
	}
	if cfg.CodeTTL, err = durationDefault("VERIFICATION_CODE_TTL", 15*time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.ResendCooldown, err = durationDefault("RESEND_COOLDOWN", 60*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.BcryptCost, err = intDefault("BCRYPT_COST", 12); err != nil {
		return Config{}, err
	}
	if cfg.MaxAttempts, err = intDefault("VERIFICATION_MAX_ATTEMPTS", 5); err != nil {
		return Config{}, err
	}

	cfg.BlobDir = os.Getenv("BLOB_DIR")
	cfg.OllamaHost = os.Getenv("OLLAMA_HOST")
	cfg.APKPath = os.Getenv("APK_PATH")
	cfg.OCRModel = getDefault("OCR_MODEL", "glm-ocr")
	cfg.TransformModel = getDefault("TRANSFORM_MODEL", "qwen2.5:14b-instruct")
	cfg.AppVersion = getDefault("APP_VERSION", "0.0.0")

	return cfg, nil
}

func getDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func required(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("config: required env %s is missing", key)
	}
	return v, nil
}

func durationDefault(key string, def time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return def, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("config: %s is not a valid duration: %w", key, err)
	}
	return d, nil
}

func intDefault(key string, def int) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("config: %s is not a valid int: %w", key, err)
	}
	return n, nil
}
