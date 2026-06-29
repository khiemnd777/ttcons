DATABASE_URL ?= postgres://consflow:consflow@127.0.0.1:5432/consflow?sslmode=disable
JWT_SECRET ?= consflow-local-dev-secret
GOCACHE ?= $(CURDIR)/.gocache

.PHONY: dev migrate migrate-check seed test fmt ent-generate

dev:
	DATABASE_URL="$(DATABASE_URL)" JWT_SECRET="$(JWT_SECRET)" go run ./cmd/server

migrate:
	MIGRATIONS_DIR=migrations DATABASE_URL="$(DATABASE_URL)" go run ./cmd/migrate

migrate-check:
	GOCACHE="$(GOCACHE)" MIGRATIONS_DIR=migrations go run ./cmd/migrate -check

seed:
	@if docker compose -f ../docker-compose.yml ps --status running postgres >/dev/null 2>&1; then \
		docker compose -f ../docker-compose.yml exec -T postgres psql -U consflow -d consflow < seed/seed.sql; \
		if docker compose -f ../docker-compose.yml ps --status running backend >/dev/null 2>&1; then \
			docker compose -f ../docker-compose.yml cp seed/uploads/. backend:/app/uploads/ >/dev/null; \
		fi; \
	else \
		psql "$(DATABASE_URL)" -f seed/seed.sql; \
		mkdir -p "$${UPLOAD_DIR:-uploads}"; \
		cp -R seed/uploads/. "$${UPLOAD_DIR:-uploads}/"; \
	fi

test:
	GOCACHE="$(GOCACHE)" go test ./...

fmt:
	gofmt -w cmd internal ent

ent-generate:
	go run entgo.io/ent/cmd/ent generate ./ent/schema
