COMPOSE ?= docker compose
SERVICE ?=
BACKEND_GOCACHE ?= $(CURDIR)/backend/.gocache

.PHONY: up down restart build run log test backend-test backend-fmt frontend-build migrate-check docker-config

up:
	$(MAKE) --no-print-directory down
	$(MAKE) --no-print-directory build
	$(MAKE) --no-print-directory run

down:
	$(COMPOSE) down --remove-orphans

restart:
	$(MAKE) --no-print-directory down
	$(MAKE) --no-print-directory build
	$(MAKE) --no-print-directory run

build:
	$(COMPOSE) build

run:
	$(COMPOSE) up -d

log:
	$(COMPOSE) logs -f --tail=200 $(SERVICE)

test:
	$(MAKE) --no-print-directory backend-test
	$(MAKE) --no-print-directory frontend-build

backend-test:
	cd backend && GOCACHE="$(BACKEND_GOCACHE)" go test ./...

backend-fmt:
	cd backend && make fmt

frontend-build:
	cd frontend && npm run build

migrate-check:
	cd backend && make migrate-check

docker-config:
	$(COMPOSE) config
