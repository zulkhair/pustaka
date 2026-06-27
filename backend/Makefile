.PHONY: help run test int-test int-test-real vet lint sqlc migrate seed db-up db-down

help: ## Show run notes and available targets
	@echo "Pustaka backend — local dev"
	@echo "  1. make db-up      # start Postgres (binds 127.0.0.1:5434)"
	@echo "  2. cp .env.example .env  # set DATABASE_URL, JWT_SECRET, RESEND_API_KEY, MAIL_FROM"
	@echo "  3. make run        # auto-migrates unless APP_ENV=prod; listens on :8002"
	@echo "  4. make seed       # idempotently upsert the admin account"

run:
	go run ./cmd/server

seed:
	go run ./cmd/seed

test:
	go test ./...

int-test:
	go test -tags=integration ./test/integration/... -count=1 -v

int-test-real:
	OLLAMA_REAL=1 go test -tags=integration ./test/integration/... -count=1 -v -timeout 20m

vet:
	go vet ./...

lint: vet

db-up:
	docker compose up -d db

db-down:
	docker compose down

sqlc:
	sqlc generate
