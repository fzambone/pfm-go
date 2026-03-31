.PHONY: help up down logs ps clean db-shell db-logs otel-logs build-docker test test-integration coverage vuln lint build generate ci

# GOBIN resolves to the user's GOPATH bin directory where tools like govulncheck and sqlc live.
# Install govulncheck once with: go install golang.org/x/vuln/cmd/govulncheck@latest
# Install sqlc once with:        go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
GOBIN ?= $(shell go env GOPATH)/bin

# COVERAGE_THRESHOLD is the minimum acceptable unit test coverage percentage.
# Packages that require testcontainers (database), have no executable statements (message,
# db), or are the composition root (cmd/pfm) are excluded via -coverpkg selection.
# Raise this value as coverage improves — target is 80%.
# Currently gated at 70% because observe (73.5%) and validate (72.4%) have pre-existing
# gaps. Open issues to cover NewTracerProvider error paths and validate rule branches.
COVERAGE_THRESHOLD ?= 70

# UNIT_TEST_PKGS is the set of packages measured by the coverage gate.
# Excluded intentionally:
#   cmd/pfm          — composition root, no unit-testable logic
#   db               — embed-only package, no executable statements
#   internal/message — pure constants, no executable statements
#   internal/platform/database — Open/Migrate require real Postgres (testcontainers only)
UNIT_TEST_PKGS := \
	github.com/zambone/pfm-go/internal/adapter/http \
	github.com/zambone/pfm-go/internal/platform/clock \
	github.com/zambone/pfm-go/internal/platform/config \
	github.com/zambone/pfm-go/internal/platform/ctxutil \
	github.com/zambone/pfm-go/internal/platform/money \
	github.com/zambone/pfm-go/internal/platform/observe \
	github.com/zambone/pfm-go/internal/platform/validate

help: ## Show available commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
			awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

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

test: ## Run unit tests with race detector
	go test -race -count=1 ./...

test-integration: ## Run full test suite including integration tests (requires Docker/Podman)
	TESTCONTAINERS_RYUK_DISABLED=true go test -tags integration -race -count=1 -v ./...

coverage: ## Run unit tests with coverage report and enforce COVERAGE_THRESHOLD (default 70)
	go test -race -count=1 -coverprofile=coverage.out -coverpkg=$(subst $(space),$(comma),$(UNIT_TEST_PKGS)) $(UNIT_TEST_PKGS)
	go tool cover -func=coverage.out
	@TOTAL=$$(go tool cover -func=coverage.out | grep '^total:' | awk '{print $$3}' | tr -d '%'); \
	  printf 'Coverage: %s%% (threshold: $(COVERAGE_THRESHOLD)%%)\n' "$$TOTAL"; \
	  if [ "$$(awk "BEGIN{print ($$TOTAL+0 < $(COVERAGE_THRESHOLD)) ? 1 : 0}")" = "1" ]; then \
	    printf 'FAIL: coverage %s%% is below $(COVERAGE_THRESHOLD)%% threshold\n' "$$TOTAL"; \
	    exit 1; \
	  fi

generate: ## Regenerate sqlc Go code and OpenAPI spec from handler annotations
	$(GOBIN)/sqlc generate
	$(GOBIN)/swag init -g cmd/pfm/main.go --output api/ --outputTypes yaml

vuln: ## Run govulncheck vulnerability scan
	$(GOBIN)/govulncheck ./...

lint: ## Run golangci-lint
	golangci-lint run ./...

build: ## Build the application binary
	go build -ldflags="-X main.Version=$$(git describe --tags --always --dirty) -X main.GitCommit=$$(git rev-parse --short HEAD) -X main.BuildTime=$$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o bin/pfm ./cmd/pfm/

ci: lint coverage vuln test-integration build ## Run full CI gate (lint + coverage + vuln + integration tests + build)

# Internal helpers for string manipulation in coverage target.
comma := ,
space := $(empty) $(empty)
