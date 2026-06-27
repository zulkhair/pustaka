package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/zulkhair/pustaka/backend/internal/config"
	"github.com/zulkhair/pustaka/backend/internal/pkg/hash"
)

func main() {
	if err := run(); err != nil {
		slog.Error("seed failed", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	username := getDefault("ADMIN_USERNAME", "admin")
	email := getDefault("ADMIN_EMAIL", "admin@pustaka.local")
	password := getDefault("ADMIN_PASSWORD", "admin123")

	ph, err := hash.HashPassword(password, cfg.BcryptCost)
	if err != nil {
		return err
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Idempotent upsert keyed on username.
	_, err = pool.Exec(ctx, `
		INSERT INTO web_user (id, username, email, password_hash, role, email_verified)
		VALUES ($1, $2, $3, $4, 'admin', true)
		ON CONFLICT (username) DO UPDATE
		SET email = EXCLUDED.email,
		    password_hash = EXCLUDED.password_hash,
		    role = 'admin',
		    email_verified = true
	`, uuid.NewString(), username, email, ph)
	if err != nil {
		return err
	}
	slog.Info("seeded admin", "username", username, "email", email)
	return nil
}

func getDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
