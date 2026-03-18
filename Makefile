.PHONY: help build build-linux clean test test-verbose run install-deps generate swagger lint migrate-up migrate-status migrate-create

# Variables
APP_NAME=ars-kit
MAIN_PATH=src/main.go
BUILD_DIR=bin
LINUX_BINARY=$(BUILD_DIR)/$(APP_NAME)-linux-amd64
MAC_BINARY=$(BUILD_DIR)/$(APP_NAME)-darwin-amd64

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOGENERATE=$(GOCMD) generate

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

install-deps: ## Install Go dependencies
	$(GOMOD) download
	$(GOMOD) tidy

build: ## Build binary for current OS
	@echo "Building for current OS..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -ldflags="-s -w" -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_PATH)
	@echo "Binary created at $(BUILD_DIR)/$(APP_NAME)"
	@ls -lh $(BUILD_DIR)/$(APP_NAME)

build-linux: ## Build binary for Linux (Debian/Ubuntu)
	@echo "Building for Linux (amd64)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) -ldflags="-s -w" -o $(LINUX_BINARY) $(MAIN_PATH)
	@echo "Linux binary created at $(LINUX_BINARY)"
	@ls -lh $(LINUX_BINARY)

build-all: build build-linux ## Build binaries for all platforms

generate: ## Generate code (mocks, etc.)
	$(GOGENERATE) ./...

swagger: ## Generate Swagger documentation
	@echo "Generating Swagger docs..."
	@command -v swag >/dev/null 2>&1 || { echo "Installing swag..."; go install github.com/swaggo/swag/cmd/swag@latest; }
	swag init -g $(MAIN_PATH) -o docs
	@echo "Swagger docs generated in docs/"

test: ## Run tests
	$(GOTEST) ./... -count=1

test-verbose: ## Run tests with verbose output
	$(GOTEST) -v ./... -count=1

test-coverage: ## Run tests with coverage
	$(GOTEST) -v -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

run: ## Run the application
	$(GOCMD) run $(MAIN_PATH)

clean: ## Clean build artifacts
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

lint: ## Run golangci-lint
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not installed. Install from https://golangci-lint.run/usage/install/"; exit 1; }
	golangci-lint run ./...

fmt: ## Format code
	$(GOCMD) fmt ./...

vet: ## Run go vet
	$(GOCMD) vet ./...

migrate-up: ## Apply all pending database migrations
	go run ./cmd/migrate up

migrate-status: ## Show current database migration version
	go run ./cmd/migrate status

migrate-create: ## Create a new migration file: make migrate-create NAME=description
	@if [ -z "$(NAME)" ]; then echo "Usage: make migrate-create NAME=your_description"; exit 1; fi
	@TS=$$(date +%Y%m%d%H%M%S); \
	FILE="database/migrations/$${TS}_$(NAME).up.sql"; \
	touch "$$FILE"; \
	echo "Created: $$FILE"

.DEFAULT_GOAL := help
