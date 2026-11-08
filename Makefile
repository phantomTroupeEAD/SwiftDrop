# ==============================================================================
# SwiftDrop System-Wide Automation Makefile
# Authority: docs/drafts/member-09-devops-k8s/doc-08-implementation-deployment-and-ops.md
# Decided by Team: Decision C22 & Work Division §3 (Member 9)
# ==============================================================================

COMPOSE_FILE := deploy/docker-compose.yml

.PHONY: help up down restart ps logs migrate-up seed test test-e2e reset clean

help:
	@echo "SwiftDrop Development & Orchestration Commands"
	@echo "------------------------------------------------"
	@echo "make up          - Start all background containers via Docker Compose"
	@echo "make down        - Stop all running containers"
	@echo "make restart     - Restart all containers"
	@echo "make ps          - Display container health and status"
	@echo "make logs        - Follow logs across all containers"
	@echo "make migrate-up  - Run database migrations via one-shot container"
	@echo "make seed        - Execute mock accounts & test data seed"
	@echo "make test        - Run Go unit & race tests in core-api"
	@echo "make test-e2e    - Run Python Playwright end-to-end smoke tests"
	@echo "make reset       - Stop containers and purge all database/message volumes"

up:
	docker compose -f $(COMPOSE_FILE) up -d

down:
	docker compose -f $(COMPOSE_FILE) down

restart:
	docker compose -f $(COMPOSE_FILE) restart

ps:
	docker compose -f $(COMPOSE_FILE) ps

logs:
	docker compose -f $(COMPOSE_FILE) logs -f

migrate-up:
	docker compose -f $(COMPOSE_FILE) run --rm core-api-migrate

seed:
	docker compose -f $(COMPOSE_FILE) exec core-api /app/migrate

test:
	cd core-api && go test -v -race ./...

test-e2e:
	pytest tests/e2e/ -v

reset:
	docker compose -f $(COMPOSE_FILE) down -v

