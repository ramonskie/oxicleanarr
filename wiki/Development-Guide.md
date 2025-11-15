# Development Guide

Guide for building, testing, and contributing to OxiCleanarr.

**Related pages:**
- [Installation Guide](Installation-Guide.md) - Install for production use
- [Architecture](Architecture.md) - System architecture and design
- [API Reference](API-Reference.md) - REST API documentation

---

## Table of Contents

- [Prerequisites](#prerequisites)
- [Getting Started](#getting-started)
- [Project Structure](#project-structure)
- [Development Workflow](#development-workflow)
- [Testing](#testing)
- [Code Style](#code-style)
- [Building](#building)
- [Contributing](#contributing)
- [Debugging](#debugging)

---

## Prerequisites

### Required

- **Go 1.21+**: [Install Go](https://golang.org/doc/install)
- **Git**: Version control
- **Make**: Build automation (optional but recommended)

### Optional

- **Node.js 18+** & **npm**: For frontend development
- **Docker**: For containerized development
- **golangci-lint**: For linting (`brew install golangci-lint` or [install guide](https://golangci-lint.run/usage/install/))
- **gitleaks**: For secret scanning (`brew install gitleaks`)

### Verify Installation

```bash
go version      # Should be 1.21 or higher
git --version
make --version
node --version  # Optional: for frontend
```

---

## Getting Started

### 1. Clone the Repository

```bash
git clone https://github.com/ramonskie/oxicleanarr.git
cd oxicleanarr
```

### 2. Install Dependencies

```bash
# Go dependencies
go mod download
go mod tidy

# Or use make
make install
```

### 3. Setup Configuration

```bash
# Create necessary directories
mkdir -p config data

# Copy example configuration
cp config/config.yaml.example config/config.yaml

# Edit configuration with your settings
nano config/config.yaml
```

### 4. Run the Application

```bash
# Backend only
make dev

# Backend + Frontend with hot reload
make dev-full

# Or run directly
go run cmd/oxicleanarr/main.go
```

The API will be available at `http://localhost:8080`  
The frontend (if using `dev-full`) will be at `http://localhost:5173`

---

## Project Structure

```
oxicleanarr/
├── cmd/
│   └── oxicleanarr/
│       └── main.go              # Application entry point
├── internal/
│   ├── api/
│   │   ├── handlers/            # HTTP request handlers
│   │   │   ├── auth.go          # Authentication endpoints
│   │   │   ├── media.go         # Media management endpoints
│   │   │   ├── sync.go          # Sync operation endpoints
│   │   │   ├── jobs.go          # Job history endpoints
│   │   │   ├── config.go        # Configuration endpoints
│   │   │   ├── rules.go         # Advanced rules endpoints
│   │   │   └── health.go        # Health check endpoint
│   │   ├── middleware/
│   │   │   ├── auth.go          # JWT authentication
│   │   │   ├── logging.go       # Request logging
│   │   │   └── recovery.go      # Panic recovery
│   │   └── router.go            # Route definitions
│   ├── cache/
│   │   └── cache.go             # In-memory caching
│   ├── clients/
│   │   ├── jellyfin.go          # Jellyfin API client
│   │   ├── radarr.go            # Radarr API client
│   │   ├── sonarr.go            # Sonarr API client
│   │   ├── jellyseerr.go        # Jellyseerr API client
│   │   ├── jellystat.go         # Jellystat API client
│   │   └── types.go             # Shared client types
│   ├── config/
│   │   ├── config.go            # Configuration management
│   │   ├── types.go             # Configuration types
│   │   ├── validation.go        # Configuration validation
│   │   ├── defaults.go          # Default values
│   │   └── watcher.go           # Hot-reload watcher
│   ├── models/
│   │   └── media.go             # Core data models
│   ├── services/
│   │   ├── auth.go              # Authentication service
│   │   ├── sync.go              # Sync engine
│   │   ├── rules.go             # Rules engine
│   │   └── symlink_library.go   # Symlink library management
│   ├── storage/
│   │   ├── exclusions.go        # Exclusions persistence
│   │   └── jobs.go              # Job history persistence
│   └── utils/
│       ├── logger.go            # Logging utilities
│       └── jwt.go               # JWT utilities
├── web/                         # React/TypeScript frontend
│   ├── src/
│   │   ├── components/          # React components
│   │   ├── pages/               # Page components
│   │   ├── lib/                 # Utilities (API client, types)
│   │   ├── hooks/               # React hooks
│   │   └── store/               # State management
│   ├── public/                  # Static assets
│   └── package.json
├── test/
│   ├── integration/             # Integration tests
│   └── assets/                  # Test fixtures
├── config/
│   └── config.yaml.example      # Example configuration
├── data/                        # Runtime data (gitignored)
│   ├── exclusions.json
│   └── jobs.json
├── Makefile                     # Build automation
├── go.mod                       # Go dependencies
├── go.sum                       # Dependency checksums
├── Dockerfile                   # Container image
└── README.md
```

---

## Development Workflow

### Make Commands

OxiCleanarr uses a Makefile for common development tasks:

```bash
# Show all available commands
make help

# Build the application
make build

# Run backend in development mode (no binary, direct execution)
make dev

# Run backend + frontend with hot reload
make dev-full

# Run with test configuration
make dev-test

# Run tests
make test

# Format code
make fmt

# Lint code
make lint

# Clean build artifacts
make clean

# Install/update dependencies
make install

# Scan for secrets in codebase
make secrets-scan

# Setup git hooks for secret scanning
make setup-hooks

# First-time setup (creates dirs, config, hooks)
make setup
```

### Hot Reload Development

#### Backend Only

```bash
# Manual: run directly with Go
go run cmd/oxicleanarr/main.go

# Or use make
make dev
```

Changes to Go files require manually restarting the process.

#### Backend + Frontend

```bash
make dev-full
```

- **Backend:** `http://localhost:8080` (requires manual restart on changes)
- **Frontend:** `http://localhost:5173` (auto-reloads on changes)

#### Frontend Only

```bash
cd web
npm install
npm run dev
```

Ensure the backend is running separately on port 8080.

### Configuration Hot-Reload

The application supports hot-reloading of `config/config.yaml`:

1. Start the application: `make dev`
2. Edit `config/config.yaml`
3. Save the file
4. Changes are automatically applied (watch logs for confirmation)

**Hot-reloadable settings:**
- Retention rules (`movie_retention`, `tv_retention`)
- Advanced rules (`advanced_rules`)
- Sync intervals (`full_interval`, `incremental_interval`)
- Integration settings (URLs, API keys)
- Leaving soon threshold (`leaving_soon_days`)

**Not hot-reloadable (requires restart):**
- Server settings (`host`, `port`)
- Admin credentials (`username`, `password`)

---

## Testing

### Running Tests

```bash
# Run all tests
go test -v ./...

# Or use make
make test

# Run tests with coverage
go test -v -cover ./...

# Run specific test
go test -v ./internal/services -run TestSyncEngine

# Run integration tests only
go test -v ./test/integration/...
```

### Test Structure

- **Unit tests:** Located alongside source files (e.g., `sync_test.go` next to `sync.go`)
- **Integration tests:** Located in `test/integration/`
- **Test fixtures:** Located in `test/assets/`

### Writing Tests

#### Table-Driven Tests

```go
func TestMediaRetention(t *testing.T) {
    tests := []struct {
        name       string
        mediaType  models.MediaType
        addedAt    time.Time
        retention  string
        wantDelete bool
    }{
        {
            name:       "Movie within retention",
            mediaType:  models.MediaTypeMovie,
            addedAt:    time.Now().AddDate(0, 0, -30),
            retention:  "90d",
            wantDelete: false,
        },
        {
            name:       "Movie past retention",
            mediaType:  models.MediaTypeMovie,
            addedAt:    time.Now().AddDate(0, 0, -100),
            retention:  "90d",
            wantDelete: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

#### HTTP Handler Tests

```go
func TestAuthHandler_Login(t *testing.T) {
    handler := handlers.NewAuthHandler(authService)
    
    body := `{"username":"admin","password":"changeme"}`
    req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
    rec := httptest.NewRecorder()
    
    handler.Login(rec, req)
    
    assert.Equal(t, http.StatusOK, rec.Code)
    // Additional assertions...
}
```

#### Using t.Helper()

```go
func setupTestConfig(t *testing.T) *config.Config {
    t.Helper() // Marks this as a helper function
    
    cfg := &config.Config{
        // Setup test configuration
    }
    return cfg
}
```

---

## Code Style

### Go Conventions

#### Imports

Organize imports in three groups (separated by blank lines):

```go
import (
    // Standard library
    "context"
    "fmt"
    "time"

    // Third-party packages
    "github.com/go-chi/chi/v5"
    "github.com/rs/zerolog/log"

    // Internal packages
    "github.com/ramonskie/oxicleanarr/internal/config"
    "github.com/ramonskie/oxicleanarr/internal/models"
)
```

#### Naming

- **Unexported:** `camelCase` (e.g., `syncRadarr`, `mediaCache`)
- **Exported:** `PascalCase` (e.g., `SyncEngine`, `MediaType`)
- **Acronyms:** Capitalize all letters when exported (e.g., `APIKey`, `HTTPClient`, `URLPath`)
- **Interfaces:** Avoid `-er` suffix unless idiomatic (e.g., `io.Reader`, but not `SyncerEngine`)

#### Error Handling

```go
// Wrap errors with context
if err != nil {
    return fmt.Errorf("failed to sync radarr: %w", err)
}

// Log errors with structured fields
log.Error().
    Err(err).
    Str("service", "radarr").
    Msg("Sync failed")
```

#### Logging

Use `zerolog` with structured fields:

```go
// Info
log.Info().
    Str("media_id", id).
    Int("days_until_due", days).
    Msg("Media scheduled for deletion")

// Debug
log.Debug().
    Interface("config", cfg).
    Msg("Loaded configuration")

// Error
log.Error().
    Err(err).
    Str("path", path).
    Msg("Failed to create symlink")
```

**Log Levels:**
- `Debug`: Detailed diagnostic information
- `Info`: General informational messages
- `Warn`: Warning messages (recoverable issues)
- `Error`: Error messages (failed operations)
- `Fatal`: Fatal errors (application exits)

#### Concurrency

```go
// Use sync.RWMutex for shared state
type Cache struct {
    mu    sync.RWMutex
    items map[string]Item
}

func (c *Cache) Get(key string) (Item, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    item, ok := c.items[key]
    return item, ok
}

func (c *Cache) Set(key string, item Item) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.items[key] = item
}
```

#### Context

Always pass `context.Context` as the first parameter:

```go
func (s *SyncEngine) FullSync(ctx context.Context) error {
    // Use ctx for cancellation and timeouts
}
```

#### JSON Tags

Use snake_case for JSON field names:

```go
type Media struct {
    ID          string    `json:"id"`
    Title       string    `json:"title"`
    AddedAt     time.Time `json:"added_at"`
    DeleteAfter time.Time `json:"delete_after"`
}
```

### Formatting

```bash
# Format all code
go fmt ./...

# Or use make
make fmt

# Use goimports for import organization (optional)
goimports -w .
```

### Linting

```bash
# Run linter
golangci-lint run

# Or use make
make lint

# Auto-fix issues (where possible)
golangci-lint run --fix
```

---

## Building

### Development Build

```bash
# Build binary
go build -o oxicleanarr cmd/oxicleanarr/main.go

# Or use make
make build

# Run the binary
./oxicleanarr
```

### Production Build

```bash
# Build with optimizations
go build -ldflags="-s -w" -o oxicleanarr cmd/oxicleanarr/main.go

# Build for specific platform
GOOS=linux GOARCH=amd64 go build -o oxicleanarr-linux-amd64 cmd/oxicleanarr/main.go
```

### Docker Build

```bash
# Build Docker image
docker build -t oxicleanarr:dev .

# Run container
docker run -d \
  -p 8080:8080 \
  -v ./config:/app/config \
  -v ./data:/app/data \
  oxicleanarr:dev
```

### Frontend Build

```bash
cd web

# Development
npm run dev

# Production build
npm run build

# Preview production build
npm run preview
```

---

## Contributing

### Contribution Workflow

1. **Fork the repository** on GitHub
2. **Clone your fork** locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/oxicleanarr.git
   cd oxicleanarr
   ```
3. **Create a feature branch**:
   ```bash
   git checkout -b feature/my-new-feature
   ```
4. **Make your changes** following the code style guidelines
5. **Run tests**:
   ```bash
   make test
   make lint
   ```
6. **Commit your changes**:
   ```bash
   git add .
   git commit -m "Add my new feature"
   ```
7. **Push to your fork**:
   ```bash
   git push origin feature/my-new-feature
   ```
8. **Open a Pull Request** on GitHub

### Commit Messages

Use clear, descriptive commit messages:

```
Add user-based retention rules

- Implement UserRule type and validation
- Add user matching by ID, username, or email
- Update rules engine to support user-based rules
- Add integration tests for user rules
```

**Format:**
- First line: Brief summary (50 chars or less)
- Blank line
- Detailed description (optional, wrap at 72 chars)

### Pull Request Guidelines

- **One feature per PR**: Keep PRs focused and small
- **Update tests**: Add or update tests for your changes
- **Update documentation**: Update wiki pages if needed
- **Follow code style**: Run `make fmt` and `make lint`
- **Describe your changes**: Explain what, why, and how in the PR description

### Secret Scanning

Before committing, scan for accidentally committed secrets:

```bash
# Scan codebase
make secrets-scan

# Setup pre-commit hook (runs automatically)
make setup-hooks
```

---

## Debugging

### Logging

Enable debug logging:

```yaml
# config/config.yaml
app:
  log_level: debug  # Options: debug, info, warn, error
```

Or use environment variable:

```bash
export LOG_LEVEL=debug
make dev
```

### Breakpoint Debugging

#### Using Delve (Go debugger)

```bash
# Install Delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Start debugger
dlv debug cmd/oxicleanarr/main.go

# Or attach to running process
dlv attach $(pgrep oxicleanarr)
```

#### VSCode

Create `.vscode/launch.json`:

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Launch OxiCleanarr",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "${workspaceFolder}/cmd/oxicleanarr",
      "env": {},
      "args": []
    }
  ]
}
```

Press `F5` to start debugging.

### Testing API Endpoints

```bash
# Health check
curl http://localhost:8080/health

# Login
TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"changeme"}' | jq -r '.token')

# Test authenticated endpoint
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/media/movies | jq

# Trigger full sync
curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/sync/full
```

### Common Issues

#### Port Already in Use

```bash
# Find process using port 8080
lsof -i :8080

# Kill process
kill -9 <PID>
```

#### Configuration Not Reloading

1. Check file permissions: `ls -l config/config.yaml`
2. Check logs for watcher errors
3. Verify file path is correct (absolute vs relative)

#### Go Module Issues

```bash
# Clean module cache
go clean -modcache

# Re-download dependencies
go mod download
go mod tidy
```

---

## Additional Resources

### Documentation

- [Go Documentation](https://golang.org/doc/)
- [Go by Example](https://gobyexample.com/)
- [Effective Go](https://golang.org/doc/effective_go)

### Libraries Used

- **Router:** [chi](https://github.com/go-chi/chi) - Lightweight HTTP router
- **Logging:** [zerolog](https://github.com/rs/zerolog) - Structured logging
- **Config:** [viper](https://github.com/spf13/viper) - Configuration management
- **JWT:** [golang-jwt](https://github.com/golang-jwt/jwt) - JWT authentication
- **Testing:** [testify](https://github.com/stretchr/testify) - Testing toolkit

### Project Links

- **GitHub Repository:** https://github.com/ramonskie/oxicleanarr
- **Issues:** https://github.com/ramonskie/oxicleanarr/issues
- **Discussions:** https://github.com/ramonskie/oxicleanarr/discussions

---

## See Also

- [Installation Guide](Installation-Guide.md) - Production installation
- [Configuration](Configuration.md) - Configuration reference
- [Architecture](Architecture.md) - System design and architecture
- [API Reference](API-Reference.md) - REST API documentation
