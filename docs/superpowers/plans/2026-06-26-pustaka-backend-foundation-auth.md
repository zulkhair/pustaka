# Pustaka Backend — Plan 1: Foundation, Auth & Security — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> The 23 adversarial-review corrections have been **applied inline** to the 22 tasks below, and the plan was re-verified for consistency. Implement task-by-task.

**Goal:** Stand up the Pustaka Go backend with an email-verified, multi-user, hardened auth API (register → verify-email → login → refresh/logout, plus `/auth/me` and `/health`), on a ports-&-adapters foundation that is fully testable without a GPU or real email.

**Architecture:** Hexagonal-lite Fiber service. Use-cases (`internal/app/*`) depend only on `internal/domain` ports (`Store`, `Mailer`); Postgres (pgx + sqlc) and Resend are adapters. Auth = short-lived access **JWT** + opaque **rotating refresh tokens** (256-bit random, SHA-256-hashed in `session`, revocable). Email verification = 6-digit **bcrypt-hashed, single-use** codes with expiry + attempt cap, sent through the swappable `Mailer` port.

**Tech Stack:** Go 1.26, Fiber v2, pgx v5, sqlc, golang-migrate (iofs + `go:embed`), golang-jwt/jwt v5, bcrypt (`golang.org/x/crypto`), google/uuid, slog, testcontainers-go + testify, Resend (HTTP).

**Plan set:** This is **Plan 1 of 4**. Plan 2 = documents/capture/OCR/transform; Plan 3 = sharing; Plan 4 = Flutter mobile. Each ships working, tested software.

## Global Constraints

- **Module:** `github.com/zulkhair/pustaka/backend`; **Go 1.26** (`.prototools`: `go = "1.26.0"`).
- **Ports & adapters:** `internal/app/*` and `internal/domain` never import adapters; adapters depend inward only.
- **Security (non-negotiable):** bcrypt (cost = `cfg.BcryptCost`, default 12) for passwords **and** verification codes; codes 6-digit CSPRNG, single-use, expiring, **atomic** attempt-cap; refresh tokens 256-bit random stored SHA-256-hex, rotated on use, revocable, reuse-after-rotation rejected **and triggers full session revocation**; access JWT ~15m / refresh ~30d; **unverified users cannot obtain a session**; **all** `/auth/*` routes rate-limited; **enumeration-resistant** generic responses on register/resend (and login for unknown/bad-password); constant-time comparisons; secrets only from env.
- **DB:** Postgres 16; UUIDv4 string PKs (`VARCHAR(36)`), `TIMESTAMPTZ`; migrations auto-run on startup unless `APP_ENV=prod`.
- **Response envelope:** every endpoint returns `{status, message, data}` (`status` 0=ok / 1=error) via `httpapi.OK` / `httpapi.Fail`.
- **Commits:** Conventional Commits, imperative mood, no trailing period. **Never add a `Co-Authored-By` trailer.**
- **Gates:** `go vet ./...` clean and `go test ./...` green (Postgres via testcontainers; `Mailer` mocked — no real email/GPU).
- **Local ports:** Postgres `127.0.0.1:5434`, API `127.0.0.1:8002` (avoid hotel 5432/8000, pasar 5433/8001, invoice 8000).

## File Structure

```
backend/
  go.mod  .prototools  .gitignore  Makefile  docker-compose.yml  .env.example  sqlc.yaml
  cmd/server/main.go                              # composition root
  cmd/seed/main.go                                # idempotent admin seed (bcrypt at runtime)
  internal/
    config/config.go
    domain/  user.go  verification.go  session.go  ports.go  errors.go
    app/auth/service.go
    adapter/
      httpapi/  response.go  router.go  auth_handler.go  health_handler.go
                middleware/auth.go  middleware/ratelimit.go  harness_test.go
      store/store.go  store/migrate.go  store/sqlc/...
      mail/resend.go  mail/mock.go
    testsupport/testsupport.go
    pkg/hash/hash.go  pkg/jwt/jwt.go
  db/  migrations/000001_init.up.sql  000001_init.down.sql
       queries/user.sql  queries/verification.sql  queries/session.sql
  .github/workflows/ci.yml
```

## Interface Contract (authoritative — names/signatures used by every task)

> Reflects the **corrected** canonical names. Where a drafted task below uses an older name, rename per this contract + the Required corrections.

- **config:** `Config{AppEnv,HTTPAddr,DatabaseURL,JWTSecret string; AccessTTL,RefreshTTL time.Duration; BcryptCost int; ResendAPIKey,MailFrom string; CodeTTL time.Duration; MaxAttempts int; ResendCooldown time.Duration}`; `Load() (Config, error)`. Defaults: HTTPAddr `:8002`, AccessTTL 15m, RefreshTTL 720h, BcryptCost 12, CodeTTL 15m, MaxAttempts 5, ResendCooldown 60s. Required: DATABASE_URL, JWT_SECRET, RESEND_API_KEY, MAIL_FROM.
- **DB (migration 000001):** `web_user(id PK, username UNIQUE, email UNIQUE, password_hash, role CHECK admin|user DEFAULT user, email_verified DEFAULT false, created_at)`; `email_verification(id PK, user_id FK CASCADE, code_hash, expires_at, attempts DEFAULT 0, consumed_at NULL, created_at)`; `session(id PK, user_id FK CASCADE, refresh_token_hash VARCHAR(64) UNIQUE, expires_at, created_at, revoked_at NULL)`; indexes on `user_id` (both children) + `refresh_token_hash`. **`sqlc.yaml` is created exactly once** (in the migration task) with `emit_pointers_for_null_types: true`.
- **pkg/hash:** `HashPassword(pw string, cost int)(string,error)`; `CheckPassword(hash,pw string) bool`; `HashCode(code string, cost int)(string,error)`; `CheckCode(hash,code string) bool`; `GenerateNumericCode(n int)(string,error)`; `HashRefreshToken(raw string) string`; `ConstantTimeEqualHex(a,b string) bool`.
- **pkg/jwt:** `Claims{UserID,Role; jwt.RegisteredClaims}`; `GenerateAccess(userID,role,secret string, ttl time.Duration)(string,error)`; `ParseAccess(token,secret string)(*Claims,error)`; `GenerateRefreshToken()(string,error)` (32 bytes base64url, opaque).
- **domain:** `RoleAdmin="admin"`, `RoleUser="user"`; `User`, `EmailVerification`, `Session` entities; `CreateUserParams`/`CreateEmailVerificationParams`/`CreateSessionParams`; `Mailer{SendVerificationCode(ctx,toEmail,code) error}`; `Store{ExecTx; CreateUser; GetUserByEmail/Username/ID; SetUserEmailVerified; CreateEmailVerification; GetActiveEmailVerification; IncrementVerificationAttempts(ctx,id)(int,error) [atomic]; ConsumeEmailVerification; DeleteEmailVerificationsByUser; CreateSession; GetSessionByTokenHash; RevokeSession; RevokeAllUserSessions}`. Errors: `ErrNotFound, ErrConflict, ErrInvalidCredentials, ErrEmailNotVerified, ErrInvalidCode, ErrCodeExpired, ErrTooManyAttempts, ErrValidation, ErrUnauthorized, ErrForbidden` (`ErrResendCooldown` is internal-only, never surfaced).
- **store adapter:** `OpenPool(ctx,databaseURL)(*pgxpool.Pool,error)` (pings); `RunMigrations(databaseURL string) error` (uses `//go:embed db/migrations/*.sql` + golang-migrate iofs + `postgres` driver; CWD-independent); `Store` wraps `*sqlc.Queries` + pool, `store.go` imports `github.com/jackc/pgx/v5/pgtype`.
- **mail adapter:** `NewResendMailer(cfg config.Config) *ResendMailer`; `MockMailer{LastEmail,LastCode string; Sends []MockSend}` + `NewMockMailer()` + `CodeFor(email)(string,bool)`.
- **app/auth:** `New(store,mailer,cfg) *Service`; `Register/VerifyEmail/ResendVerification/Login/Refresh/Logout/Me`; private `issueTokens(ctx, u domain.User)(Tokens,error)` defined **once** (in the VerifyEmail task), called by VerifyEmail/Login/Refresh. `RegisterInput`, `VerifyInput`, `LoginInput{Identifier,Password}`, `Tokens{AccessToken,RefreshToken string; ExpiresIn int}`.
- **httpapi:** `OK(c,data)`/`Fail(c,httpCode,msg)`; `mapAuthError(c,err)` defined **once** (in the Register task) per the error→HTTP table (Conflict→409; InvalidCredentials/EmailNotVerified/Unauthorized→401; Forbidden→403; NotFound→404; InvalidCode/CodeExpired/Validation→400; TooManyAttempts→429; default→500); handler type **`AuthHandler`** + `NewAuthHandler(svc) *AuthHandler`, methods on `*AuthHandler`; DTOs incl. `MeDTO{id,username,email,role,emailVerified}`. `BuildApp` sets `fiber.Config{ProxyHeader: fiber.HeaderXForwardedFor, EnableTrustedProxyCheck:true, TrustedProxies:["127.0.0.1","::1"]}`. Middleware `RequireAuth(secret)`, `RequireAdmin()`, `RateLimit(max,window)`.
- **Routes (`/api`):** `POST /auth/{register,verify-email,resend-verification,login,refresh,logout}` (each RateLimit'd); `GET /auth/me` (RequireAuth); `GET /health`.

---

---

# Tasks

## Cluster A — Foundation & infra (Tasks 1–4)

### Task 1: Repo scaffold

**Files:**
- Create: `backend/go.mod`
- Create: `backend/.prototools`
- Create: `backend/.gitignore`
- Create: `backend/Makefile`
- Create: `backend/docker-compose.yml`
- Create: `backend/.env.example`
- Create: `backend/cmd/server/main.go`
- Test: build smoke via `go build ./...` (no Go test file; the compile + healthy DB is the gate)

**Interfaces:**
- Consumes: nothing (this is the root task).
- Produces: the module `github.com/zulkhair/pustaka/backend` (so every later import path resolves), a compiling `cmd/server/main.go` entrypoint, and a healthy Postgres on `127.0.0.1:5434` for Task 3's testcontainers parity and local `make run`. `sqlc.yaml` is NOT created here — it is created once in the migration task (Task 5).

- [ ] **Step 1: Create the module skeleton and pin Go.** Create `backend/go.mod`:
  ```go
  module github.com/zulkhair/pustaka/backend

  go 1.26
  ```
  Create `backend/.prototools`:
  ```toml
  [tools]
  go = "1.26.0"
  ```

- [ ] **Step 2: Create `.gitignore`.** Create `backend/.gitignore`:
  ```gitignore
  .env
  /bin
  CLAUDE.md
  ```

- [ ] **Step 3: Write a trivial composition root that compiles.** Create `backend/cmd/server/main.go`:
  ```go
  package main

  import (
  	"log/slog"
  	"os"
  )

  func main() {
  	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
  	slog.SetDefault(logger)
  	slog.Info("pustaka backend booting")
  }
  ```

- [ ] **Step 4: Run the build smoke and state expected result.** Run `cd backend && go build ./...`. EXPECT: PASS (exit 0, no output) — the single `main` package compiles with the pinned toolchain.

- [ ] **Step 5: Create the Postgres compose service.** Create `backend/docker-compose.yml`:
  ```yaml
  services:
    db:
      image: postgres:16-alpine
      restart: unless-stopped
      environment:
        POSTGRES_USER: pustaka
        POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-pustaka}
        POSTGRES_DB: pustaka
      ports:
        - "127.0.0.1:5434:5432"
      volumes:
        - pustaka_pgdata:/var/lib/postgresql/data
      healthcheck:
        test: ["CMD-SHELL", "pg_isready -U pustaka -d pustaka"]
        interval: 5s
        timeout: 5s
        retries: 10

  volumes:
    pustaka_pgdata:
  ```

- [ ] **Step 6: Create `.env.example` listing every Config key.** Create `backend/.env.example` (the comments double as run notes since no README is created — house rule):
  ```dotenv
  # Server
  APP_ENV=dev
  HTTP_ADDR=:8002

  # Database (local compose: 127.0.0.1:5434)
  DATABASE_URL=postgres://pustaka:pustaka@127.0.0.1:5434/pustaka?sslmode=disable
  POSTGRES_PASSWORD=pustaka

  # Auth
  JWT_SECRET=change-me-in-prod
  ACCESS_TTL=15m
  REFRESH_TTL=720h
  BCRYPT_COST=12

  # Email (Resend)
  RESEND_API_KEY=re_xxxxxxxx
  MAIL_FROM=Pustaka <no-reply@example.com>

  # Email verification
  VERIFICATION_CODE_TTL=15m
  VERIFICATION_MAX_ATTEMPTS=5
  RESEND_COOLDOWN=60s

  # Seed admin (consumed by `make seed` / cmd/seed)
  ADMIN_USERNAME=admin
  ADMIN_EMAIL=admin@pustaka.local
  ADMIN_PASSWORD=admin123
  ```

- [ ] **Step 7: Create the Makefile.** Create `backend/Makefile`. The `help` target prints run notes (no README is created — house rule):
  ```makefile
  .PHONY: help run test vet lint sqlc migrate seed db-up db-down

  help: ## Show run notes and available targets
  	@echo "Pustaka backend — local dev"
  	@echo "  1. make db-up      # start Postgres (binds 127.0.0.1:5434)"
  	@echo "  2. cp .env.example .env  # set DATABASE_URL, JWT_SECRET, RESEND_API_KEY, MAIL_FROM"
  	@echo "  3. make run        # auto-migrates unless APP_ENV=prod; listens on :8002"
  	@echo "  4. make seed       # idempotently upsert the admin account"

  run:
  	go run ./cmd/server

  test:
  	go test ./...

  vet:
  	go vet ./...

  lint: vet

  db-up:
  	docker compose up -d db

  db-down:
  	docker compose down
  ```

- [ ] **Step 8: Bring Postgres up and confirm healthy.** Run `cd backend && docker compose up -d db`, then poll `docker compose ps db` (or `docker inspect --format '{{.State.Health.Status}}' $(docker compose ps -q db)`) until it reports `healthy`. EXPECT: container reaches `healthy` within ~30s. Leave it running for Task 3.

- [ ] **Step 9: Commit.** `git add backend && git commit -m "chore: scaffold pustaka backend module and compose"`. (No `Co-Authored-By` trailer.)

---

### Task 2: Config loader — `internal/config`

**Files:**
- Create: `backend/internal/config/config.go`
- Test: `backend/internal/config/config_test.go`

**Interfaces:**
- Consumes: the `github.com/zulkhair/pustaka/backend` module from Task 1.
- Produces (later tasks rely on these VERBATIM):
  ```go
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
  }
  func Load() (Config, error)
  ```

- [ ] **Step 1: Write the failing test.** Create `backend/internal/config/config_test.go`:
  ```go
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
  ```

- [ ] **Step 2: Run the test and state expected FAIL.** Run `cd backend && go get github.com/stretchr/testify@latest && go test ./internal/config/...`. EXPECT: FAIL to compile — `config.go` does not exist yet, so `Config`/`Load` are undefined.

- [ ] **Step 3: Write the minimal implementation.** Create `backend/internal/config/config.go`:
  ```go
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
  ```

- [ ] **Step 4: Run the test and state expected PASS.** Run `cd backend && go mod tidy && go test ./internal/config/...`. EXPECT: PASS — defaults applied, overrides parsed (durations + ints), missing-required and bad-duration both return errors.

- [ ] **Step 5: Vet and commit.** Run `cd backend && go vet ./...` (EXPECT: clean), then `git add backend && git commit -m "feat: add env-driven config loader"`. (No `Co-Authored-By` trailer.)

---

### Task 3: DB layer — pool, migrate runner, ping (`internal/adapter/store`)

**Files:**
- Create: `backend/internal/adapter/store/db.go`
- Test: `backend/internal/adapter/store/db_test.go`

**Interfaces:**
- Consumes: `config.Config` from Task 2 (`cfg.DatabaseURL`, `cfg.AppEnv`).
- Produces (later tasks — store wrapper in Task 6 and `main.go` in Task 22 rely on these VERBATIM):
  ```go
  func OpenPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) // pings; fail-fast
  func RunMigrations(databaseURL string) error // //go:embed db/migrations/*.sql + golang-migrate iofs; CWD-independent
  ```
  `RunMigrations` embeds the real migration FILES under `backend/db/migrations` (authored in Task 5) via `//go:embed`, so it is CWD-independent and needs no migrations-dir argument. Prod-skip is the caller's job (`main.go`: `if cfg.AppEnv != "prod"`). This task creates the embed directory placeholder so the build compiles before Task 5 fills it in.

- [ ] **Step 1: Create the embed target directory and a placeholder migration so `//go:embed` compiles.** Create `backend/db/migrations/.gitkeep` (empty) and a throwaway smoke migration this task tests against, replaced by the real init migration in Task 5:
  - Create `backend/db/migrations/000001_smoke.up.sql`:
    ```sql
    CREATE TABLE smoke (id INT PRIMARY KEY);
    ```
  - Create `backend/db/migrations/000001_smoke.down.sql`:
    ```sql
    DROP TABLE smoke;
    ```

- [ ] **Step 2: Write the failing test.** Create `backend/internal/adapter/store/db_test.go`:
  ```go
  package store

  import (
  	"context"
  	"testing"
  	"time"

  	"github.com/stretchr/testify/require"
  	"github.com/testcontainers/testcontainers-go"
  	"github.com/testcontainers/testcontainers-go/modules/postgres"
  	"github.com/testcontainers/testcontainers-go/wait"
  )

  func startPostgres(t *testing.T) string {
  	t.Helper()
  	ctx := context.Background()
  	container, err := postgres.Run(ctx,
  		"postgres:16-alpine",
  		postgres.WithDatabase("pustaka"),
  		postgres.WithUsername("pustaka"),
  		postgres.WithPassword("pustaka"),
  		testcontainers.WithWaitStrategy(
  			wait.ForListeningPort("5432/tcp").WithStartupTimeout(60*time.Second),
  		),
  	)
  	require.NoError(t, err)
  	t.Cleanup(func() { _ = container.Terminate(ctx) })

  	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
  	require.NoError(t, err)
  	return dsn
  }

  func TestOpenPoolPings(t *testing.T) {
  	dsn := startPostgres(t)
  	ctx := context.Background()
  	pool, err := OpenPool(ctx, dsn)
  	require.NoError(t, err)
  	t.Cleanup(pool.Close)
  	require.NoError(t, pool.Ping(ctx))
  }

  func TestRunMigrationsAppliesEmbeddedFiles(t *testing.T) {
  	dsn := startPostgres(t)
  	ctx := context.Background()

  	require.NoError(t, RunMigrations(dsn))

  	pool, err := OpenPool(ctx, dsn)
  	require.NoError(t, err)
  	t.Cleanup(pool.Close)

  	var exists bool
  	err = pool.QueryRow(ctx,
  		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'smoke')`,
  	).Scan(&exists)
  	require.NoError(t, err)
  	require.True(t, exists, "smoke table should exist after RunMigrations")
  }
  ```

- [ ] **Step 3: Run the test and state expected FAIL.** Run `cd backend && go test ./internal/adapter/store/...`. EXPECT: FAIL to compile — `db.go` does not exist, so `OpenPool`/`RunMigrations` are undefined (and the testcontainers/pgx/golang-migrate deps are not yet required).

- [ ] **Step 4: Write the minimal implementation.** Create `backend/internal/adapter/store/db.go`. It embeds `db/migrations/*.sql` relative to this file and drives golang-migrate via the `iofs` source + the `postgres` database driver (NOT `pgx5://`), so it is CWD-independent:
  ```go
  package store

  import (
  	"context"
  	"embed"
  	"errors"
  	"fmt"

  	"github.com/golang-migrate/migrate/v4"
  	"github.com/golang-migrate/migrate/v4/database/postgres"
  	"github.com/golang-migrate/migrate/v4/source/iofs"
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

  	db, err := stdlib.OpenDB(*mustParseConfig(databaseURL))
  	if err != nil {
  		return fmt.Errorf("store: open sql db for migrate: %w", err)
  	}
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

  func mustParseConfig(databaseURL string) *stdlib.OptionOpenDB {
  	_ = stdlib.OptionOpenDB{}
  	_ = databaseURL
  	return nil
  }
  ```
  Note: the placeholder `mustParseConfig` helper above is illustrative only — replace it with the direct `stdlib.OpenDB` form using a parsed `pgx.ConnConfig`. The concrete, compilable body is:
  ```go
  // RunMigrations applies all up migrations embedded under db/migrations.
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
  ```
  Use this concrete body. The import block becomes:
  ```go
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
  ```

- [ ] **Step 5: Run the test and state expected PASS.** Run `cd backend && go get github.com/jackc/pgx/v5 github.com/jackc/pgx/v5/pgxpool github.com/jackc/pgx/v5/stdlib github.com/golang-migrate/migrate/v4 github.com/testcontainers/testcontainers-go github.com/testcontainers/testcontainers-go/modules/postgres && go mod tidy && go test ./internal/adapter/store/...`. EXPECT: PASS — `OpenPool` connects + pings against testcontainers Postgres; `RunMigrations` creates the `smoke` table from the embedded files. (Requires Docker; the Task 1 compose daemon is sufficient.)

- [ ] **Step 6: Vet and commit.** Run `cd backend && go vet ./...` (EXPECT: clean), then `git add backend && git commit -m "feat: add pgx pool and embedded migration runner"`. (No `Co-Authored-By` trailer.)

---

### Task 4: CI workflow + lint target

**Files:**
- Create: `backend/.github/workflows/ci.yml`
- Modify: `backend/Makefile` (confirm `lint` target aliases `go vet` — already added in Task 1; verify, no change needed if present)

**Interfaces:**
- Consumes: the `backend/go.mod` from Task 1 (CI reads the Go version via `go-version-file`), and the `vet`/`test` Make targets.
- Produces: a green pipeline gate (`go vet ./...` then `go test ./...`) other clusters' work runs against. No GPU and no real email in CI — Postgres comes from testcontainers (Docker is available on `ubuntu-latest`), the `Mailer` is mocked in app-layer tests.

- [ ] **Step 1: Write the workflow.** Create `backend/.github/workflows/ci.yml`:
  ```yaml
  name: backend-ci

  on:
    push:
      branches: ["**"]
    pull_request:

  jobs:
    test:
      runs-on: ubuntu-latest
      defaults:
        run:
          working-directory: backend
      steps:
        - uses: actions/checkout@v4

        - name: Set up Go
          uses: actions/setup-go@v5
          with:
            go-version-file: backend/go.mod
            cache-dependency-path: backend/go.sum

        - name: Download modules
          run: go mod download

        - name: Vet
          run: go vet ./...

        - name: Test
          run: go test ./...
  ```

- [ ] **Step 2: Verify the `lint` Make target.** Read `backend/Makefile` and confirm the `lint: vet` alias from Task 1 is present (so `make lint` runs `go vet ./...`). If it is missing, add `lint: vet` to the target list; otherwise make no change.

- [ ] **Step 3: Locally rehearse what CI runs and state expected result.** Run `cd backend && go mod download && go vet ./... && go test ./...`. EXPECT: PASS — vet is clean and all existing tests (config + store) are green; this is the exact sequence the workflow executes. (Docker must be available for the store testcontainers tests, same as CI's `ubuntu-latest`.)

- [ ] **Step 4: Commit.** `git add backend && git commit -m "ci: add backend vet and test workflow"`. (No `Co-Authored-By` trailer.)

---

## Cluster B — Persistence, crypto & domain (Tasks 5–10)

### Task 5: Migration `000001_init` + `sqlc.yaml`

**Files:**
- Delete: `backend/db/migrations/000001_smoke.up.sql`, `backend/db/migrations/000001_smoke.down.sql` (the Task 3 placeholders)
- Create: `backend/db/migrations/000001_init.up.sql`
- Create: `backend/db/migrations/000001_init.down.sql`
- Create: `backend/sqlc.yaml`
- Test: `backend/db/migrations/migrations_test.go`

**Interfaces:**
- Consumes: nothing from other tasks. Uses `testcontainers-go` (`github.com/testcontainers/testcontainers-go/modules/postgres`), `golang-migrate` (`github.com/golang-migrate/migrate/v4` with `database/postgres` + `source/file` drivers), `database/sql` + `jackc/pgx/v5/stdlib`, and `testify`.
- Produces: the three tables (`web_user`, `email_verification`, `session`) with the exact columns/constraints/indexes other tasks rely on, and `sqlc.yaml` (created ONCE here, with `emit_pointers_for_null_types: true`) that Task 6 uses to generate `internal/adapter/store/sqlc`.

- [ ] **Step 1: Remove the Task 3 smoke placeholders.** Delete `backend/db/migrations/000001_smoke.up.sql` and `backend/db/migrations/000001_smoke.down.sql` so the embedded migration set contains only the real init migration.

- [ ] **Step 2: Write the failing migration test.** It spins up a throwaway Postgres, runs the migrations up, asserts the three tables + key columns + the role CHECK + uniqueness exist via `information_schema`, then runs down and asserts none remain. Create `backend/db/migrations/migrations_test.go`:
  ```go
  package migrations_test

  import (
  	"context"
  	"database/sql"
  	"testing"
  	"time"

  	_ "github.com/jackc/pgx/v5/stdlib"

  	"github.com/golang-migrate/migrate/v4"
  	_ "github.com/golang-migrate/migrate/v4/database/postgres"
  	_ "github.com/golang-migrate/migrate/v4/source/file"

  	"github.com/stretchr/testify/require"
  	"github.com/testcontainers/testcontainers-go"
  	"github.com/testcontainers/testcontainers-go/modules/postgres"
  	"github.com/testcontainers/testcontainers-go/wait"
  )

  func startPostgres(t *testing.T) (dsn string, stdlibDSN string) {
  	t.Helper()
  	ctx := context.Background()
  	ctr, err := postgres.Run(ctx,
  		"postgres:16-alpine",
  		postgres.WithDatabase("pustaka"),
  		postgres.WithUsername("pustaka"),
  		postgres.WithPassword("pustaka"),
  		testcontainers.WithWaitStrategy(
  			wait.ForListeningPort("5432/tcp").WithStartupTimeout(60*time.Second)),
  	)
  	require.NoError(t, err)
  	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

  	conn, err := ctr.ConnectionString(ctx, "sslmode=disable")
  	require.NoError(t, err)
  	return "pgx5://" + conn[len("postgres://"):], conn
  }

  func TestMigrationsUpDown(t *testing.T) {
  	migrateDSN, sqlDSN := startPostgres(t)

  	m, err := migrate.New("file://.", migrateDSN)
  	require.NoError(t, err)
  	require.NoError(t, m.Up())

  	db, err := sql.Open("pgx", sqlDSN)
  	require.NoError(t, err)
  	t.Cleanup(func() { _ = db.Close() })

  	for _, tbl := range []string{"web_user", "email_verification", "session"} {
  		var got string
  		err := db.QueryRow(
  			`SELECT table_name FROM information_schema.tables WHERE table_name = $1`, tbl,
  		).Scan(&got)
  		require.NoError(t, err, "table %s should exist", tbl)
  		require.Equal(t, tbl, got)
  	}

  	cols := map[string][]string{
  		"web_user":           {"id", "username", "email", "password_hash", "role", "email_verified", "created_at"},
  		"email_verification": {"id", "user_id", "code_hash", "expires_at", "attempts", "consumed_at", "created_at"},
  		"session":            {"id", "user_id", "refresh_token_hash", "expires_at", "created_at", "revoked_at"},
  	}
  	for tbl, want := range cols {
  		for _, col := range want {
  			var n int
  			err := db.QueryRow(
  				`SELECT count(*) FROM information_schema.columns WHERE table_name = $1 AND column_name = $2`,
  				tbl, col,
  			).Scan(&n)
  			require.NoError(t, err)
  			require.Equal(t, 1, n, "column %s.%s should exist", tbl, col)
  		}
  	}

  	var checkCount int
  	require.NoError(t, db.QueryRow(
  		`SELECT count(*) FROM information_schema.check_constraints
  		 WHERE constraint_schema = 'public' AND check_clause ILIKE '%role%'`,
  	).Scan(&checkCount))
  	require.GreaterOrEqual(t, checkCount, 1, "role CHECK constraint should exist")

  	for _, idx := range []string{"idx_email_verification_user", "idx_session_user", "idx_session_token"} {
  		var name string
  		require.NoError(t, db.QueryRow(
  			`SELECT indexname FROM pg_indexes WHERE indexname = $1`, idx,
  		).Scan(&name), "index %s should exist", idx)
  		require.Equal(t, idx, name)
  	}

  	require.NoError(t, m.Down())
  	for _, tbl := range []string{"web_user", "email_verification", "session"} {
  		var n int
  		require.NoError(t, db.QueryRow(
  			`SELECT count(*) FROM information_schema.tables WHERE table_name = $1`, tbl,
  		).Scan(&n))
  		require.Equal(t, 0, n, "table %s should be dropped after Down", tbl)
  	}
  }
  ```

- [ ] **Step 3: Run the test, expect FAIL.** Run `cd backend && go test ./db/migrations/...`. Expected FAIL: `go test` errors because `000001_init.up.sql`/`.down.sql` do not exist yet (`migrate.New` returns "no migration found" / file source error), so the test cannot run the schema.

- [ ] **Step 4: Write the up migration.** Create `backend/db/migrations/000001_init.up.sql` with the exact contract schema:
  ```sql
  CREATE TABLE web_user (
      id             VARCHAR(36)  PRIMARY KEY,
      username       VARCHAR(50)  UNIQUE NOT NULL,
      email          VARCHAR(255) UNIQUE NOT NULL,
      password_hash  VARCHAR(255) NOT NULL,
      role           VARCHAR(10)  NOT NULL DEFAULT 'user' CHECK (role IN ('admin', 'user')),
      email_verified BOOLEAN      NOT NULL DEFAULT false,
      created_at     TIMESTAMPTZ  NOT NULL DEFAULT now()
  );

  CREATE TABLE email_verification (
      id          VARCHAR(36)  PRIMARY KEY,
      user_id     VARCHAR(36)  NOT NULL REFERENCES web_user(id) ON DELETE CASCADE,
      code_hash   VARCHAR(255) NOT NULL,
      expires_at  TIMESTAMPTZ  NOT NULL,
      attempts    INT          NOT NULL DEFAULT 0,
      consumed_at TIMESTAMPTZ,
      created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
  );

  CREATE TABLE session (
      id                 VARCHAR(36) PRIMARY KEY,
      user_id            VARCHAR(36) NOT NULL REFERENCES web_user(id) ON DELETE CASCADE,
      refresh_token_hash VARCHAR(64) UNIQUE NOT NULL,
      expires_at         TIMESTAMPTZ NOT NULL,
      created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
      revoked_at         TIMESTAMPTZ
  );

  CREATE INDEX idx_email_verification_user ON email_verification (user_id);
  CREATE INDEX idx_session_user ON session (user_id);
  CREATE INDEX idx_session_token ON session (refresh_token_hash);
  ```

- [ ] **Step 5: Write the down migration.** Create `backend/db/migrations/000001_init.down.sql` (drop in reverse FK order):
  ```sql
  DROP TABLE IF EXISTS session;
  DROP TABLE IF EXISTS email_verification;
  DROP TABLE IF EXISTS web_user;
  ```

- [ ] **Step 6: Write `sqlc.yaml`.** Create `backend/sqlc.yaml` (this is the ONLY place `sqlc.yaml` is created) so Task 6 can generate the queries package per the contract (engine postgresql, pgx/v5, JSON tags, pointers for nullables):
  ```yaml
  version: "2"
  sql:
    - engine: "postgresql"
      queries: "db/queries"
      schema: "db/migrations"
      gen:
        go:
          package: "sqlc"
          out: "internal/adapter/store/sqlc"
          sql_package: "pgx/v5"
          emit_json_tags: true
          emit_pointers_for_null_types: true
  ```

- [ ] **Step 7: Run the test, expect PASS.** Run `cd backend && go test ./db/migrations/...`. Expected PASS: container starts, `Up` creates all three tables, every asserted column/index/CHECK is found, and `Down` drops the tables so the final counts are 0. Also confirm the Task 3 store test still passes against the real migration: `cd backend && go test ./internal/adapter/store/...` (its `RunMigrations` assertion now exercises the init schema rather than the deleted smoke table — update that store test if it still asserts a `smoke` table). Also run `cd backend && go vet ./db/...` and confirm it is clean.

- [ ] **Step 8: Commit.** `cd backend && git add db/migrations sqlc.yaml && git commit -m "feat: add init migration and sqlc config"` (no `Co-Authored-By` trailer).

---

### Task 6: sqlc queries + `Store` wrapper (`internal/adapter/store`)

**Files:**
- Create: `backend/db/queries/user.sql`
- Create: `backend/db/queries/verification.sql`
- Create: `backend/db/queries/session.sql`
- Create: `backend/internal/adapter/store/store.go`
- Modify: `backend/Makefile` (add `sqlc` target)
- Create (generated, do not hand-edit): `backend/internal/adapter/store/sqlc/*`
- Test: `backend/internal/adapter/store/store_test.go`

**Interfaces:**
- Consumes: the schema + `sqlc.yaml` from **Task 5**; the `domain` package from **Task 9** — exact types/signatures used here: `domain.Store` (the interface to implement), `domain.User`, `domain.EmailVerification`, `domain.Session`, `domain.CreateUserParams`, `domain.CreateEmailVerificationParams`, `domain.CreateSessionParams`, and `domain.ErrNotFound`, `domain.ErrConflict`.
- Produces:
  - `func New(pool *pgxpool.Pool) *Store` and `*Store` implementing every `domain.Store` method, including `ExecTx(ctx context.Context, fn func(domain.Store) error) error`. Task 6's `New` constructor signature is what the shared test harness (Task 11) and `cmd/server/main.go` (Task 22) wire up.
  - Named sqlc queries that generate `*sqlc.Queries` with method set: `CreateUser`, `GetUserByEmail`, `GetUserByUsername`, `GetUserByID`, `SetUserEmailVerified`, `CreateEmailVerification`, `GetActiveEmailVerification`, `IncrementVerificationAttempts`, `ConsumeEmailVerification`, `DeleteEmailVerificationsByUser`, `CreateSession`, `GetSessionByTokenHash`, `RevokeSession`, `RevokeAllUserSessions`.

- [ ] **Step 1: Write `db/queries/user.sql`.** Named queries backing the user-facing `domain.Store` methods. Create `backend/db/queries/user.sql`:
  ```sql
  -- name: CreateUser :one
  INSERT INTO web_user (id, username, email, password_hash, role)
  VALUES ($1, $2, $3, $4, $5)
  RETURNING id, username, email, password_hash, role, email_verified, created_at;

  -- name: GetUserByEmail :one
  SELECT id, username, email, password_hash, role, email_verified, created_at
  FROM web_user
  WHERE email = $1;

  -- name: GetUserByUsername :one
  SELECT id, username, email, password_hash, role, email_verified, created_at
  FROM web_user
  WHERE username = $1;

  -- name: GetUserByID :one
  SELECT id, username, email, password_hash, role, email_verified, created_at
  FROM web_user
  WHERE id = $1;

  -- name: SetUserEmailVerified :exec
  UPDATE web_user
  SET email_verified = true
  WHERE id = $1;
  ```

- [ ] **Step 2: Write `db/queries/verification.sql`.** Note `GetActiveEmailVerification` = newest unconsumed, and `IncrementVerificationAttempts` does the increment and RETURNS the new count atomically (one statement). Create `backend/db/queries/verification.sql`:
  ```sql
  -- name: CreateEmailVerification :one
  INSERT INTO email_verification (id, user_id, code_hash, expires_at)
  VALUES ($1, $2, $3, $4)
  RETURNING id, user_id, code_hash, expires_at, attempts, consumed_at, created_at;

  -- name: GetActiveEmailVerification :one
  SELECT id, user_id, code_hash, expires_at, attempts, consumed_at, created_at
  FROM email_verification
  WHERE user_id = $1 AND consumed_at IS NULL
  ORDER BY created_at DESC
  LIMIT 1;

  -- name: IncrementVerificationAttempts :one
  UPDATE email_verification
  SET attempts = attempts + 1
  WHERE id = $1
  RETURNING attempts;

  -- name: ConsumeEmailVerification :exec
  UPDATE email_verification
  SET consumed_at = now()
  WHERE id = $1;

  -- name: DeleteEmailVerificationsByUser :exec
  DELETE FROM email_verification
  WHERE user_id = $1;
  ```

- [ ] **Step 3: Write `db/queries/session.sql`.** Create `backend/db/queries/session.sql`:
  ```sql
  -- name: CreateSession :one
  INSERT INTO session (id, user_id, refresh_token_hash, expires_at)
  VALUES ($1, $2, $3, $4)
  RETURNING id, user_id, refresh_token_hash, expires_at, created_at, revoked_at;

  -- name: GetSessionByTokenHash :one
  SELECT id, user_id, refresh_token_hash, expires_at, created_at, revoked_at
  FROM session
  WHERE refresh_token_hash = $1;

  -- name: RevokeSession :exec
  UPDATE session
  SET revoked_at = now()
  WHERE id = $1;

  -- name: RevokeAllUserSessions :exec
  UPDATE session
  SET revoked_at = now()
  WHERE user_id = $1 AND revoked_at IS NULL;
  ```

- [ ] **Step 4: Add the `sqlc` Makefile target and generate.** Append to `backend/Makefile`:
  ```makefile
  .PHONY: sqlc
  sqlc:
  	sqlc generate
  ```
  Then run `cd backend && make sqlc` (sqlc is proto-pinned per machine convention). This writes `backend/internal/adapter/store/sqlc/{db.go,models.go,user.sql.go,verification.sql.go,session.sql.go}`. Do NOT hand-edit those files. Confirm `cd backend && go build ./internal/adapter/store/sqlc/...` compiles.

- [ ] **Step 5: Write the failing `Store` test.** Covers the CreateUser→GetUserByEmail roundtrip, `ErrNotFound` mapping, and `ExecTx` rollback-on-error. It stands up Postgres via `postgres.Run` and applies the schema via `store.RunMigrations` (Task 3). Create `backend/internal/adapter/store/store_test.go`:
  ```go
  package store_test

  import (
  	"context"
  	"errors"
  	"testing"
  	"time"

  	"github.com/google/uuid"
  	"github.com/jackc/pgx/v5/pgxpool"
  	"github.com/stretchr/testify/require"
  	"github.com/testcontainers/testcontainers-go"
  	"github.com/testcontainers/testcontainers-go/modules/postgres"
  	"github.com/testcontainers/testcontainers-go/wait"

  	"github.com/zulkhair/pustaka/backend/internal/adapter/store"
  	"github.com/zulkhair/pustaka/backend/internal/domain"
  )

  func newStore(t *testing.T) *store.Store {
  	t.Helper()
  	ctx := context.Background()
  	ctr, err := postgres.Run(ctx,
  		"postgres:16-alpine",
  		postgres.WithDatabase("pustaka"),
  		postgres.WithUsername("pustaka"),
  		postgres.WithPassword("pustaka"),
  		testcontainers.WithWaitStrategy(
  			wait.ForListeningPort("5432/tcp").WithStartupTimeout(60*time.Second)),
  	)
  	require.NoError(t, err)
  	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

  	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
  	require.NoError(t, err)

  	require.NoError(t, store.RunMigrations(dsn))

  	pool, err := pgxpool.New(ctx, dsn)
  	require.NoError(t, err)
  	t.Cleanup(pool.Close)
  	return store.New(pool)
  }

  func TestCreateAndGetUserRoundtrip(t *testing.T) {
  	s := newStore(t)
  	ctx := context.Background()

  	want, err := s.CreateUser(ctx, domain.CreateUserParams{
  		ID:           uuid.NewString(),
  		Username:     "alice",
  		Email:        "alice@example.com",
  		PasswordHash: "hash",
  		Role:         domain.RoleUser,
  	})
  	require.NoError(t, err)

  	got, err := s.GetUserByEmail(ctx, "alice@example.com")
  	require.NoError(t, err)
  	require.Equal(t, want.ID, got.ID)
  	require.Equal(t, "alice", got.Username)
  	require.Equal(t, domain.RoleUser, got.Role)
  	require.False(t, got.EmailVerified)
  }

  func TestGetUserByEmailNotFound(t *testing.T) {
  	s := newStore(t)
  	_, err := s.GetUserByEmail(context.Background(), "nobody@example.com")
  	require.ErrorIs(t, err, domain.ErrNotFound)
  }

  func TestExecTxRollsBackOnError(t *testing.T) {
  	s := newStore(t)
  	ctx := context.Background()
  	id := uuid.NewString()

  	sentinel := errors.New("boom")
  	err := s.ExecTx(ctx, func(tx domain.Store) error {
  		_, cerr := tx.CreateUser(ctx, domain.CreateUserParams{
  			ID:           id,
  			Username:     "bob",
  			Email:        "bob@example.com",
  			PasswordHash: "hash",
  			Role:         domain.RoleUser,
  		})
  		require.NoError(t, cerr)
  		return sentinel
  	})
  	require.ErrorIs(t, err, sentinel)

  	_, err = s.GetUserByID(ctx, id)
  	require.ErrorIs(t, err, domain.ErrNotFound)
  }
  ```

- [ ] **Step 6: Run the test, expect FAIL.** Run `cd backend && go test ./internal/adapter/store/...`. Expected FAIL: compile error — package `store` has no `Store` type, no `New`, and no `domain.Store` method implementations yet.

- [ ] **Step 7: Write the `Store` wrapper.** It wraps `*sqlc.Queries` + `*pgxpool.Pool`, maps sqlc rows ↔ domain entities, maps `pgx.ErrNoRows`→`domain.ErrNotFound` and unique-violation (`23505`)→`domain.ErrConflict`, and implements `ExecTx` via `pool.BeginTx` + `q.WithTx` (commit on nil error, rollback otherwise). Create `backend/internal/adapter/store/store.go`. The import block MUST include `github.com/jackc/pgx/v5/pgtype` (sqlc with `sql_package: pgx/v5` emits `pgtype.Timestamptz` for `TIMESTAMPTZ` columns and `*pgtype.Timestamptz` for nullable ones, matching the row-mapper code below):
  ```go
  package store

  import (
  	"context"
  	"errors"
  	"time"

  	"github.com/jackc/pgx/v5"
  	"github.com/jackc/pgx/v5/pgconn"
  	"github.com/jackc/pgx/v5/pgtype"
  	"github.com/jackc/pgx/v5/pgxpool"

  	"github.com/zulkhair/pustaka/backend/internal/adapter/store/sqlc"
  	"github.com/zulkhair/pustaka/backend/internal/domain"
  )

  // Store implements domain.Store over pgx + sqlc.
  type Store struct {
  	pool *pgxpool.Pool
  	q    *sqlc.Queries
  }

  func New(pool *pgxpool.Pool) *Store {
  	return &Store{pool: pool, q: sqlc.New(pool)}
  }

  // withQueries builds a Store bound to an existing sqlc.Queries (used inside a tx).
  func (s *Store) withQueries(q *sqlc.Queries) *Store {
  	return &Store{pool: s.pool, q: q}
  }

  func mapErr(err error) error {
  	if err == nil {
  		return nil
  	}
  	if errors.Is(err, pgx.ErrNoRows) {
  		return domain.ErrNotFound
  	}
  	var pgErr *pgconn.PgError
  	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
  		return domain.ErrConflict
  	}
  	return err
  }

  func (s *Store) ExecTx(ctx context.Context, fn func(domain.Store) error) error {
  	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
  	if err != nil {
  		return err
  	}
  	defer func() { _ = tx.Rollback(ctx) }()

  	if err := fn(s.withQueries(s.q.WithTx(tx))); err != nil {
  		return err
  	}
  	return tx.Commit(ctx)
  }

  func toUser(r sqlc.WebUser) domain.User {
  	return domain.User{
  		ID:            r.ID,
  		Username:      r.Username,
  		Email:         r.Email,
  		PasswordHash:  r.PasswordHash,
  		Role:          r.Role,
  		EmailVerified: r.EmailVerified,
  		CreatedAt:     r.CreatedAt.Time,
  	}
  }

  func toVerification(r sqlc.EmailVerification) domain.EmailVerification {
  	v := domain.EmailVerification{
  		ID:        r.ID,
  		UserID:    r.UserID,
  		CodeHash:  r.CodeHash,
  		ExpiresAt: r.ExpiresAt.Time,
  		Attempts:  int(r.Attempts),
  		CreatedAt: r.CreatedAt.Time,
  	}
  	if r.ConsumedAt != nil && r.ConsumedAt.Valid {
  		t := r.ConsumedAt.Time
  		v.ConsumedAt = &t
  	}
  	return v
  }

  func toSession(r sqlc.Session) domain.Session {
  	sess := domain.Session{
  		ID:               r.ID,
  		UserID:           r.UserID,
  		RefreshTokenHash: r.RefreshTokenHash,
  		ExpiresAt:        r.ExpiresAt.Time,
  		CreatedAt:        r.CreatedAt.Time,
  	}
  	if r.RevokedAt != nil && r.RevokedAt.Valid {
  		t := r.RevokedAt.Time
  		sess.RevokedAt = &t
  	}
  	return sess
  }

  func tstamp(t time.Time) pgtype.Timestamptz {
  	return pgtype.Timestamptz{Time: t, Valid: true}
  }

  func (s *Store) CreateUser(ctx context.Context, p domain.CreateUserParams) (domain.User, error) {
  	r, err := s.q.CreateUser(ctx, sqlc.CreateUserParams{
  		ID:           p.ID,
  		Username:     p.Username,
  		Email:        p.Email,
  		PasswordHash: p.PasswordHash,
  		Role:         p.Role,
  	})
  	if err != nil {
  		return domain.User{}, mapErr(err)
  	}
  	return toUser(r), nil
  }

  func (s *Store) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
  	r, err := s.q.GetUserByEmail(ctx, email)
  	if err != nil {
  		return domain.User{}, mapErr(err)
  	}
  	return toUser(r), nil
  }

  func (s *Store) GetUserByUsername(ctx context.Context, username string) (domain.User, error) {
  	r, err := s.q.GetUserByUsername(ctx, username)
  	if err != nil {
  		return domain.User{}, mapErr(err)
  	}
  	return toUser(r), nil
  }

  func (s *Store) GetUserByID(ctx context.Context, id string) (domain.User, error) {
  	r, err := s.q.GetUserByID(ctx, id)
  	if err != nil {
  		return domain.User{}, mapErr(err)
  	}
  	return toUser(r), nil
  }

  func (s *Store) SetUserEmailVerified(ctx context.Context, id string) error {
  	return mapErr(s.q.SetUserEmailVerified(ctx, id))
  }

  func (s *Store) CreateEmailVerification(ctx context.Context, p domain.CreateEmailVerificationParams) (domain.EmailVerification, error) {
  	r, err := s.q.CreateEmailVerification(ctx, sqlc.CreateEmailVerificationParams{
  		ID:        p.ID,
  		UserID:    p.UserID,
  		CodeHash:  p.CodeHash,
  		ExpiresAt: tstamp(p.ExpiresAt),
  	})
  	if err != nil {
  		return domain.EmailVerification{}, mapErr(err)
  	}
  	return toVerification(r), nil
  }

  func (s *Store) GetActiveEmailVerification(ctx context.Context, userID string) (domain.EmailVerification, error) {
  	r, err := s.q.GetActiveEmailVerification(ctx, userID)
  	if err != nil {
  		return domain.EmailVerification{}, mapErr(err)
  	}
  	return toVerification(r), nil
  }

  func (s *Store) IncrementVerificationAttempts(ctx context.Context, id string) (int, error) {
  	n, err := s.q.IncrementVerificationAttempts(ctx, id)
  	if err != nil {
  		return 0, mapErr(err)
  	}
  	return int(n), nil
  }

  func (s *Store) ConsumeEmailVerification(ctx context.Context, id string) error {
  	return mapErr(s.q.ConsumeEmailVerification(ctx, id))
  }

  func (s *Store) DeleteEmailVerificationsByUser(ctx context.Context, userID string) error {
  	return mapErr(s.q.DeleteEmailVerificationsByUser(ctx, userID))
  }

  func (s *Store) CreateSession(ctx context.Context, p domain.CreateSessionParams) (domain.Session, error) {
  	r, err := s.q.CreateSession(ctx, sqlc.CreateSessionParams{
  		ID:               p.ID,
  		UserID:           p.UserID,
  		RefreshTokenHash: p.RefreshTokenHash,
  		ExpiresAt:        tstamp(p.ExpiresAt),
  	})
  	if err != nil {
  		return domain.Session{}, mapErr(err)
  	}
  	return toSession(r), nil
  }

  func (s *Store) GetSessionByTokenHash(ctx context.Context, hash string) (domain.Session, error) {
  	r, err := s.q.GetSessionByTokenHash(ctx, hash)
  	if err != nil {
  		return domain.Session{}, mapErr(err)
  	}
  	return toSession(r), nil
  }

  func (s *Store) RevokeSession(ctx context.Context, id string) error {
  	return mapErr(s.q.RevokeSession(ctx, id))
  }

  func (s *Store) RevokeAllUserSessions(ctx context.Context, userID string) error {
  	return mapErr(s.q.RevokeAllUserSessions(ctx, userID))
  }

  var _ domain.Store = (*Store)(nil)
  ```

- [ ] **Step 8: Run the test, expect PASS.** Run `cd backend && go test ./internal/adapter/store/...`. Expected PASS: the roundtrip returns alice, the missing-email lookup yields `domain.ErrNotFound`, and after `ExecTx` returns the sentinel error the inserted user is absent (rollback confirmed). Run `cd backend && go vet ./internal/adapter/store/...` and confirm it is clean.

- [ ] **Step 9: Commit.** `cd backend && git add db/queries internal/adapter/store Makefile && git commit -m "feat: add sqlc queries and Store adapter with ExecTx"` (no `Co-Authored-By` trailer).

---

### Task 7: Crypto helpers (`pkg/hash`)

**Files:**
- Create: `backend/internal/pkg/hash/hash.go`
- Test: `backend/internal/pkg/hash/hash_test.go`

**Interfaces:**
- Consumes: `golang.org/x/crypto/bcrypt`, `crypto/rand`, `crypto/sha256`, `crypto/subtle`, `encoding/hex` (stdlib + bcrypt only — no project imports).
- Produces (later tasks rely on these VERBATIM):
  - `func HashPassword(pw string, cost int) (string, error)`
  - `func CheckPassword(hash, pw string) bool`
  - `func HashCode(code string, cost int) (string, error)`
  - `func CheckCode(hash, code string) bool`
  - `func GenerateNumericCode(n int) (string, error)`
  - `func HashRefreshToken(raw string) string`
  - `func ConstantTimeEqualHex(a, b string) bool`

- [ ] **Step 1: Write the failing test.** Create `backend/internal/pkg/hash/hash_test.go`:
  ```go
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
  ```

- [ ] **Step 2: Run the test, expect FAIL.** Run `cd backend && go test ./internal/pkg/hash/...`. Expected FAIL: compile error — package `hash` and its functions do not exist yet.

- [ ] **Step 3: Write the implementation.** Create `backend/internal/pkg/hash/hash.go`:
  ```go
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
  ```

- [ ] **Step 4: Run the test, expect PASS.** Run `cd backend && go test ./internal/pkg/hash/...`. Expected PASS: bcrypt round-trips for password and code (true on match, false otherwise), generated codes are exactly 6 digits and vary, the SHA-256 hex is deterministic + 64 chars + differs per input, and constant-time equality is true for equal hashes and false for differing/short inputs. Run `cd backend && go vet ./internal/pkg/hash/...` and confirm it is clean.

- [ ] **Step 5: Commit.** `cd backend && git add internal/pkg/hash && git commit -m "feat: add crypto helpers for passwords codes and refresh tokens"` (no `Co-Authored-By` trailer).

---

### Task 8: JWT helpers (`pkg/jwt`)

**Files:**
- Create: `backend/internal/pkg/jwt/jwt.go`
- Test: `backend/internal/pkg/jwt/jwt_test.go`

**Interfaces:**
- Consumes: `github.com/golang-jwt/jwt/v5`, `crypto/rand`, `encoding/base64`, `time` (stdlib + golang-jwt only — no project imports).
- Produces (later tasks rely on these VERBATIM):
  - `type Claims struct { UserID string \`json:"uid"\`; Role string \`json:"role"\`; jwt.RegisteredClaims }`
  - `func GenerateAccess(userID, role, secret string, ttl time.Duration) (string, error)`
  - `func ParseAccess(token, secret string) (*Claims, error)`
  - `func GenerateRefreshToken() (string, error)`

- [ ] **Step 1: Write the failing test.** Covers access round-trip exposing uid/role, expired-token rejection, wrong-secret rejection, and unique ~43-char refresh tokens. Create `backend/internal/pkg/jwt/jwt_test.go`:
  ```go
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
  ```

- [ ] **Step 2: Run the test, expect FAIL.** Run `cd backend && go test ./internal/pkg/jwt/...`. Expected FAIL: compile error — package `jwt` (the project one) and its `Claims`/`GenerateAccess`/`ParseAccess`/`GenerateRefreshToken` do not exist yet.

- [ ] **Step 3: Write the implementation.** HS256 access tokens with `exp = now + ttl`, parse validates signature + method + expiry, refresh = 32 CSPRNG bytes base64url-no-pad (opaque, not a JWT). Create `backend/internal/pkg/jwt/jwt.go`:
  ```go
  package jwt

  import (
  	"crypto/rand"
  	"encoding/base64"
  	"errors"
  	"time"

  	"github.com/golang-jwt/jwt/v5"
  )

  // Claims is the access-token payload: subject identity + role.
  type Claims struct {
  	UserID string `json:"uid"`
  	Role   string `json:"role"`
  	jwt.RegisteredClaims
  }

  // GenerateAccess mints an HS256 access token expiring at now+ttl.
  func GenerateAccess(userID, role, secret string, ttl time.Duration) (string, error) {
  	now := time.Now()
  	claims := Claims{
  		UserID: userID,
  		Role:   role,
  		RegisteredClaims: jwt.RegisteredClaims{
  			IssuedAt:  jwt.NewNumericDate(now),
  			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
  		},
  	}
  	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
  	return tok.SignedString([]byte(secret))
  }

  // ParseAccess validates the signature, signing method, and expiry, returning the claims.
  func ParseAccess(token, secret string) (*Claims, error) {
  	claims := &Claims{}
  	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
  		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
  			return nil, errors.New("unexpected signing method")
  		}
  		return []byte(secret), nil
  	})
  	if err != nil {
  		return nil, err
  	}
  	if !parsed.Valid {
  		return nil, errors.New("invalid token")
  	}
  	return claims, nil
  }

  // GenerateRefreshToken returns 32 CSPRNG bytes as a base64url (no padding) string.
  // It is an opaque random token, NOT a JWT.
  func GenerateRefreshToken() (string, error) {
  	b := make([]byte, 32)
  	if _, err := rand.Read(b); err != nil {
  		return "", err
  	}
  	return base64.RawURLEncoding.EncodeToString(b), nil
  }
  ```

- [ ] **Step 4: Run the test, expect PASS.** Run `cd backend && go test ./internal/pkg/jwt/...`. Expected PASS: a fresh token round-trips and exposes `uid`/`role`; a negative-ttl token is rejected (expired); parsing with the wrong secret errors; and 100 refresh tokens are each 43 chars and all unique. Run `cd backend && go vet ./internal/pkg/jwt/...` and confirm it is clean.

- [ ] **Step 5: Commit.** `cd backend && git add internal/pkg/jwt && git commit -m "feat: add JWT access tokens and opaque refresh token generation"` (no `Co-Authored-By` trailer).

---

### Task 9: Domain entities, ports & sentinel errors

**Files:**
- Create `backend/internal/domain/user.go`
- Create `backend/internal/domain/verification.go`
- Create `backend/internal/domain/session.go`
- Create `backend/internal/domain/ports.go`
- Create `backend/internal/domain/errors.go`
- Test `backend/internal/domain/errors_test.go`

**Interfaces:**
- Consumes: nothing (this is the innermost layer — `internal/domain` imports **only** the stdlib: `context`, `time`, `errors`). No adapter or app imports allowed.
- Produces (later tasks rely on these VERBATIM):
  - Entities: `User`, `EmailVerification`, `Session`.
  - Params: `CreateUserParams`, `CreateEmailVerificationParams`, `CreateSessionParams`.
  - Role consts: `RoleAdmin = "admin"`, `RoleUser = "user"`.
  - Ports: `Mailer` interface, `Store` interface.
  - Sentinel errors: `ErrNotFound`, `ErrConflict`, `ErrInvalidCredentials`, `ErrEmailNotVerified`, `ErrInvalidCode`, `ErrCodeExpired`, `ErrTooManyAttempts`, `ErrValidation`, `ErrResendCooldown` (internal-only — never surfaced to HTTP), `ErrUnauthorized`, `ErrForbidden`.

Steps:

- [ ] **Step 1: Write the failing test for sentinel errors.** Domain has no behaviour, so the only test asserts the package compiles and every sentinel is non-nil and mutually distinct. Create `backend/internal/domain/errors_test.go`:
  ```go
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
  ```

- [ ] **Step 2: Run the test and confirm it FAILS.** Run `go test ./internal/domain/...`. Expected FAIL: the package does not compile because `domain.ErrNotFound` (and every other identifier) is undefined (`undefined: domain.ErrNotFound`).

- [ ] **Step 3: Write `errors.go`.** Create `backend/internal/domain/errors.go`:
  ```go
  package domain

  import "errors"

  var (
  	ErrNotFound           = errors.New("not found")
  	ErrConflict           = errors.New("conflict")
  	ErrInvalidCredentials = errors.New("invalid credentials")
  	ErrEmailNotVerified   = errors.New("email not verified")
  	ErrInvalidCode        = errors.New("invalid code")
  	ErrCodeExpired        = errors.New("code expired")
  	ErrTooManyAttempts    = errors.New("too many attempts")
  	ErrValidation         = errors.New("validation failed")
  	// ErrResendCooldown is internal-only: ResendVerification returns it so the
  	// service layer can enforce the cooldown, but the handler swallows it into a
  	// generic 200 (never surfaced to the client).
  	ErrResendCooldown = errors.New("resend cooldown active")
  	ErrUnauthorized   = errors.New("unauthorized")
  	ErrForbidden      = errors.New("forbidden")
  )
  ```

- [ ] **Step 4: Write `user.go`.** Create `backend/internal/domain/user.go`:
  ```go
  package domain

  import "time"

  const (
  	RoleAdmin = "admin"
  	RoleUser  = "user"
  )

  type User struct {
  	ID            string
  	Username      string
  	Email         string
  	PasswordHash  string
  	Role          string
  	EmailVerified bool
  	CreatedAt     time.Time
  }

  type CreateUserParams struct {
  	ID           string
  	Username     string
  	Email        string
  	PasswordHash string
  	Role         string
  }
  ```

- [ ] **Step 5: Write `verification.go`.** Create `backend/internal/domain/verification.go`:
  ```go
  package domain

  import "time"

  type EmailVerification struct {
  	ID         string
  	UserID     string
  	CodeHash   string
  	ExpiresAt  time.Time
  	Attempts   int
  	ConsumedAt *time.Time
  	CreatedAt  time.Time
  }

  type CreateEmailVerificationParams struct {
  	ID        string
  	UserID    string
  	CodeHash  string
  	ExpiresAt time.Time
  }
  ```

- [ ] **Step 6: Write `session.go`.** Create `backend/internal/domain/session.go`:
  ```go
  package domain

  import "time"

  type Session struct {
  	ID               string
  	UserID           string
  	RefreshTokenHash string
  	ExpiresAt        time.Time
  	CreatedAt        time.Time
  	RevokedAt        *time.Time
  }

  type CreateSessionParams struct {
  	ID               string
  	UserID           string
  	RefreshTokenHash string
  	ExpiresAt        time.Time
  }
  ```

- [ ] **Step 7: Write `ports.go`.** Create `backend/internal/domain/ports.go` with the `Mailer` and `Store` interfaces exactly per contract:
  ```go
  package domain

  import "context"

  type Mailer interface {
  	SendVerificationCode(ctx context.Context, toEmail, code string) error
  }

  type Store interface {
  	ExecTx(ctx context.Context, fn func(Store) error) error

  	CreateUser(ctx context.Context, p CreateUserParams) (User, error)
  	GetUserByEmail(ctx context.Context, email string) (User, error)
  	GetUserByUsername(ctx context.Context, username string) (User, error)
  	GetUserByID(ctx context.Context, id string) (User, error)
  	SetUserEmailVerified(ctx context.Context, id string) error

  	CreateEmailVerification(ctx context.Context, p CreateEmailVerificationParams) (EmailVerification, error)
  	GetActiveEmailVerification(ctx context.Context, userID string) (EmailVerification, error)
  	IncrementVerificationAttempts(ctx context.Context, id string) (int, error)
  	ConsumeEmailVerification(ctx context.Context, id string) error
  	DeleteEmailVerificationsByUser(ctx context.Context, userID string) error

  	CreateSession(ctx context.Context, p CreateSessionParams) (Session, error)
  	GetSessionByTokenHash(ctx context.Context, hash string) (Session, error)
  	RevokeSession(ctx context.Context, id string) error
  	RevokeAllUserSessions(ctx context.Context, userID string) error
  }
  ```

- [ ] **Step 8: Run the test and confirm it PASSES.** Run `go vet ./internal/domain/...` (clean) and `go test ./internal/domain/...`. Expected PASS: `ok  github.com/zulkhair/pustaka/backend/internal/domain`.

- [ ] **Step 9: Commit.** `git add backend/internal/domain && git commit -m "feat(domain): add entities, ports and sentinel errors"` (no `Co-Authored-By` trailer).

---

### Task 10: Mailer adapters — Resend client and MockMailer

**Files:**
- Create `backend/internal/adapter/mail/resend.go`
- Create `backend/internal/adapter/mail/mock.go`
- Test `backend/internal/adapter/mail/resend_test.go`
- Test `backend/internal/adapter/mail/mock_test.go`

**Interfaces:**
- Consumes:
  - `domain.Mailer` — `SendVerificationCode(ctx context.Context, toEmail, code string) error` (Task 9). Both types must satisfy this interface.
  - `config.Config` fields `ResendAPIKey string`, `MailFrom string` (Task 2).
- Produces (later tasks rely on these VERBATIM):
  - `func NewResendMailer(cfg config.Config) *ResendMailer` — production `domain.Mailer`.
  - `mail.MockMailer` with exported fields `LastEmail string`, `LastCode string`, and `Sends []MockSend` (each `MockSend{Email, Code string}`); `func NewMockMailer() *MockMailer`; `func (m *MockMailer) CodeFor(email string) (string, bool)`. Used by the test harness (Task 11) and the E2E (Task 22) instead of real email.

Steps:

- [ ] **Step 1: Write the failing test for MockMailer.** Create `backend/internal/adapter/mail/mock_test.go`:
  ```go
  package mail_test

  import (
  	"context"
  	"testing"

  	"github.com/stretchr/testify/require"

  	"github.com/zulkhair/pustaka/backend/internal/adapter/mail"
  	"github.com/zulkhair/pustaka/backend/internal/domain"
  )

  func TestMockMailerRecordsSends(t *testing.T) {
  	var m domain.Mailer = mail.NewMockMailer()
  	require.NoError(t, m.SendVerificationCode(context.Background(), "a@example.com", "111111"))
  	require.NoError(t, m.SendVerificationCode(context.Background(), "b@example.com", "222222"))

  	mock := m.(*mail.MockMailer)
  	require.Equal(t, "b@example.com", mock.LastEmail)
  	require.Equal(t, "222222", mock.LastCode)
  	require.Len(t, mock.Sends, 2)
  	require.Equal(t, "a@example.com", mock.Sends[0].Email)
  	require.Equal(t, "111111", mock.Sends[0].Code)

  	code, ok := mock.CodeFor("a@example.com")
  	require.True(t, ok)
  	require.Equal(t, "111111", code)
  	_, ok = mock.CodeFor("missing@example.com")
  	require.False(t, ok)
  }
  ```

- [ ] **Step 2: Write the failing test for the Resend client.** Create `backend/internal/adapter/mail/resend_test.go`. It points the client at an `httptest.Server`, asserts the request shape (method, path, headers, JSON body), and asserts a non-2xx status yields an error:
  ```go
  package mail_test

  import (
  	"context"
  	"encoding/json"
  	"io"
  	"net/http"
  	"net/http/httptest"
  	"testing"

  	"github.com/stretchr/testify/require"

  	"github.com/zulkhair/pustaka/backend/internal/adapter/mail"
  	"github.com/zulkhair/pustaka/backend/internal/config"
  )

  func TestResendMailerSendsCorrectRequest(t *testing.T) {
  	var gotAuth, gotCT, gotMethod, gotPath string
  	var body map[string]any

  	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  		gotMethod = r.Method
  		gotPath = r.URL.Path
  		gotAuth = r.Header.Get("Authorization")
  		gotCT = r.Header.Get("Content-Type")
  		raw, _ := io.ReadAll(r.Body)
  		require.NoError(t, json.Unmarshal(raw, &body))
  		w.WriteHeader(http.StatusOK)
  		_, _ = w.Write([]byte(`{"id":"abc"}`))
  	}))
  	defer srv.Close()

  	m := mail.NewResendMailer(config.Config{ResendAPIKey: "re_test_key", MailFrom: "Pustaka <no-reply@pustaka.test>"})
  	m.BaseURL = srv.URL

  	require.NoError(t, m.SendVerificationCode(context.Background(), "user@example.com", "123456"))

  	require.Equal(t, http.MethodPost, gotMethod)
  	require.Equal(t, "/emails", gotPath)
  	require.Equal(t, "Bearer re_test_key", gotAuth)
  	require.Equal(t, "application/json", gotCT)
  	require.Equal(t, "Pustaka <no-reply@pustaka.test>", body["from"])
  	require.Equal(t, "user@example.com", body["to"])
  	require.Contains(t, body["text"], "123456")
  	require.Contains(t, body["html"], "123456")
  	require.NotEmpty(t, body["subject"])
  }

  func TestResendMailerErrorsOnNon2xx(t *testing.T) {
  	for _, code := range []int{http.StatusUnprocessableEntity, http.StatusInternalServerError} {
  		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  			w.WriteHeader(code)
  			_, _ = w.Write([]byte(`{"message":"boom"}`))
  		}))
  		m := mail.NewResendMailer(config.Config{ResendAPIKey: "re_test_key", MailFrom: "x@y.test"})
  		m.BaseURL = srv.URL
  		err := m.SendVerificationCode(context.Background(), "user@example.com", "123456")
  		require.Error(t, err)
  		srv.Close()
  	}
  }
  ```

- [ ] **Step 3: Run both tests and confirm they FAIL.** Run `go test ./internal/adapter/mail/...`. Expected FAIL: the package does not compile — `undefined: mail.NewMockMailer`, `undefined: mail.MockMailer`, `undefined: mail.NewResendMailer`.

- [ ] **Step 4: Write `mock.go`.** Create `backend/internal/adapter/mail/mock.go`:
  ```go
  package mail

  import (
  	"context"

  	"github.com/zulkhair/pustaka/backend/internal/domain"
  )

  type MockSend struct {
  	Email string
  	Code  string
  }

  type MockMailer struct {
  	LastEmail string
  	LastCode  string
  	Sends     []MockSend
  }

  func NewMockMailer() *MockMailer {
  	return &MockMailer{}
  }

  var _ domain.Mailer = (*MockMailer)(nil)

  func (m *MockMailer) SendVerificationCode(_ context.Context, toEmail, code string) error {
  	m.LastEmail = toEmail
  	m.LastCode = code
  	m.Sends = append(m.Sends, MockSend{Email: toEmail, Code: code})
  	return nil
  }

  // CodeFor returns the most recent code captured for the given email, if any.
  func (m *MockMailer) CodeFor(email string) (string, bool) {
  	for i := len(m.Sends) - 1; i >= 0; i-- {
  		if m.Sends[i].Email == email {
  			return m.Sends[i].Code, true
  		}
  	}
  	return "", false
  }
  ```

- [ ] **Step 5: Write `resend.go`.** Create `backend/internal/adapter/mail/resend.go`. POSTs to `https://api.resend.com/emails` (overridable via `BaseURL` for tests), sets `Authorization: Bearer <key>` + `Content-Type: application/json`, sends `{from, to, subject, html, text}` containing the code, and errors on non-2xx:
  ```go
  package mail

  import (
  	"bytes"
  	"context"
  	"encoding/json"
  	"fmt"
  	"io"
  	"net/http"
  	"time"

  	"github.com/zulkhair/pustaka/backend/internal/config"
  	"github.com/zulkhair/pustaka/backend/internal/domain"
  )

  const defaultResendBaseURL = "https://api.resend.com"

  type ResendMailer struct {
  	APIKey  string
  	From    string
  	BaseURL string
  	client  *http.Client
  }

  func NewResendMailer(cfg config.Config) *ResendMailer {
  	return &ResendMailer{
  		APIKey:  cfg.ResendAPIKey,
  		From:    cfg.MailFrom,
  		BaseURL: defaultResendBaseURL,
  		client:  &http.Client{Timeout: 10 * time.Second},
  	}
  }

  var _ domain.Mailer = (*ResendMailer)(nil)

  type resendEmailReq struct {
  	From    string `json:"from"`
  	To      string `json:"to"`
  	Subject string `json:"subject"`
  	HTML    string `json:"html"`
  	Text    string `json:"text"`
  }

  func (m *ResendMailer) SendVerificationCode(ctx context.Context, toEmail, code string) error {
  	payload := resendEmailReq{
  		From:    m.From,
  		To:      toEmail,
  		Subject: "Your Pustaka verification code",
  		HTML:    fmt.Sprintf("<p>Your Pustaka verification code is <strong>%s</strong>. It expires in 15 minutes.</p>", code),
  		Text:    fmt.Sprintf("Your Pustaka verification code is %s. It expires in 15 minutes.", code),
  	}
  	body, err := json.Marshal(payload)
  	if err != nil {
  		return fmt.Errorf("marshal resend payload: %w", err)
  	}

  	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.BaseURL+"/emails", bytes.NewReader(body))
  	if err != nil {
  		return fmt.Errorf("build resend request: %w", err)
  	}
  	req.Header.Set("Authorization", "Bearer "+m.APIKey)
  	req.Header.Set("Content-Type", "application/json")

  	resp, err := m.client.Do(req)
  	if err != nil {
  		return fmt.Errorf("send resend request: %w", err)
  	}
  	defer func() { _ = resp.Body.Close() }()

  	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
  		respBody, _ := io.ReadAll(resp.Body)
  		return fmt.Errorf("resend returned status %d: %s", resp.StatusCode, string(respBody))
  	}
  	return nil
  }
  ```

- [ ] **Step 6: Run the tests and confirm they PASS.** Run `go vet ./internal/adapter/mail/...` (clean) and `go test ./internal/adapter/mail/...`. Expected PASS: `ok  github.com/zulkhair/pustaka/backend/internal/adapter/mail`.

- [ ] **Step 7: Commit.** `git add backend/internal/adapter/mail && git commit -m "feat(mail): add resend mailer adapter and mock"` (no `Co-Authored-By` trailer).

---

## Cluster C — Shared test harness & registration/verification (Tasks 11–14)

### Task 11: Shared test harness (`internal/testsupport` + httpapi harness)

**Files:**
- Create `backend/internal/testsupport/testsupport.go`
- Create `backend/internal/adapter/httpapi/harness_test.go`
- Test (smoke): `backend/internal/testsupport/testsupport_test.go`

**Interfaces:**
- Consumes:
  - `store.New(pool *pgxpool.Pool) *store.Store` and `store.RunMigrations(databaseURL string) error` (Task 6 / Task 3).
  - `mail.NewMockMailer() *mail.MockMailer` (Task 10).
  - `auth.New(store domain.Store, mailer domain.Mailer, cfg config.Config) *auth.Service` and `httpapi.NewAuthHandler` / `httpapi.BuildApp` / `httpapi.RouterDeps` (Tasks 12 + 19/22). Because `BuildApp`/router land later, the httpapi harness `newTestApp` is authored here but its smoke assertion is exercised once `BuildApp` exists; the `internal/testsupport` package (DB-only) is fully testable now.
  - testcontainers `postgres.Run` + `container.ConnectionString` (Task 3 pattern), `github.com/jackc/pgx/v5/pgxpool`.
- Produces (EVERY later handler/service/E2E test uses THESE helpers — no ad-hoc `newTestStore`/mocks):
  - `func NewTestStore(t *testing.T) (*store.Store, func())` — boots a testcontainers Postgres, runs `store.RunMigrations`, returns the concrete `*store.Store` plus a cleanup func.
  - `func BackdateVerification(t *testing.T, pool *pgxpool.Pool, userID string, ts time.Time)` — sets `email_verification.created_at` for a user (used by resend cooldown tests).
  - In `httpapi` test package: `type testApp` exposing `app *fiber.App`, `store *store.Store`, `mailer *mail.MockMailer`; `func newTestApp(t *testing.T) *testApp`; helpers `doJSON(method, path, body)`, `doRaw(method, path, raw)`, `seedUnverifiedUser`, `seedVerifiedUser`, `seedUnverifiedUserWithPassword`.

Steps:

- [ ] **Step 1: Write the smoke test for `internal/testsupport`.** Create `backend/internal/testsupport/testsupport_test.go`. It asserts `NewTestStore` returns a working store (a CreateUser/GetUserByEmail roundtrip) and that `BackdateVerification` moves `created_at` into the past:
  ```go
  package testsupport_test

  import (
  	"context"
  	"testing"
  	"time"

  	"github.com/google/uuid"
  	"github.com/stretchr/testify/require"

  	"github.com/zulkhair/pustaka/backend/internal/domain"
  	"github.com/zulkhair/pustaka/backend/internal/testsupport"
  )

  func TestNewTestStoreRoundtrip(t *testing.T) {
  	st, cleanup := testsupport.NewTestStore(t)
  	defer cleanup()
  	ctx := context.Background()

  	u, err := st.CreateUser(ctx, domain.CreateUserParams{
  		ID: uuid.NewString(), Username: "smoke", Email: "smoke@example.com",
  		PasswordHash: "x", Role: domain.RoleUser,
  	})
  	require.NoError(t, err)

  	got, err := st.GetUserByEmail(ctx, "smoke@example.com")
  	require.NoError(t, err)
  	require.Equal(t, u.ID, got.ID)

  	_, err = st.CreateEmailVerification(ctx, domain.CreateEmailVerificationParams{
  		ID: uuid.NewString(), UserID: u.ID, CodeHash: "h",
  		ExpiresAt: time.Now().Add(15 * time.Minute),
  	})
  	require.NoError(t, err)

  	testsupport.BackdateVerification(t, st.Pool(), u.ID, time.Now().Add(-10*time.Minute))

  	ev, err := st.GetActiveEmailVerification(ctx, u.ID)
  	require.NoError(t, err)
  	require.WithinDuration(t, time.Now().Add(-10*time.Minute), ev.CreatedAt, time.Minute)
  }
  ```

- [ ] **Step 2: Run the smoke test and confirm it FAILS.** Run `cd backend && go test ./internal/testsupport/...`. Expected FAIL: compile error — `undefined: testsupport.NewTestStore`, `undefined: testsupport.BackdateVerification`, and `(*store.Store).Pool` is undefined.

- [ ] **Step 3: Expose the pool on `*store.Store` for test backdating.** Add a small accessor to `backend/internal/adapter/store/store.go` (kept tiny; used only by the harness):
  ```go
  // Pool returns the underlying pgx pool (used by the shared test harness).
  func (s *Store) Pool() *pgxpool.Pool { return s.pool }
  ```

- [ ] **Step 4: Write `internal/testsupport/testsupport.go`.** Create it:
  ```go
  package testsupport

  import (
  	"context"
  	"testing"
  	"time"

  	"github.com/jackc/pgx/v5/pgxpool"
  	"github.com/stretchr/testify/require"
  	"github.com/testcontainers/testcontainers-go"
  	"github.com/testcontainers/testcontainers-go/modules/postgres"
  	"github.com/testcontainers/testcontainers-go/wait"

  	"github.com/zulkhair/pustaka/backend/internal/adapter/store"
  )

  // NewTestStore boots an ephemeral Postgres, applies migrations, and returns a
  // concrete *store.Store plus a cleanup func. Used by every DB-backed test.
  func NewTestStore(t *testing.T) (*store.Store, func()) {
  	t.Helper()
  	ctx := context.Background()
  	ctr, err := postgres.Run(ctx,
  		"postgres:16-alpine",
  		postgres.WithDatabase("pustaka"),
  		postgres.WithUsername("pustaka"),
  		postgres.WithPassword("pustaka"),
  		testcontainers.WithWaitStrategy(
  			wait.ForListeningPort("5432/tcp").WithStartupTimeout(60*time.Second)),
  	)
  	require.NoError(t, err)

  	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
  	require.NoError(t, err)
  	require.NoError(t, store.RunMigrations(dsn))

  	pool, err := pgxpool.New(ctx, dsn)
  	require.NoError(t, err)

  	st := store.New(pool)
  	cleanup := func() {
  		pool.Close()
  		_ = ctr.Terminate(ctx)
  	}
  	return st, cleanup
  }

  // BackdateVerification rewrites email_verification.created_at for a user, so
  // resend-cooldown tests can simulate a stale verification row.
  func BackdateVerification(t *testing.T, pool *pgxpool.Pool, userID string, ts time.Time) {
  	t.Helper()
  	_, err := pool.Exec(context.Background(),
  		`UPDATE email_verification SET created_at = $2 WHERE user_id = $1`, userID, ts)
  	require.NoError(t, err)
  }
  ```

- [ ] **Step 5: Run the smoke test and confirm it PASSES.** Run `cd backend && go test ./internal/testsupport/...`. Expected PASS: roundtrip succeeds and the backdated `created_at` is ~10m in the past. Run `cd backend && go vet ./internal/testsupport/...` (clean).

- [ ] **Step 6: Write the httpapi test harness.** Create `backend/internal/adapter/httpapi/harness_test.go`. It builds a real Fiber app over a testcontainers-backed store + `MockMailer`, and exposes JSON helpers + seeders that every later handler/E2E test reuses. (It depends on `httpapi.BuildApp`/`NewAuthHandler` from Tasks 19/22; the file compiles once those exist — until then, the handler-cluster tasks that consume `newTestApp` will note the dependency.)
  ```go
  package httpapi_test

  import (
  	"bytes"
  	"context"
  	"encoding/json"
  	"io"
  	"net/http"
  	"net/http/httptest"
  	"strings"
  	"testing"
  	"time"

  	"github.com/google/uuid"
  	"github.com/stretchr/testify/require"

  	"github.com/zulkhair/pustaka/backend/internal/adapter/httpapi"
  	"github.com/zulkhair/pustaka/backend/internal/adapter/mail"
  	"github.com/zulkhair/pustaka/backend/internal/adapter/store"
  	"github.com/zulkhair/pustaka/backend/internal/app/auth"
  	"github.com/zulkhair/pustaka/backend/internal/config"
  	"github.com/zulkhair/pustaka/backend/internal/domain"
  	"github.com/zulkhair/pustaka/backend/internal/pkg/hash"
  	"github.com/zulkhair/pustaka/backend/internal/testsupport"

  	"github.com/gofiber/fiber/v2"
  )

  const harnessSecret = "harness-secret-0123456789"

  type testApp struct {
  	app    *fiber.App
  	store  *store.Store
  	mailer *mail.MockMailer
  }

  func newTestApp(t *testing.T) *testApp {
  	t.Helper()
  	st, cleanup := testsupport.NewTestStore(t)
  	t.Cleanup(cleanup)

  	mailer := mail.NewMockMailer()
  	cfg := config.Config{
  		JWTSecret:      harnessSecret,
  		AccessTTL:      15 * time.Minute,
  		RefreshTTL:     720 * time.Hour,
  		BcryptCost:     4,
  		CodeTTL:        15 * time.Minute,
  		MaxAttempts:    5,
  		ResendCooldown: 60 * time.Second,
  	}
  	svc := auth.New(st, mailer, cfg)
  	app := httpapi.BuildApp(httpapi.RouterDeps{
  		Auth:      httpapi.NewAuthHandler(svc),
  		Pinger:    st.Pool(),
  		JWTSecret: cfg.JWTSecret,
  	})
  	return &testApp{app: app, store: st, mailer: mailer}
  }

  // doJSON marshals body and POSTs/GETs it, returning the *http.Response.
  func doJSON(t *testing.T, ta *testApp, method, path string, body any) *http.Response {
  	t.Helper()
  	raw, err := json.Marshal(body)
  	require.NoError(t, err)
  	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
  	req.Header.Set("Content-Type", "application/json")
  	resp, err := ta.app.Test(req, -1)
  	require.NoError(t, err)
  	return resp
  }

  // doJSONBody is doJSON plus a decoded envelope map.
  func doJSONBody(t *testing.T, ta *testApp, method, path string, body any) (*http.Response, map[string]any) {
  	t.Helper()
  	resp := doJSON(t, ta, method, path, body)
  	b, _ := io.ReadAll(resp.Body)
  	var env map[string]any
  	require.NoError(t, json.Unmarshal(b, &env), "body: %s", string(b))
  	return resp, env
  }

  // doRaw sends an arbitrary (possibly invalid-JSON) body.
  func doRaw(t *testing.T, ta *testApp, method, path, raw string) *http.Response {
  	t.Helper()
  	req := httptest.NewRequest(method, path, strings.NewReader(raw))
  	req.Header.Set("Content-Type", "application/json")
  	resp, err := ta.app.Test(req, -1)
  	require.NoError(t, err)
  	return resp
  }

  // seedUnverifiedUser inserts an unverified user plus a fresh verification row.
  func seedUnverifiedUser(t *testing.T, st *store.Store, username, email string) domain.User {
  	t.Helper()
  	ctx := context.Background()
  	u, err := st.CreateUser(ctx, domain.CreateUserParams{
  		ID: uuid.NewString(), Username: username, Email: email,
  		PasswordHash: "x", Role: domain.RoleUser,
  	})
  	require.NoError(t, err)
  	_, err = st.CreateEmailVerification(ctx, domain.CreateEmailVerificationParams{
  		ID: uuid.NewString(), UserID: u.ID, CodeHash: "seed-hash",
  		ExpiresAt: time.Now().Add(15 * time.Minute),
  	})
  	require.NoError(t, err)
  	return u
  }

  // seedUnverifiedUserWithPassword inserts an unverified user with a known bcrypt password.
  func seedUnverifiedUserWithPassword(t *testing.T, st *store.Store, username, email, pw string) domain.User {
  	t.Helper()
  	ctx := context.Background()
  	ph, err := hash.HashPassword(pw, 4)
  	require.NoError(t, err)
  	u, err := st.CreateUser(ctx, domain.CreateUserParams{
  		ID: uuid.NewString(), Username: username, Email: email,
  		PasswordHash: ph, Role: domain.RoleUser,
  	})
  	require.NoError(t, err)
  	return u
  }

  // seedVerifiedUser inserts a verified user with a known bcrypt password.
  func seedVerifiedUser(t *testing.T, st *store.Store, username, email, pw string) domain.User {
  	t.Helper()
  	ctx := context.Background()
  	u := seedUnverifiedUserWithPassword(t, st, username, email, pw)
  	require.NoError(t, st.SetUserEmailVerified(ctx, u.ID))
  	u.EmailVerified = true
  	return u
  }
  ```

- [ ] **Step 7: Boot smoke for `newTestApp` (health responds).** Add a smoke test to `harness_test.go` proving the app boots and `/api/health` answers 200. This is the harness task's "test" (a compile/smoke check):
  ```go
  func TestHarnessBootsAndHealthResponds(t *testing.T) {
  	ta := newTestApp(t)
  	req := httptest.NewRequest("GET", "/api/health", nil)
  	resp, err := ta.app.Test(req, -1)
  	require.NoError(t, err)
  	require.Equal(t, 200, resp.StatusCode)
  }
  ```
  Run `cd backend && go test ./internal/adapter/httpapi/... -run TestHarnessBootsAndHealthResponds`. EXPECT (when run after Tasks 12/19/22 land): PASS. If `BuildApp`/handlers are not yet present in the working tree, state the observed compile failure explicitly and revisit after the consuming tasks.

- [ ] **Step 8: Vet and commit.** Run `cd backend && go vet ./internal/testsupport/... ./internal/adapter/httpapi/...` (clean for the DB-only package), then `git add backend/internal/testsupport backend/internal/adapter/store/store.go backend/internal/adapter/httpapi/harness_test.go && git commit -m "test: add shared testcontainers store + httpapi harness"` (no `Co-Authored-By` trailer).

---

### Task 12: AuthService.Register + register handler + `mapAuthError`

**Files:**
- Create `backend/internal/app/auth/service.go`
- Create `backend/internal/adapter/httpapi/auth_handler.go`
- Create `backend/internal/adapter/httpapi/errors.go` (defines `mapAuthError` ONCE)
- Test `backend/internal/app/auth/register_test.go`
- Test `backend/internal/adapter/httpapi/errors_test.go`

**Interfaces:**
- Consumes:
  - `domain.Store` and its methods `ExecTx`, `GetUserByEmail`, `GetUserByUsername`, `CreateUser`, `CreateEmailVerification` (Task 9); sentinels `domain.ErrConflict`, `domain.ErrNotFound`, `domain.ErrValidation`; `domain.CreateUserParams`, `domain.CreateEmailVerificationParams`; `domain.RoleUser` (Task 9).
  - `domain.Mailer.SendVerificationCode` (Task 9); the shared test harness `testsupport.NewTestStore` + `mail.NewMockMailer()` in tests (Tasks 10–11).
  - `config.Config` fields `BcryptCost`, `CodeTTL` (Task 2).
  - `hash.HashPassword`, `hash.GenerateNumericCode`, `hash.HashCode` (Task 7).
  - `httpapi.OK`, `httpapi.Fail` (Task 19 `response.go`); `httpapi.RegisterReq{Username, Email, Password string}` request DTO (this task).
- Produces (later tasks rely on these VERBATIM):
  - `type Service struct { store domain.Store; mailer domain.Mailer; cfg config.Config }`
  - `func New(store domain.Store, mailer domain.Mailer, cfg config.Config) *Service`
  - `type RegisterInput struct { Username, Email, Password string }`; also the shared `VerifyInput`, `LoginInput`, `Tokens` types and `normalizeEmail` helper.
  - `func (s *Service) Register(ctx context.Context, in RegisterInput) error`
  - `type AuthHandler struct { svc *auth.Service }` and `func NewAuthHandler(svc *auth.Service) *AuthHandler` plus `func (h *AuthHandler) Register(c *fiber.Ctx) error` (later tasks add `VerifyEmail`, `ResendVerification`, `Login`, `Refresh`, `Logout`, `Me` to the same `*AuthHandler`).
  - `func mapAuthError(c *fiber.Ctx, err error) error` (defined ONCE here; EVERY handler calls it).

Steps:

- [ ] **Step 1: Write the failing service test.** Create `backend/internal/app/auth/register_test.go`. It uses the shared harness `testsupport.NewTestStore`. It asserts a successful register creates an unverified user + a verification row + the mock captured the 6-digit code; a duplicate email yields `domain.ErrConflict`; and validation failures (empty field, bad email, short password) yield `domain.ErrValidation`. Note `cfg.CodeTTL` is set explicitly (no zero value):
  ```go
  package auth_test

  import (
  	"context"
  	"testing"
  	"time"

  	"github.com/stretchr/testify/require"

  	"github.com/zulkhair/pustaka/backend/internal/adapter/mail"
  	"github.com/zulkhair/pustaka/backend/internal/app/auth"
  	"github.com/zulkhair/pustaka/backend/internal/config"
  	"github.com/zulkhair/pustaka/backend/internal/domain"
  	"github.com/zulkhair/pustaka/backend/internal/testsupport"
  )

  func registerCfg() config.Config {
  	return config.Config{BcryptCost: 4, CodeTTL: 15 * time.Minute}
  }

  func TestRegisterCreatesUnverifiedUserAndSendsCode(t *testing.T) {
  	store, cleanup := testsupport.NewTestStore(t)
  	defer cleanup()
  	mock := mail.NewMockMailer()
  	svc := auth.New(store, mock, registerCfg())
  	ctx := context.Background()

  	err := svc.Register(ctx, auth.RegisterInput{
  		Username: "alice",
  		Email:    "alice@example.com",
  		Password: "supersecret",
  	})
  	require.NoError(t, err)

  	u, err := store.GetUserByEmail(ctx, "alice@example.com")
  	require.NoError(t, err)
  	require.False(t, u.EmailVerified)
  	require.Equal(t, domain.RoleUser, u.Role)
  	require.NotEqual(t, "supersecret", u.PasswordHash)

  	ev, err := store.GetActiveEmailVerification(ctx, u.ID)
  	require.NoError(t, err)
  	require.NotEmpty(t, ev.CodeHash)

  	require.Equal(t, "alice@example.com", mock.LastEmail)
  	require.Len(t, mock.LastCode, 6)
  }

  func TestRegisterDuplicateEmailConflict(t *testing.T) {
  	store, cleanup := testsupport.NewTestStore(t)
  	defer cleanup()
  	svc := auth.New(store, mail.NewMockMailer(), registerCfg())
  	ctx := context.Background()

  	in := auth.RegisterInput{Username: "bob", Email: "bob@example.com", Password: "supersecret"}
  	require.NoError(t, svc.Register(ctx, in))

  	dup := auth.RegisterInput{Username: "bob2", Email: "bob@example.com", Password: "supersecret"}
  	err := svc.Register(ctx, dup)
  	require.ErrorIs(t, err, domain.ErrConflict)
  }

  func TestRegisterValidationErrors(t *testing.T) {
  	store, cleanup := testsupport.NewTestStore(t)
  	defer cleanup()
  	svc := auth.New(store, mail.NewMockMailer(), registerCfg())
  	ctx := context.Background()

  	cases := []auth.RegisterInput{
  		{Username: "", Email: "x@example.com", Password: "supersecret"},
  		{Username: "carol", Email: "not-an-email", Password: "supersecret"},
  		{Username: "carol", Email: "carol@example.com", Password: "short"},
  	}
  	for _, in := range cases {
  		err := svc.Register(ctx, in)
  		require.ErrorIs(t, err, domain.ErrValidation)
  	}
  }
  ```

- [ ] **Step 2: Run the test and confirm it FAILS.** Run `go test ./internal/app/auth/...`. Expected FAIL: the package does not compile — `undefined: auth.New`, `undefined: auth.RegisterInput`, `undefined: (*auth.Service).Register`.

- [ ] **Step 3: Write `service.go` with `New`, the shared types, and `Register`.** Create `backend/internal/app/auth/service.go`. Validation returns `domain.ErrValidation` for empty fields / invalid email (`net/mail.ParseAddress`) / password length < 8. Inside `ExecTx`: reject duplicate username/email with `domain.ErrConflict`, create the user (role `user`, unverified), create the verification row with `ExpiresAt = now + cfg.CodeTTL`. After commit, call `mailer.SendVerificationCode`:
  ```go
  package auth

  import (
  	"context"
  	"errors"
  	"fmt"
  	"net/mail"
  	"strings"
  	"time"

  	"github.com/google/uuid"

  	"github.com/zulkhair/pustaka/backend/internal/config"
  	"github.com/zulkhair/pustaka/backend/internal/domain"
  	"github.com/zulkhair/pustaka/backend/internal/pkg/hash"
  )

  type Service struct {
  	store  domain.Store
  	mailer domain.Mailer
  	cfg    config.Config
  }

  func New(store domain.Store, mailer domain.Mailer, cfg config.Config) *Service {
  	return &Service{store: store, mailer: mailer, cfg: cfg}
  }

  type RegisterInput struct {
  	Username string
  	Email    string
  	Password string
  }

  type VerifyInput struct {
  	Email string
  	Code  string
  }

  type LoginInput struct {
  	Identifier string
  	Password   string
  }

  type Tokens struct {
  	AccessToken  string
  	RefreshToken string
  	ExpiresIn    int
  }

  func normalizeEmail(email string) string {
  	return strings.ToLower(strings.TrimSpace(email))
  }

  func (s *Service) Register(ctx context.Context, in RegisterInput) error {
  	username := strings.TrimSpace(in.Username)
  	email := normalizeEmail(in.Email)
  	if username == "" || email == "" || in.Password == "" {
  		return fmt.Errorf("%w: missing required field", domain.ErrValidation)
  	}
  	if _, err := mail.ParseAddress(email); err != nil {
  		return fmt.Errorf("%w: invalid email", domain.ErrValidation)
  	}
  	if len(in.Password) < 8 {
  		return fmt.Errorf("%w: password too short", domain.ErrValidation)
  	}

  	pwHash, err := hash.HashPassword(in.Password, s.cfg.BcryptCost)
  	if err != nil {
  		return fmt.Errorf("hash password: %w", err)
  	}
  	code, err := hash.GenerateNumericCode(6)
  	if err != nil {
  		return fmt.Errorf("generate code: %w", err)
  	}
  	codeHash, err := hash.HashCode(code, s.cfg.BcryptCost)
  	if err != nil {
  		return fmt.Errorf("hash code: %w", err)
  	}

  	txErr := s.store.ExecTx(ctx, func(st domain.Store) error {
  		if _, err := st.GetUserByEmail(ctx, email); err == nil {
  			return domain.ErrConflict
  		} else if !errors.Is(err, domain.ErrNotFound) {
  			return err
  		}
  		if _, err := st.GetUserByUsername(ctx, username); err == nil {
  			return domain.ErrConflict
  		} else if !errors.Is(err, domain.ErrNotFound) {
  			return err
  		}

  		user, err := st.CreateUser(ctx, domain.CreateUserParams{
  			ID:           uuid.NewString(),
  			Username:     username,
  			Email:        email,
  			PasswordHash: pwHash,
  			Role:         domain.RoleUser,
  		})
  		if err != nil {
  			return err
  		}

  		_, err = st.CreateEmailVerification(ctx, domain.CreateEmailVerificationParams{
  			ID:        uuid.NewString(),
  			UserID:    user.ID,
  			CodeHash:  codeHash,
  			ExpiresAt: time.Now().Add(s.cfg.CodeTTL),
  		})
  		return err
  	})
  	if txErr != nil {
  		return txErr
  	}

  	if err := s.mailer.SendVerificationCode(ctx, email, code); err != nil {
  		return fmt.Errorf("send verification code: %w", err)
  	}
  	return nil
  }
  ```

- [ ] **Step 4: Run the test and confirm it PASSES.** Run `go test ./internal/app/auth/...`. Expected PASS: register creates an unverified `user`-role row + an active verification row + a 6-digit mock code; duplicate email returns `domain.ErrConflict`; validation cases return `domain.ErrValidation`.

- [ ] **Step 5: Write the `mapAuthError` micro-test.** Create `backend/internal/adapter/httpapi/errors_test.go`. It drives a tiny Fiber app whose handler returns a chosen sentinel through `mapAuthError`, asserting the contract's status table. `ErrResendCooldown` is internal-only and must NOT appear here (handlers swallow it):
  ```go
  package httpapi

  import (
  	"net/http/httptest"
  	"testing"

  	"github.com/gofiber/fiber/v2"
  	"github.com/stretchr/testify/require"

  	"github.com/zulkhair/pustaka/backend/internal/domain"
  )

  func TestMapAuthError(t *testing.T) {
  	cases := []struct {
  		err  error
  		code int
  	}{
  		{domain.ErrConflict, 409},
  		{domain.ErrInvalidCredentials, 401},
  		{domain.ErrEmailNotVerified, 401},
  		{domain.ErrUnauthorized, 401},
  		{domain.ErrForbidden, 403},
  		{domain.ErrNotFound, 404},
  		{domain.ErrInvalidCode, 400},
  		{domain.ErrCodeExpired, 400},
  		{domain.ErrValidation, 400},
  		{domain.ErrTooManyAttempts, 429},
  		{errSentinelOther, 500},
  	}
  	for _, tc := range cases {
  		app := fiber.New()
  		app.Get("/x", func(c *fiber.Ctx) error { return mapAuthError(c, tc.err) })
  		resp, err := app.Test(httptest.NewRequest("GET", "/x", nil), -1)
  		require.NoError(t, err)
  		require.Equal(t, tc.code, resp.StatusCode, "err=%v", tc.err)
  	}
  }

  var errSentinelOther = fiber.NewError(0, "other") // any non-mapped error -> 500
  ```

- [ ] **Step 6: Run the micro-test and confirm it FAILS.** Run `go test ./internal/adapter/httpapi/ -run TestMapAuthError`. Expected FAIL: compile error — `undefined: mapAuthError` (and `OK`/`Fail` from `response.go`, which is created in Task 19; if `response.go` is not yet present, this file still fails on `mapAuthError`).

- [ ] **Step 7: Write `errors.go` defining `mapAuthError` once.** Create `backend/internal/adapter/httpapi/errors.go` per the contract table:
  ```go
  package httpapi

  import (
  	"errors"

  	"github.com/gofiber/fiber/v2"

  	"github.com/zulkhair/pustaka/backend/internal/domain"
  )

  // mapAuthError translates a domain sentinel into an HTTP status + generic message.
  // Defined ONCE; every handler calls it. ErrResendCooldown is intentionally absent —
  // it is internal-only and swallowed by the resend handler into a generic 200.
  func mapAuthError(c *fiber.Ctx, err error) error {
  	switch {
  	case errors.Is(err, domain.ErrConflict):
  		return Fail(c, fiber.StatusConflict, "conflict")
  	case errors.Is(err, domain.ErrInvalidCredentials),
  		errors.Is(err, domain.ErrEmailNotVerified),
  		errors.Is(err, domain.ErrUnauthorized):
  		return Fail(c, fiber.StatusUnauthorized, "unauthorized")
  	case errors.Is(err, domain.ErrForbidden):
  		return Fail(c, fiber.StatusForbidden, "forbidden")
  	case errors.Is(err, domain.ErrNotFound):
  		return Fail(c, fiber.StatusNotFound, "not found")
  	case errors.Is(err, domain.ErrInvalidCode),
  		errors.Is(err, domain.ErrCodeExpired),
  		errors.Is(err, domain.ErrValidation):
  		return Fail(c, fiber.StatusBadRequest, "bad request")
  	case errors.Is(err, domain.ErrTooManyAttempts):
  		return Fail(c, fiber.StatusTooManyRequests, "too many attempts")
  	default:
  		return Fail(c, fiber.StatusInternalServerError, "internal error")
  	}
  }
  ```

- [ ] **Step 8: Write the register handler + DTO.** Create `backend/internal/adapter/httpapi/auth_handler.go`. Parse `RegisterReq`, call `svc.Register`, map errors via `mapAuthError`, and on success return a generic `OK(c, nil)` (enumeration-resistant — no nested message, no hint whether the email already existed):
  ```go
  package httpapi

  import (
  	"github.com/gofiber/fiber/v2"

  	"github.com/zulkhair/pustaka/backend/internal/app/auth"
  )

  type AuthHandler struct {
  	svc *auth.Service
  }

  func NewAuthHandler(svc *auth.Service) *AuthHandler {
  	return &AuthHandler{svc: svc}
  }

  type RegisterReq struct {
  	Username string `json:"username"`
  	Email    string `json:"email"`
  	Password string `json:"password"`
  }

  func (h *AuthHandler) Register(c *fiber.Ctx) error {
  	var req RegisterReq
  	if err := c.BodyParser(&req); err != nil {
  		return Fail(c, fiber.StatusBadRequest, "invalid request body")
  	}
  	if err := h.svc.Register(c.Context(), auth.RegisterInput{
  		Username: req.Username,
  		Email:    req.Email,
  		Password: req.Password,
  	}); err != nil {
  		return mapAuthError(c, err)
  	}
  	return OK(c, nil)
  }
  ```

- [ ] **Step 9: Confirm the auth package compiles and tests pass.** Run `go vet ./internal/app/auth/...` (clean) and `go test ./internal/app/auth/...`. Expected PASS. (The `httpapi` package's `mapAuthError` micro-test passes once `response.go` from Task 19 is in the tree; if running before Task 19, note the `OK`/`Fail` dependency explicitly.)

- [ ] **Step 10: Commit.** `git add backend/internal/app/auth backend/internal/adapter/httpapi && git commit -m "feat(auth): add register use-case, handler, and mapAuthError"` (no `Co-Authored-By` trailer).

---

### Task 13: AuthService.VerifyEmail + verify-email handler (defines `issueTokens`)

**Files:**
- Modify `backend/internal/app/auth/service.go` (add `issueTokens` + `VerifyEmail`)
- Modify `backend/internal/adapter/httpapi/auth_handler.go` (add `VerifyEmail` + DTOs)
- Test `backend/internal/app/auth/verify_test.go`

**Interfaces:**
- Consumes:
  - `domain.Store` methods `GetUserByEmail`, `GetActiveEmailVerification`, `IncrementVerificationAttempts`, `ExecTx`, `SetUserEmailVerified`, `ConsumeEmailVerification`, `CreateSession` (Task 9); sentinels `domain.ErrInvalidCode`, `domain.ErrCodeExpired`, `domain.ErrTooManyAttempts`, `domain.ErrNotFound` (Task 9); `domain.CreateSessionParams` (Task 9).
  - `config.Config` fields `MaxAttempts`, `AccessTTL`, `RefreshTTL`, `JWTSecret` (Task 2).
  - `hash.CheckCode`, `hash.HashRefreshToken` (Task 7).
  - `jwt.GenerateAccess`, `jwt.GenerateRefreshToken` (Task 8).
  - `Service`, `New`, `VerifyInput`, `Tokens`, `normalizeEmail` (Task 12); `AuthHandler`, `NewAuthHandler`, `mapAuthError`, `httpapi.OK`/`Fail` (Task 12 + Task 19).
- Produces (later tasks rely on these VERBATIM):
  - `func (s *Service) issueTokens(ctx context.Context, u domain.User) (Tokens, error)` — defined ONCE here; **reused** by Login (Task 15) and called within Refresh's tx path (Task 16). Login/Refresh must NOT redefine it.
  - `func (s *Service) VerifyEmail(ctx context.Context, in VerifyInput) (Tokens, error)`
  - `func (h *AuthHandler) VerifyEmail(c *fiber.Ctx) error`; DTOs `VerifyReq{Email, Code string}` and `TokensDTO{accessToken, refreshToken, expiresIn}`.

Steps:

- [ ] **Step 1: Write the failing test.** Create `backend/internal/app/auth/verify_test.go`. Uses the shared harness (`testsupport.NewTestStore`) and `testsupport.BackdateVerification` for the expiry case. Register a user (capturing the real code from the mock), then exercise: wrong code increments attempts and returns `domain.ErrInvalidCode`; attempts at/over the cap returns `domain.ErrTooManyAttempts`; correct code verifies the user, returns non-empty `Tokens` (`ExpiresIn == 900`), and writes a session row keyed by `hash.HashRefreshToken(tokens.RefreshToken)`:
  ```go
  package auth_test

  import (
  	"context"
  	"testing"
  	"time"

  	"github.com/stretchr/testify/require"

  	"github.com/zulkhair/pustaka/backend/internal/adapter/mail"
  	"github.com/zulkhair/pustaka/backend/internal/app/auth"
  	"github.com/zulkhair/pustaka/backend/internal/config"
  	"github.com/zulkhair/pustaka/backend/internal/domain"
  	"github.com/zulkhair/pustaka/backend/internal/pkg/hash"
  )

  func verifyTestCfg() config.Config {
  	return config.Config{
  		BcryptCost:  4,
  		CodeTTL:     15 * time.Minute,
  		MaxAttempts: 5,
  		AccessTTL:   15 * time.Minute,
  		RefreshTTL:  720 * time.Hour,
  		JWTSecret:   "test-secret",
  	}
  }

  func registerAlice(t *testing.T, store domain.Store, mock *mail.MockMailer, svc *auth.Service) (domain.User, string) {
  	t.Helper()
  	ctx := context.Background()
  	require.NoError(t, svc.Register(ctx, auth.RegisterInput{
  		Username: "alice", Email: "alice@example.com", Password: "supersecret",
  	}))
  	u, err := store.GetUserByEmail(ctx, "alice@example.com")
  	require.NoError(t, err)
  	return u, mock.LastCode
  }

  func TestVerifyEmailWrongCodeIncrementsAndFails(t *testing.T) {
  	store, cleanup := newTestStore(t)
  	defer cleanup()
  	mock := mail.NewMockMailer()
  	svc := auth.New(store, mock, verifyTestCfg())
  	ctx := context.Background()
  	u, _ := registerAlice(t, store, mock, svc)

  	_, err := svc.VerifyEmail(ctx, auth.VerifyInput{Email: "alice@example.com", Code: "000000"})
  	require.ErrorIs(t, err, domain.ErrInvalidCode)

  	ev, err := store.GetActiveEmailVerification(ctx, u.ID)
  	require.NoError(t, err)
  	require.Equal(t, 1, ev.Attempts)
  }

  func TestVerifyEmailCorrectCodeIssuesTokens(t *testing.T) {
  	store, cleanup := newTestStore(t)
  	defer cleanup()
  	mock := mail.NewMockMailer()
  	svc := auth.New(store, mock, verifyTestCfg())
  	ctx := context.Background()
  	u, code := registerAlice(t, store, mock, svc)

  	tokens, err := svc.VerifyEmail(ctx, auth.VerifyInput{Email: "alice@example.com", Code: code})
  	require.NoError(t, err)
  	require.NotEmpty(t, tokens.AccessToken)
  	require.NotEmpty(t, tokens.RefreshToken)
  	require.Equal(t, 900, tokens.ExpiresIn)

  	verified, err := store.GetUserByID(ctx, u.ID)
  	require.NoError(t, err)
  	require.True(t, verified.EmailVerified)

  	sess, err := store.GetSessionByTokenHash(ctx, hash.HashRefreshToken(tokens.RefreshToken))
  	require.NoError(t, err)
  	require.Equal(t, u.ID, sess.UserID)
  }
  ```
  > This test (and every other `package auth_test` file) relies on a tiny local adapter `newTestStore(t) (*store.Store, func())` defined once in the auth test package that simply calls `testsupport.NewTestStore(t)`. Add it in a shared `auth_helpers_test.go` so the auth tests read cleanly; it must NOT duplicate testcontainers setup — it only forwards to the shared harness.

- [ ] **Step 2: Add the auth-package test adapter.** Create `backend/internal/app/auth/auth_helpers_test.go`:
  ```go
  package auth_test

  import (
  	"testing"

  	"github.com/zulkhair/pustaka/backend/internal/adapter/store"
  	"github.com/zulkhair/pustaka/backend/internal/testsupport"
  )

  // newTestStore forwards to the shared harness so every auth test uses the same
  // testcontainers-backed store (no ad-hoc setup).
  func newTestStore(t *testing.T) (*store.Store, func()) {
  	return testsupport.NewTestStore(t)
  }
  ```

- [ ] **Step 3: Run the test and confirm it FAILS.** Run `go test ./internal/app/auth/...`. Expected FAIL: the package does not compile — `undefined: (*auth.Service).VerifyEmail`.

- [ ] **Step 4: Add `issueTokens` and `VerifyEmail` to `service.go`.** Append to `backend/internal/app/auth/service.go`. Order per contract: lookup user (not found → `ErrInvalidCode`, enumeration-safe); fetch active verification (none → `ErrInvalidCode`); expired → `ErrCodeExpired`; constant-time `CheckCode` — wrong → atomically increment (the UPDATE...RETURNING count) then enforce `MaxAttempts` on the RETURNED count (increment-then-compare) → `ErrTooManyAttempts` if at/over cap, else `ErrInvalidCode`; right → `ExecTx` { `SetUserEmailVerified`, `ConsumeEmailVerification`, `CreateSession` } and return `Tokens`. Add `"github.com/zulkhair/pustaka/backend/internal/pkg/jwt"` to the imports:
  ```go
  func (s *Service) issueTokens(ctx context.Context, u domain.User) (Tokens, error) {
  	access, err := jwt.GenerateAccess(u.ID, u.Role, s.cfg.JWTSecret, s.cfg.AccessTTL)
  	if err != nil {
  		return Tokens{}, fmt.Errorf("generate access token: %w", err)
  	}
  	refresh, err := jwt.GenerateRefreshToken()
  	if err != nil {
  		return Tokens{}, fmt.Errorf("generate refresh token: %w", err)
  	}
  	_, err = s.store.CreateSession(ctx, domain.CreateSessionParams{
  		ID:               uuid.NewString(),
  		UserID:           u.ID,
  		RefreshTokenHash: hash.HashRefreshToken(refresh),
  		ExpiresAt:        time.Now().Add(s.cfg.RefreshTTL),
  	})
  	if err != nil {
  		return Tokens{}, fmt.Errorf("create session: %w", err)
  	}
  	return Tokens{
  		AccessToken:  access,
  		RefreshToken: refresh,
  		ExpiresIn:    int(s.cfg.AccessTTL.Seconds()),
  	}, nil
  }

  func (s *Service) VerifyEmail(ctx context.Context, in VerifyInput) (Tokens, error) {
  	email := normalizeEmail(in.Email)

  	user, err := s.store.GetUserByEmail(ctx, email)
  	if err != nil {
  		if errors.Is(err, domain.ErrNotFound) {
  			return Tokens{}, domain.ErrInvalidCode
  		}
  		return Tokens{}, err
  	}

  	ev, err := s.store.GetActiveEmailVerification(ctx, user.ID)
  	if err != nil {
  		if errors.Is(err, domain.ErrNotFound) {
  			return Tokens{}, domain.ErrInvalidCode
  		}
  		return Tokens{}, err
  	}

  	if time.Now().After(ev.ExpiresAt) {
  		return Tokens{}, domain.ErrCodeExpired
  	}

  	if !hash.CheckCode(ev.CodeHash, in.Code) {
  		// Atomic increment-then-compare: the UPDATE returns the new count.
  		attempts, incErr := s.store.IncrementVerificationAttempts(ctx, ev.ID)
  		if incErr != nil {
  			return Tokens{}, incErr
  		}
  		if attempts >= s.cfg.MaxAttempts {
  			return Tokens{}, domain.ErrTooManyAttempts
  		}
  		return Tokens{}, domain.ErrInvalidCode
  	}

  	var tokens Tokens
  	txErr := s.store.ExecTx(ctx, func(st domain.Store) error {
  		if err := st.SetUserEmailVerified(ctx, user.ID); err != nil {
  			return err
  		}
  		if err := st.ConsumeEmailVerification(ctx, ev.ID); err != nil {
  			return err
  		}
  		access, err := jwt.GenerateAccess(user.ID, user.Role, s.cfg.JWTSecret, s.cfg.AccessTTL)
  		if err != nil {
  			return fmt.Errorf("generate access token: %w", err)
  		}
  		refresh, err := jwt.GenerateRefreshToken()
  		if err != nil {
  			return fmt.Errorf("generate refresh token: %w", err)
  		}
  		if _, err := st.CreateSession(ctx, domain.CreateSessionParams{
  			ID:               uuid.NewString(),
  			UserID:           user.ID,
  			RefreshTokenHash: hash.HashRefreshToken(refresh),
  			ExpiresAt:        time.Now().Add(s.cfg.RefreshTTL),
  		}); err != nil {
  			return err
  		}
  		tokens = Tokens{
  			AccessToken:  access,
  			RefreshToken: refresh,
  			ExpiresIn:    int(s.cfg.AccessTTL.Seconds()),
  		}
  		return nil
  	})
  	if txErr != nil {
  		return Tokens{}, txErr
  	}
  	return tokens, nil
  }
  ```
  > `issueTokens` is provided for Login/Refresh to reuse for the non-transactional path; `VerifyEmail` inlines the same steps inside its `ExecTx` so verification + session creation are atomic. The attempt cap is enforced on the RETURNED count from the atomic `IncrementVerificationAttempts` (increment-then-compare), so concurrent wrong guesses can't slip past the cap.

- [ ] **Step 5: Run the test and confirm it PASSES.** Run `go test ./internal/app/auth/...`. Expected PASS: wrong code → `ErrInvalidCode` with `Attempts == 1`; correct code → `EmailVerified == true`, non-empty `Tokens` with `ExpiresIn == 900`, and a session row found by `HashRefreshToken(tokens.RefreshToken)`.

- [ ] **Step 6: Add the verify-email handler + DTOs.** Append to `backend/internal/adapter/httpapi/auth_handler.go`. Parse `VerifyReq`, call `svc.VerifyEmail`, map errors via `mapAuthError`, and on success return the `TokensDTO`:
  ```go
  type VerifyReq struct {
  	Email string `json:"email"`
  	Code  string `json:"code"`
  }

  type TokensDTO struct {
  	AccessToken  string `json:"accessToken"`
  	RefreshToken string `json:"refreshToken"`
  	ExpiresIn    int    `json:"expiresIn"`
  }

  func (h *AuthHandler) VerifyEmail(c *fiber.Ctx) error {
  	var req VerifyReq
  	if err := c.BodyParser(&req); err != nil {
  		return Fail(c, fiber.StatusBadRequest, "invalid request body")
  	}
  	tokens, err := h.svc.VerifyEmail(c.Context(), auth.VerifyInput{
  		Email: req.Email,
  		Code:  req.Code,
  	})
  	if err != nil {
  		return mapAuthError(c, err)
  	}
  	return OK(c, TokensDTO{
  		AccessToken:  tokens.AccessToken,
  		RefreshToken: tokens.RefreshToken,
  		ExpiresIn:    tokens.ExpiresIn,
  	})
  }
  ```

- [ ] **Step 7: Confirm the auth package compiles and tests pass.** Run `go vet ./internal/app/auth/...` (clean) and `go test ./internal/app/auth/...`. Expected PASS.

- [ ] **Step 8: Commit.** `git add backend/internal/app/auth backend/internal/adapter/httpapi && git commit -m "feat(auth): add verify-email use-case, issueTokens, and handler"` (no `Co-Authored-By` trailer).

---

### Task 14: `AuthService.ResendVerification` + handler (silent generic 200)

**Files:**
- Modify: `backend/internal/app/auth/service.go`
- Modify: `backend/internal/adapter/httpapi/auth_handler.go`
- Modify: `backend/internal/adapter/httpapi/router.go`
- Test: `backend/internal/app/auth/resend_test.go`
- Test: `backend/internal/adapter/httpapi/auth_resend_test.go`

**Interfaces:**
- Consumes (from earlier tasks):
  - `domain.Store`: `GetUserByEmail`, `GetActiveEmailVerification`, `DeleteEmailVerificationsByUser`, `CreateEmailVerification`
  - `domain.Mailer`: `SendVerificationCode`
  - `hash.GenerateNumericCode`, `hash.HashCode`
  - `domain.ErrResendCooldown` (internal-only), `domain.ErrNotFound`
  - `httpapi.OK`, `httpapi.Fail`, `mapAuthError` (Task 12)
  - `auth.New` (Task 12)
  - the shared harness: `testsupport.NewTestStore`, `testsupport.BackdateVerification`, `mail.MockMailer`, and the httpapi harness `newTestApp` / `doJSON` / `seedUnverifiedUser` (Task 11)
  - `config.Config` fields `CodeTTL`, `ResendCooldown`, `BcryptCost`
- Produces (later tasks rely on):
  - `func (s *Service) ResendVerification(ctx context.Context, email string) error` — enforces cooldown as a SILENT no-op (returns nil), and is a no-op for unknown/verified emails
  - HTTP route `POST /api/auth/resend-verification` — the handler ALWAYS returns the same generic 200

- [ ] **Step 1: Write the failing service test for the silent-cooldown + no-op + success paths.** Create `backend/internal/app/auth/resend_test.go`. The cooldown case asserts a SILENT no-op (no error, no mail), and uses `testsupport.BackdateVerification` to age a row past the cooldown for the success case:
  ```go
  package auth_test

  import (
  	"context"
  	"testing"
  	"time"

  	"github.com/google/uuid"
  	"github.com/stretchr/testify/require"

  	"github.com/zulkhair/pustaka/backend/internal/adapter/mail"
  	"github.com/zulkhair/pustaka/backend/internal/app/auth"
  	"github.com/zulkhair/pustaka/backend/internal/config"
  	"github.com/zulkhair/pustaka/backend/internal/domain"
  	"github.com/zulkhair/pustaka/backend/internal/testsupport"
  )

  func newResendService(t *testing.T, store domain.Store) (*auth.Service, *mail.MockMailer) {
  	t.Helper()
  	mailer := mail.NewMockMailer()
  	cfg := config.Config{
  		BcryptCost:     4,
  		CodeTTL:        15 * time.Minute,
  		ResendCooldown: 60 * time.Second,
  	}
  	return auth.New(store, mailer, cfg), mailer
  }

  func TestResendVerification_UnknownEmail_NoOp(t *testing.T) {
  	store, cleanup := newTestStore(t)
  	defer cleanup()
  	svc, mailer := newResendService(t, store)

  	err := svc.ResendVerification(context.Background(), "nobody@example.com")
  	require.NoError(t, err)
  	require.Len(t, mailer.Sends, 0, "no mail must be sent for unknown email")
  }

  func TestResendVerification_AlreadyVerified_NoOp(t *testing.T) {
  	store, cleanup := newTestStore(t)
  	defer cleanup()
  	svc, mailer := newResendService(t, store)
  	ctx := context.Background()

  	u, err := store.CreateUser(ctx, domain.CreateUserParams{
  		ID: uuid.NewString(), Username: "verified", Email: "verified@example.com",
  		PasswordHash: "x", Role: domain.RoleUser,
  	})
  	require.NoError(t, err)
  	require.NoError(t, store.SetUserEmailVerified(ctx, u.ID))

  	err = svc.ResendVerification(ctx, "verified@example.com")
  	require.NoError(t, err)
  	require.Len(t, mailer.Sends, 0, "verified users must not get a resend")
  }

  func TestResendVerification_WithinCooldown_SilentNoOp(t *testing.T) {
  	store, cleanup := newTestStore(t)
  	defer cleanup()
  	svc, mailer := newResendService(t, store)
  	ctx := context.Background()

  	u, err := store.CreateUser(ctx, domain.CreateUserParams{
  		ID: uuid.NewString(), Username: "fresh", Email: "fresh@example.com",
  		PasswordHash: "x", Role: domain.RoleUser,
  	})
  	require.NoError(t, err)
  	_, err = store.CreateEmailVerification(ctx, domain.CreateEmailVerificationParams{
  		ID: uuid.NewString(), UserID: u.ID, CodeHash: "h",
  		ExpiresAt: time.Now().Add(15 * time.Minute),
  	})
  	require.NoError(t, err)

  	// SILENT no-op: the service swallows the cooldown internally and returns nil.
  	err = svc.ResendVerification(ctx, "fresh@example.com")
  	require.NoError(t, err)
  	require.Len(t, mailer.Sends, 0)
  }

  func TestResendVerification_AfterCooldown_SendsNewCode(t *testing.T) {
  	store, cleanup := newTestStore(t)
  	defer cleanup()
  	svc, mailer := newResendService(t, store)
  	ctx := context.Background()

  	u, err := store.CreateUser(ctx, domain.CreateUserParams{
  		ID: uuid.NewString(), Username: "stale", Email: "stale@example.com",
  		PasswordHash: "x", Role: domain.RoleUser,
  	})
  	require.NoError(t, err)
  	_, err = store.CreateEmailVerification(ctx, domain.CreateEmailVerificationParams{
  		ID: uuid.NewString(), UserID: u.ID, CodeHash: "old",
  		ExpiresAt: time.Now().Add(15 * time.Minute),
  	})
  	require.NoError(t, err)
  	// Age the verification past the cooldown window via the shared harness helper.
  	testsupport.BackdateVerification(t, store.Pool(), u.ID, time.Now().Add(-5*time.Minute))

  	err = svc.ResendVerification(ctx, "stale@example.com")
  	require.NoError(t, err)
  	require.Len(t, mailer.Sends, 1)
  	require.Equal(t, "stale@example.com", mailer.LastEmail)
  	require.Len(t, mailer.LastCode, 6)
  }
  ```

- [ ] **Step 2: Run the test — expect FAIL.** Run `cd backend && go test ./internal/app/auth/ -run TestResendVerification 2>&1 | tail -n 20`. Expected FAIL: `undefined: (*auth.Service).ResendVerification`.

- [ ] **Step 3: Implement `ResendVerification` (silent, enumeration-safe).** The cooldown is enforced internally by returning `domain.ErrResendCooldown` and immediately swallowing it to nil (so the method is a silent no-op; the sentinel never escapes). Append to `backend/internal/app/auth/service.go`:
  ```go
  func (s *Service) ResendVerification(ctx context.Context, email string) error {
  	email = normalizeEmail(email)
  	if err := s.resendVerification(ctx, email); err != nil {
  		if errors.Is(err, domain.ErrResendCooldown) {
  			return nil // SILENT no-op: cooldown is never surfaced
  		}
  		return err
  	}
  	return nil
  }

  func (s *Service) resendVerification(ctx context.Context, email string) error {
  	u, err := s.store.GetUserByEmail(ctx, email)
  	if err != nil {
  		if errors.Is(err, domain.ErrNotFound) {
  			return nil // enumeration-safe no-op
  		}
  		return err
  	}
  	if u.EmailVerified {
  		return nil // enumeration-safe no-op
  	}

  	existing, err := s.store.GetActiveEmailVerification(ctx, u.ID)
  	if err == nil {
  		if time.Since(existing.CreatedAt) < s.cfg.ResendCooldown {
  			return domain.ErrResendCooldown
  		}
  	} else if !errors.Is(err, domain.ErrNotFound) {
  		return err
  	}

  	if err := s.store.DeleteEmailVerificationsByUser(ctx, u.ID); err != nil {
  		return err
  	}

  	code, err := hash.GenerateNumericCode(6)
  	if err != nil {
  		return err
  	}
  	codeHash, err := hash.HashCode(code, s.cfg.BcryptCost)
  	if err != nil {
  		return err
  	}
  	if _, err := s.store.CreateEmailVerification(ctx, domain.CreateEmailVerificationParams{
  		ID:        uuid.NewString(),
  		UserID:    u.ID,
  		CodeHash:  codeHash,
  		ExpiresAt: time.Now().Add(s.cfg.CodeTTL),
  	}); err != nil {
  		return err
  	}

  	return s.mailer.SendVerificationCode(ctx, u.Email, code)
  }
  ```

- [ ] **Step 4: Run the test — expect PASS.** Run `cd backend && go test ./internal/app/auth/ -run TestResendVerification 2>&1 | tail -n 20`. Expected PASS: unknown/verified are no-ops, within-cooldown is a silent no-op (no error, no mail), after-cooldown sends a fresh 6-digit code.

- [ ] **Step 5: Write failing handler test (uniform generic 200).** Create `backend/internal/adapter/httpapi/auth_resend_test.go`. It asserts a UNIFORM 200 across unknown / verified / cooldown / fresh, and 400 only on a malformed body:
  ```go
  package httpapi_test

  import (
  	"context"
  	"net/http"
  	"testing"

  	"github.com/stretchr/testify/require"
  )

  func TestResendHandler_UniformGeneric200(t *testing.T) {
  	ta := newTestApp(t)

  	// unknown email
  	resp := doJSON(t, ta, http.MethodPost, "/api/auth/resend-verification",
  		map[string]string{"email": "nobody@example.com"})
  	require.Equal(t, http.StatusOK, resp.StatusCode)

  	// verified user
  	v := seedUnverifiedUser(t, ta.store, "verif", "verif@example.com")
  	require.NoError(t, ta.store.SetUserEmailVerified(context.Background(), v.ID))
  	resp = doJSON(t, ta, http.MethodPost, "/api/auth/resend-verification",
  		map[string]string{"email": "verif@example.com"})
  	require.Equal(t, http.StatusOK, resp.StatusCode)

  	// fresh unverified user, immediate resend -> within cooldown, still 200 (silent)
  	seedUnverifiedUser(t, ta.store, "fresh", "fresh@example.com")
  	resp = doJSON(t, ta, http.MethodPost, "/api/auth/resend-verification",
  		map[string]string{"email": "fresh@example.com"})
  	require.Equal(t, http.StatusOK, resp.StatusCode)
  }

  func TestResendHandler_BadBody_400(t *testing.T) {
  	ta := newTestApp(t)
  	resp := doRaw(t, ta, http.MethodPost, "/api/auth/resend-verification", "not-json")
  	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
  }
  ```

- [ ] **Step 6: Run the handler test — expect FAIL.** Run `cd backend && go test ./internal/adapter/httpapi/ -run TestResendHandler 2>&1 | tail -n 20`. Expected FAIL: `404` because the route `POST /api/auth/resend-verification` is not registered yet (or the handler method is undefined).

- [ ] **Step 7: Implement the resend handler and wire the route.** Because the service already swallows the cooldown, the handler simply returns a generic 200 on success. Add to `backend/internal/adapter/httpapi/auth_handler.go`:
  ```go
  type ResendReq struct {
  	Email string `json:"email"`
  }

  func (h *AuthHandler) ResendVerification(c *fiber.Ctx) error {
  	var req ResendReq
  	if err := c.BodyParser(&req); err != nil {
  		return Fail(c, fiber.StatusBadRequest, "invalid request body")
  	}
  	if req.Email == "" {
  		return Fail(c, fiber.StatusBadRequest, "email is required")
  	}
  	if err := h.svc.ResendVerification(c.Context(), req.Email); err != nil {
  		return mapAuthError(c, err)
  	}
  	return OK(c, nil) // always the same generic 200 (no cooldown signal leaks)
  }
  ```
  The route is wired in `router.go` (Task 19); when that task lands, add inside the `/auth` group:
  ```go
  authGrp.Post("/resend-verification", rl(), deps.Auth.ResendVerification)
  ```
  > `mapAuthError` never surfaces `ErrResendCooldown` (the service already swallowed it), so no 429 path exists for resend. The generic 200 is uniform across unknown/verified/cooldown/fresh.

- [ ] **Step 8: Run the handler test — expect PASS.** Run `cd backend && go test ./internal/adapter/httpapi/ -run TestResendHandler 2>&1 | tail -n 20`. Expected PASS: uniform 200 across unknown/verified/cooldown/fresh; 400 on bad body.

- [ ] **Step 9: Vet and commit.** Run `cd backend && go vet ./... && go test ./internal/app/auth/ ./internal/adapter/httpapi/ -run 'Resend' 2>&1 | tail -n 5`, then `git add -A && git commit -m "feat: add resend-verification use-case and endpoint with silent cooldown"` (no `Co-Authored-By` trailer).

---

## Cluster D — Login & Session Lifecycle (Tasks 15–17)

### Task 15: `AuthService.Login` + handler

**Files:**
- Modify: `backend/internal/app/auth/service.go`
- Modify: `backend/internal/adapter/httpapi/auth_handler.go`
- Modify: `backend/internal/adapter/httpapi/router.go`
- Test: `backend/internal/app/auth/login_test.go`
- Test: `backend/internal/adapter/httpapi/auth_login_test.go`

**Interfaces:**
- Consumes (from earlier tasks):
  - `domain.Store`: `GetUserByEmail`, `GetUserByUsername`, `CreateSession`
  - `hash.CheckPassword`, `hash.HashPassword`, `hash.HashRefreshToken`
  - `jwt.GenerateAccess`, `jwt.GenerateRefreshToken`
  - `domain.ErrInvalidCredentials`, `domain.ErrEmailNotVerified`, `domain.ErrNotFound`
  - `config.Config` fields `JWTSecret`, `AccessTTL`, `RefreshTTL`
  - `auth.Tokens`, the `issueTokens` helper defined in Task 13, and `normalizeEmail` (Task 12)
  - the shared harness seeders `seedVerifiedUser`, `seedUnverifiedUserWithPassword` and `doJSONBody` (Task 11)
- Produces (later tasks rely on):
  - `func (s *Service) Login(ctx context.Context, in LoginInput) (Tokens, error)` (`LoginInput` already declared in Task 12)
  - HTTP route `POST /api/auth/login`, request DTO `LoginReq{identifier,password}`, response `TokensDTO{accessToken,refreshToken,expiresIn}`

- [ ] **Step 1: Write failing service test for all login paths.** Create `backend/internal/app/auth/login_test.go`:
  ```go
  package auth_test

  import (
  	"context"
  	"testing"
  	"time"

  	"github.com/google/uuid"
  	"github.com/stretchr/testify/require"

  	"github.com/zulkhair/pustaka/backend/internal/app/auth"
  	"github.com/zulkhair/pustaka/backend/internal/config"
  	"github.com/zulkhair/pustaka/backend/internal/domain"
  	"github.com/zulkhair/pustaka/backend/internal/pkg/hash"
  )

  func newLoginService(t *testing.T, store domain.Store) *auth.Service {
  	t.Helper()
  	cfg := config.Config{
  		BcryptCost: 4,
  		JWTSecret:  "test-secret",
  		AccessTTL:  15 * time.Minute,
  		RefreshTTL: 720 * time.Hour,
  	}
  	return auth.New(store, mail.NewMockMailer(), cfg)
  }

  func seedVerifiedUserSvc(t *testing.T, store domain.Store, username, email, pw string) domain.User {
  	t.Helper()
  	ctx := context.Background()
  	ph, err := hash.HashPassword(pw, 4)
  	require.NoError(t, err)
  	u, err := store.CreateUser(ctx, domain.CreateUserParams{
  		ID: uuid.NewString(), Username: username, Email: email,
  		PasswordHash: ph, Role: domain.RoleUser,
  	})
  	require.NoError(t, err)
  	require.NoError(t, store.SetUserEmailVerified(ctx, u.ID))
  	u.EmailVerified = true
  	return u
  }

  func TestLogin_GoodCreds_IssuesTokensAndSession(t *testing.T) {
  	store, cleanup := newTestStore(t)
  	defer cleanup()
  	svc := newLoginService(t, store)
  	u := seedVerifiedUserSvc(t, store, "alice", "alice@example.com", "hunter2pw")

  	tok, err := svc.Login(context.Background(), auth.LoginInput{
  		Identifier: "alice@example.com", Password: "hunter2pw",
  	})
  	require.NoError(t, err)
  	require.NotEmpty(t, tok.AccessToken)
  	require.NotEmpty(t, tok.RefreshToken)
  	require.Equal(t, int((15 * time.Minute).Seconds()), tok.ExpiresIn)

  	sess, err := store.GetSessionByTokenHash(context.Background(), hash.HashRefreshToken(tok.RefreshToken))
  	require.NoError(t, err)
  	require.Equal(t, u.ID, sess.UserID)
  }

  func TestLogin_ByUsername_Works(t *testing.T) {
  	store, cleanup := newTestStore(t)
  	defer cleanup()
  	svc := newLoginService(t, store)
  	seedVerifiedUserSvc(t, store, "bob", "bob@example.com", "passwordpw")

  	tok, err := svc.Login(context.Background(), auth.LoginInput{
  		Identifier: "bob", Password: "passwordpw",
  	})
  	require.NoError(t, err)
  	require.NotEmpty(t, tok.AccessToken)
  }

  func TestLogin_ByEmail_NormalizesCase(t *testing.T) {
  	store, cleanup := newTestStore(t)
  	defer cleanup()
  	svc := newLoginService(t, store)
  	seedVerifiedUserSvc(t, store, "cara", "cara@example.com", "carapassword1")

  	tok, err := svc.Login(context.Background(), auth.LoginInput{
  		Identifier: "  Cara@Example.com ", Password: "carapassword1",
  	})
  	require.NoError(t, err)
  	require.NotEmpty(t, tok.AccessToken)
  }

  func TestLogin_WrongPassword_InvalidCredentials(t *testing.T) {
  	store, cleanup := newTestStore(t)
  	defer cleanup()
  	svc := newLoginService(t, store)
  	seedVerifiedUserSvc(t, store, "carol", "carol@example.com", "rightpassword")

  	_, err := svc.Login(context.Background(), auth.LoginInput{
  		Identifier: "carol@example.com", Password: "wrongpassword",
  	})
  	require.ErrorIs(t, err, domain.ErrInvalidCredentials)
  }

  func TestLogin_UnknownIdentifier_InvalidCredentials(t *testing.T) {
  	store, cleanup := newTestStore(t)
  	defer cleanup()
  	svc := newLoginService(t, store)

  	_, err := svc.Login(context.Background(), auth.LoginInput{
  		Identifier: "ghost@example.com", Password: "whatever123",
  	})
  	require.ErrorIs(t, err, domain.ErrInvalidCredentials) // identical to wrong-password = enumeration-safe
  }

  func TestLogin_Unverified_EmailNotVerified(t *testing.T) {
  	store, cleanup := newTestStore(t)
  	defer cleanup()
  	svc := newLoginService(t, store)
  	ctx := context.Background()
  	ph, err := hash.HashPassword("secretpass1", 4)
  	require.NoError(t, err)
  	_, err = store.CreateUser(ctx, domain.CreateUserParams{
  		ID: uuid.NewString(), Username: "dan", Email: "dan@example.com",
  		PasswordHash: ph, Role: domain.RoleUser,
  	})
  	require.NoError(t, err)

  	_, err = svc.Login(ctx, auth.LoginInput{Identifier: "dan@example.com", Password: "secretpass1"})
  	require.ErrorIs(t, err, domain.ErrEmailNotVerified)
  }
  ```
  > This file also needs the `mail` import: `"github.com/zulkhair/pustaka/backend/internal/adapter/mail"` (used by `newLoginService`).

- [ ] **Step 2: Run the test — expect FAIL.** Run `cd backend && go test ./internal/app/auth/ -run TestLogin 2>&1 | tail -n 20`. Expected FAIL: `undefined: (*auth.Service).Login`.

- [ ] **Step 3: Implement `Login` (reusing `issueTokens`).** `issueTokens` is already defined in Task 13 — do NOT redefine it; Login simply calls it. When the identifier contains `@`, look up via `GetUserByEmail(normalizeEmail(...))` so it matches Register's lowercase+trim. Append to `backend/internal/app/auth/service.go`:
  ```go
  func (s *Service) Login(ctx context.Context, in LoginInput) (Tokens, error) {
  	var (
  		u   domain.User
  		err error
  	)
  	if strings.Contains(in.Identifier, "@") {
  		u, err = s.store.GetUserByEmail(ctx, normalizeEmail(in.Identifier))
  	} else {
  		u, err = s.store.GetUserByUsername(ctx, strings.TrimSpace(in.Identifier))
  	}
  	if err != nil {
  		if errors.Is(err, domain.ErrNotFound) {
  			return Tokens{}, domain.ErrInvalidCredentials // enumeration-safe
  		}
  		return Tokens{}, err
  	}

  	// Deliberate trade-off: an unknown identifier OR a wrong password both return
  	// the identical ErrInvalidCredentials (enumeration-safe). The verified-status
  	// signal (ErrEmailNotVerified) is only revealed AFTER valid credentials, so it
  	// cannot be used to probe which emails exist.
  	if !hash.CheckPassword(u.PasswordHash, in.Password) {
  		return Tokens{}, domain.ErrInvalidCredentials
  	}
  	if !u.EmailVerified {
  		return Tokens{}, domain.ErrEmailNotVerified
  	}

  	return s.issueTokens(ctx, u)
  }
  ```

- [ ] **Step 4: Run the test — expect PASS.** Run `cd backend && go test ./internal/app/auth/ -run TestLogin 2>&1 | tail -n 20`. Expected PASS: good creds issue tokens + session, username + case-normalized email login works, wrong-password and unknown-identifier both `ErrInvalidCredentials`, unverified `ErrEmailNotVerified`.

- [ ] **Step 5: Write failing handler test (status codes + identical wrong-creds message).** Create `backend/internal/adapter/httpapi/auth_login_test.go`:
  ```go
  package httpapi_test

  import (
  	"net/http"
  	"testing"

  	"github.com/stretchr/testify/require"
  )

  func TestLoginHandler_Good_200WithTokens(t *testing.T) {
  	ta := newTestApp(t)
  	seedVerifiedUser(t, ta.store, "eve", "eve@example.com", "longpassword1")

  	resp, body := doJSONBody(t, ta, http.MethodPost, "/api/auth/login",
  		map[string]string{"identifier": "eve@example.com", "password": "longpassword1"})
  	require.Equal(t, http.StatusOK, resp.StatusCode)

  	data := body["data"].(map[string]any)
  	require.NotEmpty(t, data["accessToken"])
  	require.NotEmpty(t, data["refreshToken"])
  	require.Greater(t, data["expiresIn"].(float64), float64(0))
  }

  func TestLoginHandler_WrongPassword_401(t *testing.T) {
  	ta := newTestApp(t)
  	seedVerifiedUser(t, ta.store, "frank", "frank@example.com", "correctpass1")

  	resp, body := doJSONBody(t, ta, http.MethodPost, "/api/auth/login",
  		map[string]string{"identifier": "frank@example.com", "password": "nope"})
  	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
  	require.Equal(t, wrongCredsMsg(t, ta), body["message"])
  }

  func TestLoginHandler_UnknownIdentifier_401_SameMessage(t *testing.T) {
  	ta := newTestApp(t)
  	resp, body := doJSONBody(t, ta, http.MethodPost, "/api/auth/login",
  		map[string]string{"identifier": "ghost@example.com", "password": "whatever"})
  	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
  	require.Equal(t, wrongCredsMsg(t, ta), body["message"]) // enumeration-safe: identical to wrong-password
  }

  func TestLoginHandler_Unverified_401(t *testing.T) {
  	ta := newTestApp(t)
  	seedUnverifiedUserWithPassword(t, ta.store, "gina", "gina@example.com", "verifyme123")

  	resp, _ := doJSONBody(t, ta, http.MethodPost, "/api/auth/login",
  		map[string]string{"identifier": "gina@example.com", "password": "verifyme123"})
  	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
  }

  // wrongCredsMsg captures the generic invalid-credentials message once so the
  // two enumeration-safe assertions compare against the same source of truth.
  func wrongCredsMsg(t *testing.T, ta *testApp) string {
  	t.Helper()
  	_, body := doJSONBody(t, ta, http.MethodPost, "/api/auth/login",
  		map[string]string{"identifier": "zzz@example.com", "password": "x"})
  	return body["message"].(string)
  }
  ```

- [ ] **Step 6: Run the handler test — expect FAIL.** Run `cd backend && go test ./internal/adapter/httpapi/ -run TestLoginHandler 2>&1 | tail -n 20`. Expected FAIL: route `POST /api/auth/login` returns 404 (handler/route not registered).

- [ ] **Step 7: Implement the login handler and wire the route.** Add to `backend/internal/adapter/httpapi/auth_handler.go`:
  ```go
  type LoginReq struct {
  	Identifier string `json:"identifier"`
  	Password   string `json:"password"`
  }

  func (h *AuthHandler) Login(c *fiber.Ctx) error {
  	var req LoginReq
  	if err := c.BodyParser(&req); err != nil {
  		return Fail(c, fiber.StatusBadRequest, "invalid request body")
  	}
  	if req.Identifier == "" || req.Password == "" {
  		return Fail(c, fiber.StatusBadRequest, "identifier and password are required")
  	}
  	tok, err := h.svc.Login(c.Context(), auth.LoginInput{
  		Identifier: req.Identifier,
  		Password:   req.Password,
  	})
  	if err != nil {
  		return mapAuthError(c, err)
  	}
  	return OK(c, TokensDTO{
  		AccessToken:  tok.AccessToken,
  		RefreshToken: tok.RefreshToken,
  		ExpiresIn:    tok.ExpiresIn,
  	})
  }
  ```
  Wire the route in `router.go` (Task 19), inside the `/auth` group:
  ```go
  authGrp.Post("/login", rl(), deps.Auth.Login)
  ```
  > `mapAuthError` maps both `domain.ErrInvalidCredentials` and `domain.ErrEmailNotVerified` to 401 with the identical generic "unauthorized" message (per the contract table), so the wrong-password / unknown-identifier / unverified responses are indistinguishable. `TokensDTO` was declared in Task 13 — reuse it; do NOT redeclare.

- [ ] **Step 8: Run the handler test — expect PASS.** Run `cd backend && go test ./internal/adapter/httpapi/ -run TestLoginHandler 2>&1 | tail -n 20`. Expected PASS: 200 with tokens, 401 wrong-password, 401 unknown-identifier with the identical message, 401 unverified.

- [ ] **Step 9: Vet and commit.** Run `cd backend && go vet ./... && go test ./internal/app/auth/ ./internal/adapter/httpapi/ -run 'Login' 2>&1 | tail -n 5`, then `git add -A && git commit -m "feat: add login use-case and endpoint with enumeration-safe errors"` (no `Co-Authored-By` trailer).

---

### Task 16: `AuthService.Refresh` + handler (rotation + theft response)

**Files:**
- Modify: `backend/internal/app/auth/service.go`
- Modify: `backend/internal/adapter/httpapi/auth_handler.go`
- Modify: `backend/internal/adapter/httpapi/router.go`
- Test: `backend/internal/app/auth/refresh_test.go`
- Test: `backend/internal/adapter/httpapi/auth_refresh_test.go`

**Interfaces:**
- Consumes (from earlier tasks):
  - `domain.Store`: `GetSessionByTokenHash`, `GetUserByID`, `RevokeSession`, `RevokeAllUserSessions`, `CreateSession`, `ExecTx`
  - `hash.HashRefreshToken`
  - `jwt.GenerateAccess`, `jwt.GenerateRefreshToken`
  - `Service.Login` (Task 15, for tests to mint a starting token)
  - `domain.ErrUnauthorized`, `domain.ErrNotFound`
  - the shared harness seeders + `doJSONBody`/`doRaw` (Task 11)
- Produces (later tasks rely on):
  - `func (s *Service) Refresh(ctx context.Context, refreshToken string) (Tokens, error)`
  - HTTP route `POST /api/auth/refresh`, request DTO `RefreshReq{refreshToken}`

- [ ] **Step 1: Write failing service test: rotation, reuse-as-theft, expiry.** Create `backend/internal/app/auth/refresh_test.go`. The reuse case asserts that replaying a rotated (revoked) token kills the LIVE session too (theft response):
  ```go
  package auth_test

  import (
  	"context"
  	"testing"
  	"time"

  	"github.com/google/uuid"
  	"github.com/stretchr/testify/require"

  	"github.com/zulkhair/pustaka/backend/internal/app/auth"
  	"github.com/zulkhair/pustaka/backend/internal/domain"
  	"github.com/zulkhair/pustaka/backend/internal/pkg/hash"
  )

  func TestRefresh_Valid_RotatesAndRevokesOld(t *testing.T) {
  	store, cleanup := newTestStore(t)
  	defer cleanup()
  	svc := newLoginService(t, store)
  	ctx := context.Background()
  	seedVerifiedUserSvc(t, store, "rita", "rita@example.com", "ritapassword1")

  	first, err := svc.Login(ctx, auth.LoginInput{Identifier: "rita@example.com", Password: "ritapassword1"})
  	require.NoError(t, err)

  	second, err := svc.Refresh(ctx, first.RefreshToken)
  	require.NoError(t, err)
  	require.NotEmpty(t, second.AccessToken)
  	require.NotEqual(t, first.RefreshToken, second.RefreshToken)

  	old, err := store.GetSessionByTokenHash(ctx, hash.HashRefreshToken(first.RefreshToken))
  	require.NoError(t, err)
  	require.NotNil(t, old.RevokedAt, "old session must be revoked after rotation")

  	fresh, err := store.GetSessionByTokenHash(ctx, hash.HashRefreshToken(second.RefreshToken))
  	require.NoError(t, err)
  	require.Nil(t, fresh.RevokedAt)
  }

  func TestRefresh_ReuseAfterRotation_RevokesAllAndUnauthorized(t *testing.T) {
  	store, cleanup := newTestStore(t)
  	defer cleanup()
  	svc := newLoginService(t, store)
  	ctx := context.Background()
  	seedVerifiedUserSvc(t, store, "sam", "sam@example.com", "sampassword1")

  	first, err := svc.Login(ctx, auth.LoginInput{Identifier: "sam@example.com", Password: "sampassword1"})
  	require.NoError(t, err)
  	second, err := svc.Refresh(ctx, first.RefreshToken)
  	require.NoError(t, err)

  	// Replaying the old (now-revoked) token is theft: reject AND kill the live session.
  	_, err = svc.Refresh(ctx, first.RefreshToken)
  	require.ErrorIs(t, err, domain.ErrUnauthorized)

  	live, err := store.GetSessionByTokenHash(ctx, hash.HashRefreshToken(second.RefreshToken))
  	require.NoError(t, err)
  	require.NotNil(t, live.RevokedAt, "replay must revoke the live session too (theft response)")
  }

  func TestRefresh_UnknownToken_Unauthorized(t *testing.T) {
  	store, cleanup := newTestStore(t)
  	defer cleanup()
  	svc := newLoginService(t, store)

  	_, err := svc.Refresh(context.Background(), "this-token-was-never-issued")
  	require.ErrorIs(t, err, domain.ErrUnauthorized)
  }

  func TestRefresh_Expired_Unauthorized(t *testing.T) {
  	store, cleanup := newTestStore(t)
  	defer cleanup()
  	svc := newLoginService(t, store)
  	ctx := context.Background()
  	u := seedVerifiedUserSvc(t, store, "tina", "tina@example.com", "tinapassword1")

  	raw := "expired-refresh-token-value"
  	_, err := store.CreateSession(ctx, domain.CreateSessionParams{
  		ID:               uuid.NewString(),
  		UserID:           u.ID,
  		RefreshTokenHash: hash.HashRefreshToken(raw),
  		ExpiresAt:        time.Now().Add(-time.Minute), // already expired
  	})
  	require.NoError(t, err)

  	_, err = svc.Refresh(ctx, raw)
  	require.ErrorIs(t, err, domain.ErrUnauthorized)
  }
  ```

- [ ] **Step 2: Run the test — expect FAIL.** Run `cd backend && go test ./internal/app/auth/ -run TestRefresh 2>&1 | tail -n 20`. Expected FAIL: `undefined: (*auth.Service).Refresh`.

- [ ] **Step 3: Implement `Refresh` with transactional rotation + theft response.** When the matched session is already revoked, call `RevokeAllUserSessions` before returning `ErrUnauthorized` (a revoked token being replayed signals theft). Append to `backend/internal/app/auth/service.go`:
  ```go
  func (s *Service) Refresh(ctx context.Context, refreshToken string) (Tokens, error) {
  	tokenHash := hash.HashRefreshToken(refreshToken)

  	sess, err := s.store.GetSessionByTokenHash(ctx, tokenHash)
  	if err != nil {
  		if errors.Is(err, domain.ErrNotFound) {
  			return Tokens{}, domain.ErrUnauthorized
  		}
  		return Tokens{}, err
  	}

  	// Reuse of a revoked token is treated as theft: revoke ALL of the user's
  	// sessions (including the live rotated one) before rejecting.
  	if sess.RevokedAt != nil {
  		if revErr := s.store.RevokeAllUserSessions(ctx, sess.UserID); revErr != nil {
  			return Tokens{}, revErr
  		}
  		return Tokens{}, domain.ErrUnauthorized
  	}
  	if time.Now().After(sess.ExpiresAt) {
  		return Tokens{}, domain.ErrUnauthorized
  	}

  	u, err := s.store.GetUserByID(ctx, sess.UserID)
  	if err != nil {
  		if errors.Is(err, domain.ErrNotFound) {
  			return Tokens{}, domain.ErrUnauthorized
  		}
  		return Tokens{}, err
  	}

  	access, err := jwt.GenerateAccess(u.ID, u.Role, s.cfg.JWTSecret, s.cfg.AccessTTL)
  	if err != nil {
  		return Tokens{}, err
  	}
  	refresh, err := jwt.GenerateRefreshToken()
  	if err != nil {
  		return Tokens{}, err
  	}

  	if err := s.store.ExecTx(ctx, func(tx domain.Store) error {
  		if err := tx.RevokeSession(ctx, sess.ID); err != nil {
  			return err
  		}
  		_, err := tx.CreateSession(ctx, domain.CreateSessionParams{
  			ID:               uuid.NewString(),
  			UserID:           u.ID,
  			RefreshTokenHash: hash.HashRefreshToken(refresh),
  			ExpiresAt:        time.Now().Add(s.cfg.RefreshTTL),
  		})
  		return err
  	}); err != nil {
  		return Tokens{}, err
  	}

  	return Tokens{
  		AccessToken:  access,
  		RefreshToken: refresh,
  		ExpiresIn:    int(s.cfg.AccessTTL.Seconds()),
  	}, nil
  }
  ```
  > Token generation happens before the transaction so a CSPRNG/JWT error never leaves a revoked-but-unreplaced session. `issueTokens` is not reused for the rotation path because rotation must revoke and create atomically in one `ExecTx`.

- [ ] **Step 4: Run the test — expect PASS.** Run `cd backend && go test ./internal/app/auth/ -run TestRefresh 2>&1 | tail -n 20`. Expected PASS: valid rotation revokes old + issues new; replay of the old token is `ErrUnauthorized` AND revokes the live session; unknown token `ErrUnauthorized`; expired session `ErrUnauthorized`.

- [ ] **Step 5: Write failing handler test.** Create `backend/internal/adapter/httpapi/auth_refresh_test.go`:
  ```go
  package httpapi_test

  import (
  	"net/http"
  	"testing"

  	"github.com/stretchr/testify/require"
  )

  func TestRefreshHandler_Valid_200NewTokens(t *testing.T) {
  	ta := newTestApp(t)
  	seedVerifiedUser(t, ta.store, "uma", "uma@example.com", "umapassword1")

  	_, loginBody := doJSONBody(t, ta, http.MethodPost, "/api/auth/login",
  		map[string]string{"identifier": "uma@example.com", "password": "umapassword1"})
  	first := loginBody["data"].(map[string]any)["refreshToken"].(string)

  	resp, body := doJSONBody(t, ta, http.MethodPost, "/api/auth/refresh",
  		map[string]string{"refreshToken": first})
  	require.Equal(t, http.StatusOK, resp.StatusCode)
  	second := body["data"].(map[string]any)["refreshToken"].(string)
  	require.NotEqual(t, first, second)
  }

  func TestRefreshHandler_ReuseAfterRotation_401(t *testing.T) {
  	ta := newTestApp(t)
  	seedVerifiedUser(t, ta.store, "vic", "vic@example.com", "vicpassword1")

  	_, loginBody := doJSONBody(t, ta, http.MethodPost, "/api/auth/login",
  		map[string]string{"identifier": "vic@example.com", "password": "vicpassword1"})
  	first := loginBody["data"].(map[string]any)["refreshToken"].(string)

  	resp1, _ := doJSONBody(t, ta, http.MethodPost, "/api/auth/refresh",
  		map[string]string{"refreshToken": first})
  	require.Equal(t, http.StatusOK, resp1.StatusCode)

  	resp2, _ := doJSONBody(t, ta, http.MethodPost, "/api/auth/refresh",
  		map[string]string{"refreshToken": first})
  	require.Equal(t, http.StatusUnauthorized, resp2.StatusCode)
  }

  func TestRefreshHandler_BadBody_400(t *testing.T) {
  	ta := newTestApp(t)
  	resp := doRaw(t, ta, http.MethodPost, "/api/auth/refresh", "{bad")
  	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
  }
  ```

- [ ] **Step 6: Run the handler test — expect FAIL.** Run `cd backend && go test ./internal/adapter/httpapi/ -run TestRefreshHandler 2>&1 | tail -n 20`. Expected FAIL: 404 on `POST /api/auth/refresh` (route/handler missing).

- [ ] **Step 7: Implement the refresh handler and wire the route.** Add to `backend/internal/adapter/httpapi/auth_handler.go`:
  ```go
  type RefreshReq struct {
  	RefreshToken string `json:"refreshToken"`
  }

  func (h *AuthHandler) Refresh(c *fiber.Ctx) error {
  	var req RefreshReq
  	if err := c.BodyParser(&req); err != nil {
  		return Fail(c, fiber.StatusBadRequest, "invalid request body")
  	}
  	if req.RefreshToken == "" {
  		return Fail(c, fiber.StatusBadRequest, "refreshToken is required")
  	}
  	tok, err := h.svc.Refresh(c.Context(), req.RefreshToken)
  	if err != nil {
  		return mapAuthError(c, err)
  	}
  	return OK(c, TokensDTO{
  		AccessToken:  tok.AccessToken,
  		RefreshToken: tok.RefreshToken,
  		ExpiresIn:    tok.ExpiresIn,
  	})
  }
  ```
  Wire the route in `router.go` (Task 19), inside the `/auth` group:
  ```go
  authGrp.Post("/refresh", rl(), deps.Auth.Refresh)
  ```
  > `mapAuthError` maps `domain.ErrUnauthorized` → 401 (per the contract). Reuse the existing `TokensDTO`.

- [ ] **Step 8: Run the handler test — expect PASS.** Run `cd backend && go test ./internal/adapter/httpapi/ -run TestRefreshHandler 2>&1 | tail -n 20`. Expected PASS: 200 new tokens (rotated), 401 on reuse of the rotated token, 400 on bad body.

- [ ] **Step 9: Vet and commit.** Run `cd backend && go vet ./... && go test ./internal/app/auth/ ./internal/adapter/httpapi/ -run 'Refresh' 2>&1 | tail -n 5`, then `git add -A && git commit -m "feat: add refresh endpoint with rotating revocable tokens and theft response"` (no `Co-Authored-By` trailer).

---

### Task 17: `AuthService.Logout` + handler (idempotent revoke)

**Files:**
- Modify: `backend/internal/app/auth/service.go`
- Modify: `backend/internal/adapter/httpapi/auth_handler.go`
- Modify: `backend/internal/adapter/httpapi/router.go`
- Test: `backend/internal/app/auth/logout_test.go`
- Test: `backend/internal/adapter/httpapi/auth_logout_test.go`

**Interfaces:**
- Consumes (from earlier tasks):
  - `domain.Store`: `GetSessionByTokenHash`, `RevokeSession`
  - `hash.HashRefreshToken`
  - `Service.Login`, `Service.Refresh` (Tasks 15–16, used by tests)
  - `domain.ErrNotFound`, `domain.ErrUnauthorized`
  - the shared harness seeders + `doJSONBody`/`doRaw` (Task 11)
- Produces (later tasks rely on):
  - `func (s *Service) Logout(ctx context.Context, refreshToken string) error`
  - HTTP route `POST /api/auth/logout`, request DTO `LogoutReq{refreshToken}`

- [ ] **Step 1: Write failing service test: revoke + idempotency.** Create `backend/internal/app/auth/logout_test.go`:
  ```go
  package auth_test

  import (
  	"context"
  	"testing"

  	"github.com/stretchr/testify/require"

  	"github.com/zulkhair/pustaka/backend/internal/app/auth"
  	"github.com/zulkhair/pustaka/backend/internal/domain"
  	"github.com/zulkhair/pustaka/backend/internal/pkg/hash"
  )

  func TestLogout_RevokesSession(t *testing.T) {
  	store, cleanup := newTestStore(t)
  	defer cleanup()
  	svc := newLoginService(t, store)
  	ctx := context.Background()
  	seedVerifiedUserSvc(t, store, "walt", "walt@example.com", "waltpassword1")

  	tok, err := svc.Login(ctx, auth.LoginInput{Identifier: "walt@example.com", Password: "waltpassword1"})
  	require.NoError(t, err)

  	require.NoError(t, svc.Logout(ctx, tok.RefreshToken))

  	sess, err := store.GetSessionByTokenHash(ctx, hash.HashRefreshToken(tok.RefreshToken))
  	require.NoError(t, err)
  	require.NotNil(t, sess.RevokedAt, "session must be revoked after logout")
  }

  func TestLogout_ThenRefresh_Unauthorized(t *testing.T) {
  	store, cleanup := newTestStore(t)
  	defer cleanup()
  	svc := newLoginService(t, store)
  	ctx := context.Background()
  	seedVerifiedUserSvc(t, store, "xena", "xena@example.com", "xenapassword1")

  	tok, err := svc.Login(ctx, auth.LoginInput{Identifier: "xena@example.com", Password: "xenapassword1"})
  	require.NoError(t, err)
  	require.NoError(t, svc.Logout(ctx, tok.RefreshToken))

  	_, err = svc.Refresh(ctx, tok.RefreshToken)
  	require.ErrorIs(t, err, domain.ErrUnauthorized)
  }

  func TestLogout_UnknownToken_Idempotent(t *testing.T) {
  	store, cleanup := newTestStore(t)
  	defer cleanup()
  	svc := newLoginService(t, store)

  	err := svc.Logout(context.Background(), "never-issued-token")
  	require.NoError(t, err) // idempotent: unknown token still succeeds
  }
  ```

- [ ] **Step 2: Run the test — expect FAIL.** Run `cd backend && go test ./internal/app/auth/ -run TestLogout 2>&1 | tail -n 20`. Expected FAIL: `undefined: (*auth.Service).Logout`.

- [ ] **Step 3: Implement `Logout` (idempotent).** Append to `backend/internal/app/auth/service.go`:
  ```go
  func (s *Service) Logout(ctx context.Context, refreshToken string) error {
  	sess, err := s.store.GetSessionByTokenHash(ctx, hash.HashRefreshToken(refreshToken))
  	if err != nil {
  		if errors.Is(err, domain.ErrNotFound) {
  			return nil // idempotent: unknown token is a successful no-op
  		}
  		return err
  	}
  	return s.store.RevokeSession(ctx, sess.ID)
  }
  ```

- [ ] **Step 4: Run the test — expect PASS.** Run `cd backend && go test ./internal/app/auth/ -run TestLogout 2>&1 | tail -n 20`. Expected PASS: logout revokes the session, a subsequent refresh is `ErrUnauthorized`, unknown token returns nil.

- [ ] **Step 5: Write failing handler test.** Create `backend/internal/adapter/httpapi/auth_logout_test.go`:
  ```go
  package httpapi_test

  import (
  	"net/http"
  	"testing"

  	"github.com/stretchr/testify/require"
  )

  func TestLogoutHandler_RevokesThenRefresh401(t *testing.T) {
  	ta := newTestApp(t)
  	seedVerifiedUser(t, ta.store, "yuri", "yuri@example.com", "yuripassword1")

  	_, loginBody := doJSONBody(t, ta, http.MethodPost, "/api/auth/login",
  		map[string]string{"identifier": "yuri@example.com", "password": "yuripassword1"})
  	rt := loginBody["data"].(map[string]any)["refreshToken"].(string)

  	resp, _ := doJSONBody(t, ta, http.MethodPost, "/api/auth/logout",
  		map[string]string{"refreshToken": rt})
  	require.Equal(t, http.StatusOK, resp.StatusCode)

  	resp2, _ := doJSONBody(t, ta, http.MethodPost, "/api/auth/refresh",
  		map[string]string{"refreshToken": rt})
  	require.Equal(t, http.StatusUnauthorized, resp2.StatusCode)
  }

  func TestLogoutHandler_UnknownToken_200(t *testing.T) {
  	ta := newTestApp(t)
  	resp, _ := doJSONBody(t, ta, http.MethodPost, "/api/auth/logout",
  		map[string]string{"refreshToken": "never-issued"})
  	require.Equal(t, http.StatusOK, resp.StatusCode)
  }

  func TestLogoutHandler_BadBody_400(t *testing.T) {
  	ta := newTestApp(t)
  	resp := doRaw(t, ta, http.MethodPost, "/api/auth/logout", "}{")
  	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
  }
  ```

- [ ] **Step 6: Run the handler test — expect FAIL.** Run `cd backend && go test ./internal/adapter/httpapi/ -run TestLogoutHandler 2>&1 | tail -n 20`. Expected FAIL: 404 on `POST /api/auth/logout` (route/handler missing).

- [ ] **Step 7: Implement the logout handler and wire the route.** Add to `backend/internal/adapter/httpapi/auth_handler.go`:
  ```go
  type LogoutReq struct {
  	RefreshToken string `json:"refreshToken"`
  }

  func (h *AuthHandler) Logout(c *fiber.Ctx) error {
  	var req LogoutReq
  	if err := c.BodyParser(&req); err != nil {
  		return Fail(c, fiber.StatusBadRequest, "invalid request body")
  	}
  	if req.RefreshToken == "" {
  		return Fail(c, fiber.StatusBadRequest, "refreshToken is required")
  	}
  	if err := h.svc.Logout(c.Context(), req.RefreshToken); err != nil {
  		return mapAuthError(c, err)
  	}
  	return OK(c, nil)
  }
  ```
  Wire the route in `router.go` (Task 19), inside the `/auth` group:
  ```go
  authGrp.Post("/logout", rl(), deps.Auth.Logout)
  ```

- [ ] **Step 8: Run the handler test — expect PASS.** Run `cd backend && go test ./internal/adapter/httpapi/ -run TestLogoutHandler 2>&1 | tail -n 20`. Expected PASS: logout returns 200 then refresh is 401, unknown token returns 200, bad body returns 400.

- [ ] **Step 9: Full vet + full auth/httpapi suite, then commit.** Run `cd backend && go vet ./... && go test ./internal/app/auth/ ./internal/adapter/httpapi/ 2>&1 | tail -n 10`, then `git add -A && git commit -m "feat: add idempotent logout endpoint that revokes the session"` (no `Co-Authored-By` trailer).

---

### Task 18: `AuthService.Me` + handler

**Files:**
- Modify: `backend/internal/app/auth/service.go` (add `Me`)
- Modify: `backend/internal/adapter/httpapi/auth_handler.go` (add `Me` + `MeDTO`)
- Modify: `backend/internal/adapter/httpapi/router.go` (wire `GET /api/auth/me` behind `RequireAuth`)
- Test: `backend/internal/app/auth/me_test.go`
- Test: `backend/internal/adapter/httpapi/auth_me_test.go`

**Interfaces:**
- Consumes (from earlier tasks):
  - `domain.Store`: `GetUserByID`; `domain.ErrNotFound`; `domain.User`
  - `middleware.RequireAuth(secret string) fiber.Handler` (Task 19) — it sets `c.Locals("userID", ...)`
  - the shared harness `newTestApp` + seeders + `jwt.GenerateAccess` (Tasks 11, 8)
- Produces (later tasks rely on these VERBATIM):
  - `func (s *Service) Me(ctx context.Context, userID string) (domain.User, error)` — `GetUserByID`, mapping not-found → `domain.ErrNotFound`
  - `func (h *AuthHandler) Me(c *fiber.Ctx) error` — reads `c.Locals("userID")`, returns `MeDTO`
  - `type MeDTO struct { ID, Username, Email, Role string; EmailVerified bool }` (JSON `id,username,email,role,emailVerified`)
  - the router wires `GET /api/auth/me` with `RequireAuth`

> Sequencing note: this task references `middleware.RequireAuth` (Task 19) and `httpapi.Mount`/`BuildApp` (Task 22) for its HTTP test. The SERVICE-level `Me` test (Step 1) is independent and runs now; the HTTP-level `Me` test runs once Task 19's middleware and Task 22's wiring are present. The router wiring snippet for `/auth/me` (Step 7) is added to `Mount` in Task 19; if Task 19 already includes it from Task 19's own snippet, this step is a confirm-only no-op.

- [ ] **Step 1: Write the failing service test.** Create `backend/internal/app/auth/me_test.go`:
  ```go
  package auth_test

  import (
  	"context"
  	"testing"

  	"github.com/stretchr/testify/require"

  	"github.com/zulkhair/pustaka/backend/internal/domain"
  )

  func TestMe_ReturnsUser(t *testing.T) {
  	store, cleanup := newTestStore(t)
  	defer cleanup()
  	svc := newLoginService(t, store)
  	u := seedVerifiedUserSvc(t, store, "mia", "mia@example.com", "miapassword1")

  	got, err := svc.Me(context.Background(), u.ID)
  	require.NoError(t, err)
  	require.Equal(t, u.ID, got.ID)
  	require.Equal(t, "mia", got.Username)
  	require.Equal(t, "mia@example.com", got.Email)
  	require.True(t, got.EmailVerified)
  }

  func TestMe_UnknownUser_NotFound(t *testing.T) {
  	store, cleanup := newTestStore(t)
  	defer cleanup()
  	svc := newLoginService(t, store)

  	_, err := svc.Me(context.Background(), "no-such-id")
  	require.ErrorIs(t, err, domain.ErrNotFound)
  }
  ```

- [ ] **Step 2: Run the test — expect FAIL.** Run `cd backend && go test ./internal/app/auth/ -run TestMe 2>&1 | tail -n 20`. Expected FAIL: `undefined: (*auth.Service).Me`.

- [ ] **Step 3: Implement `Me`.** Append to `backend/internal/app/auth/service.go`:
  ```go
  func (s *Service) Me(ctx context.Context, userID string) (domain.User, error) {
  	u, err := s.store.GetUserByID(ctx, userID)
  	if err != nil {
  		if errors.Is(err, domain.ErrNotFound) {
  			return domain.User{}, domain.ErrNotFound
  		}
  		return domain.User{}, err
  	}
  	return u, nil
  }
  ```

- [ ] **Step 4: Run the test — expect PASS.** Run `cd backend && go test ./internal/app/auth/ -run TestMe 2>&1 | tail -n 20`. Expected PASS: known id returns the user, unknown id returns `domain.ErrNotFound`.

- [ ] **Step 5: Write the failing handler test.** Create `backend/internal/adapter/httpapi/auth_me_test.go`. It logs in to get an access token, then calls `GET /api/auth/me` with the Bearer token:
  ```go
  package httpapi_test

  import (
  	"io"
  	"net/http"
  	"net/http/httptest"
  	"testing"

  	"github.com/stretchr/testify/require"
  )

  func TestMeHandler_NoToken_401(t *testing.T) {
  	ta := newTestApp(t)
  	req := httptest.NewRequest("GET", "/api/auth/me", nil)
  	resp, err := ta.app.Test(req, -1)
  	require.NoError(t, err)
  	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
  }

  func TestMeHandler_ValidToken_200(t *testing.T) {
  	ta := newTestApp(t)
  	seedVerifiedUser(t, ta.store, "nadia", "nadia@example.com", "nadiapassword1")

  	_, loginBody := doJSONBody(t, ta, http.MethodPost, "/api/auth/login",
  		map[string]string{"identifier": "nadia@example.com", "password": "nadiapassword1"})
  	access := loginBody["data"].(map[string]any)["accessToken"].(string)

  	req := httptest.NewRequest("GET", "/api/auth/me", nil)
  	req.Header.Set("Authorization", "Bearer "+access)
  	resp, err := ta.app.Test(req, -1)
  	require.NoError(t, err)
  	require.Equal(t, http.StatusOK, resp.StatusCode)

  	body, _ := io.ReadAll(resp.Body)
  	require.Contains(t, string(body), "nadia")
  	require.Contains(t, string(body), "nadia@example.com")
  	require.Contains(t, string(body), `"emailVerified":true`)
  }
  ```

- [ ] **Step 6: Run the handler test — expect FAIL.** Run `cd backend && go test ./internal/adapter/httpapi/ -run TestMeHandler 2>&1 | tail -n 20`. Expected FAIL: `undefined: (*AuthHandler).Me` / `undefined: MeDTO` (and 404/route until wired).

- [ ] **Step 7: Implement the `Me` handler + DTO and confirm the route.** Add to `backend/internal/adapter/httpapi/auth_handler.go`:
  ```go
  type MeDTO struct {
  	ID            string `json:"id"`
  	Username      string `json:"username"`
  	Email         string `json:"email"`
  	Role          string `json:"role"`
  	EmailVerified bool   `json:"emailVerified"`
  }

  func (h *AuthHandler) Me(c *fiber.Ctx) error {
  	userID, _ := c.Locals("userID").(string)
  	if userID == "" {
  		return Fail(c, fiber.StatusUnauthorized, "unauthorized")
  	}
  	u, err := h.svc.Me(c.Context(), userID)
  	if err != nil {
  		return mapAuthError(c, err)
  	}
  	return OK(c, MeDTO{
  		ID:            u.ID,
  		Username:      u.Username,
  		Email:         u.Email,
  		Role:          u.Role,
  		EmailVerified: u.EmailVerified,
  	})
  }
  ```
  Confirm the route is wired in `router.go` (Task 19) inside the `/auth` group (added there as part of Task 19's `Mount` body):
  ```go
  authGrp.Get("/me", middleware.RequireAuth(deps.JWTSecret), deps.Auth.Me)
  ```

- [ ] **Step 8: Run the handler test — expect PASS.** Run `cd backend && go test ./internal/adapter/httpapi/ -run TestMeHandler 2>&1 | tail -n 20`. Expected PASS: no token → 401; valid token → 200 with `id/username/email/role/emailVerified`.

- [ ] **Step 9: Vet and commit.** Run `cd backend && go vet ./... && go test ./internal/app/auth/ ./internal/adapter/httpapi/ -run 'Me' 2>&1 | tail -n 5`, then `git add -A && git commit -m "feat(auth): add Me use-case, handler, and MeDTO"` (no `Co-Authored-By` trailer).

---

## Cluster E — HTTP middleware, wiring & E2E (Tasks 19–22)

> Depends on: `pkg/jwt` (`ParseAccess`, `GenerateAccess`, `Claims`), `pkg/hash`, `internal/config.Load`, `internal/domain` (ports/entities/errors), `internal/app/auth` (`Service`, inputs, `Tokens`, `Me`), `internal/adapter/store` (`Store` + `New(pool)` + `RunMigrations` + `OpenPool`), `internal/adapter/mail` (`NewResendMailer`, `MockMailer`). Use those exact signatures; do not redefine them.

### Task 19: Auth & admin middleware (`RequireAuth`, `RequireAdmin`)

**Files:**
- Create: `backend/internal/adapter/httpapi/middleware/auth.go`
- Test: `backend/internal/adapter/httpapi/middleware/auth_test.go`
- Consumes (existing): `backend/internal/adapter/httpapi/response.go` (`httpapi.Fail`). `response.go` is created in Task 21; if running this task first, either order Task 21's `response.go` ahead of it, or note the `httpapi.Fail` dependency explicitly. Assume `httpapi.Fail` exists.

**Interfaces:**
- Consumes:
  - `jwt.ParseAccess(token, secret string) (*jwt.Claims, error)` and `jwt.Claims{UserID, Role string; jwt.RegisteredClaims}` from `internal/pkg/jwt`.
  - `jwt.GenerateAccess(userID, role, secret string, ttl time.Duration) (string, error)` (tests only).
  - `httpapi.Fail(c *fiber.Ctx, httpCode int, msg string) error` from `internal/adapter/httpapi`.
  - `domain.RoleAdmin`, `domain.RoleUser` from `internal/domain`.
- Produces:
  - `func RequireAuth(secret string) fiber.Handler` — parses `Authorization: Bearer <jwt>`, on failure `Fail(c, 401, ...)`; on success sets `c.Locals("userID", claims.UserID)` and `c.Locals("role", claims.Role)` then `c.Next()`.
  - `func RequireAdmin() fiber.Handler` — `Fail(c, 403, ...)` unless `c.Locals("role") == domain.RoleAdmin`, else `c.Next()`.

- [ ] **Step 1: Write the failing test for `RequireAuth` (no token, bad token, good token).** Create `backend/internal/adapter/httpapi/middleware/auth_test.go`. Build a Fiber app with the middleware mounted on a probe route that echoes the locals. No Postgres needed.
  ```go
  package middleware_test

  import (
  	"io"
  	"net/http/httptest"
  	"testing"
  	"time"

  	"github.com/gofiber/fiber/v2"
  	"github.com/stretchr/testify/require"

  	"github.com/zulkhair/pustaka/backend/internal/adapter/httpapi/middleware"
  	"github.com/zulkhair/pustaka/backend/internal/domain"
  	"github.com/zulkhair/pustaka/backend/internal/pkg/jwt"
  )

  const testSecret = "test-secret-0123456789"

  func newProbeApp() *fiber.App {
  	app := fiber.New()
  	app.Get("/protected", middleware.RequireAuth(testSecret), func(c *fiber.Ctx) error {
  		return c.JSON(fiber.Map{
  			"userID": c.Locals("userID"),
  			"role":   c.Locals("role"),
  		})
  	})
  	app.Get("/admin", middleware.RequireAuth(testSecret), middleware.RequireAdmin(), func(c *fiber.Ctx) error {
  		return c.SendString("admin-ok")
  	})
  	return app
  }

  func TestRequireAuth_NoToken_401(t *testing.T) {
  	app := newProbeApp()
  	req := httptest.NewRequest("GET", "/protected", nil)
  	resp, err := app.Test(req, -1)
  	require.NoError(t, err)
  	require.Equal(t, 401, resp.StatusCode)
  }

  func TestRequireAuth_BadToken_401(t *testing.T) {
  	app := newProbeApp()
  	req := httptest.NewRequest("GET", "/protected", nil)
  	req.Header.Set("Authorization", "Bearer not-a-jwt")
  	resp, err := app.Test(req, -1)
  	require.NoError(t, err)
  	require.Equal(t, 401, resp.StatusCode)
  }

  func TestRequireAuth_GoodToken_SetsLocals(t *testing.T) {
  	app := newProbeApp()
  	token, err := jwt.GenerateAccess("user-123", domain.RoleUser, testSecret, 15*time.Minute)
  	require.NoError(t, err)

  	req := httptest.NewRequest("GET", "/protected", nil)
  	req.Header.Set("Authorization", "Bearer "+token)
  	resp, err := app.Test(req, -1)
  	require.NoError(t, err)
  	require.Equal(t, 200, resp.StatusCode)

  	body, _ := io.ReadAll(resp.Body)
  	require.Contains(t, string(body), "user-123")
  	require.Contains(t, string(body), domain.RoleUser)
  }
  ```

- [ ] **Step 2: Run the test, expect FAIL (compile error: package `middleware` has no `RequireAuth`/`RequireAdmin`).** Run from `backend/`:
  ```bash
  go test ./internal/adapter/httpapi/middleware/ -run TestRequireAuth
  ```
  Expected: build failure — `undefined: middleware.RequireAuth` (and `RequireAdmin`), so all three tests fail to compile.

- [ ] **Step 3: Write the failing test for `RequireAdmin` (admin allowed, user blocked).** Append to `auth_test.go`:
  ```go
  func TestRequireAdmin_AdminAllowed(t *testing.T) {
  	app := newProbeApp()
  	token, err := jwt.GenerateAccess("admin-1", domain.RoleAdmin, testSecret, 15*time.Minute)
  	require.NoError(t, err)

  	req := httptest.NewRequest("GET", "/admin", nil)
  	req.Header.Set("Authorization", "Bearer "+token)
  	resp, err := app.Test(req, -1)
  	require.NoError(t, err)
  	require.Equal(t, 200, resp.StatusCode)
  	body, _ := io.ReadAll(resp.Body)
  	require.Equal(t, "admin-ok", string(body))
  }

  func TestRequireAdmin_UserBlocked_403(t *testing.T) {
  	app := newProbeApp()
  	token, err := jwt.GenerateAccess("user-2", domain.RoleUser, testSecret, 15*time.Minute)
  	require.NoError(t, err)

  	req := httptest.NewRequest("GET", "/admin", nil)
  	req.Header.Set("Authorization", "Bearer "+token)
  	resp, err := app.Test(req, -1)
  	require.NoError(t, err)
  	require.Equal(t, 403, resp.StatusCode)
  }
  ```

- [ ] **Step 4: Implement `auth.go` (minimal, real).** Create `backend/internal/adapter/httpapi/middleware/auth.go`:
  ```go
  package middleware

  import (
  	"strings"

  	"github.com/gofiber/fiber/v2"

  	"github.com/zulkhair/pustaka/backend/internal/adapter/httpapi"
  	"github.com/zulkhair/pustaka/backend/internal/domain"
  	"github.com/zulkhair/pustaka/backend/internal/pkg/jwt"
  )

  // RequireAuth validates a Bearer access JWT and stores the principal in c.Locals.
  func RequireAuth(secret string) fiber.Handler {
  	return func(c *fiber.Ctx) error {
  		authz := c.Get("Authorization")
  		const prefix = "Bearer "
  		if len(authz) <= len(prefix) || !strings.EqualFold(authz[:len(prefix)], prefix) {
  			return httpapi.Fail(c, fiber.StatusUnauthorized, "missing or malformed authorization header")
  		}
  		token := strings.TrimSpace(authz[len(prefix):])
  		claims, err := jwt.ParseAccess(token, secret)
  		if err != nil {
  			return httpapi.Fail(c, fiber.StatusUnauthorized, "invalid or expired token")
  		}
  		c.Locals("userID", claims.UserID)
  		c.Locals("role", claims.Role)
  		return c.Next()
  	}
  }

  // RequireAdmin allows the request only when the principal's role is admin.
  // It must run after RequireAuth so that c.Locals("role") is populated.
  func RequireAdmin() fiber.Handler {
  	return func(c *fiber.Ctx) error {
  		role, _ := c.Locals("role").(string)
  		if role != domain.RoleAdmin {
  			return httpapi.Fail(c, fiber.StatusForbidden, "admin access required")
  		}
  		return c.Next()
  	}
  }
  ```

- [ ] **Step 5: Run the tests, expect PASS.** Run from `backend/`:
  ```bash
  go vet ./internal/adapter/httpapi/middleware/
  go test ./internal/adapter/httpapi/middleware/ -run 'TestRequireAuth|TestRequireAdmin' -v
  ```
  Expected: all five subtests PASS (`TestRequireAuth_NoToken_401`, `_BadToken_401`, `_GoodToken_SetsLocals`, `TestRequireAdmin_AdminAllowed`, `_UserBlocked_403`).

- [ ] **Step 6: Commit.**
  ```bash
  git add backend/internal/adapter/httpapi/middleware/auth.go backend/internal/adapter/httpapi/middleware/auth_test.go
  git commit -m "feat(httpapi): add RequireAuth and RequireAdmin middleware"
  ```

---

### Task 20: In-memory rate-limit middleware (`RateLimit`)

**Files:**
- Create: `backend/internal/adapter/httpapi/middleware/ratelimit.go`
- Test: `backend/internal/adapter/httpapi/middleware/ratelimit_test.go`

**Interfaces:**
- Consumes:
  - `httpapi.Fail(c *fiber.Ctx, httpCode int, msg string) error` from `internal/adapter/httpapi`.
- Produces:
  - `func RateLimit(max int, window time.Duration) fiber.Handler` — in-memory, mutex-guarded fixed-window counter keyed by `c.IP() + ":" + c.Path()`. Returns `Fail(c, 429, ...)` when the count for the current window exceeds `max`; otherwise `c.Next()`. The `window` is injectable so tests can pass a short value; expired windows are pruned/reset.

> **Scope note (per-account limiting deferred):** Plan-1 rate-limiting is strictly per-IP+path (the `c.IP()+":"+c.Path()` key). The spec also calls for a per-account / login-lockout dimension (§11: "per-IP + per-account; login backoff/lockout"). That per-account/login-lockout dimension is intentionally OUT of scope for this plan and is deferred to a named follow-up hardening task ("auth abuse-hardening: per-account login backoff + lockout"). This task implements only the per-IP+path limiter. Behind Caddy, `c.IP()` is the real client IP only because `BuildApp` trusts `X-Forwarded-For` from the loopback proxy (see Task 22).

- [ ] **Step 1: Write the failing test (under limit passes, over limit 429, resets after window).** Create `backend/internal/adapter/httpapi/middleware/ratelimit_test.go`. Set distinct paths per subtest so window keys don't collide.
  ```go
  package middleware_test

  import (
  	"net/http/httptest"
  	"testing"
  	"time"

  	"github.com/gofiber/fiber/v2"
  	"github.com/stretchr/testify/require"

  	"github.com/zulkhair/pustaka/backend/internal/adapter/httpapi/middleware"
  )

  func newRateLimitApp(path string, max int, window time.Duration) *fiber.App {
  	app := fiber.New()
  	app.Get(path, middleware.RateLimit(max, window), func(c *fiber.Ctx) error {
  		return c.SendString("ok")
  	})
  	return app
  }

  func doGet(t *testing.T, app *fiber.App, path string) int {
  	t.Helper()
  	req := httptest.NewRequest("GET", path, nil)
  	// Fixed remote addr so c.IP() is stable across calls.
  	req.RemoteAddr = "10.0.0.9:1234"
  	resp, err := app.Test(req, -1)
  	require.NoError(t, err)
  	return resp.StatusCode
  }

  func TestRateLimit_UnderLimitPasses(t *testing.T) {
  	app := newRateLimitApp("/rl-under", 3, time.Minute)
  	require.Equal(t, 200, doGet(t, app, "/rl-under"))
  	require.Equal(t, 200, doGet(t, app, "/rl-under"))
  	require.Equal(t, 200, doGet(t, app, "/rl-under"))
  }

  func TestRateLimit_OverLimitReturns429(t *testing.T) {
  	app := newRateLimitApp("/rl-over", 2, time.Minute)
  	require.Equal(t, 200, doGet(t, app, "/rl-over"))
  	require.Equal(t, 200, doGet(t, app, "/rl-over"))
  	require.Equal(t, 429, doGet(t, app, "/rl-over"))
  }

  func TestRateLimit_ResetsAfterWindow(t *testing.T) {
  	app := newRateLimitApp("/rl-reset", 1, 50*time.Millisecond)
  	require.Equal(t, 200, doGet(t, app, "/rl-reset"))
  	require.Equal(t, 429, doGet(t, app, "/rl-reset"))
  	time.Sleep(80 * time.Millisecond)
  	require.Equal(t, 200, doGet(t, app, "/rl-reset"))
  }
  ```

- [ ] **Step 2: Run the test, expect FAIL (compile error: `undefined: middleware.RateLimit`).** Run from `backend/`:
  ```bash
  go test ./internal/adapter/httpapi/middleware/ -run TestRateLimit
  ```
  Expected: build failure — `undefined: middleware.RateLimit`; the three subtests cannot compile.

- [ ] **Step 3: Implement `ratelimit.go` (minimal, real).** Create `backend/internal/adapter/httpapi/middleware/ratelimit.go`. Fixed-window counter; a background-free lazy reset where each request checks whether its window has rolled over, plus opportunistic pruning of stale keys.
  ```go
  package middleware

  import (
  	"sync"
  	"time"

  	"github.com/gofiber/fiber/v2"

  	"github.com/zulkhair/pustaka/backend/internal/adapter/httpapi"
  )

  type rlEntry struct {
  	count       int
  	windowStart time.Time
  }

  // RateLimit is an in-memory, mutex-guarded fixed-window limiter keyed by IP+Path.
  // The window is injectable so tests can use a short duration.
  func RateLimit(max int, window time.Duration) fiber.Handler {
  	var mu sync.Mutex
  	entries := make(map[string]*rlEntry)

  	return func(c *fiber.Ctx) error {
  		key := c.IP() + ":" + c.Path()
  		now := time.Now()

  		mu.Lock()
  		// Opportunistically prune stale windows to bound memory growth.
  		for k, e := range entries {
  			if now.Sub(e.windowStart) >= window {
  				delete(entries, k)
  			}
  		}
  		e, ok := entries[key]
  		if !ok || now.Sub(e.windowStart) >= window {
  			e = &rlEntry{count: 0, windowStart: now}
  			entries[key] = e
  		}
  		e.count++
  		over := e.count > max
  		mu.Unlock()

  		if over {
  			return httpapi.Fail(c, fiber.StatusTooManyRequests, "too many requests")
  		}
  		return c.Next()
  	}
  }
  ```

- [ ] **Step 4: Run the tests, expect PASS.** Run from `backend/`:
  ```bash
  go vet ./internal/adapter/httpapi/middleware/
  go test ./internal/adapter/httpapi/middleware/ -run TestRateLimit -v
  ```
  Expected: all three subtests PASS (`_UnderLimitPasses`, `_OverLimitReturns429`, `_ResetsAfterWindow`).

- [ ] **Step 5: Commit.**
  ```bash
  git add backend/internal/adapter/httpapi/middleware/ratelimit.go backend/internal/adapter/httpapi/middleware/ratelimit_test.go
  git commit -m "feat(httpapi): add in-memory fixed-window RateLimit middleware"
  ```

---

### Task 21: Response envelope, health handler & full router wiring

**Files:**
- Create: `backend/internal/adapter/httpapi/response.go`
- Create: `backend/internal/adapter/httpapi/health_handler.go`
- Create: `backend/internal/adapter/httpapi/router.go`
- Test: `backend/internal/adapter/httpapi/router_test.go`

> Note: `auth_handler.go` (Tasks 12–18) defines `func NewAuthHandler(svc *auth.Service) *AuthHandler` and its methods (`Register`, `VerifyEmail`, `ResendVerification`, `Login`, `Refresh`, `Logout`, `Me`) plus request/response DTOs; `errors.go` defines `mapAuthError`. This task **consumes** those; do not redefine them. `response.go` here defines `OK`/`Fail`, which the handler tasks and middleware already reference — so this task (or at least its `response.go`) should land early enough that `mapAuthError`/handlers compile.

**Interfaces:**
- Consumes:
  - `auth.Service` and `func (s *Service) Me(ctx, userID string) (domain.User, error)` from `internal/app/auth` (Task 18).
  - `*AuthHandler` (same package) with `NewAuthHandler(svc *auth.Service) *AuthHandler` and the seven handler methods (Tasks 12–18).
  - `middleware.RequireAuth(secret string) fiber.Handler` and `middleware.RateLimit(max int, window time.Duration) fiber.Handler` (Tasks 19, 20).
  - `pgxpool.Pool.Ping(ctx) error` for the health DB ping.
- Produces:
  - `func OK(c *fiber.Ctx, data any) error` — 200, body `{status:0, message:"ok", data}`.
  - `func Fail(c *fiber.Ctx, httpCode int, msg string) error` — `httpCode`, body `{status:1, message:msg, data:null}`.
  - `type Pinger interface { Ping(ctx context.Context) error }` and `func HealthHandler(p Pinger) fiber.Handler` — always 200, body data `{db:"up"|"down"}`.
  - `type RouterDeps struct { Auth *AuthHandler; Pinger Pinger; JWTSecret string }` and `func Mount(app *fiber.App, deps RouterDeps)` — wires all `/api` routes (every `/auth/*` POST is RateLimit'd; `/auth/me` is behind `RequireAuth`; `/health` is open).

- [ ] **Step 1: Write `response.go` first (no test of its own; exercised via Step 4 router test and the earlier `mapAuthError` micro-test).** Create `backend/internal/adapter/httpapi/response.go`:
  ```go
  package httpapi

  import "github.com/gofiber/fiber/v2"

  type envelope struct {
  	Status  int    `json:"status"`
  	Message string `json:"message"`
  	Data    any    `json:"data"`
  }

  // OK writes a 200 success envelope: {status:0, message:"ok", data}.
  func OK(c *fiber.Ctx, data any) error {
  	return c.Status(fiber.StatusOK).JSON(envelope{Status: 0, Message: "ok", Data: data})
  }

  // Fail writes an error envelope with the given HTTP status: {status:1, message, data:null}.
  func Fail(c *fiber.Ctx, httpCode int, msg string) error {
  	return c.Status(httpCode).JSON(envelope{Status: 1, Message: msg, Data: nil})
  }
  ```

- [ ] **Step 2: Write the failing router/health test.** Create `backend/internal/adapter/httpapi/router_test.go`. Use a fake `Pinger` (in-process, no Postgres). For `/auth/me`, drive the JWT + `RequireAuth` path with a `Service` built on a minimal in-test `domain.Store` whose `GetUserByID` returns a fixed user (avoids testcontainers here; the full DB-backed E2E lives in Task 22).
  ```go
  package httpapi_test

  import (
  	"context"
  	"encoding/json"
  	"io"
  	"net/http/httptest"
  	"testing"
  	"time"

  	"github.com/gofiber/fiber/v2"
  	"github.com/stretchr/testify/require"

  	"github.com/zulkhair/pustaka/backend/internal/adapter/httpapi"
  	"github.com/zulkhair/pustaka/backend/internal/app/auth"
  	"github.com/zulkhair/pustaka/backend/internal/config"
  	"github.com/zulkhair/pustaka/backend/internal/domain"
  	"github.com/zulkhair/pustaka/backend/internal/pkg/jwt"
  )

  const routerSecret = "router-secret-abcdefgh"

  // fakePinger lets us flip DB up/down without Postgres.
  type fakePinger struct{ err error }

  func (f fakePinger) Ping(ctx context.Context) error { return f.err }

  // meStore is a minimal domain.Store; only GetUserByID is used by /auth/me.
  type meStore struct {
  	domain.Store // embed nil interface; unused methods will panic if called (they aren't)
  	user         domain.User
  	err          error
  }

  func (m meStore) GetUserByID(ctx context.Context, id string) (domain.User, error) {
  	return m.user, m.err
  }

  func buildRouterApp(t *testing.T, p httpapi.Pinger, store domain.Store) *fiber.App {
  	t.Helper()
  	cfg := config.Config{JWTSecret: routerSecret, AccessTTL: 15 * time.Minute}
  	svc := auth.New(store, nil, cfg)
  	app := fiber.New()
  	httpapi.Mount(app, httpapi.RouterDeps{
  		Auth:      httpapi.NewAuthHandler(svc),
  		Pinger:    p,
  		JWTSecret: routerSecret,
  	})
  	return app
  }

  func TestHealth_ReportsDBUp(t *testing.T) {
  	app := buildRouterApp(t, fakePinger{err: nil}, meStore{})
  	req := httptest.NewRequest("GET", "/api/health", nil)
  	resp, err := app.Test(req, -1)
  	require.NoError(t, err)
  	require.Equal(t, 200, resp.StatusCode)

  	body, _ := io.ReadAll(resp.Body)
  	var env struct {
  		Status int `json:"status"`
  		Data   struct {
  			DB string `json:"db"`
  		} `json:"data"`
  	}
  	require.NoError(t, json.Unmarshal(body, &env))
  	require.Equal(t, 0, env.Status)
  	require.Equal(t, "up", env.Data.DB)
  }

  func TestHealth_ReportsDBDownStill200(t *testing.T) {
  	app := buildRouterApp(t, fakePinger{err: context.DeadlineExceeded}, meStore{})
  	req := httptest.NewRequest("GET", "/api/health", nil)
  	resp, err := app.Test(req, -1)
  	require.NoError(t, err)
  	require.Equal(t, 200, resp.StatusCode)
  	body, _ := io.ReadAll(resp.Body)
  	require.Contains(t, string(body), `"db":"down"`)
  }

  func TestAuthMe_NoToken_401(t *testing.T) {
  	app := buildRouterApp(t, fakePinger{}, meStore{})
  	req := httptest.NewRequest("GET", "/api/auth/me", nil)
  	resp, err := app.Test(req, -1)
  	require.NoError(t, err)
  	require.Equal(t, 401, resp.StatusCode)
  }

  func TestAuthMe_ValidToken_200(t *testing.T) {
  	store := meStore{user: domain.User{
  		ID: "u-9", Username: "alice", Email: "alice@example.com",
  		Role: domain.RoleUser, EmailVerified: true,
  	}}
  	app := buildRouterApp(t, fakePinger{}, store)

  	token, err := jwt.GenerateAccess("u-9", domain.RoleUser, routerSecret, 15*time.Minute)
  	require.NoError(t, err)

  	req := httptest.NewRequest("GET", "/api/auth/me", nil)
  	req.Header.Set("Authorization", "Bearer "+token)
  	resp, err := app.Test(req, -1)
  	require.NoError(t, err)
  	require.Equal(t, 200, resp.StatusCode)

  	body, _ := io.ReadAll(resp.Body)
  	require.Contains(t, string(body), "alice")
  	require.Contains(t, string(body), "alice@example.com")
  }
  ```

- [ ] **Step 3: Run the test, expect FAIL (compile error: `undefined: httpapi.Pinger`, `httpapi.Mount`, `httpapi.RouterDeps`, and `HealthHandler`).** Run from `backend/`:
  ```bash
  go test ./internal/adapter/httpapi/ -run 'TestHealth|TestAuthMe'
  ```
  Expected: build failure — `undefined: httpapi.Mount` / `httpapi.RouterDeps` / `httpapi.Pinger`. (`response.go` already compiles.)

- [ ] **Step 4: Implement `health_handler.go`.** Create `backend/internal/adapter/httpapi/health_handler.go`:
  ```go
  package httpapi

  import (
  	"context"
  	"time"

  	"github.com/gofiber/fiber/v2"
  )

  // Pinger is the minimal surface the health check needs (satisfied by *pgxpool.Pool).
  type Pinger interface {
  	Ping(ctx context.Context) error
  }

  // HealthHandler always returns 200; data.db is "up" when the DB ping succeeds, else "down".
  func HealthHandler(p Pinger) fiber.Handler {
  	return func(c *fiber.Ctx) error {
  		ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
  		defer cancel()
  		db := "up"
  		if p == nil || p.Ping(ctx) != nil {
  			db = "down"
  		}
  		return OK(c, fiber.Map{"db": db})
  	}
  }
  ```

- [ ] **Step 5: Implement `router.go` wiring all routes.** Create `backend/internal/adapter/httpapi/router.go`. Each `/auth/*` POST is wrapped in its own `RateLimit` (10 requests / minute per IP+path); `/auth/me` is behind `RequireAuth`; `/health` is open.
  ```go
  package httpapi

  import (
  	"time"

  	"github.com/gofiber/fiber/v2"

  	"github.com/zulkhair/pustaka/backend/internal/adapter/httpapi/middleware"
  )

  // RouterDeps holds the dependencies needed to wire the HTTP routes.
  type RouterDeps struct {
  	Auth      *AuthHandler
  	Pinger    Pinger
  	JWTSecret string
  }

  const (
  	authRateMax    = 10
  	authRateWindow = time.Minute
  )

  // Mount registers all /api routes on the given Fiber app.
  func Mount(app *fiber.App, deps RouterDeps) {
  	api := app.Group("/api")

  	rl := func() fiber.Handler { return middleware.RateLimit(authRateMax, authRateWindow) }

  	authGrp := api.Group("/auth")
  	authGrp.Post("/register", rl(), deps.Auth.Register)
  	authGrp.Post("/verify-email", rl(), deps.Auth.VerifyEmail)
  	authGrp.Post("/resend-verification", rl(), deps.Auth.ResendVerification)
  	authGrp.Post("/login", rl(), deps.Auth.Login)
  	authGrp.Post("/refresh", rl(), deps.Auth.Refresh)
  	authGrp.Post("/logout", rl(), deps.Auth.Logout)
  	authGrp.Get("/me", middleware.RequireAuth(deps.JWTSecret), deps.Auth.Me)

  	api.Get("/health", HealthHandler(deps.Pinger))
  }
  ```

- [ ] **Step 6: Run the tests, expect PASS.** Run from `backend/`:
  ```bash
  go vet ./internal/adapter/httpapi/...
  go test ./internal/adapter/httpapi/ -run 'TestHealth|TestAuthMe' -v
  ```
  Expected: all four subtests PASS (`TestHealth_ReportsDBUp`, `TestHealth_ReportsDBDownStill200`, `TestAuthMe_NoToken_401`, `TestAuthMe_ValidToken_200`).

- [ ] **Step 7: Commit.**
  ```bash
  git add backend/internal/adapter/httpapi/response.go backend/internal/adapter/httpapi/health_handler.go backend/internal/adapter/httpapi/router.go backend/internal/adapter/httpapi/router_test.go
  git commit -m "feat(httpapi): add response envelope, health handler and route wiring"
  ```

---

### Task 22: Composition root, seed command, and full auth E2E

**Files:**
- Create: `backend/internal/adapter/httpapi/app.go` (test-friendly Fiber app builder, reused by `main.go`)
- Create: `backend/cmd/server/main.go` (replaces the Task 1 placeholder)
- Create: `backend/cmd/seed/main.go` (idempotent admin upsert)
- Modify: `backend/Makefile` (add `seed` target → `go run ./cmd/seed`)
- Modify: `backend/.env.example` (confirm `ADMIN_*` keys present — added in Task 1)
- Test: `backend/internal/adapter/httpapi/e2e_test.go`

**Interfaces:**
- Consumes:
  - `config.Load() (config.Config, error)` (Task 2).
  - `store.OpenPool(ctx, databaseURL) (*pgxpool.Pool, error)`, `store.RunMigrations(databaseURL string) error`, `store.New(pool *pgxpool.Pool) *store.Store` (Tasks 3, 6).
  - `mail.NewResendMailer(cfg config.Config) *mail.ResendMailer` and `mail.NewMockMailer() *mail.MockMailer` with `(m *mail.MockMailer) CodeFor(email string) (string, bool)` (Task 10).
  - `auth.New(store domain.Store, mailer domain.Mailer, cfg config.Config) *auth.Service`.
  - `httpapi.NewAuthHandler`, `httpapi.Mount`, `httpapi.RouterDeps`, `httpapi.Pinger` (Task 21).
  - `hash.HashPassword(pw string, cost int) (string, error)` (Task 7) — used by the seed command.
  - the shared E2E `startPostgres`/helpers are local to the E2E test file (testcontainers `postgres.Run`).
- Produces:
  - `func BuildApp(deps httpapi.RouterDeps) *fiber.App` (in `httpapi/app.go`) — constructs a Fiber app with the Caddy-aware proxy config + `recover` + `logger` middleware and calls `Mount`. Reused by `main.go`, the harness (Task 11), and the E2E test.
  - `cmd/seed` — idempotent admin upsert (`make seed`).

- [ ] **Step 1: Write `app.go` — the shared Fiber app builder.** Create `backend/internal/adapter/httpapi/app.go`. The Fiber config trusts `X-Forwarded-For` ONLY from loopback (Caddy on the same host), so `RateLimit`'s `c.IP()` is the real client IP behind the reverse proxy:
  ```go
  package httpapi

  import (
  	"github.com/gofiber/fiber/v2"
  	"github.com/gofiber/fiber/v2/middleware/logger"
  	"github.com/gofiber/fiber/v2/middleware/recover"
  )

  // BuildApp constructs the Fiber app with the Caddy-aware proxy config, recover +
  // logger middleware, and all routes mounted. Used by the composition root, the
  // shared test harness, and the end-to-end tests.
  //
  // ProxyHeader + EnableTrustedProxyCheck + TrustedProxies make c.IP() resolve to
  // the real client via X-Forwarded-For when the request comes from the loopback
  // reverse proxy (Caddy). The Caddyfile fronting this service MUST forward
  // X-Forwarded-For for per-IP rate-limiting to see real client addresses.
  func BuildApp(deps RouterDeps) *fiber.App {
  	app := fiber.New(fiber.Config{
  		AppName:                 "pustaka",
  		DisableStartupMessage:   true,
  		ProxyHeader:             fiber.HeaderXForwardedFor,
  		EnableTrustedProxyCheck: true,
  		TrustedProxies:          []string{"127.0.0.1", "::1"},
  	})
  	app.Use(recover.New())
  	app.Use(logger.New())
  	Mount(app, deps)
  	return app
  }
  ```

- [ ] **Step 2: Write the failing E2E test (testcontainers Postgres + MockMailer).** Create `backend/internal/adapter/httpapi/e2e_test.go`. The full happy path: register → read code from MockMailer via `CodeFor(email)` → verify-email → login → GET /auth/me → refresh → logout → refresh-again fails. It uses `store.RunMigrations(dsn)` and `store.New(pool)`:
  ```go
  package httpapi_test

  import (
  	"bytes"
  	"context"
  	"encoding/json"
  	"io"
  	"net/http"
  	"net/http/httptest"
  	"testing"
  	"time"

  	"github.com/jackc/pgx/v5/pgxpool"
  	"github.com/stretchr/testify/require"
  	"github.com/testcontainers/testcontainers-go"
  	"github.com/testcontainers/testcontainers-go/modules/postgres"
  	"github.com/testcontainers/testcontainers-go/wait"

  	"github.com/zulkhair/pustaka/backend/internal/adapter/httpapi"
  	"github.com/zulkhair/pustaka/backend/internal/adapter/mail"
  	"github.com/zulkhair/pustaka/backend/internal/adapter/store"
  	"github.com/zulkhair/pustaka/backend/internal/app/auth"
  	"github.com/zulkhair/pustaka/backend/internal/config"
  )

  // startE2EPostgres spins up an ephemeral Postgres, runs migrations, returns pool + DSN.
  func startE2EPostgres(t *testing.T) (*pgxpool.Pool, string) {
  	t.Helper()
  	ctx := context.Background()
  	ctr, err := postgres.Run(ctx,
  		"postgres:16-alpine",
  		postgres.WithDatabase("pustaka"),
  		postgres.WithUsername("pustaka"),
  		postgres.WithPassword("pustaka"),
  		testcontainers.WithWaitStrategy(
  			wait.ForListeningPort("5432/tcp").WithStartupTimeout(60*time.Second)),
  	)
  	require.NoError(t, err)
  	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

  	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
  	require.NoError(t, err)

  	require.NoError(t, store.RunMigrations(dsn))

  	pool, err := pgxpool.New(ctx, dsn)
  	require.NoError(t, err)
  	t.Cleanup(pool.Close)
  	return pool, dsn
  }

  type envResp struct {
  	Status  int             `json:"status"`
  	Message string          `json:"message"`
  	Data    json.RawMessage `json:"data"`
  }

  func e2ePost(t *testing.T, app interface {
  	Test(*http.Request, ...int) (*http.Response, error)
  }, path string, body any, bearer string) (int, envResp) {
  	t.Helper()
  	raw, err := json.Marshal(body)
  	require.NoError(t, err)
  	req := httptest.NewRequest("POST", path, bytes.NewReader(raw))
  	req.Header.Set("Content-Type", "application/json")
  	if bearer != "" {
  		req.Header.Set("Authorization", "Bearer "+bearer)
  	}
  	resp, err := app.Test(req, -1)
  	require.NoError(t, err)
  	b, _ := io.ReadAll(resp.Body)
  	var e envResp
  	require.NoError(t, json.Unmarshal(b, &e), "body: %s", string(b))
  	return resp.StatusCode, e
  }

  func TestAuthFlow_E2E(t *testing.T) {
  	if testing.Short() {
  		t.Skip("skipping e2e in short mode")
  	}
  	pool, _ := startE2EPostgres(t)
  	st := store.New(pool)
  	mockMail := mail.NewMockMailer()
  	cfg := config.Config{
  		JWTSecret:   "e2e-secret-0123456789",
  		AccessTTL:   15 * time.Minute,
  		RefreshTTL:  720 * time.Hour,
  		BcryptCost:  10, // lower cost keeps the test fast
  		CodeTTL:     15 * time.Minute,
  		MaxAttempts: 5,
  	}
  	svc := auth.New(st, mockMail, cfg)
  	app := httpapi.BuildApp(httpapi.RouterDeps{
  		Auth:      httpapi.NewAuthHandler(svc),
  		Pinger:    pool,
  		JWTSecret: cfg.JWTSecret,
  	})

  	const email = "e2e@example.com"

  	// 1. register (generic success, no tokens yet)
  	code, _ := e2ePost(t, app, "/api/auth/register",
  		map[string]string{"username": "e2euser", "email": email, "password": "hunter2pass"}, "")
  	require.Equal(t, 200, code)

  	// 2. read the verification code from the mock mailer
  	vcode, ok := mockMail.CodeFor(email)
  	require.True(t, ok, "mock mailer should have captured a code")
  	require.Len(t, vcode, 6)

  	// 3. verify-email -> tokens
  	code, env := e2ePost(t, app, "/api/auth/verify-email",
  		map[string]string{"email": email, "code": vcode}, "")
  	require.Equal(t, 200, code)
  	var verifyTokens struct {
  		AccessToken  string `json:"accessToken"`
  		RefreshToken string `json:"refreshToken"`
  		ExpiresIn    int    `json:"expiresIn"`
  	}
  	require.NoError(t, json.Unmarshal(env.Data, &verifyTokens))
  	require.NotEmpty(t, verifyTokens.AccessToken)
  	require.NotEmpty(t, verifyTokens.RefreshToken)

  	// 4. login -> tokens
  	code, env = e2ePost(t, app, "/api/auth/login",
  		map[string]string{"identifier": email, "password": "hunter2pass"}, "")
  	require.Equal(t, 200, code)
  	var loginTokens struct {
  		AccessToken  string `json:"accessToken"`
  		RefreshToken string `json:"refreshToken"`
  	}
  	require.NoError(t, json.Unmarshal(env.Data, &loginTokens))
  	require.NotEmpty(t, loginTokens.AccessToken)
  	require.NotEmpty(t, loginTokens.RefreshToken)

  	// 5. GET /auth/me with the access token
  	meReq := httptest.NewRequest("GET", "/api/auth/me", nil)
  	meReq.Header.Set("Authorization", "Bearer "+loginTokens.AccessToken)
  	meResp, err := app.Test(meReq, -1)
  	require.NoError(t, err)
  	require.Equal(t, 200, meResp.StatusCode)
  	meBody, _ := io.ReadAll(meResp.Body)
  	require.Contains(t, string(meBody), "e2euser")
  	require.Contains(t, string(meBody), email)

  	// 6. refresh -> new tokens, old refresh token is rotated out
  	code, env = e2ePost(t, app, "/api/auth/refresh",
  		map[string]string{"refreshToken": loginTokens.RefreshToken}, "")
  	require.Equal(t, 200, code)
  	var refreshed struct {
  		RefreshToken string `json:"refreshToken"`
  	}
  	require.NoError(t, json.Unmarshal(env.Data, &refreshed))
  	require.NotEmpty(t, refreshed.RefreshToken)
  	require.NotEqual(t, loginTokens.RefreshToken, refreshed.RefreshToken)

  	// 7. logout the rotated refresh token
  	code, _ = e2ePost(t, app, "/api/auth/logout",
  		map[string]string{"refreshToken": refreshed.RefreshToken}, "")
  	require.Equal(t, 200, code)

  	// 8. refresh again with the logged-out token must fail (401)
  	code, _ = e2ePost(t, app, "/api/auth/refresh",
  		map[string]string{"refreshToken": refreshed.RefreshToken}, "")
  	require.Equal(t, 401, code)
  }

  func TestSeededAdminCanLogin_E2E(t *testing.T) {
  	if testing.Short() {
  		t.Skip("skipping e2e in short mode")
  	}
  	pool, _ := startE2EPostgres(t)
  	st := store.New(pool)
  	cfg := config.Config{
  		JWTSecret:  "e2e-secret-0123456789",
  		AccessTTL:  15 * time.Minute,
  		RefreshTTL: 720 * time.Hour,
  		BcryptCost: 10,
  	}
  	svc := auth.New(st, mail.NewMockMailer(), cfg)
  	app := httpapi.BuildApp(httpapi.RouterDeps{
  		Auth:      httpapi.NewAuthHandler(svc),
  		Pinger:    pool,
  		JWTSecret: cfg.JWTSecret,
  	})

  	// Seed an admin (pre-verified) directly via the store, mirroring cmd/seed.
  	require.NoError(t, seedAdminForTest(t, st, "admin", "admin@pustaka.local", "admin123", cfg.BcryptCost))

  	code, _ := e2ePost(t, app, "/api/auth/login",
  		map[string]string{"identifier": "admin", "password": "admin123"}, "")
  	require.Equal(t, 200, code, "seeded admin must be able to log in")
  }
  ```
  > `seedAdminForTest` mirrors the `cmd/seed` upsert logic (hash password via `hash.HashPassword`, role=admin, email_verified=true). Add it once in a small `e2e_seed_test.go` (or inline in this file) using `st.CreateUser` + `st.SetUserEmailVerified`; it must hash with `cfg.BcryptCost` and verify the user so login succeeds.

- [ ] **Step 3: Run the E2E test, expect FAIL (compile error: `undefined: httpapi.BuildApp` until Step 1 lands; then missing `seedAdminForTest`/wiring).** Run from `backend/`:
  ```bash
  go test ./internal/adapter/httpapi/ -run 'TestAuthFlow_E2E|TestSeededAdminCanLogin_E2E'
  ```
  Expected: build failure (`undefined: httpapi.BuildApp` / `undefined: seedAdminForTest`). State the observed failure (compile or assertion) explicitly before proceeding.

- [ ] **Step 4: Write `seedAdminForTest`.** Create `backend/internal/adapter/httpapi/e2e_seed_test.go`:
  ```go
  package httpapi_test

  import (
  	"context"
  	"testing"

  	"github.com/google/uuid"

  	"github.com/zulkhair/pustaka/backend/internal/adapter/store"
  	"github.com/zulkhair/pustaka/backend/internal/domain"
  	"github.com/zulkhair/pustaka/backend/internal/pkg/hash"
  )

  // seedAdminForTest mirrors cmd/seed: hash the password, create a pre-verified admin.
  func seedAdminForTest(t *testing.T, st *store.Store, username, email, pw string, cost int) error {
  	t.Helper()
  	ctx := context.Background()
  	ph, err := hash.HashPassword(pw, cost)
  	if err != nil {
  		return err
  	}
  	u, err := st.CreateUser(ctx, domain.CreateUserParams{
  		ID: uuid.NewString(), Username: username, Email: email,
  		PasswordHash: ph, Role: domain.RoleAdmin,
  	})
  	if err != nil {
  		return err
  	}
  	return st.SetUserEmailVerified(ctx, u.ID)
  }
  ```

- [ ] **Step 5: Implement `cmd/seed/main.go` — idempotent admin upsert.** Create `backend/cmd/seed/main.go`. It reads `ADMIN_USERNAME`/`ADMIN_EMAIL`/`ADMIN_PASSWORD` (with sane defaults), hashes via `pkg/hash.HashPassword` (cost from config), and upserts a pre-verified admin (`role=admin`, `email_verified=true`). The upsert is idempotent via `ON CONFLICT (username) DO UPDATE`:
  ```go
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
  ```
  > No `db/seed.sql` is created — the seed is a real, idempotent Go command that hashes the password at runtime (no hardcoded bcrypt hash, no fake `gen_random_uuid`/`pgcrypto` comment). The bcrypt cost matches `cfg.BcryptCost`, so the seeded admin can log in via the same `CheckPassword` path.

- [ ] **Step 6: Implement `cmd/server/main.go` — composition root.** Replace the Task 1 placeholder at `backend/cmd/server/main.go`. Loads config, runs migrations unless `APP_ENV=prod`, opens the pool via `store.OpenPool` (fail-fast ping), builds Store/Mailer/Service via `mail.NewResendMailer(cfg)`, builds the app via `BuildApp`, serves with graceful shutdown:
  ```go
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
  ```

- [ ] **Step 7: Add the `seed` target to the Makefile.** Edit `backend/Makefile`, appending (no `psql`, no `seed.sql` — the Go command is the source of truth):
  ```make
  .PHONY: seed
  seed: ## Seed the admin account (reads ADMIN_USERNAME/EMAIL/PASSWORD; idempotent)
  	go run ./cmd/seed
  ```
  Confirm the `help` target's run notes (Task 1) still describe the four steps; no README is created (house rule) — run notes live in `make help` and `.env.example` comments.

- [ ] **Step 8: Run vet + the full test suite, expect PASS.** Run from `backend/`:
  ```bash
  go vet ./...
  go test ./...
  ```
  Expected: `go vet` clean; `TestAuthFlow_E2E` and `TestSeededAdminCanLogin_E2E` PASS along with all earlier tests (Postgres via testcontainers, MockMailer used — no real email or GPU). State the green result explicitly.

- [ ] **Step 9: Final commit.**
  ```bash
  git add backend/cmd backend/internal/adapter/httpapi/app.go backend/internal/adapter/httpapi/e2e_test.go backend/internal/adapter/httpapi/e2e_seed_test.go backend/Makefile backend/.env.example
  git commit -m "feat(server): wire composition root, idempotent seed command, and auth E2E"
  ```

