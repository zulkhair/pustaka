package store

import (
	"context"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

//go:embed db/migrations/*.sql
var migrationFS embed.FS

// OpenPool opens a pgx connection pool and verifies connectivity (fail-fast).
func OpenPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("store: open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: initial ping: %w", err)
	}
	return pool, nil
}

// RunMigrations applies all up migrations embedded under db/migrations. It is
// CWD-independent (//go:embed) and uses the postgres database driver. The
// prod-skip decision is the caller's responsibility (main.go).
func RunMigrations(databaseURL string) error {
	src, err := iofs.New(migrationFS, "db/migrations")
	if err != nil {
		return fmt.Errorf("store: open embedded migrations: %w", err)
	}

	connCfg, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return fmt.Errorf("store: parse database url: %w", err)
	}
	db := stdlib.OpenDB(*connCfg)
	defer func() { _ = db.Close() }()

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("store: build postgres migrate driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		return fmt.Errorf("store: init migrate: %w", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("store: run migrations: %w", err)
	}
	return nil
}
