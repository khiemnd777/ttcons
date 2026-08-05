DATABASE_URL ?= postgres://consflow:consflow@127.0.0.1:5432/consflow?sslmode=disable
JWT_SECRET ?= consflow-local-dev-secret
GOCACHE ?= $(CURDIR)/.gocache

.PHONY: dev migrate migrate-check seed seed-validate seed-fixtures-check test fmt ent-generate

dev:
	DATABASE_URL="$(DATABASE_URL)" JWT_SECRET="$(JWT_SECRET)" go run ./cmd/server

migrate:
	MIGRATIONS_DIR=migrations DATABASE_URL="$(DATABASE_URL)" go run ./cmd/migrate

migrate-check:
	GOCACHE="$(GOCACHE)" MIGRATIONS_DIR=migrations go run ./cmd/migrate -check

seed:
	@if docker compose -f ../docker-compose.yml ps --status running postgres >/dev/null 2>&1; then \
		docker compose -f ../docker-compose.yml exec -T postgres psql -v ON_ERROR_STOP=1 -U consflow -d consflow < seed/seed.sql; \
		docker compose -f ../docker-compose.yml exec -T postgres psql -v ON_ERROR_STOP=1 -U consflow -d consflow < migrations/V66__planning_construction_handoff.sql; \
		docker compose -f ../docker-compose.yml exec -T postgres psql -v ON_ERROR_STOP=1 -U consflow -d consflow < migrations/V85__official_client_workflow_rev3.sql; \
		docker compose -f ../docker-compose.yml exec -T postgres psql -v ON_ERROR_STOP=1 -U consflow -d consflow < migrations/V86__official_client_workflow_rev4.sql; \
		docker compose -f ../docker-compose.yml exec -T postgres psql -v ON_ERROR_STOP=1 -U consflow -d consflow < migrations/V87__rev4_quality_dossier_step_type.sql; \
		docker compose -f ../docker-compose.yml exec -T postgres psql -v ON_ERROR_STOP=1 -U consflow -d consflow < seed/golden_project.sql; \
		docker compose -f ../docker-compose.yml exec -T postgres psql -v ON_ERROR_STOP=1 -U consflow -d consflow < seed/golden_project_domains.sql; \
		docker compose -f ../docker-compose.yml exec -T postgres psql -v ON_ERROR_STOP=1 -U consflow -d consflow < seed/golden_project_closure.sql; \
		docker compose -f ../docker-compose.yml exec -T postgres psql -v ON_ERROR_STOP=1 -U consflow -d consflow < seed/golden_project_construction.sql; \
		docker compose -f ../docker-compose.yml exec -T postgres psql -v ON_ERROR_STOP=1 -U consflow -d consflow < seed/golden_project_finance.sql; \
		docker compose -f ../docker-compose.yml exec -T postgres psql -v ON_ERROR_STOP=1 -U consflow -d consflow < seed/golden_live_project.sql; \
		docker compose -f ../docker-compose.yml exec -T postgres psql -v ON_ERROR_STOP=1 -U consflow -d consflow < seed/golden_client_workflow_rev3.sql; \
		docker compose -f ../docker-compose.yml exec -T postgres psql -v ON_ERROR_STOP=1 -U consflow -d consflow < seed/golden_client_workflow_rev4_uat.sql; \
		docker compose -f ../docker-compose.yml exec -T postgres psql -v ON_ERROR_STOP=1 -U consflow -d consflow < seed/validate_golden_project.sql; \
		if docker compose -f ../docker-compose.yml ps --status running backend >/dev/null 2>&1; then \
			docker compose -f ../docker-compose.yml cp seed/uploads/. backend:/app/uploads/ >/dev/null; \
		fi; \
	else \
		psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f seed/seed.sql \
			-f migrations/V66__planning_construction_handoff.sql \
			-f migrations/V85__official_client_workflow_rev3.sql \
			-f migrations/V86__official_client_workflow_rev4.sql \
			-f migrations/V87__rev4_quality_dossier_step_type.sql \
			-f seed/golden_project.sql \
			-f seed/golden_project_domains.sql \
			-f seed/golden_project_closure.sql \
			-f seed/golden_project_construction.sql \
			-f seed/golden_project_finance.sql \
			-f seed/golden_live_project.sql \
			-f seed/golden_client_workflow_rev3.sql \
			-f seed/golden_client_workflow_rev4_uat.sql \
			-f seed/validate_golden_project.sql; \
		mkdir -p "$${UPLOAD_DIR:-uploads}"; \
		cp -R seed/uploads/. "$${UPLOAD_DIR:-uploads}/"; \
	fi

seed-validate:
	@if docker compose -f ../docker-compose.yml ps --status running postgres >/dev/null 2>&1; then \
		docker compose -f ../docker-compose.yml exec -T postgres psql -v ON_ERROR_STOP=1 -U consflow -d consflow < seed/validate_golden_project.sql; \
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
