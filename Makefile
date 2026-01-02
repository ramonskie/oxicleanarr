.PHONY: build run clean test test-integration test-integration-up test-integration-down test-all dev dev-full dev-test install help secrets-scan setup-hooks

# Binary name
BINARY_NAME=oxicleanarr
CONFIG_DIR=./config
DATA_DIR=./data

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	@go build -o $(BINARY_NAME) cmd/oxicleanarr/main.go
	@echo "Build complete: ./$(BINARY_NAME)"

# Run the application
run: build
	@echo "Starting $(BINARY_NAME)..."
	@./$(BINARY_NAME)

# Run in development mode (without building binary)
dev:
	@echo "Running in development mode..."
	@go run cmd/oxicleanarr/main.go

# Run backend + frontend in development mode with hot reload
dev-full:
	@echo "Starting backend + frontend with hot reload..."
	@echo "Backend: http://localhost:8080"
	@echo "Frontend: http://localhost:5173"
	@echo ""
	@trap 'kill 0' SIGINT; \
	(cd web && npm run dev) & \
	go run cmd/oxicleanarr/main.go

# Run with test config (backend + frontend)
dev-test:
	@echo "Starting backend (test config) + frontend with hot reload..."
	@echo "Backend: http://localhost:8080"
	@echo "Frontend: http://localhost:5173"
	@echo "Config: config/config.test.yaml"
	@echo ""
	@trap 'kill 0' SIGINT; \
	(cd web && npm run dev) & \
	go run cmd/oxicleanarr/main.go --config config/config.test.yaml

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME)
	@rm -f *.log *.pid
	@echo "Clean complete"

# Run tests
test:
	@echo "Running unit tests..."
	@go test -v ./internal/...

# Start integration test environment (containers + infrastructure setup)
test-integration-up:
	@echo "Building Docker image and starting integration test environment..."
	@KEEP_TEST_ENV=1 go test -v ./test/integration/... -run TestIntegrationSuite/InfrastructureSetup -timeout 10m
	@echo ""
	@echo "✅ Integration test environment is ready"
	@echo "   Jellyfin: http://localhost:8096"
	@echo "   Radarr:   http://localhost:7878"
	@echo "   OxiCleanarr: http://localhost:9709"
	@echo ""
	@echo "Run 'make test-integration' to run tests"
	@echo "Run 'make test-integration-down' to stop"

# Stop integration test containers
test-integration-down:
	@echo "Stopping integration test containers..."
	@cd test/assets && docker-compose down -v --remove-orphans
	@echo "Cleanup complete"

# Run integration tests (requires environment to be running via test-integration-up)
test-integration:
	@echo "Running integration tests..."
	@go test -v ./test/integration/... -run TestIntegrationSuite -timeout 10m

# Install dependencies
install:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Lint code
lint:
	@echo "Linting code..."
	@golangci-lint run || true

# Create necessary directories
dirs:
	@mkdir -p $(CONFIG_DIR) $(DATA_DIR)

# Scan for secrets in codebase
secrets-scan:
	@echo "Scanning for secrets..."
	@if command -v gitleaks >/dev/null 2>&1; then \
		gitleaks detect --redact --verbose; \
	else \
		echo "⚠️  gitleaks not installed. Install with: brew install gitleaks"; \
		exit 1; \
	fi

# Setup git hooks for secret scanning
setup-hooks:
	@echo "Setting up git hooks..."
	@if [ -f .git/hooks/pre-commit ]; then \
		chmod +x .git/hooks/pre-commit; \
		echo "✅ Pre-commit hook is active"; \
	else \
		echo "❌ Pre-commit hook not found"; \
		exit 1; \
	fi
	@if ! command -v gitleaks >/dev/null 2>&1; then \
		echo "⚠️  Warning: gitleaks not installed"; \
		echo "Install with: brew install gitleaks"; \
	else \
		echo "✅ gitleaks is installed"; \
	fi

# Setup for first run
setup: dirs setup-hooks
	@if [ ! -f $(CONFIG_DIR)/config.yaml ]; then \
		echo "Creating config from example..."; \
		cp $(CONFIG_DIR)/config.yaml.example $(CONFIG_DIR)/config.yaml; \
	else \
		echo "Config already exists"; \
	fi

# Show help
help:
	@echo "OxiCleanarr - Media Cleanup Automation Tool"
	@echo ""
	@echo "Usage:"
	@echo "  make build                - Build the application"
	@echo "  make run                  - Build and run the application"
	@echo "  make dev                  - Run backend in development mode (no binary)"
	@echo "  make dev-full             - Run backend + frontend with hot reload"
	@echo "  make dev-test             - Run with test config + frontend (hot reload)"
	@echo "  make test                 - Run unit tests"
	@echo "  make test-integration     - Run integration tests (containers must be running)"
	@echo "  make test-integration-up  - Start containers + run infrastructure setup"
	@echo "  make test-integration-down - Stop integration test containers"
	@echo "  make test-all             - Run unit + integration tests (full cycle)"
	@echo "  make clean                - Remove build artifacts"
	@echo "  make install              - Install/update dependencies"
	@echo "  make fmt                  - Format code"
	@echo "  make lint                 - Lint code"
	@echo "  make secrets-scan         - Scan codebase for secrets/API keys"
	@echo "  make setup-hooks          - Setup git hooks for secret scanning"
	@echo "  make setup                - Setup config for first run"
	@echo "  make help                 - Show this help message"
