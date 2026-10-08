ENV_FILE ?= ../.env
-include $(ENV_FILE)

GOCACHE ?= $(CURDIR)/.gocache
BACKEND_COMPOSE ?= docker compose --env-file $(ENV_FILE) -f ../docker-compose.yml

export APP_ENV PORT DATABASE_URL REDIS_URL JWT_SECRET FRONTEND_ORIGIN MIGRATIONS_DIR AUTO_MIGRATE
export AUTH_RECOVERY_ENABLED AUTH_RECOVERY_TOKEN_TTL AUTH_RECOVERY_VERIFY_TTL
export SMTP_HOST SMTP_PORT SMTP_USERNAME SMTP_PASSWORD SMTP_PASSWORD_FILE SMTP_FROM_ADDRESS SMTP_FROM_NAME SMTP_TLS_MODE SMTP_TIMEOUT
export STORAGE_PATH STORAGE_PROVIDER STORAGE_OPERATION_TIMEOUT MAX_UPLOAD_BYTES
export STORAGE_S3_BUCKET STORAGE_S3_REGION STORAGE_S3_ENDPOINT STORAGE_S3_USE_PATH_STYLE STORAGE_S3_ALLOW_INSECURE_ENDPOINT
export AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_SESSION_TOKEN

.PHONY: check-local-env dev migrate migrate-check seed seed-validate seed-fixtures-check test fmt ent-generate

check-local-env:
	@test -f "$(ENV_FILE)" || (echo "Missing $(ENV_FILE); copy ../.env.example to $(ENV_FILE) and configure it."; exit 1)
	@test -n "$(DATABASE_URL)" || (echo "DATABASE_URL is required in $(ENV_FILE)."; exit 1)
	@test -n "$(REDIS_URL)" || (echo "REDIS_URL is required in $(ENV_FILE)."; exit 1)
	@test -n "$(JWT_SECRET)" || (echo "JWT_SECRET is required in $(ENV_FILE)."; exit 1)
	@test -n "$(STORAGE_PROVIDER)" || (echo "STORAGE_PROVIDER is required in $(ENV_FILE)."; exit 1)
	@test -n "$(STORAGE_PATH)" || (echo "STORAGE_PATH is required in $(ENV_FILE)."; exit 1)
	@test -n "$(POSTGRES_USER)" || (echo "POSTGRES_USER is required in $(ENV_FILE)."; exit 1)
	@test -n "$(POSTGRES_DB)" || (echo "POSTGRES_DB is required in $(ENV_FILE)."; exit 1)

dev: check-local-env
	go run ./cmd/server

migrate: check-local-env
	MIGRATIONS_DIR=migrations DATABASE_URL="$(DATABASE_URL)" go run ./cmd/migrate

migrate-check:
	GOCACHE="$(GOCACHE)" MIGRATIONS_DIR=migrations go run ./cmd/migrate -check

seed: check-local-env
	@if $(BACKEND_COMPOSE) ps --status running postgres >/dev/null 2>&1; then \
		$(BACKEND_COMPOSE) exec -T postgres psql -v ON_ERROR_STOP=1 -U "$(POSTGRES_USER)" -d "$(POSTGRES_DB)" < seed/seed.sql && \
		$(BACKEND_COMPOSE) exec -T postgres psql -v ON_ERROR_STOP=1 -U "$(POSTGRES_USER)" -d "$(POSTGRES_DB)" < seed/validate_current_seed.sql; \
	else \
		psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f seed/seed.sql -f seed/validate_current_seed.sql; \
	fi

seed-validate: check-local-env
	@if $(BACKEND_COMPOSE) ps --status running postgres >/dev/null 2>&1; then \
		$(BACKEND_COMPOSE) exec -T postgres psql -v ON_ERROR_STOP=1 -U "$(POSTGRES_USER)" -d "$(POSTGRES_DB)" < seed/validate_current_seed.sql; \
	else \
		psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f seed/validate_current_seed.sql; \
	fi

seed-fixtures-check:
	python3 seed/generate_golden_fixtures.py --check

test:
	GOCACHE="$(GOCACHE)" go test ./...

fmt:
	gofmt -w cmd internal ent

ent-generate:
	go run entgo.io/ent/cmd/ent generate ./ent/schema
