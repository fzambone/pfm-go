.PHONY: help up down logs ps clean db-shell db-logs otel-logs

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
