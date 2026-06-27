package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zulkhair/pustaka/backend/internal/adapter/httpapi"
	"github.com/zulkhair/pustaka/backend/internal/adapter/mail"
	"github.com/zulkhair/pustaka/backend/internal/adapter/store"
	"github.com/zulkhair/pustaka/backend/internal/app/auth"
	"github.com/zulkhair/pustaka/backend/internal/config"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := run(); err != nil {
		slog.Error("server exited with error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if cfg.AppEnv != "prod" {
		if err := store.RunMigrations(cfg.DatabaseURL); err != nil {
			return err
		}
		slog.Info("migrations applied")
	} else {
		slog.Info("APP_ENV=prod: skipping auto-migrate")
	}

	ctx := context.Background()
	pool, err := store.OpenPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	st := store.New(pool)
	mailer := mail.NewResendMailer(cfg)
	svc := auth.New(st, mailer, cfg)

	app := httpapi.BuildApp(httpapi.RouterDeps{
		Auth:      httpapi.NewAuthHandler(svc),
		Pinger:    pool,
		JWTSecret: cfg.JWTSecret,
	})

	errCh := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", cfg.HTTPAddr)
		errCh <- app.Listen(cfg.HTTPAddr)
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	case sig := <-stop:
		slog.Info("shutdown signal received", "signal", sig.String())
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := app.ShutdownWithContext(shutdownCtx); err != nil {
			return err
		}
		slog.Info("server stopped cleanly")
		return nil
	}
}
