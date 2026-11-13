# ==============================================================================
# SwiftDrop System-Wide Automation Makefile
# Authority: docs/drafts/member-09-devops-k8s/doc-08-implementation-deployment-and-ops.md
# Decided by Team: Decision C22 & Work Division §3 (Member 9)
# ==============================================================================

COMPOSE_FILE := deploy/docker-compose.yml

.PHONY: help up down restart ps logs migrate-up seed test test-e2e reset clean swagger

help:
	@echo "SwiftDrop Development & Orchestration Commands"
	@echo "------------------------------------------------"
	@echo "make up          - Start all background containers via Docker Compose"
	@echo "make down        - Stop all running containers"
	@echo "make restart     - Restart all containers"
	@echo "make ps          - Display container health and status"
	@echo "make logs        - Follow logs across all containers"
	@echo "make migrate-up  - Run database migrations via one-shot container"
	@echo "make seed        - Load deploy/seeds/*.sql into the running database"
	@echo "make test        - Run Go unit & race tests in core-api"
	@echo "make swagger     - Regenerate unified Swagger/OpenAPI docs in core-api"
	@echo "make test-e2e    - Run the Playwright (TypeScript) end-to-end suite"
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

# Loads every deploy/seeds/*.sql (after migrations) into the running postgres container. Seeds are idempotent.
seed:
	@for f in deploy/seeds/*.sql; do 		echo "Seeding $$f"; 		docker compose -f $(COMPOSE_FILE) exec -T postgres sh -c 'psql -v ON_ERROR_STOP=1 -U "$$POSTGRES_USER" -d "$$POSTGRES_DB"' < $$f || exit 1; 	done

test:
	cd core-api && go test -v -race ./...

swagger:
	cd core-api && go run github.com/swaggo/swag/cmd/swag@v1.16.3 init -g cmd/api/main.go -o docs --ot json,yaml --parseDependency --parseInternal
	cp core-api/docs/swagger.json core-api/internal/events/swagger/swagger.json

test-e2e:
	cd tests/e2e && npx playwright test

reset:
	docker compose -f $(COMPOSE_FILE) down -v

