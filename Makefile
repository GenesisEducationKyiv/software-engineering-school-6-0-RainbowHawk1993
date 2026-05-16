.PHONY: help test integration-test integration-test-clean integration-test-debug build run clean

help: ## Display this help screen
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

test: ## Run unit tests
	go test ./...

test-verbose: ## Run unit tests with verbose output
	go test -v ./...

test-coverage: ## Run unit tests with coverage
	go test -cover ./...

integration-test: ## Run integration tests using Docker Compose
	./run-integration-tests.sh

integration-test-clean: ## Run integration tests with clean state (remove containers/volumes first)
	./run-integration-tests.sh --clean

integration-test-debug: ## Run integration tests but keep containers for debugging
	./run-integration-tests.sh --no-cleanup

integration-test-local: ## Run integration tests locally (requires PostgreSQL and Redis)
	go test -tags=integration -v ./internal/api -run Integration

build: ## Build the application Docker image
	docker build --target runtime -t releases-api:latest .

build-test: ## Build the test Docker image
	docker build --target test -t releases-api-test:latest .

build-integration-test: ## Build the integration test Docker image
	docker build --target integration-tests -t releases-api-integration-tests:latest .

run: build ## Build and run the application
	docker compose up -d

run-dev: ## Run the application with docker compose (development)
	docker compose up --build

stop: ## Stop the application
	docker compose down

clean: ## Clean up Docker resources
	docker compose down -v
	docker compose -f docker-compose.integration.yml down -v

logs: ## Show application logs
	docker compose logs -f

logs-integration: ## Show integration test logs
	docker-compose -f docker-compose.integration.yml logs -f

db-shell: ## Connect to the database shell (requires app running)
	docker compose exec db psql -U postgres -d releases

db-shell-integration: ## Connect to the integration test database (requires integration-test-debug)
	docker-compose -f docker-compose.integration.yml exec db psql -U postgres -d releases

redis-shell: ## Connect to the Redis CLI (requires app running)
	docker compose exec redis redis-cli

redis-shell-integration: ## Connect to the integration test Redis (requires integration-test-debug)
	docker-compose -f docker-compose.integration.yml exec redis redis-cli

fmt: ## Format code
	go fmt ./...

lint: ## Run linter
	golangci-lint run ./...

.DEFAULT_GOAL := help
