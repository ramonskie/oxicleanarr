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
    OxiCleanarr accepts it on every protected endpoint. If `admin.api_key` is empty it
    is auto-generated on first start, logged, and persisted to the config file. The
    integration test config sets `admin.api_key` and the plugin installer writes it
    into the plugin `config.xml`.

## Deletion overlay (poster banner)

OxiCleanarr draws a "leaving in {days} days" banner onto scheduled-deletion media
(`overlay.*` config). Operational requirements and caveats:

- Upload contract: `POST /Items/{id}/Images/Primary` with a **base64** body and an
  explicit `Content-Type: image/jpeg`. Raw binary returns HTTP 500 (see
  jellyfin/jellyfin#12447). Auth uses `Authorization: MediaBrowser Token="<api_key>"`.
- Durability: the banner only survives a Jellyfin image **replace-all** refresh when the
  library keeps its artwork on disk. Enable **"Save artwork into media folders"** on the
  Movies and Shows libraries. With it off, the banner is a Jellyfin metadata image that is
  reverted to the on-disk `folder.jpg` by a scan (e.g. `TMDbBoxSets` processing duplicates).
- Duplicates: Jellyfin returns BOTH the main-library item AND the Leaving-Soon symlink copy
  for the same title (same TMDB/TVDB id). The matcher must resolve `mediaServerId` to the
  **original** main-library item: `isLeavingSoonCopy` excludes any path under the plugin
  BasePath (`/config/leaving-soon`) or containing a `leaving-soon` segment, with a
  deterministic tie-break. The LeavingSoon plugin then propagates the original's (overlaid)
  image file to the copy, so both views show the banner.
- Self-heal: every overlay pass fetches the server's current Primary and re-uploads when it
  differs from the rendered banner (no reliance on the day count alone). Force a full redraw
  with `POST /api/overlay/run?force=true`.
- State: items are tracked in `data/overlays/state.json`; originals are backed up (atomically)
  under `data/overlays/originals/`. A 404 on upload/revert is confirmed with `ItemExists`
  before state+backup are dropped, so a misconfigured base URL cannot destroy the only
  recovery artifact.