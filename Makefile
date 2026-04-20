-include .env
export

DB_DSN ?= postgres://uniscoot:uniscoot@localhost:5432/uniscoot?sslmode=disable

.PHONY: migrate-up migrate-down migrate-status lint test run-api docker-up docker-down tidy sqlc

migrate-up:
	GOOSE_DRIVER=postgres \
	GOOSE_DBSTRING="$(DB_DSN)" \
	GOOSE_MIGRATION_DIR=app/internal/storage/postgres/migrations \
	goose up

migrate-down:
	GOOSE_DRIVER=postgres \
	GOOSE_DBSTRING="$(DB_DSN)" \
	GOOSE_MIGRATION_DIR=app/internal/storage/postgres/migrations \
	goose down

migrate-status:
	GOOSE_DRIVER=postgres \
	GOOSE_DBSTRING="$(DB_DSN)" \
	GOOSE_MIGRATION_DIR=app/internal/storage/postgres/migrations \
	goose status

lint:
	golangci-lint run

test:
	go test ./...

run-api:
	go run ./app/cmd/api

tidy:
	go mod tidy

sqlc:
	cd app/internal/storage/postgres/sqlc && sqlc generate -f sqlc.yml

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down -v
