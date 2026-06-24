.PHONY: run test lint build docker-up docker-down docker-build schema-apply sqlc-generate clean

APP_NAME=skolva
BUILD_DIR=bin
GO=$(shell which go)
GOLANGCI_LINT=$(shell which golangci-lint)
SQLC=$(shell which sqlc)

run:
	$(GO) run ./cmd/api

test:
	$(GO) test -v -count=1 -coverprofile=coverage.out ./...

test-short:
	$(GO) test -short -v -count=1 ./...

coverage: test
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

lint:
	$(GOLANGCI_LINT) run ./...

build:
	$(GO) build -o $(BUILD_DIR)/$(APP_NAME) ./cmd/api

docker-up:
	docker compose up -d

docker-down:
	docker compose down -v

docker-build:
	docker build -t $(APP_NAME):latest .

schema-apply:
	docker compose exec -T postgres psql -U vv -d vv < schema.sql

sqlc-generate:
	$(SQLC) generate -f sqlc/sqlc.yaml

tidy:
	$(GO) mod tidy

clean:
	rm -rf $(BUILD_DIR) coverage.out coverage.html

all: lint test build
