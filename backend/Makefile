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
		$(BACKEND_COMPOSE) exec -T postgres psql -v ON_ERROR_STOP=1 -U "$(POSTGRES_USER)" -d "$(POSTGRES_DB)" < seed/seed.sql; \
		$(BACKEND_COMPOSE) exec -T postgres psql -v ON_ERROR_STOP=1 -U "$(POSTGRES_USER)" -d "$(POSTGRES_DB)" < migrations/V41__official_workflow_business_forms.sql; \
		$(BACKEND_COMPOSE) exec -T postgres psql -v ON_ERROR_STOP=1 -U "$(POSTGRES_USER)" -d "$(POSTGRES_DB)" < migrations/V66__planning_construction_handoff.sql; \
		$(BACKEND_COMPOSE) exec -T postgres psql -v ON_ERROR_STOP=1 -U "$(POSTGRES_USER)" -d "$(POSTGRES_DB)" < migrations/V85__official_client_workflow_rev3.sql; \
		$(BACKEND_COMPOSE) exec -T postgres psql -v ON_ERROR_STOP=1 -U "$(POSTGRES_USER)" -d "$(POSTGRES_DB)" < migrations/V86__official_client_workflow_rev4.sql; \
		$(BACKEND_COMPOSE) exec -T postgres psql -v ON_ERROR_STOP=1 -U "$(POSTGRES_USER)" -d "$(POSTGRES_DB)" < migrations/V87__rev4_quality_dossier_step_type.sql; \
		$(BACKEND_COMPOSE) exec -T postgres psql -v ON_ERROR_STOP=1 -U "$(POSTGRES_USER)" -d "$(POSTGRES_DB)" < migrations/V92__official_client_workflow_rev5_dynamic_intake.sql; \
		$(BACKEND_COMPOSE) exec -T postgres psql -v ON_ERROR_STOP=1 -U "$(POSTGRES_USER)" -d "$(POSTGRES_DB)" < migrations/V102__lean_official_workflow_forms.sql; \
		$(BACKEND_COMPOSE) exec -T postgres psql -v ON_ERROR_STOP=1 -U "$(POSTGRES_USER)" -d "$(POSTGRES_DB)" < migrations/V103__operational_official_workflow_rev7.sql; \
		$(BACKEND_COMPOSE) exec -T postgres psql -v ON_ERROR_STOP=1 -U "$(POSTGRES_USER)" -d "$(POSTGRES_DB)" < migrations/V104__rev7_returned_scan_source_gate.sql; \
		$(BACKEND_COMPOSE) exec -T postgres psql -v ON_ERROR_STOP=1 -U "$(POSTGRES_USER)" -d "$(POSTGRES_DB)" < migrations/V105__lean_official_workflow_rev8.sql; \
		$(BACKEND_COMPOSE) exec -T postgres psql -v ON_ERROR_STOP=1 -U "$(POSTGRES_USER)" -d "$(POSTGRES_DB)" < migrations/V131__accounting_handoff_workspace.sql; \
		$(BACKEND_COMPOSE) exec -T postgres psql -v ON_ERROR_STOP=1 -U "$(POSTGRES_USER)" -d "$(POSTGRES_DB)" < seed/golden_project.sql; \
		$(BACKEND_COMPOSE) exec -T postgres psql -v ON_ERROR_STOP=1 -U "$(POSTGRES_USER)" -d "$(POSTGRES_DB)" < seed/golden_project_domains.sql; \
		$(BACKEND_COMPOSE) exec -T postgres psql -v ON_ERROR_STOP=1 -U "$(POSTGRES_USER)" -d "$(POSTGRES_DB)" < seed/golden_project_closure.sql; \
		$(BACKEND_COMPOSE) exec -T postgres psql -v ON_ERROR_STOP=1 -U "$(POSTGRES_USER)" -d "$(POSTGRES_DB)" < seed/golden_project_construction.sql; \
		$(BACKEND_COMPOSE) exec -T postgres psql -v ON_ERROR_STOP=1 -U "$(POSTGRES_USER)" -d "$(POSTGRES_DB)" < seed/golden_project_finance.sql; \
		$(BACKEND_COMPOSE) exec -T postgres psql -v ON_ERROR_STOP=1 -U "$(POSTGRES_USER)" -d "$(POSTGRES_DB)" < seed/golden_live_project.sql; \
		$(BACKEND_COMPOSE) exec -T postgres psql -v ON_ERROR_STOP=1 -U "$(POSTGRES_USER)" -d "$(POSTGRES_DB)" < seed/golden_client_workflow_rev3.sql; \
		$(BACKEND_COMPOSE) exec -T postgres psql -v ON_ERROR_STOP=1 -U "$(POSTGRES_USER)" -d "$(POSTGRES_DB)" < seed/golden_client_workflow_rev4_uat.sql; \
		$(BACKEND_COMPOSE) exec -T postgres psql -v ON_ERROR_STOP=1 -U "$(POSTGRES_USER)" -d "$(POSTGRES_DB)" < seed/golden_client_workflow_rev6_lean.sql; \
		$(BACKEND_COMPOSE) exec -T postgres psql -v ON_ERROR_STOP=1 -U "$(POSTGRES_USER)" -d "$(POSTGRES_DB)" < seed/golden_client_workflow_rev7_operational.sql; \
		$(BACKEND_COMPOSE) exec -T postgres psql -v ON_ERROR_STOP=1 -U "$(POSTGRES_USER)" -d "$(POSTGRES_DB)" < seed/golden_client_workflow_rev8_lean.sql; \
		$(BACKEND_COMPOSE) exec -T postgres psql -v ON_ERROR_STOP=1 -U "$(POSTGRES_USER)" -d "$(POSTGRES_DB)" < seed/validate_golden_project.sql; \
	else \
		psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f seed/seed.sql \
			-f migrations/V41__official_workflow_business_forms.sql \
			-f migrations/V66__planning_construction_handoff.sql \
			-f migrations/V85__official_client_workflow_rev3.sql \
			-f migrations/V86__official_client_workflow_rev4.sql \
			-f migrations/V87__rev4_quality_dossier_step_type.sql \
			-f migrations/V92__official_client_workflow_rev5_dynamic_intake.sql \
			-f migrations/V102__lean_official_workflow_forms.sql \
			-f migrations/V103__operational_official_workflow_rev7.sql \
			-f migrations/V104__rev7_returned_scan_source_gate.sql \
			-f migrations/V105__lean_official_workflow_rev8.sql \
			-f migrations/V131__accounting_handoff_workspace.sql \
			-f seed/golden_project.sql \
			-f seed/golden_project_domains.sql \
			-f seed/golden_project_closure.sql \
			-f seed/golden_project_construction.sql \
			-f seed/golden_project_finance.sql \
			-f seed/golden_live_project.sql \
			-f seed/golden_client_workflow_rev3.sql \
			-f seed/golden_client_workflow_rev4_uat.sql \
			-f seed/golden_client_workflow_rev6_lean.sql \
			-f seed/golden_client_workflow_rev7_operational.sql \
			-f seed/golden_client_workflow_rev8_lean.sql \
			-f seed/validate_golden_project.sql; \
	fi; \
	mkdir -p "$(STORAGE_PATH)"; \
	cp -R seed/uploads/. "$(STORAGE_PATH)/"

seed-validate: check-local-env
	@if $(BACKEND_COMPOSE) ps --status running postgres >/dev/null 2>&1; then \
		$(BACKEND_COMPOSE) exec -T postgres psql -v ON_ERROR_STOP=1 -U "$(POSTGRES_USER)" -d "$(POSTGRES_DB)" < seed/validate_golden_project.sql; \
	else \
		psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f seed/validate_golden_project.sql; \
	fi

seed-fixtures-check:
	python3 seed/generate_golden_fixtures.py --check

test:
	GOCACHE="$(GOCACHE)" go test ./...

fmt:
	gofmt -w cmd internal ent

ent-generate:
	go run entgo.io/ent/cmd/ent generate ./ent/schema
