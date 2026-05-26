.DEFAULT_GOAL := help

.PHONY: help up down logs psql run build tidy fmt vet test clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

up: ## Start Postgres in Docker
	docker compose up -d

down: ## Stop Postgres (keeps data volume)
	docker compose down

reset: ## Stop Postgres and DELETE its data volume
	docker compose down -v

logs: ## Tail Postgres logs
	docker compose logs -f postgres_db

psql: ## Open a psql shell inside the container
	docker compose exec postgres_db psql -U $${DB_USER:-taskos} -d $${DB_NAME:-taskos_db}

run: ## Run the server (loads .env)
	go run .

build: ## Build the server binary into ./bin
	go build -o bin/taskos-server .

tidy: ## Tidy module dependencies
	go mod tidy

fmt: ## Format code
	go fmt ./...

vet: ## Run go vet
	go vet ./...

test: ## Run tests
	go test ./...

clean: ## Remove build artifacts
	rm -rf bin
