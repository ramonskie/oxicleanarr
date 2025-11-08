.PHONY: build run clean test dev dev-full dev-test install help

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
	@echo "Config: config/oxicleanarr.test.yaml"
	@echo ""
	@trap 'kill 0' SIGINT; \
	(cd web && npm run dev) & \
	go run cmd/oxicleanarr/main.go --config config/oxicleanarr.test.yaml

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME)
	@rm -f *.log *.pid
	@echo "Clean complete"

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

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

# Setup for first run
setup: dirs
	@if [ ! -f $(CONFIG_DIR)/oxicleanarr.yaml ]; then \
		echo "Creating config from example..."; \
		cp $(CONFIG_DIR)/oxicleanarr.yaml.example $(CONFIG_DIR)/oxicleanarr.yaml; \
	else \
		echo "Config already exists"; \
	fi

# Show help
help:
	@echo "OxiCleanarr - Media Cleanup Automation Tool"
	@echo ""
	@echo "Usage:"
	@echo "  make build     - Build the application"
	@echo "  make run       - Build and run the application"
	@echo "  make dev       - Run backend in development mode (no binary)"
	@echo "  make dev-full  - Run backend + frontend with hot reload"
	@echo "  make dev-test  - Run with test config + frontend (hot reload)"
	@echo "  make test      - Run tests"
	@echo "  make clean     - Remove build artifacts"
	@echo "  make install   - Install/update dependencies"
	@echo "  make fmt       - Format code"
	@echo "  make lint      - Lint code"
	@echo "  make setup     - Setup config for first run"
	@echo "  make help      - Show this help message"
