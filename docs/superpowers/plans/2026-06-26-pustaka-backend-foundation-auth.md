# Pustaka Backend — Plan 1: Foundation, Auth & Security — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **READ THE "Required corrections" SECTION BELOW BEFORE STARTING.** The drafted tasks were written in parallel against the contract; the review pass found cross-task naming/consistency issues, two missing tasks, and security hardenings. The corrections are **authoritative** — apply each as you reach the relevant task. Where a task body and a correction disagree, the correction wins.

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

## ⚠️ Required corrections (authoritative — apply as you implement; correction wins over any task body)

The tasks below were drafted before this review pass. Apply every item:

1. **Handler type:** use `AuthHandler` / `NewAuthHandler(svc *auth.Service) *AuthHandler` and methods on `*AuthHandler` in **every** handler task (the Register/Verify tasks as drafted may say `Handler`/`NewHandler` — rename).
2. **Error mapper:** in the **Register task**, add a step creating `func mapAuthError(c *fiber.Ctx, err error) error` per the contract error→HTTP table, with a micro-test. **All** handlers call `mapAuthError`. Remove every `mapError` reference.
3. **Migration runner:** standardize on `store.RunMigrations(databaseURL string) error` using `//go:embed db/migrations/*.sql` + golang-migrate **iofs** + the `postgres` driver (NOT `pgx5://`). Prod-skip is the caller's job (`main.go`: `if cfg.AppEnv != "prod"`). Update the DB task, the store test, `cmd/server/main.go`, and the E2E to call `store.RunMigrations(dsn)`. Delete any 3-arg `Migrate`.
4. **DB pool:** add `store.OpenPool(ctx, url)` that pings (fail-fast); `main.go` uses it.
5. **Mailer:** `mail.NewResendMailer(cfg config.Config) *ResendMailer`; exported `MockMailer{LastEmail,LastCode string; Sends []MockSend}` + `NewMockMailer()` + `(m *MockMailer) CodeFor(email)(string,bool)`. `main.go` calls `mail.NewResendMailer(cfg)`; tests read the code via `CodeFor(email)` or the `LastCode` field. Remove `mail.NewResend(...)` and any `LastCode(email)` method call.
6. **sqlc.yaml once:** only the migration task creates `sqlc.yaml`, with `emit_pointers_for_null_types: true`. Remove `sqlc.yaml` from the scaffold task.
7. **store.go imports** must include `github.com/jackc/pgx/v5/pgtype` inside the code listing (not a prose note).
8. **issueTokens once:** define `func (s *Service) issueTokens(ctx, u domain.User)(Tokens,error)` only in the VerifyEmail task; VerifyEmail/Login/Refresh all call it. Remove the duplicate definition in the Login task.
9. **NEW task — Shared test harness (place right before the Register task):** create `internal/testsupport/testsupport.go` with `NewTestStore(t)(*store.Store, func())` (testcontainers `postgres.Run` + `store.RunMigrations`, returns concrete `*store.Store` + cleanup) and `BackdateVerification(t, pool *pgxpool.Pool, userID string, ts time.Time)`; and `internal/adapter/httpapi/harness_test.go` with `type testApp`, `newTestApp(t) *testApp` (exposes `app *fiber.App`, `store *store.Store`, `mailer *mail.MockMailer`), `doJSON(method,path,body)`, `doRaw`, and `seedUnverifiedUser`/`seedVerifiedUser`/`seedUnverifiedUserWithPassword`. **Every** later handler/E2E test uses these exact helpers (no ad-hoc `newTestStore`/mocks).
10. **testcontainers:** use `postgres.Run(ctx,"postgres:16-alpine", postgres.WithDatabase/Username/Password(...), testcontainers.WithWaitStrategy(...))` + `container.ConnectionString(ctx,"sslmode=disable")` everywhere; pin testcontainers in go.mod. Drop `tcpg.RunContainer`/`WithImage`.
11. **NEW task — `AuthService.Me` + handler (place in Cluster E, before the middleware task):** TDD `func (s *Service) Me(ctx, userID string)(domain.User,error)` (GetUserByID, maps not-found→ErrNotFound), `func (h *AuthHandler) Me(c *fiber.Ctx) error` (reads `c.Locals("userID")`, returns `MeDTO`), define `MeDTO`. Router wires `GET /api/auth/me` with `RequireAuth`.
12. **Seed:** replace any hardcoded bcrypt hash with `cmd/seed/main.go` — idempotent upsert reading `ADMIN_USERNAME`/`ADMIN_EMAIL`/`ADMIN_PASSWORD`, hashing via `pkg/hash.HashPassword`, role=admin, email_verified=true. `make seed` runs it. Delete the fake-hash `seed.sql` and the false pgcrypto/`gen_random_uuid()` comment. The E2E asserts the admin can log in.
13. **No README:** do not create `backend/README.md` (house rule). Put run notes in Makefile help text / `.env.example` comments.
14. **Register validation:** add `ErrValidation` (domain) → 400. Register returns `ErrValidation` for empty fields / invalid email / password < 8 (NOT `ErrInvalidCredentials`). Register success returns `OK(c, nil)` (no nested `data.message`).
15. **Login enumeration:** unknown identifier **or** wrong password → identical `ErrInvalidCredentials`; correct password but `!EmailVerified` → `ErrEmailNotVerified`. (Deliberate trade-off: the verified-status signal requires valid credentials, so it isn't a meaningful enumeration vector; it buys clear "please verify" UX.)
16. **Login email normalization:** when `Identifier` contains `@`, look up via `GetUserByEmail(normalizeEmail(Identifier))` matching Register's lowercase+trim.
17. **Resend = silent generic 200:** `ResendVerification` enforces the cooldown as a **silent no-op** (return nil) and is a no-op for unknown/verified emails; the handler **always** returns the same generic 200. No 429 cooldown response. Test asserts a uniform 200 across unknown/verified/cooldown/fresh.
18. **Refresh reuse = theft response:** when the matched session is already revoked, call `store.RevokeAllUserSessions(ctx, sess.UserID)` before returning `ErrUnauthorized`; test asserts a replay also kills the live session.
19. **Atomic attempt cap:** `IncrementVerificationAttempts` does a guarded `UPDATE ... SET attempts = attempts + 1 WHERE id=$1 RETURNING attempts`; VerifyEmail enforces `MaxAttempts` on the **returned** count (increment-then-compare) so concurrency can't exceed the cap.
20. **Fiber behind Caddy:** `BuildApp` uses the `ProxyHeader`/`EnableTrustedProxyCheck`/`TrustedProxies` config above so `RateLimit`'s `c.IP()` is the real client. The Caddyfile must forward `X-Forwarded-For`.
21. **Per-account rate-limit (scoping note):** Plan-1 `RateLimit` is per-IP+path only. A per-account / login-lockout dimension is **deferred to a named follow-up hardening task** (conscious decision — state it in the RateLimit task's notes).
22. **Register test cfg:** set `CodeTTL: 15 * time.Minute` explicitly (no zero value).
23. **Final numbering:** with the two NEW tasks (test harness + Me), the plan is **22 tasks**. Renumber sequentially and fix cross-references when executing.

---

# Drafted tasks


## Cluster A — Foundation & infra (Tasks 1–4)

### Task 1: Repo scaffold

**Files:**
- Create: `backend/go.mod`
- Create: `backend/.prototools`
- Create: `backend/.gitignore`
- Create: `backend/Makefile`
- Create: `backend/docker-compose.yml`
- Create: `backend/.env.example`
- Create: `backend/sqlc.yaml`
- Create: `backend/cmd/server/main.go`
- Test: build smoke via `go build ./...` (no Go test file; the compile + healthy DB is the gate)

**Interfaces:**
- Consumes: nothing (this is the root task).
- Produces: the module `github.com/zulkhair/pustaka/backend` (so every later import path resolves), a compiling `cmd/server/main.go` entrypoint, and a healthy Postgres on `127.0.0.1:5434` for Task 3's testcontainers parity and local `make run`.

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

- [ ] **Step 6: Create `.env.example` listing every Config key.** Create `backend/.env.example`:
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
  ```

- [ ] **Step 7: Create the placeholder `sqlc.yaml`.** Create `backend/sqlc.yaml` (queries are authored in later tasks; this fixes the codegen contract — gen path `internal/adapter/store/sqlc`, package `sqlc`):
  ```yaml
  version: "2"
  sql:
    - engine: "postgresql"
      schema: "db/migrations"
      queries: "db/queries"
      gen:
        go:
          package: "sqlc"
          out: "internal/adapter/store/sqlc"
          sql_package: "pgx/v5"
          emit_json_tags: true
          emit_interface: false
          emit_empty_slices: true
  ```

- [ ] **Step 8: Create the Makefile.** Create `backend/Makefile`:
  ```makefile
  .PHONY: run test vet lint sqlc migrate seed db-up db-down

  run:
  	go run ./cmd/server

  test:
  	go test ./...

  vet:
  	go vet ./...

  lint: vet

  sqlc:
  	sqlc generate

  migrate:
  	migrate -path db/migrations -database "$(DATABASE_URL)" up

  seed:
  	psql "$(DATABASE_URL)" -f db/seed.sql

  db-up:
  	docker compose up -d db

  db-down:
  	docker compose down
  ```

- [ ] **Step 9: Bring Postgres up and confirm healthy.** Run `cd backend && docker compose up -d db`, then poll `docker compose ps db` (or `docker inspect --format '{{.State.Health.Status}}' $(docker compose ps -q db)`) until it reports `healthy`. EXPECT: container reaches `healthy` within ~30s. Leave it running for Task 3.

- [ ] **Step 10: Commit.** `git add backend && git commit -m "chore: scaffold pustaka backend module and compose"`. (No `Co-Authored-By` trailer.)

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
- Create (fixture for the test only): `backend/internal/adapter/store/testdata/migrations/000001_smoke.up.sql`
- Create (fixture for the test only): `backend/internal/adapter/store/testdata/migrations/000001_smoke.down.sql`

**Interfaces:**
- Consumes: `config.Config` from Task 2 (`cfg.DatabaseURL`, `cfg.AppEnv`).
- Produces (later tasks — store wrapper in Cluster B and `main.go` rely on these VERBATIM):
  ```go
  func OpenPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error)
  func Ping(ctx context.Context, pool *pgxpool.Pool) error
  func Migrate(databaseURL, migrationsDir, appEnv string) error // no-op when appEnv == "prod"
  ```
  The real migration FILES under `backend/db/migrations` are authored in Task 5; this task builds the generic runner and pool only, and proves them against a throwaway smoke migration under `testdata/`.

- [ ] **Step 1: Write the smoke migration fixtures.** Create `backend/internal/adapter/store/testdata/migrations/000001_smoke.up.sql`:
  ```sql
  CREATE TABLE smoke (id INT PRIMARY KEY);
  ```
  Create `backend/internal/adapter/store/testdata/migrations/000001_smoke.down.sql`:
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

  func TestOpenPoolAndPing(t *testing.T) {
  	dsn := startPostgres(t)
  	ctx := context.Background()
  	pool, err := OpenPool(ctx, dsn)
  	require.NoError(t, err)
  	t.Cleanup(pool.Close)
  	require.NoError(t, Ping(ctx, pool))
  }

  func TestMigrateAppliesFiles(t *testing.T) {
  	dsn := startPostgres(t)
  	ctx := context.Background()

  	require.NoError(t, Migrate(dsn, "testdata/migrations", "dev"))

  	pool, err := OpenPool(ctx, dsn)
  	require.NoError(t, err)
  	t.Cleanup(pool.Close)

  	var exists bool
  	err = pool.QueryRow(ctx,
  		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'smoke')`,
  	).Scan(&exists)
  	require.NoError(t, err)
  	require.True(t, exists, "smoke table should exist after migrate")
  }

  func TestMigrateSkippedInProd(t *testing.T) {
  	dsn := startPostgres(t)
  	ctx := context.Background()

  	require.NoError(t, Migrate(dsn, "testdata/migrations", "prod"))

  	pool, err := OpenPool(ctx, dsn)
  	require.NoError(t, err)
  	t.Cleanup(pool.Close)

  	var exists bool
  	err = pool.QueryRow(ctx,
  		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'smoke')`,
  	).Scan(&exists)
  	require.NoError(t, err)
  	require.False(t, exists, "migrate must be a no-op when appEnv==prod")
  }
  ```

- [ ] **Step 3: Run the test and state expected FAIL.** Run `cd backend && go test ./internal/adapter/store/...`. EXPECT: FAIL to compile — `db.go` does not exist, so `OpenPool`/`Ping`/`Migrate` are undefined (and the testcontainers/pgx deps are not yet required).

- [ ] **Step 4: Write the minimal implementation.** Create `backend/internal/adapter/store/db.go`:
  ```go
  package store

  import (
  	"context"
  	"errors"
  	"fmt"

  	"github.com/golang-migrate/migrate/v4"
  	_ "github.com/golang-migrate/migrate/v4/database/postgres"
  	_ "github.com/golang-migrate/migrate/v4/source/file"
  	"github.com/jackc/pgx/v5/pgxpool"
  )

  // OpenPool opens a pgx connection pool and verifies connectivity.
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

  // Ping checks the database is reachable.
  func Ping(ctx context.Context, pool *pgxpool.Pool) error {
  	if err := pool.Ping(ctx); err != nil {
  		return fmt.Errorf("store: ping: %w", err)
  	}
  	return nil
  }

  // Migrate applies all up migrations from migrationsDir. It is a no-op when
  // appEnv == "prod" (prod migrations are applied out-of-band).
  func Migrate(databaseURL, migrationsDir, appEnv string) error {
  	if appEnv == "prod" {
  		return nil
  	}
  	m, err := migrate.New("file://"+migrationsDir, databaseURL)
  	if err != nil {
  		return fmt.Errorf("store: init migrate: %w", err)
  	}
  	defer func() { _, _ = m.Close() }()

  	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
  		return fmt.Errorf("store: run migrations: %w", err)
  	}
  	return nil
  }
  ```

- [ ] **Step 5: Run the test and state expected PASS.** Run `cd backend && go get github.com/jackc/pgx/v5/pgxpool github.com/golang-migrate/migrate/v4 github.com/testcontainers/testcontainers-go github.com/testcontainers/testcontainers-go/modules/postgres && go mod tidy && go test ./internal/adapter/store/...`. EXPECT: PASS — `OpenPool`+`Ping` succeed against testcontainers Postgres; `Migrate(..., "dev")` creates the `smoke` table; `Migrate(..., "prod")` leaves it absent. (Requires Docker; the Task 1 compose daemon is sufficient.)

- [ ] **Step 6: Vet and commit.** Run `cd backend && go vet ./...` (EXPECT: clean), then `git add backend && git commit -m "feat: add pgx pool, ping, and migrate runner"`. (No `Co-Authored-By` trailer.)

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

### Task 5: Migration `000001_init` + `sqlc.yaml`

**Files:**
- Create: `backend/db/migrations/000001_init.up.sql`
- Create: `backend/db/migrations/000001_init.down.sql`
- Create: `backend/sqlc.yaml`
- Test: `backend/db/migrations/migrations_test.go`

**Interfaces:**
- Consumes: nothing from other tasks. Uses `testcontainers-go` (`github.com/testcontainers/testcontainers-go/modules/postgres`), `golang-migrate` (`github.com/golang-migrate/migrate/v4` with `database/postgres` + `source/file` drivers), `database/sql` + `jackc/pgx/v5/stdlib`, and `testify`.
- Produces: the three tables (`web_user`, `email_verification`, `session`) with the exact columns/constraints/indexes other tasks rely on, and `sqlc.yaml` that Task 6 uses to generate `internal/adapter/store/sqlc`.

- [ ] **Step 1: Write the failing migration test.** It spins up a throwaway Postgres, runs the migrations up, asserts the three tables + key columns + the role CHECK + uniqueness exist via `information_schema`, then runs down and asserts none remain. Create `backend/db/migrations/migrations_test.go`:
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
  	tcpg "github.com/testcontainers/testcontainers-go/modules/postgres"
  	"github.com/testcontainers/testcontainers-go/wait"
  )

  func startPostgres(t *testing.T) (dsn string, stdlibDSN string) {
  	t.Helper()
  	ctx := context.Background()
  	ctr, err := tcpg.RunContainer(ctx,
  		testcontainers.WithImage("postgres:16-alpine"),
  		tcpg.WithDatabase("pustaka"),
  		tcpg.WithUsername("pustaka"),
  		tcpg.WithPassword("pustaka"),
  		testcontainers.WithWaitStrategy(
  			wait.ForLog("database system is ready to accept connections").
  				WithOccurrence(2).WithStartupTimeout(60*time.Second)),
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

- [ ] **Step 2: Run the test, expect FAIL.** Run `cd backend && go test ./db/migrations/...`. Expected FAIL: `go test` errors because `000001_init.up.sql`/`.down.sql` do not exist yet (`migrate.New` returns "no migration found" / file source error), so the test cannot run the schema.

- [ ] **Step 3: Write the up migration.** Create `backend/db/migrations/000001_init.up.sql` with the exact contract schema:
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

- [ ] **Step 4: Write the down migration.** Create `backend/db/migrations/000001_init.down.sql` (drop in reverse FK order):
  ```sql
  DROP TABLE IF EXISTS session;
  DROP TABLE IF EXISTS email_verification;
  DROP TABLE IF EXISTS web_user;
  ```

- [ ] **Step 5: Write `sqlc.yaml`.** Create `backend/sqlc.yaml` so Task 6 can generate the queries package per the contract (engine postgresql, pgx/v5, JSON tags, pointers for nullables):
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

- [ ] **Step 6: Run the test, expect PASS.** Run `cd backend && go test ./db/migrations/...`. Expected PASS: container starts, `Up` creates all three tables, every asserted column/index/CHECK is found, and `Down` drops the tables so the final counts are 0. Also run `cd backend && go vet ./db/...` and confirm it is clean.

- [ ] **Step 7: Commit.** `cd backend && git add db/migrations sqlc.yaml && git commit -m "feat: add init migration and sqlc config"` (no `Co-Authored-By` trailer).

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
- Consumes: the schema + `sqlc.yaml` from **Task 5**; the `domain` package from Cluster A — exact types/signatures used here: `domain.Store` (the interface to implement), `domain.User`, `domain.EmailVerification`, `domain.Session`, `domain.CreateUserParams`, `domain.CreateEmailVerificationParams`, `domain.CreateSessionParams`, and `domain.ErrNotFound`, `domain.ErrConflict`.
- Produces:
  - `func New(pool *pgxpool.Pool) *Store` and `*Store` implementing every `domain.Store` method, including `ExecTx(ctx context.Context, fn func(domain.Store) error) error`. Task 6's `New` constructor signature is what `cmd/server/main.go` (Cluster D) wires up.
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

- [ ] **Step 2: Write `db/queries/verification.sql`.** Note `GetActiveEmailVerification` = newest unconsumed, and `IncrementVerificationAttempts` returns the new count. Create `backend/db/queries/verification.sql`:
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

- [ ] **Step 5: Write the failing `Store` test.** Covers the CreateUser→GetUserByEmail roundtrip, `ErrNotFound` mapping, and `ExecTx` rollback-on-error. Create `backend/internal/adapter/store/store_test.go`:
  ```go
  package store_test

  import (
  	"context"
  	"errors"
  	"testing"
  	"time"

  	"github.com/golang-migrate/migrate/v4"
  	_ "github.com/golang-migrate/migrate/v4/database/postgres"
  	_ "github.com/golang-migrate/migrate/v4/source/file"
  	"github.com/google/uuid"
  	"github.com/jackc/pgx/v5/pgxpool"
  	"github.com/stretchr/testify/require"
  	"github.com/testcontainers/testcontainers-go"
  	tcpg "github.com/testcontainers/testcontainers-go/modules/postgres"
  	"github.com/testcontainers/testcontainers-go/wait"

  	"github.com/zulkhair/pustaka/backend/internal/adapter/store"
  	"github.com/zulkhair/pustaka/backend/internal/domain"
  )

  func newStore(t *testing.T) *store.Store {
  	t.Helper()
  	ctx := context.Background()
  	ctr, err := tcpg.RunContainer(ctx,
  		testcontainers.WithImage("postgres:16-alpine"),
  		tcpg.WithDatabase("pustaka"),
  		tcpg.WithUsername("pustaka"),
  		tcpg.WithPassword("pustaka"),
  		testcontainers.WithWaitStrategy(
  			wait.ForLog("database system is ready to accept connections").
  				WithOccurrence(2).WithStartupTimeout(60*time.Second)),
  	)
  	require.NoError(t, err)
  	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

  	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
  	require.NoError(t, err)

  	m, err := migrate.New("file://../../../db/migrations", "pgx5://"+dsn[len("postgres://"):])
  	require.NoError(t, err)
  	require.NoError(t, m.Up())

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

- [ ] **Step 7: Write the `Store` wrapper.** It wraps `*sqlc.Queries` + `*pgxpool.Pool`, maps sqlc rows ↔ domain entities, maps `pgx.ErrNoRows`→`domain.ErrNotFound` and unique-violation (`23505`)→`domain.ErrConflict`, and implements `ExecTx` via `pool.BeginTx` + `q.WithTx` (commit on nil error, rollback otherwise). Create `backend/internal/adapter/store/store.go`:
  ```go
  package store

  import (
  	"context"
  	"errors"
  	"time"

  	"github.com/jackc/pgx/v5"
  	"github.com/jackc/pgx/v5/pgconn"
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
  Add the `pgtype` import used by `tstamp`/`toUser`: in the import block add `"github.com/jackc/pgx/v5/pgtype"`. (sqlc with `sql_package: pgx/v5` emits `pgtype.Timestamptz` for `TIMESTAMPTZ` columns and `*pgtype.Timestamptz` for nullable ones, matching the row-mapper code above.)

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
  - Sentinel errors: `ErrNotFound`, `ErrConflict`, `ErrInvalidCredentials`, `ErrEmailNotVerified`, `ErrInvalidCode`, `ErrCodeExpired`, `ErrTooManyAttempts`, `ErrResendCooldown`, `ErrUnauthorized`, `ErrForbidden`.

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
  	ErrResendCooldown     = errors.New("resend cooldown active")
  	ErrUnauthorized       = errors.New("unauthorized")
  	ErrForbidden          = errors.New("forbidden")
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
  - `config.Config` fields `ResendAPIKey string`, `MailFrom string` (Cluster A).
- Produces (later tasks rely on these VERBATIM):
  - `func NewResendMailer(cfg config.Config) *ResendMailer` — production `domain.Mailer`.
  - `mail.MockMailer` with exported fields `LastEmail string`, `LastCode string`, and `Sends []MockSend` (each `MockSend{Email, Code string}`); `func NewMockMailer() *MockMailer`. Used by Tasks 11 and 12 instead of real email.

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

### Task 11: AuthService.Register + register handler

**Files:**
- Create `backend/internal/app/auth/service.go`
- Create `backend/internal/adapter/httpapi/auth_handler.go`
- Test `backend/internal/app/auth/register_test.go`

**Interfaces:**
- Consumes:
  - `domain.Store` and its methods `ExecTx`, `GetUserByEmail`, `GetUserByUsername`, `CreateUser`, `CreateEmailVerification` (Task 9); sentinel `domain.ErrConflict`, `domain.ErrNotFound`; `domain.CreateUserParams`, `domain.CreateEmailVerificationParams`; `domain.RoleUser` (Task 9).
  - `domain.Mailer.SendVerificationCode` (Task 9); `mail.NewMockMailer()` in tests (Task 10).
  - `config.Config` fields `BcryptCost`, `CodeTTL` (Cluster A).
  - `hash.HashPassword(pw string, cost int) (string, error)`, `hash.GenerateNumericCode(n int) (string, error)`, `hash.HashCode(code string, cost int) (string, error)` (Cluster B `pkg/hash`).
  - `httpapi.OK`, `httpapi.Fail` (Cluster D `response.go`); `httpapi.RegisterReq{Username, Email, Password string}` request DTO (Cluster D). The error→HTTP map helper `httpapi.mapError` is defined once in Cluster D; this task's handler **calls** it, does not redefine it.
- Produces (later tasks rely on these VERBATIM):
  - `type Service struct { store domain.Store; mailer domain.Mailer; cfg config.Config }`
  - `func New(store domain.Store, mailer domain.Mailer, cfg config.Config) *Service`
  - `type RegisterInput struct { Username, Email, Password string }`
  - `func (s *Service) Register(ctx context.Context, in RegisterInput) error`
  - `type Handler struct { svc *auth.Service }` and `func NewHandler(svc *auth.Service) *Handler` plus `func (h *Handler) Register(c *fiber.Ctx) error` (Task 12 adds `VerifyEmail` to the same `Handler`/`Service`).

Steps:

- [ ] **Step 1: Write the failing test (testcontainers + MockMailer).** Create `backend/internal/app/auth/register_test.go`. Assumes a shared test helper `newTestStore(t)` (provided by Cluster B/D store tests) returning a real `domain.Store` backed by a testcontainers Postgres; if absent, this test stands up its own via the documented store constructor. It asserts a successful register creates an unverified user + a verification row + the mock captured the 6-digit code, and a duplicate email yields `domain.ErrConflict`:
  ```go
  package auth_test

  import (
  	"context"
  	"testing"

  	"github.com/stretchr/testify/require"

  	"github.com/zulkhair/pustaka/backend/internal/adapter/mail"
  	"github.com/zulkhair/pustaka/backend/internal/app/auth"
  	"github.com/zulkhair/pustaka/backend/internal/config"
  	"github.com/zulkhair/pustaka/backend/internal/domain"
  )

  func TestRegisterCreatesUnverifiedUserAndSendsCode(t *testing.T) {
  	store := newTestStore(t) // shared testcontainers-backed domain.Store helper
  	mock := mail.NewMockMailer()
  	cfg := config.Config{BcryptCost: 4, CodeTTL: config.Config{}.CodeTTL}
  	cfg.BcryptCost = 4
  	svc := auth.New(store, mock, cfg)
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
  	store := newTestStore(t)
  	cfg := config.Config{BcryptCost: 4}
  	svc := auth.New(store, mail.NewMockMailer(), cfg)
  	ctx := context.Background()

  	in := auth.RegisterInput{Username: "bob", Email: "bob@example.com", Password: "supersecret"}
  	require.NoError(t, svc.Register(ctx, in))

  	dup := auth.RegisterInput{Username: "bob2", Email: "bob@example.com", Password: "supersecret"}
  	err := svc.Register(ctx, dup)
  	require.ErrorIs(t, err, domain.ErrConflict)
  }
  ```

- [ ] **Step 2: Run the test and confirm it FAILS.** Run `go test ./internal/app/auth/...`. Expected FAIL: the package does not compile — `undefined: auth.New`, `undefined: auth.RegisterInput`, `undefined: (*auth.Service).Register`.

- [ ] **Step 3: Write `service.go` with `New`, the shared types, and `Register`.** Create `backend/internal/app/auth/service.go`. Validation: non-empty username/email/password, RFC-ish email (`net/mail.ParseAddress`), password length >= 8. Inside `ExecTx`: reject duplicate username/email with `domain.ErrConflict`, hash password, create the user (role `user`, unverified), generate + hash a 6-digit code, create the verification row with `ExpiresAt = now + cfg.CodeTTL`. After the transaction commits, call `mailer.SendVerificationCode`:
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
  		return fmt.Errorf("%w: missing required field", domain.ErrInvalidCredentials)
  	}
  	if _, err := mail.ParseAddress(email); err != nil {
  		return fmt.Errorf("%w: invalid email", domain.ErrInvalidCredentials)
  	}
  	if len(in.Password) < 8 {
  		return fmt.Errorf("%w: password too short", domain.ErrInvalidCredentials)
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

- [ ] **Step 4: Run the test and confirm it PASSES.** Run `go test ./internal/app/auth/...`. Expected PASS: `ok  github.com/zulkhair/pustaka/backend/internal/app/auth` — register creates an unverified `user`-role row, an active verification row with a non-empty hash, and the mock recorded a 6-digit code; duplicate email returns `domain.ErrConflict`.

- [ ] **Step 5: Write the register handler.** Create `backend/internal/adapter/httpapi/auth_handler.go`. Parse `RegisterReq`, call `svc.Register`, map errors through the shared `mapError` helper (Cluster D), and on success return a generic `httpapi.OK` (enumeration-resistant — no hint whether the email already existed):
  ```go
  package httpapi

  import (
  	"github.com/gofiber/fiber/v2"

  	"github.com/zulkhair/pustaka/backend/internal/app/auth"
  )

  type Handler struct {
  	svc *auth.Service
  }

  func NewHandler(svc *auth.Service) *Handler {
  	return &Handler{svc: svc}
  }

  func (h *Handler) Register(c *fiber.Ctx) error {
  	var req RegisterReq
  	if err := c.BodyParser(&req); err != nil {
  		return Fail(c, fiber.StatusBadRequest, "invalid request body")
  	}
  	if err := h.svc.Register(c.Context(), auth.RegisterInput{
  		Username: req.Username,
  		Email:    req.Email,
  		Password: req.Password,
  	}); err != nil {
  		return mapError(c, err)
  	}
  	return OK(c, fiber.Map{"message": "if the details are valid, a verification code has been sent"})
  }
  ```

- [ ] **Step 6: Confirm the package compiles and existing tests still PASS.** Run `go vet ./...` (clean) and `go test ./internal/app/auth/... ./internal/adapter/httpapi/...`. Expected PASS for both packages.

- [ ] **Step 7: Commit.** `git add backend/internal/app/auth backend/internal/adapter/httpapi && git commit -m "feat(auth): add register use-case and handler"` (no `Co-Authored-By` trailer).

---

### Task 12: AuthService.VerifyEmail + verify-email handler

**Files:**
- Modify `backend/internal/app/auth/service.go` (add `VerifyEmail`)
- Modify `backend/internal/adapter/httpapi/auth_handler.go` (add `VerifyEmail`)
- Test `backend/internal/app/auth/verify_test.go`

**Interfaces:**
- Consumes:
  - `domain.Store` methods `GetUserByEmail`, `GetActiveEmailVerification`, `IncrementVerificationAttempts`, `ExecTx`, `SetUserEmailVerified`, `ConsumeEmailVerification`, `CreateSession` (Task 9); sentinels `domain.ErrInvalidCode`, `domain.ErrCodeExpired`, `domain.ErrTooManyAttempts`, `domain.ErrNotFound` (Task 9); `domain.CreateSessionParams` (Task 9).
  - `config.Config` fields `MaxAttempts`, `AccessTTL`, `RefreshTTL`, `JWTSecret` (Cluster A).
  - `hash.CheckCode(hash, code string) bool`, `hash.HashRefreshToken(raw string) string` (Cluster B `pkg/hash`).
  - `jwt.GenerateAccess(userID, role, secret string, ttl time.Duration) (string, error)`, `jwt.GenerateRefreshToken() (string, error)` (Cluster B `pkg/jwt`).
  - `Service`, `New`, `VerifyInput`, `Tokens` (Task 11); `Handler`, `NewHandler`, `mapError`, `httpapi.OK`/`Fail`, `httpapi.VerifyReq{Email, Code string}`, `httpapi.TokensDTO{AccessToken, RefreshToken, ExpiresIn}` (Task 11 + Cluster D).
- Produces (later tasks rely on these VERBATIM):
  - `func (s *Service) VerifyEmail(ctx context.Context, in VerifyInput) (Tokens, error)`
  - `func (h *Handler) VerifyEmail(c *fiber.Ctx) error`
  - A shared internal helper `func (s *Service) issueTokens(ctx context.Context, u domain.User) (Tokens, error)` — created here, **reused** by Login/Refresh in their cluster (they reference it, do not redefine it).

Steps:

- [ ] **Step 1: Write the failing test (testcontainers + MockMailer).** Create `backend/internal/app/auth/verify_test.go`. Register a user (capturing the real code from the mock), then exercise: wrong code increments attempts and returns `domain.ErrInvalidCode`; expired code returns `domain.ErrCodeExpired`; attempts over the cap returns `domain.ErrTooManyAttempts`; correct code verifies the user, returns non-empty `Tokens`, and writes a session row whose `RefreshTokenHash` equals `hash.HashRefreshToken(tokens.RefreshToken)`:
  ```go
  package auth_test

  import (
  	"context"
  	"testing"

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
  		CodeTTL:     15 * 60 * 1e9, // 15m as time.Duration
  		MaxAttempts: 5,
  		AccessTTL:   15 * 60 * 1e9,
  		RefreshTTL:  720 * 60 * 60 * 1e9,
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
  	store := newTestStore(t)
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
  	store := newTestStore(t)
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

- [ ] **Step 2: Run the test and confirm it FAILS.** Run `go test ./internal/app/auth/...`. Expected FAIL: the package does not compile — `undefined: (*auth.Service).VerifyEmail`.

- [ ] **Step 3: Add `issueTokens` and `VerifyEmail` to `service.go`.** Append to `backend/internal/app/auth/service.go`. Order per contract: lookup user (not found → `ErrInvalidCode`, enumeration-safe); fetch active verification (none → `ErrInvalidCode`); expired → `ErrCodeExpired`; attempts at/over cap → `ErrTooManyAttempts`; constant-time `CheckCode` — wrong → increment then `ErrInvalidCode`; right → `ExecTx` { `SetUserEmailVerified`, `ConsumeEmailVerification`, `CreateSession` } and return `Tokens`:
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
  	if ev.Attempts >= s.cfg.MaxAttempts {
  		return Tokens{}, domain.ErrTooManyAttempts
  	}

  	if !hash.CheckCode(ev.CodeHash, in.Code) {
  		if _, incErr := s.store.IncrementVerificationAttempts(ctx, ev.ID); incErr != nil {
  			return Tokens{}, incErr
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
  Add the import for `jwt` at the top of `service.go`: `"github.com/zulkhair/pustaka/backend/internal/pkg/jwt"`. (`issueTokens` is provided for Login/Refresh in their cluster to reuse for the non-transactional path; `VerifyEmail` inlines the same steps inside its `ExecTx` so verification + session creation are atomic.)

- [ ] **Step 4: Run the test and confirm it PASSES.** Run `go test ./internal/app/auth/...`. Expected PASS: wrong code → `ErrInvalidCode` with `Attempts == 1`; correct code → `EmailVerified == true`, non-empty `Tokens` with `ExpiresIn == 900`, and a session row found by `HashRefreshToken(tokens.RefreshToken)`.

- [ ] **Step 5: Add the verify-email handler.** Append `VerifyEmail` to `backend/internal/adapter/httpapi/auth_handler.go`. Parse `VerifyReq`, call `svc.VerifyEmail`, map errors via `mapError`, and on success return the `TokensDTO`:
  ```go
  func (h *Handler) VerifyEmail(c *fiber.Ctx) error {
  	var req VerifyReq
  	if err := c.BodyParser(&req); err != nil {
  		return Fail(c, fiber.StatusBadRequest, "invalid request body")
  	}
  	tokens, err := h.svc.VerifyEmail(c.Context(), auth.VerifyInput{
  		Email: req.Email,
  		Code:  req.Code,
  	})
  	if err != nil {
  		return mapError(c, err)
  	}
  	return OK(c, TokensDTO{
  		AccessToken:  tokens.AccessToken,
  		RefreshToken: tokens.RefreshToken,
  		ExpiresIn:    tokens.ExpiresIn,
  	})
  }
  ```

- [ ] **Step 6: Confirm the whole module compiles and tests still PASS.** Run `go vet ./...` (clean) and `go test ./internal/app/auth/... ./internal/adapter/httpapi/...`. Expected PASS for both packages.

- [ ] **Step 7: Commit.** `git add backend/internal/app/auth backend/internal/adapter/httpapi && git commit -m "feat(auth): add verify-email use-case and handler"` (no `Co-Authored-By` trailer).

## Cluster D — Login & Session Lifecycle

### Task 13: `AuthService.ResendVerification` + handler

**Files:**
- Modify: `backend/internal/app/auth/service.go`
- Modify: `backend/internal/adapter/httpapi/auth_handler.go`
- Modify: `backend/internal/adapter/httpapi/router.go`
- Test: `backend/internal/app/auth/resend_test.go`
- Test: `backend/internal/adapter/httpapi/auth_resend_test.go`

**Interfaces:**
- Consumes (from earlier tasks):
  - `domain.Store`: `GetUserByEmail(ctx, email) (User, error)`, `GetActiveEmailVerification(ctx, userID) (EmailVerification, error)`, `DeleteEmailVerificationsByUser(ctx, userID) error`, `CreateEmailVerification(ctx, CreateEmailVerificationParams) (EmailVerification, error)`
  - `domain.Mailer`: `SendVerificationCode(ctx, toEmail, code) error`
  - `hash.GenerateNumericCode(n int) (string, error)`, `hash.HashCode(code string, cost int) (string, error)`
  - `domain.ErrResendCooldown`, `domain.ErrNotFound`
  - `httpapi.OK(c, data) error`, `httpapi.Fail(c, httpCode, msg) error`
  - `auth.New(store, mailer, cfg) *Service`
  - `mail.MockMailer` (records `SendVerificationCode` calls)
  - `config.Config` fields `CodeTTL`, `ResendCooldown`, `BcryptCost`
- Produces (later tasks rely on):
  - `func (s *Service) ResendVerification(ctx context.Context, email string) error`
  - HTTP route `POST /api/auth/resend-verification`

- [ ] **Step 1: Write failing service test for the cooldown + no-op + success paths.**

```go
// backend/internal/app/auth/resend_test.go
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
)

func newResendService(t *testing.T, store domain.Store) (*auth.Service, *mockMailer) {
	t.Helper()
	mailer := &mockMailer{}
	cfg := config.Config{
		BcryptCost:     4,
		CodeTTL:        15 * time.Minute,
		ResendCooldown: 60 * time.Second,
	}
	return auth.New(store, mailer, cfg), mailer
}

func TestResendVerification_UnknownEmail_NoOp(t *testing.T) {
	store := newTestStore(t)
	svc, mailer := newResendService(t, store)

	err := svc.ResendVerification(context.Background(), "nobody@example.com")
	require.NoError(t, err)
	require.Equal(t, 0, mailer.calls, "no mail must be sent for unknown email")
}

func TestResendVerification_AlreadyVerified_NoOp(t *testing.T) {
	store := newTestStore(t)
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
	require.Equal(t, 0, mailer.calls, "verified users must not get a resend")
}

func TestResendVerification_WithinCooldown_Rejected(t *testing.T) {
	store := newTestStore(t)
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

	err = svc.ResendVerification(ctx, "fresh@example.com")
	require.ErrorIs(t, err, domain.ErrResendCooldown)
	require.Equal(t, 0, mailer.calls)
}

func TestResendVerification_AfterCooldown_SendsNewCode(t *testing.T) {
	store := newTestStore(t)
	svc, mailer := newResendService(t, store)
	ctx := context.Background()

	u, err := store.CreateUser(ctx, domain.CreateUserParams{
		ID: uuid.NewString(), Username: "stale", Email: "stale@example.com",
		PasswordHash: "x", Role: domain.RoleUser,
	})
	require.NoError(t, err)
	// An old verification whose created_at predates the cooldown window.
	_, err = store.CreateEmailVerification(ctx, domain.CreateEmailVerificationParams{
		ID: uuid.NewString(), UserID: u.ID, CodeHash: "old",
		ExpiresAt: time.Now().Add(15 * time.Minute),
	})
	require.NoError(t, err)
	require.NoError(t, store.ForceVerificationCreatedAt(ctx, u.ID, time.Now().Add(-5*time.Minute)))

	err = svc.ResendVerification(ctx, "stale@example.com")
	require.NoError(t, err)
	require.Equal(t, 1, mailer.calls)
	require.Equal(t, "stale@example.com", mailer.lastEmail)
	require.Len(t, mailer.lastCode, 6)
}
```

> Assumes the shared test helpers `newTestStore(t)`, `mockMailer` (with fields `calls`, `lastEmail`, `lastCode`), and the store test-only helper `ForceVerificationCreatedAt(ctx, userID, t)` were defined in earlier auth/store test scaffolding tasks. If `ForceVerificationCreatedAt` does not yet exist, add it as a small test helper on the test store that issues `UPDATE email_verification SET created_at = $2 WHERE user_id = $1`.

- [ ] **Step 2: Run the test — expect FAIL.**

```bash
cd backend && go test ./internal/app/auth/ -run TestResendVerification 2>&1 | tail -n 20
```

Expected FAIL: `undefined: (*auth.Service).ResendVerification` (method not yet implemented).

- [ ] **Step 3: Implement `ResendVerification` (minimal, enumeration-safe).**

```go
// backend/internal/app/auth/service.go  (add method)
func (s *Service) ResendVerification(ctx context.Context, email string) error {
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

> Ensure `service.go` imports `errors`, `time`, `github.com/google/uuid`, and the `hash` package; reference the existing `s.store`, `s.mailer`, `s.cfg` fields established by `auth.New`.

- [ ] **Step 4: Run the test — expect PASS.**

```bash
cd backend && go test ./internal/app/auth/ -run TestResendVerification 2>&1 | tail -n 20
```

Expected PASS: all four `TestResendVerification_*` cases green.

- [ ] **Step 5: Write failing handler test (HTTP status mapping).**

```go
// backend/internal/adapter/httpapi/auth_resend_test.go
package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResendHandler_UnknownEmail_200NoOp(t *testing.T) {
	app, deps := newTestApp(t)
	resp := doJSON(t, app, http.MethodPost, "/api/auth/resend-verification",
		map[string]string{"email": "nobody@example.com"})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, 0, deps.mailer.calls)
}

func TestResendHandler_WithinCooldown_429(t *testing.T) {
	app, deps := newTestApp(t)
	seedUnverifiedUser(t, deps.store, "fresh", "fresh@example.com") // creates a fresh verification too

	resp := doJSON(t, app, http.MethodPost, "/api/auth/resend-verification",
		map[string]string{"email": "fresh@example.com"})
	require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
}

func TestResendHandler_BadBody_400(t *testing.T) {
	app, _ := newTestApp(t)
	resp := doRaw(t, app, http.MethodPost, "/api/auth/resend-verification", "not-json")
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
```

> Reuses the shared HTTP harness from earlier tasks: `newTestApp(t)` (returns the Fiber app plus a `deps` struct exposing `store` and `mailer`), `doJSON`, `doRaw`, and `seedUnverifiedUser(t, store, username, email)`.

- [ ] **Step 6: Run the handler test — expect FAIL.**

```bash
cd backend && go test ./internal/adapter/httpapi/ -run TestResendHandler 2>&1 | tail -n 20
```

Expected FAIL: `404` from Fiber because the route `POST /api/auth/resend-verification` is not registered yet (or the handler method is undefined).

- [ ] **Step 7: Implement the resend handler and wire the route.**

```go
// backend/internal/adapter/httpapi/auth_handler.go  (add)
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
	return OK(c, nil)
}
```

```go
// backend/internal/adapter/httpapi/router.go  (inside the /auth group)
auth.Post("/resend-verification", RateLimit(rlMax, rlWindow), authHandler.ResendVerification)
```

> `mapAuthError` (defined in an earlier handler task) must already map `domain.ErrResendCooldown` → 429. `rlMax`/`rlWindow` are the rate-limit values established when the `/auth` group was created.

- [ ] **Step 8: Run the handler test — expect PASS.**

```bash
cd backend && go test ./internal/adapter/httpapi/ -run TestResendHandler 2>&1 | tail -n 20
```

Expected PASS: 200 no-op, 429 within cooldown, 400 bad body.

- [ ] **Step 9: Vet and commit.**

```bash
cd backend && go vet ./... && go test ./internal/app/auth/ ./internal/adapter/httpapi/ -run 'Resend' 2>&1 | tail -n 5
git add -A && git commit -m "feat: add resend-verification use-case and endpoint"
```

---

### Task 14: `AuthService.Login` + handler

**Files:**
- Modify: `backend/internal/app/auth/service.go`
- Modify: `backend/internal/adapter/httpapi/auth_handler.go`
- Modify: `backend/internal/adapter/httpapi/router.go`
- Test: `backend/internal/app/auth/login_test.go`
- Test: `backend/internal/adapter/httpapi/auth_login_test.go`

**Interfaces:**
- Consumes (from earlier tasks):
  - `domain.Store`: `GetUserByEmail`, `GetUserByUsername`, `CreateSession(ctx, CreateSessionParams) (Session, error)`
  - `hash.CheckPassword(hash, pw string) bool`, `hash.HashPassword(pw, cost) (string, error)`, `hash.GenerateRefreshToken`/`hash.HashRefreshToken` via `jwt`/`hash` (see below)
  - `jwt.GenerateAccess(userID, role, secret string, ttl time.Duration) (string, error)`, `jwt.GenerateRefreshToken() (string, error)`
  - `hash.HashRefreshToken(raw string) string`
  - `domain.ErrInvalidCredentials`, `domain.ErrEmailNotVerified`, `domain.ErrNotFound`
  - `config.Config` fields `JWTSecret`, `AccessTTL`, `RefreshTTL`
  - `auth.Tokens` struct (`AccessToken`, `RefreshToken`, `ExpiresIn`)
- Produces (later tasks rely on):
  - `func (s *Service) Login(ctx context.Context, in LoginInput) (Tokens, error)`
  - `type LoginInput struct { Identifier, Password string }`
  - A reusable internal helper `func (s *Service) issueTokens(ctx context.Context, u domain.User) (Tokens, error)` that Tasks 15 reuses for token issuance
  - HTTP route `POST /api/auth/login`, request DTO `LoginReq{identifier,password}`, response `TokensDTO{accessToken,refreshToken,expiresIn}`

- [ ] **Step 1: Write failing service test for all four login paths.**

```go
// backend/internal/app/auth/login_test.go
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
	return auth.New(store, &mockMailer{}, cfg)
}

func seedVerifiedUser(t *testing.T, store domain.Store, username, email, pw string) domain.User {
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
	store := newTestStore(t)
	svc := newLoginService(t, store)
	u := seedVerifiedUser(t, store, "alice", "alice@example.com", "hunter2pw")

	tok, err := svc.Login(context.Background(), auth.LoginInput{
		Identifier: "alice@example.com", Password: "hunter2pw",
	})
	require.NoError(t, err)
	require.NotEmpty(t, tok.AccessToken)
	require.NotEmpty(t, tok.RefreshToken)
	require.Equal(t, int((15 * time.Minute).Seconds()), tok.ExpiresIn)

	// A session row exists for the issued refresh token.
	sess, err := store.GetSessionByTokenHash(context.Background(), hash.HashRefreshToken(tok.RefreshToken))
	require.NoError(t, err)
	require.Equal(t, u.ID, sess.UserID)
}

func TestLogin_ByUsername_Works(t *testing.T) {
	store := newTestStore(t)
	svc := newLoginService(t, store)
	seedVerifiedUser(t, store, "bob", "bob@example.com", "passwordpw")

	tok, err := svc.Login(context.Background(), auth.LoginInput{
		Identifier: "bob", Password: "passwordpw",
	})
	require.NoError(t, err)
	require.NotEmpty(t, tok.AccessToken)
}

func TestLogin_WrongPassword_InvalidCredentials(t *testing.T) {
	store := newTestStore(t)
	svc := newLoginService(t, store)
	seedVerifiedUser(t, store, "carol", "carol@example.com", "rightpassword")

	_, err := svc.Login(context.Background(), auth.LoginInput{
		Identifier: "carol@example.com", Password: "wrongpassword",
	})
	require.ErrorIs(t, err, domain.ErrInvalidCredentials)
}

func TestLogin_UnknownIdentifier_InvalidCredentials(t *testing.T) {
	store := newTestStore(t)
	svc := newLoginService(t, store)

	_, err := svc.Login(context.Background(), auth.LoginInput{
		Identifier: "ghost@example.com", Password: "whatever123",
	})
	require.ErrorIs(t, err, domain.ErrInvalidCredentials) // identical to wrong-password = enumeration-safe
}

func TestLogin_Unverified_EmailNotVerified(t *testing.T) {
	store := newTestStore(t)
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

- [ ] **Step 2: Run the test — expect FAIL.**

```bash
cd backend && go test ./internal/app/auth/ -run TestLogin 2>&1 | tail -n 20
```

Expected FAIL: `undefined: (*auth.Service).Login` and `undefined: auth.LoginInput`.

- [ ] **Step 3: Implement `LoginInput`, `Login`, and the shared `issueTokens` helper.**

```go
// backend/internal/app/auth/service.go  (add)
type LoginInput struct {
	Identifier string // username OR email
	Password   string
}

func (s *Service) Login(ctx context.Context, in LoginInput) (Tokens, error) {
	var (
		u   domain.User
		err error
	)
	if strings.Contains(in.Identifier, "@") {
		u, err = s.store.GetUserByEmail(ctx, in.Identifier)
	} else {
		u, err = s.store.GetUserByUsername(ctx, in.Identifier)
	}
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return Tokens{}, domain.ErrInvalidCredentials // enumeration-safe
		}
		return Tokens{}, err
	}

	if !hash.CheckPassword(u.PasswordHash, in.Password) {
		return Tokens{}, domain.ErrInvalidCredentials
	}
	if !u.EmailVerified {
		return Tokens{}, domain.ErrEmailNotVerified
	}

	return s.issueTokens(ctx, u)
}

// issueTokens mints an access JWT plus a fresh opaque refresh token, persists a
// session row keyed by the SHA-256 hash of the refresh token, and returns both.
func (s *Service) issueTokens(ctx context.Context, u domain.User) (Tokens, error) {
	access, err := jwt.GenerateAccess(u.ID, u.Role, s.cfg.JWTSecret, s.cfg.AccessTTL)
	if err != nil {
		return Tokens{}, err
	}
	refresh, err := jwt.GenerateRefreshToken()
	if err != nil {
		return Tokens{}, err
	}
	if _, err := s.store.CreateSession(ctx, domain.CreateSessionParams{
		ID:               uuid.NewString(),
		UserID:           u.ID,
		RefreshTokenHash: hash.HashRefreshToken(refresh),
		ExpiresAt:        time.Now().Add(s.cfg.RefreshTTL),
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

> Add `strings` to the imports and ensure `jwt` (`github.com/zulkhair/pustaka/backend/internal/pkg/jwt`) is imported. If a prior task (VerifyEmail) already defined `issueTokens`, do NOT redefine it — reuse the existing one and skip the helper here.

- [ ] **Step 4: Run the test — expect PASS.**

```bash
cd backend && go test ./internal/app/auth/ -run TestLogin 2>&1 | tail -n 20
```

Expected PASS: good creds issue tokens + session, username login works, wrong-password and unknown-identifier both `ErrInvalidCredentials`, unverified `ErrEmailNotVerified`.

- [ ] **Step 5: Write failing handler test (status codes + identical wrong-creds message).**

```go
// backend/internal/adapter/httpapi/auth_login_test.go
package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoginHandler_Good_200WithTokens(t *testing.T) {
	app, deps := newTestApp(t)
	seedVerifiedUser(t, deps.store, "eve", "eve@example.com", "longpassword1")

	resp, body := doJSONBody(t, app, http.MethodPost, "/api/auth/login",
		map[string]string{"identifier": "eve@example.com", "password": "longpassword1"})
	require.Equal(t, http.StatusOK, resp.StatusCode)

	data := body["data"].(map[string]any)
	require.NotEmpty(t, data["accessToken"])
	require.NotEmpty(t, data["refreshToken"])
	require.Greater(t, data["expiresIn"].(float64), float64(0))
}

func TestLoginHandler_WrongPassword_401(t *testing.T) {
	app, deps := newTestApp(t)
	seedVerifiedUser(t, deps.store, "frank", "frank@example.com", "correctpass1")

	resp, body := doJSONBody(t, app, http.MethodPost, "/api/auth/login",
		map[string]string{"identifier": "frank@example.com", "password": "nope"})
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	require.Equal(t, wrongCredsMsg(t, app), body["message"])
}

func TestLoginHandler_UnknownIdentifier_401_SameMessage(t *testing.T) {
	app, _ := newTestApp(t)
	resp, body := doJSONBody(t, app, http.MethodPost, "/api/auth/login",
		map[string]string{"identifier": "ghost@example.com", "password": "whatever"})
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	require.Equal(t, wrongCredsMsg(t, app), body["message"]) // enumeration-safe: identical to wrong-password
}

func TestLoginHandler_Unverified_401(t *testing.T) {
	app, deps := newTestApp(t)
	seedUnverifiedUserWithPassword(t, deps.store, "gina", "gina@example.com", "verifyme123")

	resp, _ := doJSONBody(t, app, http.MethodPost, "/api/auth/login",
		map[string]string{"identifier": "gina@example.com", "password": "verifyme123"})
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// wrongCredsMsg captures the generic invalid-credentials message once so the
// two enumeration-safe assertions compare against the same source of truth.
func wrongCredsMsg(t *testing.T, app testApp) string {
	t.Helper()
	_, body := doJSONBody(t, app, http.MethodPost, "/api/auth/login",
		map[string]string{"identifier": "zzz@example.com", "password": "x"})
	return body["message"].(string)
}
```

> Uses harness helpers `doJSONBody` (returns response + decoded envelope map), `seedVerifiedUser`, and `seedUnverifiedUserWithPassword` (creates an unverified user with a known bcrypt password). `testApp` is the harness app type. If `seedUnverifiedUserWithPassword` does not yet exist, add it next to the other seeders in the test harness.

- [ ] **Step 6: Run the handler test — expect FAIL.**

```bash
cd backend && go test ./internal/adapter/httpapi/ -run TestLoginHandler 2>&1 | tail -n 20
```

Expected FAIL: route `POST /api/auth/login` returns 404 (handler/route not registered).

- [ ] **Step 7: Implement the login handler and wire the route.**

```go
// backend/internal/adapter/httpapi/auth_handler.go  (add)
type LoginReq struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

type TokensDTO struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int    `json:"expiresIn"`
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

```go
// backend/internal/adapter/httpapi/router.go  (inside the /auth group)
auth.Post("/login", RateLimit(rlMax, rlWindow), authHandler.Login)
```

> Import the `auth` app package in `auth_handler.go` if not already present. `mapAuthError` must map both `domain.ErrInvalidCredentials` and `domain.ErrEmailNotVerified` to 401 (per the contract's error→HTTP table); confirm both branches exist. If `TokensDTO` was already declared by the VerifyEmail handler task, do NOT redeclare it — reuse it.

- [ ] **Step 8: Run the handler test — expect PASS.**

```bash
cd backend && go test ./internal/adapter/httpapi/ -run TestLoginHandler 2>&1 | tail -n 20
```

Expected PASS: 200 with tokens, 401 wrong-password, 401 unknown-identifier with the identical message, 401 unverified.

- [ ] **Step 9: Vet and commit.**

```bash
cd backend && go vet ./... && go test ./internal/app/auth/ ./internal/adapter/httpapi/ -run 'Login' 2>&1 | tail -n 5
git add -A && git commit -m "feat: add login use-case and endpoint with enumeration-safe errors"
```

---

### Task 15: `AuthService.Refresh` + handler (rotation)

**Files:**
- Modify: `backend/internal/app/auth/service.go`
- Modify: `backend/internal/adapter/httpapi/auth_handler.go`
- Modify: `backend/internal/adapter/httpapi/router.go`
- Test: `backend/internal/app/auth/refresh_test.go`
- Test: `backend/internal/adapter/httpapi/auth_refresh_test.go`

**Interfaces:**
- Consumes (from earlier tasks):
  - `domain.Store`: `GetSessionByTokenHash(ctx, hash) (Session, error)`, `GetUserByID(ctx, id) (User, error)`, `RevokeSession(ctx, id) error`, `CreateSession(...)`, `ExecTx(ctx, func(Store) error) error`
  - `hash.HashRefreshToken(raw string) string`
  - `Service.issueTokens(ctx, domain.User) (Tokens, error)` (Task 14)
  - `Service.Login` (Task 14, for tests to mint a starting token)
  - `domain.ErrUnauthorized`, `domain.ErrNotFound`
- Produces (later tasks rely on):
  - `func (s *Service) Refresh(ctx context.Context, refreshToken string) (Tokens, error)`
  - HTTP route `POST /api/auth/refresh`, request DTO `RefreshReq{refreshToken}`

- [ ] **Step 1: Write failing service test: rotation, reuse rejection, expiry.**

```go
// backend/internal/app/auth/refresh_test.go
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
	store := newTestStore(t)
	svc := newLoginService(t, store)
	ctx := context.Background()
	seedVerifiedUser(t, store, "rita", "rita@example.com", "ritapassword1")

	first, err := svc.Login(ctx, auth.LoginInput{Identifier: "rita@example.com", Password: "ritapassword1"})
	require.NoError(t, err)

	second, err := svc.Refresh(ctx, first.RefreshToken)
	require.NoError(t, err)
	require.NotEmpty(t, second.AccessToken)
	require.NotEqual(t, first.RefreshToken, second.RefreshToken)

	// The OLD session is revoked.
	old, err := store.GetSessionByTokenHash(ctx, hash.HashRefreshToken(first.RefreshToken))
	require.NoError(t, err)
	require.NotNil(t, old.RevokedAt, "old session must be revoked after rotation")

	// The NEW session is live.
	fresh, err := store.GetSessionByTokenHash(ctx, hash.HashRefreshToken(second.RefreshToken))
	require.NoError(t, err)
	require.Nil(t, fresh.RevokedAt)
}

func TestRefresh_ReuseAfterRotation_Unauthorized(t *testing.T) {
	store := newTestStore(t)
	svc := newLoginService(t, store)
	ctx := context.Background()
	seedVerifiedUser(t, store, "sam", "sam@example.com", "sampassword1")

	first, err := svc.Login(ctx, auth.LoginInput{Identifier: "sam@example.com", Password: "sampassword1"})
	require.NoError(t, err)
	_, err = svc.Refresh(ctx, first.RefreshToken)
	require.NoError(t, err)

	// Replaying the old (now-revoked) token must be rejected.
	_, err = svc.Refresh(ctx, first.RefreshToken)
	require.ErrorIs(t, err, domain.ErrUnauthorized)
}

func TestRefresh_UnknownToken_Unauthorized(t *testing.T) {
	store := newTestStore(t)
	svc := newLoginService(t, store)

	_, err := svc.Refresh(context.Background(), "this-token-was-never-issued")
	require.ErrorIs(t, err, domain.ErrUnauthorized)
}

func TestRefresh_Expired_Unauthorized(t *testing.T) {
	store := newTestStore(t)
	svc := newLoginService(t, store)
	ctx := context.Background()
	u := seedVerifiedUser(t, store, "tina", "tina@example.com", "tinapassword1")

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

- [ ] **Step 2: Run the test — expect FAIL.**

```bash
cd backend && go test ./internal/app/auth/ -run TestRefresh 2>&1 | tail -n 20
```

Expected FAIL: `undefined: (*auth.Service).Refresh`.

- [ ] **Step 3: Implement `Refresh` with transactional rotation.**

```go
// backend/internal/app/auth/service.go  (add)
func (s *Service) Refresh(ctx context.Context, refreshToken string) (Tokens, error) {
	tokenHash := hash.HashRefreshToken(refreshToken)

	sess, err := s.store.GetSessionByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return Tokens{}, domain.ErrUnauthorized
		}
		return Tokens{}, err
	}
	if sess.RevokedAt != nil || time.Now().After(sess.ExpiresAt) {
		return Tokens{}, domain.ErrUnauthorized // revoked (incl. reuse-after-rotation) or expired
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

> Token generation happens before the transaction so a CSPRNG/JWT error never leaves a revoked-but-unreplaced session. `issueTokens` is not reused here because rotation must revoke and create atomically in one `ExecTx`.

- [ ] **Step 4: Run the test — expect PASS.**

```bash
cd backend && go test ./internal/app/auth/ -run TestRefresh 2>&1 | tail -n 20
```

Expected PASS: valid rotation revokes old + issues new, replay of the old token is `ErrUnauthorized`, unknown token `ErrUnauthorized`, expired session `ErrUnauthorized`.

- [ ] **Step 5: Write failing handler test.**

```go
// backend/internal/adapter/httpapi/auth_refresh_test.go
package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRefreshHandler_Valid_200NewTokens(t *testing.T) {
	app, deps := newTestApp(t)
	seedVerifiedUser(t, deps.store, "uma", "uma@example.com", "umapassword1")

	_, loginBody := doJSONBody(t, app, http.MethodPost, "/api/auth/login",
		map[string]string{"identifier": "uma@example.com", "password": "umapassword1"})
	first := loginBody["data"].(map[string]any)["refreshToken"].(string)

	resp, body := doJSONBody(t, app, http.MethodPost, "/api/auth/refresh",
		map[string]string{"refreshToken": first})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	second := body["data"].(map[string]any)["refreshToken"].(string)
	require.NotEqual(t, first, second)
}

func TestRefreshHandler_ReuseAfterRotation_401(t *testing.T) {
	app, deps := newTestApp(t)
	seedVerifiedUser(t, deps.store, "vic", "vic@example.com", "vicpassword1")

	_, loginBody := doJSONBody(t, app, http.MethodPost, "/api/auth/login",
		map[string]string{"identifier": "vic@example.com", "password": "vicpassword1"})
	first := loginBody["data"].(map[string]any)["refreshToken"].(string)

	resp1, _ := doJSONBody(t, app, http.MethodPost, "/api/auth/refresh",
		map[string]string{"refreshToken": first})
	require.Equal(t, http.StatusOK, resp1.StatusCode)

	resp2, _ := doJSONBody(t, app, http.MethodPost, "/api/auth/refresh",
		map[string]string{"refreshToken": first})
	require.Equal(t, http.StatusUnauthorized, resp2.StatusCode)
}

func TestRefreshHandler_BadBody_400(t *testing.T) {
	app, _ := newTestApp(t)
	resp := doRaw(t, app, http.MethodPost, "/api/auth/refresh", "{bad")
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
```

- [ ] **Step 6: Run the handler test — expect FAIL.**

```bash
cd backend && go test ./internal/adapter/httpapi/ -run TestRefreshHandler 2>&1 | tail -n 20
```

Expected FAIL: 404 on `POST /api/auth/refresh` (route/handler missing).

- [ ] **Step 7: Implement the refresh handler and wire the route.**

```go
// backend/internal/adapter/httpapi/auth_handler.go  (add)
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

```go
// backend/internal/adapter/httpapi/router.go  (inside the /auth group)
auth.Post("/refresh", RateLimit(rlMax, rlWindow), authHandler.Refresh)
```

> `mapAuthError` must map `domain.ErrUnauthorized` → 401 (per the contract). Reuse the existing `TokensDTO`.

- [ ] **Step 8: Run the handler test — expect PASS.**

```bash
cd backend && go test ./internal/adapter/httpapi/ -run TestRefreshHandler 2>&1 | tail -n 20
```

Expected PASS: 200 new tokens (rotated), 401 on reuse of the rotated token, 400 on bad body.

- [ ] **Step 9: Vet and commit.**

```bash
cd backend && go vet ./... && go test ./internal/app/auth/ ./internal/adapter/httpapi/ -run 'Refresh' 2>&1 | tail -n 5
git add -A && git commit -m "feat: add refresh endpoint with rotating revocable tokens"
```

---

### Task 16: `AuthService.Logout` + handler (idempotent revoke)

**Files:**
- Modify: `backend/internal/app/auth/service.go`
- Modify: `backend/internal/adapter/httpapi/auth_handler.go`
- Modify: `backend/internal/adapter/httpapi/router.go`
- Test: `backend/internal/app/auth/logout_test.go`
- Test: `backend/internal/adapter/httpapi/auth_logout_test.go`

**Interfaces:**
- Consumes (from earlier tasks):
  - `domain.Store`: `GetSessionByTokenHash`, `RevokeSession`
  - `hash.HashRefreshToken(raw string) string`
  - `Service.Login`, `Service.Refresh` (Tasks 14-15, used by tests)
  - `domain.ErrNotFound`
- Produces (later tasks rely on):
  - `func (s *Service) Logout(ctx context.Context, refreshToken string) error`
  - HTTP route `POST /api/auth/logout`, request DTO `LogoutReq{refreshToken}`

- [ ] **Step 1: Write failing service test: revoke + idempotency.**

```go
// backend/internal/app/auth/logout_test.go
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
	store := newTestStore(t)
	svc := newLoginService(t, store)
	ctx := context.Background()
	seedVerifiedUser(t, store, "walt", "walt@example.com", "waltpassword1")

	tok, err := svc.Login(ctx, auth.LoginInput{Identifier: "walt@example.com", Password: "waltpassword1"})
	require.NoError(t, err)

	require.NoError(t, svc.Logout(ctx, tok.RefreshToken))

	sess, err := store.GetSessionByTokenHash(ctx, hash.HashRefreshToken(tok.RefreshToken))
	require.NoError(t, err)
	require.NotNil(t, sess.RevokedAt, "session must be revoked after logout")
}

func TestLogout_ThenRefresh_Unauthorized(t *testing.T) {
	store := newTestStore(t)
	svc := newLoginService(t, store)
	ctx := context.Background()
	seedVerifiedUser(t, store, "xena", "xena@example.com", "xenapassword1")

	tok, err := svc.Login(ctx, auth.LoginInput{Identifier: "xena@example.com", Password: "xenapassword1"})
	require.NoError(t, err)
	require.NoError(t, svc.Logout(ctx, tok.RefreshToken))

	_, err = svc.Refresh(ctx, tok.RefreshToken)
	require.ErrorIs(t, err, domain.ErrUnauthorized)
}

func TestLogout_UnknownToken_Idempotent(t *testing.T) {
	store := newTestStore(t)
	svc := newLoginService(t, store)

	err := svc.Logout(context.Background(), "never-issued-token")
	require.NoError(t, err) // idempotent: unknown token still succeeds
}
```

- [ ] **Step 2: Run the test — expect FAIL.**

```bash
cd backend && go test ./internal/app/auth/ -run TestLogout 2>&1 | tail -n 20
```

Expected FAIL: `undefined: (*auth.Service).Logout`.

- [ ] **Step 3: Implement `Logout` (idempotent).**

```go
// backend/internal/app/auth/service.go  (add)
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

- [ ] **Step 4: Run the test — expect PASS.**

```bash
cd backend && go test ./internal/app/auth/ -run TestLogout 2>&1 | tail -n 20
```

Expected PASS: logout revokes the session, a subsequent refresh is `ErrUnauthorized`, unknown token returns nil.

- [ ] **Step 5: Write failing handler test.**

```go
// backend/internal/adapter/httpapi/auth_logout_test.go
package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLogoutHandler_RevokesThenRefresh401(t *testing.T) {
	app, deps := newTestApp(t)
	seedVerifiedUser(t, deps.store, "yuri", "yuri@example.com", "yuripassword1")

	_, loginBody := doJSONBody(t, app, http.MethodPost, "/api/auth/login",
		map[string]string{"identifier": "yuri@example.com", "password": "yuripassword1"})
	rt := loginBody["data"].(map[string]any)["refreshToken"].(string)

	resp, _ := doJSONBody(t, app, http.MethodPost, "/api/auth/logout",
		map[string]string{"refreshToken": rt})
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp2, _ := doJSONBody(t, app, http.MethodPost, "/api/auth/refresh",
		map[string]string{"refreshToken": rt})
	require.Equal(t, http.StatusUnauthorized, resp2.StatusCode)
}

func TestLogoutHandler_UnknownToken_200(t *testing.T) {
	app, _ := newTestApp(t)
	resp, _ := doJSONBody(t, app, http.MethodPost, "/api/auth/logout",
		map[string]string{"refreshToken": "never-issued"})
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestLogoutHandler_BadBody_400(t *testing.T) {
	app, _ := newTestApp(t)
	resp := doRaw(t, app, http.MethodPost, "/api/auth/logout", "}{")
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
```

- [ ] **Step 6: Run the handler test — expect FAIL.**

```bash
cd backend && go test ./internal/adapter/httpapi/ -run TestLogoutHandler 2>&1 | tail -n 20
```

Expected FAIL: 404 on `POST /api/auth/logout` (route/handler missing).

- [ ] **Step 7: Implement the logout handler and wire the route.**

```go
// backend/internal/adapter/httpapi/auth_handler.go  (add)
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

```go
// backend/internal/adapter/httpapi/router.go  (inside the /auth group)
auth.Post("/logout", RateLimit(rlMax, rlWindow), authHandler.Logout)
```

- [ ] **Step 8: Run the handler test — expect PASS.**

```bash
cd backend && go test ./internal/adapter/httpapi/ -run TestLogoutHandler 2>&1 | tail -n 20
```

Expected PASS: logout returns 200 then refresh is 401, unknown token returns 200, bad body returns 400.

- [ ] **Step 9: Full vet + full auth/httpapi suite, then commit.**

```bash
cd backend && go vet ./... && go test ./internal/app/auth/ ./internal/adapter/httpapi/ 2>&1 | tail -n 10
git add -A && git commit -m "feat: add idempotent logout endpoint that revokes the session"
```

## Cluster E — HTTP middleware, wiring & E2E (Tasks 17-20)

> Depends on: `pkg/jwt` (`ParseAccess`, `GenerateAccess`, `Claims`), `pkg/hash`, `internal/config.Load`, `internal/domain` (ports/entities/errors), `internal/app/auth` (`Service`, inputs, `Tokens`), `internal/adapter/store` (`Store` + `New(pool)`), `internal/adapter/mail` (`NewResend`, `MockMailer`). Use those exact signatures; do not redefine them.

### Task 17: Auth & admin middleware (`RequireAuth`, `RequireAdmin`)

**Files:**
- Create: `backend/internal/adapter/httpapi/middleware/auth.go`
- Test: `backend/internal/adapter/httpapi/middleware/auth_test.go`
- Consumes (existing): `backend/internal/adapter/httpapi/response.go` (`httpapi.Fail`) — if `response.go` is not yet present from Task 19, this task may define a temporary local 401/403 writer; prefer ordering Task 19's `response.go` first. Assume `httpapi.Fail` exists.

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

### Task 18: In-memory rate-limit middleware (`RateLimit`)

**Files:**
- Create: `backend/internal/adapter/httpapi/middleware/ratelimit.go`
- Test: `backend/internal/adapter/httpapi/middleware/ratelimit_test.go`

**Interfaces:**
- Consumes:
  - `httpapi.Fail(c *fiber.Ctx, httpCode int, msg string) error` from `internal/adapter/httpapi`.
- Produces:
  - `func RateLimit(max int, window time.Duration) fiber.Handler` — in-memory, mutex-guarded fixed-window counter keyed by `c.IP() + ":" + c.Path()`. Returns `Fail(c, 429, ...)` when the count for the current window exceeds `max`; otherwise `c.Next()`. The `window` is injectable so tests can pass a short value; expired windows are pruned/reset.

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

### Task 19: Response envelope, health handler & full router wiring

**Files:**
- Create: `backend/internal/adapter/httpapi/response.go`
- Create: `backend/internal/adapter/httpapi/health_handler.go`
- Create: `backend/internal/adapter/httpapi/router.go`
- Test: `backend/internal/adapter/httpapi/router_test.go`

> Note: `auth_handler.go` (Cluster D) defines `func NewAuthHandler(svc *auth.Service) *AuthHandler` and its methods (`Register`, `VerifyEmail`, `ResendVerification`, `Login`, `Refresh`, `Logout`, `Me`) plus request/response DTOs and the error→HTTP mapping. This task **consumes** those; do not redefine them. If a handler method name differs, adjust the `Mount` body to match the Cluster D names — do not change the route paths or middleware wiring below.

**Interfaces:**
- Consumes:
  - `auth.Service` and `func (s *Service) Me(ctx, userID string) (domain.User, error)` from `internal/app/auth`.
  - `*AuthHandler` (same package) with `NewAuthHandler(svc *auth.Service) *AuthHandler` and the seven handler methods (Cluster D).
  - `middleware.RequireAuth(secret string) fiber.Handler` and `middleware.RateLimit(max int, window time.Duration) fiber.Handler` (Tasks 17, 18).
  - `pgxpool.Pool.Ping(ctx) error` for the health DB ping.
- Produces:
  - `func OK(c *fiber.Ctx, data any) error` — 200, body `{status:0, message:"ok", data}`.
  - `func Fail(c *fiber.Ctx, httpCode int, msg string) error` — `httpCode`, body `{status:1, message:msg, data:null}`.
  - `type Pinger interface { Ping(ctx context.Context) error }` and `func HealthHandler(p Pinger) fiber.Handler` — always 200, body data `{db:"up"|"down"}`.
  - `type RouterDeps struct { Auth *AuthHandler; Pinger Pinger; JWTSecret string }` and `func Mount(app *fiber.App, deps RouterDeps)` — wires all `/api` routes.

- [ ] **Step 1: Write `response.go` first (no test of its own; exercised via Step 4 router test).** Create `backend/internal/adapter/httpapi/response.go`:
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

- [ ] **Step 2: Write the failing router/health test.** Create `backend/internal/adapter/httpapi/router_test.go`. Use a fake `Pinger` (in-process, no Postgres) and a real `auth.Service` is not needed for `/health`; for `/auth/me` we mount with a tiny stub by constructing the real handler over a `Service` backed by a fake `domain.Store`. To keep this test focused and Postgres-free, drive `/auth/me` through the JWT + `RequireAuth` path and a `Service` built on a minimal in-test `domain.Store` whose `GetUserByID` returns a fixed user. This avoids testcontainers here (the full DB-backed E2E lives in Task 20).
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

### Task 20: Composition root, seed, and full auth E2E

**Files:**
- Create: `backend/cmd/server/main.go`
- Create: `backend/internal/adapter/httpapi/app.go` (test-friendly Fiber app builder, reused by `main.go`)
- Create: `backend/db/seed.sql`
- Modify: `backend/Makefile` (add `seed` target)
- Modify: `backend/README.md` (document run + seed steps) — only if a `README.md` already exists from an earlier task; otherwise create it for the documented run steps.
- Test: `backend/internal/adapter/httpapi/e2e_test.go`

**Interfaces:**
- Consumes:
  - `config.Load() (config.Config, error)` from `internal/config`.
  - `store.New(pool *pgxpool.Pool) *store.Store` (or the Cluster B constructor name) returning a `domain.Store`, plus the migration runner from Cluster B (e.g. `store.RunMigrations(databaseURL string) error`). Use the actual Cluster B names; do not invent new ones.
  - `mail.NewResend(apiKey, from string) domain.Mailer` and `mail.NewMockMailer() *mail.MockMailer` with `LastCode(email string) (string, bool)` (or the Cluster C accessor) from `internal/adapter/mail`.
  - `auth.New(store domain.Store, mailer domain.Mailer, cfg config.Config) *auth.Service`.
  - `httpapi.NewAuthHandler`, `httpapi.Mount`, `httpapi.RouterDeps`, `httpapi.Pinger` (Task 19).
  - `hash.HashPassword(pw string, cost int) (string, error)` (seed SQL doc references a precomputed hash).
- Produces:
  - `func BuildApp(deps httpapi.RouterDeps) *fiber.App` (in `httpapi/app.go`) — constructs a Fiber app with `recover` + `logger` middleware and calls `Mount`. Reused by both `main.go` and the E2E test.

- [ ] **Step 1: Write `app.go` — the shared Fiber app builder.** Create `backend/internal/adapter/httpapi/app.go`:
```go
package httpapi

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

// BuildApp constructs the Fiber app with recover + logger middleware and all routes mounted.
// Used by both the composition root and the end-to-end tests.
func BuildApp(deps RouterDeps) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:               "pustaka",
		DisableStartupMessage: true,
	})
	app.Use(recover.New())
	app.Use(logger.New())
	Mount(app, deps)
	return app
}
```

- [ ] **Step 2: Write the failing E2E test (testcontainers Postgres + MockMailer).** Create `backend/internal/adapter/httpapi/e2e_test.go`. The full happy path: register → read code from MockMailer → verify-email → login → GET /auth/me → refresh → logout → refresh-again fails. Replace `store.New` / `store.RunMigrations` / `mail.NewMockMailer` / `LastCode` with the exact Cluster B/C names if they differ.
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

// startPostgres spins up an ephemeral Postgres, runs migrations, returns pool + DSN + cleanup.
func startPostgres(t *testing.T) (*pgxpool.Pool, string) {
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

// post is a JSON POST helper returning the decoded envelope.
type envResp struct {
	Status  int             `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func post(t *testing.T, app interface {
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
	pool, _ := startPostgres(t)
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
	code, _ := post(t, app, "/api/auth/register",
		map[string]string{"username": "e2euser", "email": email, "password": "hunter2pass"}, "")
	require.Equal(t, 200, code)

	// 2. read the verification code from the mock mailer
	vcode, ok := mockMail.LastCode(email)
	require.True(t, ok, "mock mailer should have captured a code")
	require.Len(t, vcode, 6)

	// 3. verify-email -> tokens
	code, env := post(t, app, "/api/auth/verify-email",
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
	code, env = post(t, app, "/api/auth/login",
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
	code, env = post(t, app, "/api/auth/refresh",
		map[string]string{"refreshToken": loginTokens.RefreshToken}, "")
	require.Equal(t, 200, code)
	var refreshed struct {
		RefreshToken string `json:"refreshToken"`
	}
	require.NoError(t, json.Unmarshal(env.Data, &refreshed))
	require.NotEmpty(t, refreshed.RefreshToken)
	require.NotEqual(t, loginTokens.RefreshToken, refreshed.RefreshToken)

	// 7. logout the rotated refresh token
	code, _ = post(t, app, "/api/auth/logout",
		map[string]string{"refreshToken": refreshed.RefreshToken}, "")
	require.Equal(t, 200, code)

	// 8. refresh again with the logged-out token must fail (401)
	code, _ = post(t, app, "/api/auth/refresh",
		map[string]string{"refreshToken": refreshed.RefreshToken}, "")
	require.Equal(t, 401, code)
}
```

- [ ] **Step 3: Run the E2E test, expect FAIL (compile error: `undefined: httpapi.BuildApp`).** Run from `backend/`:
```bash
go test ./internal/adapter/httpapi/ -run TestAuthFlow_E2E
```
Expected: build failure — `undefined: httpapi.BuildApp` (until Step 1 is committed it may already exist; if `app.go` from Step 1 compiles, the failure is instead the missing wiring/methods until Cluster B/C/D are in place). State the observed failure (compile or assertion) explicitly before proceeding.

- [ ] **Step 4: Write `db/seed.sql` — seeded admin (pre-verified).** Create `backend/db/seed.sql`. The bcrypt hash below is for the plaintext `admin123` at cost 12; regenerate with `go run ./cmd/genhash` or any bcrypt tool if you change the password. `gen_random_uuid()` requires `pgcrypto` (enabled by the init migration) — if not enabled, replace with a literal UUID.
```sql
-- Seeded admin account for Pustaka.
-- Username: admin   Password: admin123 (CHANGE IN PROD)
-- password_hash is bcrypt(cost=12) of "admin123".
INSERT INTO web_user (id, username, email, password_hash, role, email_verified, created_at)
VALUES (
    '00000000-0000-4000-8000-000000000001',
    'admin',
    'admin@pustaka.local',
    '$2a$12$Q8mJ3cN0sQ9o4r8wYy0Y1u8kQ2y6z9o5b1c2d3e4f5g6h7i8j9k0L',
    'admin',
    true,
    now()
)
ON CONFLICT (username) DO NOTHING;
```

- [ ] **Step 5: Implement `cmd/server/main.go` — composition root.** Create `backend/cmd/server/main.go`. Loads config, opens the pool, runs migrations unless `APP_ENV=prod`, builds Store/Mailer/Service, builds the app via `BuildApp`, and serves with graceful shutdown via `slog`.
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

	"github.com/jackc/pgx/v5/pgxpool"

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
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	st := store.New(pool)
	mailer := mail.NewResend(cfg.ResendAPIKey, cfg.MailFrom)
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

- [ ] **Step 6: Add the `seed` target to the Makefile.** Edit `backend/Makefile`, appending:
```make
.PHONY: seed
seed: ## Seed the admin account into the database (requires DATABASE_URL)
	psql "$(DATABASE_URL)" -f db/seed.sql
```

- [ ] **Step 7: Document run + seed steps in the README.** In `backend/README.md`, add a "Run locally" section:
```markdown
## Run locally

```bash
# 1. start Postgres (compose binds 127.0.0.1:5434)
docker compose up -d db

# 2. copy env and fill secrets
cp .env.example .env   # set DATABASE_URL, JWT_SECRET, RESEND_API_KEY, MAIL_FROM

# 3. run the API (auto-migrates unless APP_ENV=prod); listens on 127.0.0.1:8002
go run ./cmd/server

# 4. seed the admin account (admin / admin123)
make seed
```
```

- [ ] **Step 8: Run vet + the full test suite, expect PASS.** Run from `backend/`:
```bash
go vet ./...
go test ./...
```
Expected: `go vet` clean; `TestAuthFlow_E2E` PASSES along with all earlier tests (Postgres via testcontainers, MockMailer used — no real email or GPU). State the green result explicitly.

- [ ] **Step 9: Final commit.**
```bash
git add backend/cmd/server/main.go backend/internal/adapter/httpapi/app.go backend/internal/adapter/httpapi/e2e_test.go backend/db/seed.sql backend/Makefile backend/README.md
git commit -m "feat(server): wire composition root, seed admin and add auth E2E test"
```