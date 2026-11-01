# Member 7 local commands. Run from this folder. See README.md.
.PHONY: up down wipe demo shutdown-demo test test-db lint gateway-check build docker-build
TEST_DATABASE_URL ?= postgres://swiftdrop_app:app_dev_only@localhost:5432/swiftdrop_db?sslmode=disable

up:             ## start PostgreSQL, migrations, seed, core-api, Traefik
	bash scripts/local-up.sh
down:           ## stop everything
	bash scripts/local-down.sh
wipe:           ## stop and delete the local database
	bash scripts/local-down.sh --wipe
demo:           ## 30 checks through the gateway (needs a fresh database)
	bash scripts/demo.sh
shutdown-demo:  ## show graceful shutdown, then restart core-api
	bash scripts/shutdown-demo.sh
test:           ## unit tests (no database needed)
	cd core-api && go test -race ./...
test-db:        ## unit + real PostgreSQL tests (needs make up)
	cd core-api && TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test -race -count=1 -cover ./...
lint:
	cd core-api && test -z "$$(gofmt -l .)" && go vet ./...
gateway-check:  ## parity + Kubernetes schema + 21 security checks on real Traefik
	python3 scripts/gateway_parity.py
	kubeconform -strict -summary -schema-location default -schema-location 'https://raw.githubusercontent.com/datreeio/CRDs-catalog/main/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json' deploy/kubernetes/core-api deploy/gateway/traefik/kubernetes
	bash tools/security_gateway_test.sh deploy/gateway/traefik/traefik.yml deploy/gateway/traefik/dynamic-routes.yml $${TRAEFIK:-traefik}
build:
	cd core-api && CGO_ENABLED=0 go build -trimpath -o bin/ ./cmd/...
docker-build:   ## needs a Docker daemon
	docker build -f core-api/Dockerfile -t swiftdrop/core-api:dev core-api
	docker build -f core-api/Dockerfile.migrate -t swiftdrop/core-api-migrations:dev core-api
