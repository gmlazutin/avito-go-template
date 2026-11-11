-include .env
export

.PHONY: generate migrate migrate-down run test test-integration lint build

GO ?= go
GOLANGCI_LINT ?= golangci-lint
COVERAGE_FILE ?= /tmp/trip-service-coverage.out

generate:
	$(GO) generate ./...

migrate:
	$(GO) tool goose -dir migrations postgres "$$DATABASE_URL" up

migrate-down:
	$(GO) tool goose -dir migrations postgres "$$DATABASE_URL" down-to 0

run:
	$(GO) run ./cmd/trip-service

test:
	$(GO) test -race -count=2 -coverprofile="$(COVERAGE_FILE)" ./...
	$(GO) tool cover -func="$(COVERAGE_FILE)" | tail -n 1

test-integration:
	$(GO) test -race -count=2 -tags=integration ./internal/repository/...

lint:
	$(GOLANGCI_LINT) run ./...

build:
	mkdir -p dist
	$(GO) build -o dist/trip-service ./cmd/trip-service
