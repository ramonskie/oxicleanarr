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
OxiCleanarr is an orchestrator that manages media lifecycle through APIs. It automatically
identifies media that should be cleaned up (e.g., watched content, expired content) and
schedules it for deletion. Scheduled-deletion media is surfaced to the standalone
[jellyfin-plugin-leaving-soon](https://github.com/ramonskie/jellyfin-plugin-leaving-soon)
plugin, which polls OxiCleanarr's `GET /api/media/leaving-soon` endpoint and manages the
"leaving soon" symlink libraries in Jellyfin.

It coordinates between:
- **Jellyfin** - Media server for playback and user interaction
- **Jellyseerr** - Media request and discovery platform
- **Jellystat** - Analytics and watch history tracking
- **Radarr/Sonarr** - Media acquisition and management

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
- **Jellyfin** - Watch data, poster proxying, and post-deletion library refreshes.
- **Leaving Soon plugin** (optional): The standalone
  [jellyfin-plugin-leaving-soon](https://github.com/ramonskie/jellyfin-plugin-leaving-soon)
  plugin manages all "leaving soon" symlink libraries inside Jellyfin. It polls provider
  apps for scheduled-deletion media and reconciles the symlink libraries itself.
  - Provider contract (shared by every provider, e.g. OxiCleanarr, Maintainerr):
    `GET /api/media/leaving-soon` returns `{version: 1, items: [{mediaServerId, type,
    title?, deletionDate?, sourcePath?}]}` where `type` is `movie`/`show` and
    `mediaServerId` is the Jellyfin item GUID.
  - The plugin authenticates with the static `admin.api_key` sent as a Bearer token;
    OxiCleanarr accepts it on every protected endpoint. The integration test config sets
    `admin.api_key` and the plugin installer writes it into the plugin `config.xml`.