.PHONY: help dev air build run test clean docker-stop

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

dev: ## Run with Air (auto-reload) on port 8001
	@echo "🚀 Starting Air on port 8001..."
	@~/go/bin/air

air: dev ## Alias for dev

build: ## Build the Go application
	@echo "🔨 Building application..."
	@go build -o server ./cmd/api

run: build ## Build and run the application
	@echo "▶️  Running application..."
	@./server

test: ## Run tests
	@echo "🧪 Running tests..."
	@go test -v ./...

test-cover: ## Run tests with coverage
	@echo "🧪 Running tests with coverage..."
	@go test -cover ./...

clean: ## Clean build artifacts
	@echo "🧹 Cleaning..."
	@rm -rf tmp/ server
	@echo "✅ Clean complete"

docker-stop: ## Stop Docker backend container (keep database running)
	@echo "🛑 Stopping Docker backend container..."
	@cd .. && docker compose stop backend
	@echo "✅ Docker backend stopped. Database still running."

docker-start: ## Start Docker backend container
	@echo "🚀 Starting Docker backend container..."
	@cd .. && docker compose start backend
	@echo "✅ Docker backend started"

docker-restart: ## Restart Docker backend container
	@echo "🔄 Restarting Docker backend container..."
	@cd .. && docker compose restart backend
	@echo "✅ Docker backend restarted"

fmt: ## Format Go code
	@echo "📝 Formatting code..."
	@go fmt ./...

lint: ## Run linter
	@echo "🔍 Running linter..."
	@golangci-lint run

deps: ## Download dependencies
	@echo "📦 Downloading dependencies..."
	@go mod download
	@go mod tidy

install-air: ## Install Air
	@echo "📦 Installing Air..."
	@go install github.com/air-verse/air@latest
	@echo "✅ Air installed"

# Development workflow shortcuts
dev-clean: docker-stop clean dev ## Stop Docker backend, clean, and start Air

.DEFAULT_GOAL := help
