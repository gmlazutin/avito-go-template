SHELL := /bin/bash
.ONESHELL:

.PHONY: generate migrate migrate-down run test build

generate:
	go generate ./...

migrate:
	set -a
	source .env
	set +a
	go tool goose -dir migrations postgres "$$DATABASE_URL" up

migrate-down:
	set -a
	source .env
	set +a
	go tool goose -dir migrations postgres "$$DATABASE_URL" down-to 0

run:
	set -a
	source .env
	set +a
	go run ./cmd/trip-service

test:
	go test -race -count=2 ./...

build:
	mkdir -p dist
	go build -o dist/trip-service ./cmd/trip-service
