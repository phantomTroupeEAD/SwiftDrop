.PHONY: test it up down clean build fmt

test:
	go test -v -count=1 ./...

it:
	INTEGRATION=1 go test -v -count=1 ./tests/...

up:
	docker compose -f docker-compose.dev.yml up -d

down:
	docker compose -f docker-compose.dev.yml down

fmt:
	gofmt -w .

build:
	docker build -t swiftdrop/messaging:latest .

