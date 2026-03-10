.PHONY: build run test test-unit test-integration docker-up docker-down lint

build:
	go build -o bin/task-manager ./cmd/server

run:
	CONFIG_PATH=config.yaml go run ./cmd/server

test-unit:
	go test -v -race -coverprofile=coverage.out ./tests/unit/...
	go tool cover -func=coverage.out

test-integration:
	go test -v -timeout 120s ./tests/integration/...

test: test-unit test-integration

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down -v

lint:
	golangci-lint run ./...

migrate:
	psql "postgres://taskuser:taskpass@localhost:5432/taskdb?sslmode=disable" -f migrations/001_initial_schema.sql

coverage-html:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

swagger:
	swag init -g ./cmd/server/main.go -o ./docs

