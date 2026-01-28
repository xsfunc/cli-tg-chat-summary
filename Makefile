# Project variables
APP_NAME := tg-summary
# Get short commit hash for versioning
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "undefined")

# Default goal (runs when calling 'make' without arguments)
.DEFAULT_GOAL := help

# ==============================================================================
# Main commands
# ==============================================================================

.PHONY: all
all: tidy lint test build ## Full cycle: deps, lint, test, build

.PHONY: ci
ci: clean all ## CI pipeline: clean build from scratch

.PHONY: build
build: ## Compile binary to bin/ directory
	@echo "Building $(APP_NAME) (commit: $(COMMIT))..."
	go build -ldflags "-X main.version=$(COMMIT)" -o bin/$(APP_NAME) ./cmd/tg-summary

.PHONY: install
install: ## Install to $GOPATH/bin
	go install -ldflags "-X main.version=$(COMMIT)" ./cmd/tg-summary

.PHONY: run
run: ## Run application via go run (example: make run ARGS="-since 24h")
	go run ./cmd/tg-summary $(ARGS)

.PHONY: exec
exec: build ## Build and run binary (example: make exec ARGS="-since 12h")
	./bin/$(APP_NAME) $(ARGS)

.PHONY: clean
clean: ## Clean build artifacts
	rm -rf bin/ coverage.out coverage.html

# ==============================================================================
# Checks and Tests
# ==============================================================================

.PHONY: tidy
tidy: ## Update dependencies (go mod tidy)
	go mod tidy
	go mod verify

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run

.PHONY: test
test: ## Run unit tests
	go test ./... -v

.PHONY: test-nocache
test-nocache: ## Run tests without cache
	go test ./... -v -count=1

.PHONY: test-cover
test-cover: ## Run tests with coverage report
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# ==============================================================================
# Tools
# ==============================================================================

.PHONY: setup-hooks
setup-hooks: ## Install git hooks
	@echo "Setting up git hooks..."
	@test -f .git/hooks/pre-commit && chmod +x .git/hooks/pre-commit || true
	@test -f .git/hooks/pre-push && chmod +x .git/hooks/pre-push || true
	@echo "✅ Git hooks configured"

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'
