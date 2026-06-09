COMPOSE ?= docker compose
SERVICE ?=

.PHONY: up down restart build run log

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
