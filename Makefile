ENV_FILE ?= .env
-include $(ENV_FILE)

COMPOSE ?= docker compose --env-file $(ENV_FILE)
SERVICE ?=
BACKEND_GOCACHE ?= $(CURDIR)/backend/.gocache

.PHONY: check-local-env up down restart build run log test backend-test backend-fmt frontend-build migrate-check docker-config production-bootstrap production-check production-render production-github-secrets production-provision release

check-local-env:
	@test -f "$(ENV_FILE)" || (echo "Missing $(ENV_FILE); copy .env.example to $(ENV_FILE) and configure it."; exit 1)
	@test -n "$(STORAGE_PATH)" || (echo "STORAGE_PATH is required in $(ENV_FILE)."; exit 1)
	@test -n "$(LOCAL_UID)" || (echo "LOCAL_UID is required in $(ENV_FILE)."; exit 1)
	@test -n "$(LOCAL_GID)" || (echo "LOCAL_GID is required in $(ENV_FILE)."; exit 1)

up: check-local-env
	$(MAKE) --no-print-directory down
	$(MAKE) --no-print-directory build
	$(MAKE) --no-print-directory run

down: check-local-env
	$(COMPOSE) down --remove-orphans

restart: check-local-env
	$(MAKE) --no-print-directory down
	$(MAKE) --no-print-directory build
	$(MAKE) --no-print-directory run

build: check-local-env
	$(COMPOSE) build

run: check-local-env
	mkdir -p "$(STORAGE_PATH)"
	$(COMPOSE) up -d

log: check-local-env
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

docker-config: check-local-env
	$(COMPOSE) config

production-bootstrap:
	bash deploy/scripts/bootstrap-production.sh

production-check:
	bash deploy/scripts/validate-production-config.sh --all

production-render:
	@test -n "$(OUTPUT)" || (echo "OUTPUT is required, for example: make production-render OUTPUT=/tmp/consflow-production"; exit 1)
	bash deploy/scripts/render-production-config.sh "$(OUTPUT)"

production-github-secrets:
	@test -n "$(SCOPE)" || (echo "SCOPE is required: repository or environment"; exit 1)
	bash deploy/scripts/configure-github-secrets.sh --scope "$(SCOPE)"

production-provision:
	bash deploy/scripts/provision-vps.sh

release:
	bash deploy/scripts/release-tag.sh "$(TAG)"
