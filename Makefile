.PHONY: help up down logs ps clean db-shell db-logs otel-logs build-docker test test-integration lint build ci

help: ## Show available commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
			awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

up: ## Start local infrastructure (Postgres + OTEL Collector)
	docker compose -f deploy/docker/docker-compose.yml up -d

down: ## Stop services and remove volumes
	docker compose -f deploy/docker/docker-compose.yml down -v

logs: ## Show logs from all services
	docker compose -f deploy/docker/docker-compose.yml logs -f

ps: ## Show running services
	docker compose -f deploy/docker/docker-compose.yml ps

clean: ## Stop services (keep volumes)
	docker compose -f deploy/docker/docker-compose.yml down

db-shell: ## Connect to PostgreSQL shell
	docker exec -it pfm-postgres psql -U pfm_user -d pfm_dev

db-logs: ## Show PostgreSQL logs
	docker compose -f deploy/docker/docker-compose.yml logs -f postgres

otel-logs: ## Show OTEL Collector logs
	docker compose -f deploy/docker/docker-compose.yml logs -f otel-collector

build-docker: ## Build the application Docker image
	docker build -f deploy/docker/Dockerfile -t pfm-go:dev \
		--build-arg GIT_COMMIT=$$(git rev-parse --short HEAD) \
		--build-arg BUILD_TIME=$$(date -u +%Y-%m-%dT%H:%M:%SZ) .

test: ## Run tests with race detector
	go test ./... -race -count=1

lint: ## Run golangci-lint
	golangci-lint run ./...

build: ## Build the application binary
	go build -ldflags="-X main.Version=$$(git describe --tags --always --dirty) -X main.GitCommit=$$(git rev-parse --short HEAD) -X main.BuildTime=$$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o bin/pfm ./cmd/pfm/

ci: lint test-integration build ## Run full CI gate (lint + test + build)

test-integration: ## Run full integration tests suite with race detector (requires Docker)
	TESTCONTAINERS_RYUK_DISABLED=true go test -tags integration -race -count=1 -v ./...
