# Agent Guidelines for OxiCleanarr

## ⚠️ CRITICAL: GIT COMMIT POLICY - READ THIS FIRST

**NEVER create git commits without EXPLICIT user permission. NEVER.**

- Even if files are staged
- Even if a summary says "ready to commit"
- Even if a summary says "waiting for approval"
- **ALWAYS ASK FIRST**: "Should I commit these changes?" or "Ready for me to create a commit?"
- **NO EXCEPTIONS**: If uncertain, ask. If you think you should commit, ask. If a previous session said to commit, ask.

**Violation of this policy is a critical failure.** The user MUST have final control over all commits.

## Project Overview
OxiCleanarr is an orchestrator that manages media lifecycle through APIs. Its primary purpose is to automatically identify and handle media that should be cleaned up (e.g., watched content, expired content) by creating "leaving soon" libraries with symlinks, allowing users to take action before deletion.

It coordinates between:
- **Jellyfin** - Media server for playback and user interaction
- **Jellyseerr** - Media request and discovery platform
- **Jellystat** - Analytics and watch history tracking
- **Radarr/Sonarr** - Media acquisition and management
- **Jellyfin Plugin** - File system operations (symlink management)

All operations are API-driven. OxiCleanarr maintains no direct file system access except for configuration.

## Build & Test Commands
- **Build**: `make build` or `go build -o oxicleanarr cmd/oxicleanarr/main.go`
- **Run**: `make dev` (backend) or `make dev-full` (backend + frontend hot-reload)
- **Test**: `go test -v ./...` or `make test`
- **Single test**: `go test -v ./path/to/package -run TestName`
- **Format**: `go fmt ./...` or `make fmt`
- **Lint**: `golangci-lint run` or `make lint`
- **Frontend**: `cd web && npm run dev` (dev), `npm run build` (production)

## Project Structure
- `cmd/oxicleanarr/main.go` - Application entry point
- `internal/` - Go packages (api, clients, config, models, services, storage, utils)
- `web/` - React/TypeScript frontend with Vite
- `config/config.yaml` - YAML configuration (supports hot-reload)
- `data/` - Runtime data (exclusions.json, jobs.json)

## Code Style
- **Imports**: Standard library first, then third-party, then internal (grouped with blank lines)
- **Naming**: camelCase for unexported, PascalCase for exported; descriptive names (e.g., `syncRadarr`, `JellyfinClient`)
- **Types**: Explicit types; use `context.Context` for API calls; pointer receivers for methods modifying state
- **Error handling**: Return errors with `fmt.Errorf("context: %w", err)` for wrapping; log with zerolog at appropriate level
- **Logging**: Use `github.com/rs/zerolog/log` with structured fields (e.g., `log.Info().Str("job_id", id).Msg("...")`)
- **Testing**: Table-driven tests with `t.Helper()` for setup functions; use `httptest` for handlers; test concurrent access
- **Concurrency**: Use `sync.RWMutex` for shared state; always defer unlock after lock
- **JSON**: Use struct tags (e.g., `json:"field_name"`) for API types

## Dependencies
- **Jellyfin Plugin**: Requires [jellyfin-plugin-oxicleanarr](https://github.com/ramonskie/jellyfin-plugin-oxicleanarr) installed in Jellyfin
  - Plugin provides file system operations on the Jellyfin server
  - API endpoints (relative to Jellyfin base URL):
    - `GET /api/oxicleanarr/status` - Check plugin status (no auth required)
    - `POST /api/oxicleanarr/symlinks/add` - Create symlinks (body: `{items: [{sourcePath, targetDirectory}]}`)
    - `POST /api/oxicleanarr/symlinks/remove` - Remove symlinks (body: `{symlinkPaths: []}`)
    - `GET /api/oxicleanarr/symlinks/list?directory=/path` - List symlinks in specified directory
    - `POST /api/oxicleanarr/directories/create` - Create directory (body: `{directory}`)
    - `DELETE /api/oxicleanarr/directories/remove` - Delete directory (body: `{directory, force}`)
  - All requests except `/status` require Jellyfin API key header: `X-Emby-Token`
  - Plugin is stateless - all paths are provided via API requests