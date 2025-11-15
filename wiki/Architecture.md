# Architecture

OxiCleanarr is built as a lightweight, API-first orchestrator that coordinates between multiple media services. This document explains the system design, data flow, and component interactions.

## System Overview

```
┌──────────────────────────────────────────────────────────────┐
│                      OxiCleanarr                             │
│                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │ REST API     │  │ Sync Engine  │  │ Rules Engine │      │
│  │ (Chi Router) │  │              │  │              │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
│         │                 │                  │              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │ React SPA    │  │ go-cache     │  │ Config       │      │
│  │ (Web UI)     │  │ (In-Memory)  │  │ (Hot-Reload) │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
└──────────────────────────────────────────────────────────────┘
         │                 │                  │
         ▼                 ▼                  ▼
┌─────────────┐  ┌──────────────┐  ┌──────────────┐
│  Jellyfin   │  │   Radarr     │  │   Sonarr     │
│  + Plugin   │  │              │  │              │
└─────────────┘  └──────────────┘  └──────────────┘
         │                 │                  │
         ▼                 ▼                  ▼
┌─────────────┐  ┌──────────────┐  ┌──────────────┐
│ Jellyseerr  │  │  Jellystat   │  │ Media Files  │
│ (Optional)  │  │  (Optional)  │  │ (Read-Only)  │
└─────────────┘  └──────────────┘  └──────────────┘
```

## Core Components

### 1. REST API Layer

**Technology:** Go Chi v5 router

**Responsibilities:**
- HTTP request routing and handling
- JWT authentication
- CORS middleware
- Request logging and recovery
- API endpoint implementation

**Key Files:**
- `internal/api/router.go` - Route definitions
- `internal/api/handlers/` - Request handlers
- `internal/api/middleware/` - Auth, logging, recovery

**Endpoints:**
```
Public:
  GET /health                   - Health check

Auth:
  POST /api/auth/login          - JWT authentication

Media:
  GET  /api/media/movies        - List movies
  GET  /api/media/shows         - List TV shows
  GET  /api/media/leaving-soon  - Items in deletion window
  POST /api/media/{id}/exclude  - Add exclusion

Sync:
  POST /api/sync/full           - Trigger full sync
  POST /api/sync/incremental    - Trigger quick sync
  GET  /api/sync/status         - Sync engine status

Jobs:
  GET /api/jobs                 - Job history
  GET /api/jobs/latest          - Latest job details
```

### 2. Sync Engine

**Technology:** Go concurrency (goroutines + channels)

**Responsibilities:**
- Periodic synchronization with external services
- Media library aggregation
- Watch history tracking
- Deletion candidate identification
- Symlink library management

**Key Files:**
- `internal/services/sync.go` - Sync orchestration
- `internal/services/symlink_library.go` - Symlink management

**Data Flow:**
```
Full Sync (Every 1 hour):
  1. Fetch all movies from Radarr
  2. Fetch all TV shows from Sonarr
  3. Fetch library metadata from Jellyfin
  4. Fetch watch history from Jellyfin
  5. Fetch requests from Jellyseerr (optional)
  6. Fetch watch stats from Jellystat (optional)
  7. Aggregate into unified library
  8. Apply exclusions
  9. Apply retention rules
  10. Calculate leaving soon window
  11. Update symlink libraries
  12. Store job history

Incremental Sync (Every 15 minutes):
  1. Fetch recent watch history only
  2. Update existing library items
  3. Recalculate deletion timeline
  4. Update symlink libraries if needed
```

### 3. Rules Engine

**Technology:** Go rule evaluation system

**Responsibilities:**
- Retention policy evaluation
- Tag-based rule matching
- User-based rule matching
- Watched-based rule evaluation
- Deletion reason generation

**Key Files:**
- `internal/services/rules.go` - Rule evaluation logic

**Rule Priority:**
```
1. Tag-based rules (highest)
   ↓
2. User-based rules
   ↓
3. Watched-based rules
   ↓
4. Default retention (lowest)
```

**Evaluation Algorithm:**
```go
for each media item:
  // Check tag rules
  if item.Tags contains matching tag rule:
    apply tag rule retention
    continue
  
  // Check user rules
  if item.RequestedBy matches user rule:
    apply user retention
    if require_watched && !watched:
      skip deletion
    continue
  
  // Check watched rules
  if watched rule enabled && item.LastWatched exists:
    if time.Since(LastWatched) > watched retention:
      schedule deletion
    continue
  
  // Default retention
  if time.Since(AddedDate) > default retention:
    schedule deletion
```

### 4. Integration Clients

**Technology:** Go HTTP clients with caching

**Services:**

#### Jellyfin Client
```
Purpose: Media server integration
Endpoints Used:
  - /Users/{userId}/Items - Library items
  - /System/Info/Public - Server info
  - /api/oxicleanarr/* - Bridge plugin (symlinks)
Data Retrieved:
  - Library items with metadata
  - Watch history (play counts, last played)
  - User information
```

#### Radarr Client
```
Purpose: Movie management
Endpoints Used:
  - /api/v3/movie - Movie list
  - /api/v3/movie/{id} - Movie details
  - /api/v3/movie/{id} DELETE - Delete movie
Data Retrieved:
  - Movie files and paths
  - Tags and quality profiles
  - Import dates
```

#### Sonarr Client
```
Purpose: TV show management
Endpoints Used:
  - /api/v3/series - Series list
  - /api/v3/episode - Episode details
  - /api/v3/series/{id} DELETE - Delete series
Data Retrieved:
  - Series and episode data
  - Tags and quality profiles
  - Import dates
```

#### Jellyseerr Client (Optional)
```
Purpose: Request tracking
Endpoints Used:
  - /api/v1/request - Request list
  - /api/v1/request/{id} - Request details
Data Retrieved:
  - Who requested each media item
  - Request timestamps
  - User information
```

#### Jellystat Client (Optional)
```
Purpose: Watch analytics
Endpoints Used:
  - /api/stats - Watch statistics
  - /api/history - Watch history
Data Retrieved:
  - Detailed watch history per user
  - Last watched timestamps
  - Play counts
```

### 5. Cache Layer

**Technology:** go-cache (in-memory, thread-safe)

**Purpose:** Reduce API calls and improve performance

**Cache Keys:**
```
jellyfin:library:{id}       - 1 hour TTL
radarr:movies               - 30 minutes TTL
sonarr:shows                - 30 minutes TTL
jellyseerr:requests         - 15 minutes TTL
jellystat:watch:{id}        - 5 minutes TTL
rule:eval:{id}              - Until next sync
timeline:deletion           - 5 minutes TTL
```

**Invalidation Strategy:**
```
Event: Manual sync triggered
  → Clear ALL caches

Event: Rule modified
  → Clear rule:eval:* and timeline:*

Event: Exclusion added/removed
  → Clear item cache + rule eval + timeline

Event: Config changed (hot-reload)
  → Clear ALL caches
```

### 6. Storage Layer

**Technology:** File-based JSON storage

**Files:**

#### config/config.yaml
```
Purpose: Application configuration
Format: YAML
Features:
  - Hot-reload (fsnotify)
  - Validation on load
  - Environment variable overrides
```

#### data/exclusions.json
```
Purpose: User exclusions (Keep button)
Format: JSON
Structure:
  {
    "version": "1.0",
    "items": {
      "media-id": {
        "external_id": "tt1234567",
        "title": "Movie Title",
        "excluded_at": "2024-11-15T10:30:00Z",
        "excluded_by": "admin"
      }
    }
  }
```

#### data/jobs.json
```
Purpose: Job execution history
Format: JSON
Retention: Last 100 jobs (circular buffer)
Structure:
  {
    "version": "1.0",
    "jobs": [
      {
        "id": "uuid",
        "type": "full_sync",
        "status": "completed",
        "started_at": "...",
        "completed_at": "...",
        "summary": {...}
      }
    ]
  }
```

### 7. Web UI

**Technology:** React 19 + Vite + shadcn/ui

**Features:**
- Dashboard with statistics
- Deletion timeline view
- Library browser
- Scheduled deletions page
- Job history
- Real-time countdown timers

**State Management:**
- TanStack Query (server state + caching)
- Zustand (global UI state)

**Build Process:**
```
Development:
  npm run dev (Vite dev server on :5173)

Production:
  npm run build → dist/
  Embedded in Go binary
  Served at /*
```

## Data Flow

### Full Sync Flow

```
1. Trigger:
   - Scheduled (every 1 hour)
   - Manual (API call)
   - Startup (auto_start: true)

2. Radarr Sync:
   GET /api/v3/movie
   → Parse movie data
   → Extract file paths, tags, import dates
   → Store in library map

3. Sonarr Sync:
   GET /api/v3/series
   GET /api/v3/episode
   → Parse TV show data
   → Extract file paths, tags, import dates
   → Store in library map

4. Jellyfin Sync:
   GET /Users/{userId}/Items
   → Match items with library (by IMDb/TVDB ID)
   → Update watch counts
   → Update last watched timestamps

5. Jellyseerr Sync (Optional):
   GET /api/v1/request
   → Match requests to media items
   → Populate requested_by_user_id
   → Store requester info

6. Jellystat Sync (Optional):
   GET /api/stats
   → Get detailed watch history
   → Populate watched_by_users
   → Update last_watched timestamps

7. Apply Exclusions:
   → Load exclusions.json
   → Mark excluded items

8. Apply Retention Rules:
   → Evaluate each item against rules
   → Calculate delete_after date
   → Calculate days_until_due
   → Generate deletion_reason

9. Update Symlink Libraries:
   → Identify items in leaving soon window
   → Create/update symlinks via Jellyfin plugin
   → Trigger Jellyfin library scan

10. Store Job History:
    → Create job record
    → Save to jobs.json
    → Trim to last 100 jobs
```

### Incremental Sync Flow

```
1. Trigger:
   - Scheduled (every 15 minutes)
   - Manual (API call)

2. Fetch Recent Watch History:
   GET /Users/{userId}/Items?ChangedSince=lastSync
   → Update watch counts for changed items

3. Recalculate Timeline:
   → Re-evaluate rules for updated items
   → Adjust delete_after dates if watch status changed

4. Update Symlinks:
   → If items entered/exited leaving soon window
   → Create/remove symlinks accordingly

5. Store Job History:
   → Create incremental_sync job record
```

### Deletion Flow

```
1. Trigger:
   - Manual (API call: DELETE /api/media/{id})
   - Automatic (when dry_run: false)

2. Pre-Check:
   → Verify item not excluded
   → Verify delete_after date passed
   → Check dry_run setting

3. Delete from Radarr/Sonarr:
   DELETE /api/v3/movie/{id}?deleteFiles=true
   → Removes file from disk
   → Updates Radarr/Sonarr database

4. Remove from Jellyfin:
   → Jellyfin auto-detects file missing
   → Or trigger manual library scan

5. Remove Symlink:
   → Delete symlink via Jellyfin plugin
   → Trigger library refresh

6. Log Deletion:
   → Record in job history
   → Include reason, size, date
```

### Exclusion Flow

```
1. User Clicks "Keep" Button:
   POST /api/media/{id}/exclude
   
2. Add to Exclusions:
   → Load exclusions.json
   → Add item with metadata
   → Save exclusions.json

3. Update Library:
   → Set item.IsExcluded = true
   → Remove from deletion timeline
   → Remove symlink (if exists)

4. Persist Through Syncs:
   → applyExclusions() runs after each sync
   → Re-marks excluded items
   → Ensures exclusions survive restarts
```

## Performance Characteristics

### Memory Usage

```
Idle: 30-40 MB
Sync (1,000 items): 45-50 MB
Sync (10,000 items): 55-60 MB
Peak: ~80 MB
```

### API Response Times

```
Cached endpoints: <50ms
Uncached endpoints: 100-200ms
Full sync (1,000 items): ~20 seconds
Incremental sync: ~2 seconds
```

### Scaling Limits

```
Media Items: Tested up to 10,000
Concurrent Users: 50+ (stateless API)
Sync Interval: Minimum 5 minutes (recommended 15)
Cache Size: ~10 MB for 10,000 items
```

## Concurrency Model

### Goroutines

```go
// Sync engine uses goroutines for parallelism
go syncRadarr()     // Concurrent
go syncSonarr()     // Concurrent
go syncJellyfin()   // Concurrent

// Wait for all to complete
wg.Wait()

// Sequential operations after sync
applyExclusions()
applyRetentionRules()
updateSymlinkLibraries()
```

### Thread Safety

```go
// Library map protected by RWMutex
sync.RWMutex library access

// File storage protected by mutex
sync.Mutex for exclusions.json writes

// Cache is thread-safe (go-cache)
No additional locking needed
```

## Configuration Hot-Reload

### Implementation

```go
// fsnotify watches config.yaml
watcher.Add("config/config.yaml")

// On file change:
1. Reload config from disk
2. Validate new config
3. Clear affected caches
4. Re-initialize affected services
5. Log reload event
```

### What Reloads

```
✅ Retention periods
✅ Advanced rules
✅ Sync intervals
✅ Integration URLs/API keys
✅ leaving_soon_days

❌ Server port (requires restart)
❌ Log level (requires restart)
```

## Security Model

### Authentication

```
JWT Token:
  - Generated on login
  - Signed with secret (32+ chars)
  - 24-hour expiration (configurable)
  - Required for all /api/* endpoints (except login)
```

### Authorization

```
Single Admin User:
  - Username/password in config.yaml
  - Password stored in plain text (file permissions 600)
  - No multi-user support (by design)
```

### File Permissions

```
config.yaml: 600 (read-write, owner only)
data/: 755 (read-execute, group/others)
exclusions.json: 644 (read-write owner, read others)
```

### API Security

```
- CORS enabled for web UI
- No path traversal (all paths validated)
- API keys never logged
- Read-only media mount (container)
```

## Deployment Architecture

### Single Container (Recommended)

```
┌─────────────────────────────────────┐
│  OxiCleanarr Container              │
│  ┌──────────────┐                   │
│  │ Go Binary    │                   │
│  │ + Embedded   │                   │
│  │   React SPA  │                   │
│  └──────────────┘                   │
│         :8080                        │
└─────────────────────────────────────┘
          ↓
     Host Network
          ↓
┌─────────────────────────────────────┐
│  Other Containers                   │
│  - jellyfin:8096                    │
│  - radarr:7878                      │
│  - sonarr:8989                      │
└─────────────────────────────────────┘
```

### Reverse Proxy (Optional)

```
┌──────────────────┐
│  Nginx/Caddy     │
│  :443 (HTTPS)    │
└──────────────────┘
         ↓
┌──────────────────┐
│  OxiCleanarr     │
│  :8080 (HTTP)    │
└──────────────────┘
```

## Error Handling

### Graceful Degradation

```
Jellyfin offline:
  → Sync continues with Radarr/Sonarr only
  → Watch data not updated
  → Symlinks not updated

Radarr offline:
  → Movie sync skipped
  → TV shows still sync
  → Existing library data used

Sonarr offline:
  → TV show sync skipped
  → Movies still sync
  → Existing library data used
```

### Retry Logic

```
HTTP Clients:
  - 3 retries with exponential backoff
  - 30-second timeout per request
  - Log failures but don't block sync
```

### Recovery

```
Sync Failure:
  - Log error
  - Keep previous library state
  - Retry on next scheduled sync

Cache Corruption:
  - Auto-clear cache
  - Rebuild on next sync

File Corruption:
  - Log error
  - Attempt to recover (JSON parsing)
  - Fallback to empty state if unrecoverable
```

## Monitoring and Observability

### Logs

```
Format: JSON (structured)
Fields:
  - level (debug/info/warn/error)
  - timestamp
  - message
  - context (job_id, media_id, etc.)
```

### Metrics

```
Available via /api/sync/status:
  - media_count
  - movies_count
  - tv_shows_count
  - excluded_count
  - last_full_sync
  - last_incr_sync
```

### Health Check

```
GET /health
Response:
  {
    "status": "ok",
    "uptime": "5h3m",
    "version": "1.0.0"
  }
```

## Related Pages

- [API Reference](API-Reference.md) - Complete API documentation
- [Configuration](Configuration.md) - Config file reference
- [Development Guide](Development-Guide.md) - Build and contribute
- [Docker Deployment](Docker-Deployment.md) - Container setup
