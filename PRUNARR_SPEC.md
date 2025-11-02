# Prunarr - Complete Project Specification

## Executive Summary

**Prunarr** is a lightweight media cleanup automation tool for the *arr stack (Sonarr, Radarr, Jellyfin). Built with Go + React, it provides intelligent retention policies, deletion visibility, and a modern web UI.

**Key Features**:
- 🪶 **Lightweight**: 15MB Docker image, <40MB RAM usage
- ⚡ **Fast**: <50ms startup, <100ms API responses
- 🎯 **Simple Config**: Sensible defaults, minimal YAML
- 👀 **Deletion Visibility**: Timeline view, countdown timers, "Keep" button
- 🔄 **Hot-Reload**: Live config changes without restart
- 🎨 **Modern UI**: React 19 + shadcn/ui

**Performance Targets**:
- Docker image: **15MB** (single all-in-one)
- Memory: **<40MB** idle, **<60MB** during sync
- Startup: **<50ms**
- Support: **10,000+ media items**

---

## 1. Technology Stack

### Backend
```
Language:  Go 1.23+
Router:    Chi v5 (lightweight, standard net/http compatible)
Config:    Viper (YAML/ENV support with hot-reload)
Storage:   File-based (YAML + JSON)
Cache:     go-cache (in-memory, thread-safe)
Auth:      JWT (single admin only)
Logger:    zerolog (structured JSON logging)
```

### Frontend
```
Framework: React 19
Build:     Vite 6
UI:        shadcn/ui (Tailwind CSS)
State:     Zustand (global state)
Data:      TanStack Query v5 (server state + caching)
Router:    React Router v7
```

### DevOps
```
Build:     Multi-stage Dockerfile (Alpine-based)
Deploy:    Single binary + embedded frontend
CI/CD:     GitHub Actions
Logging:   Structured JSON logs
```

---

## 2. Configuration Philosophy

### Core Principles
1. **Zero-config startup** - Works with just integration credentials
2. **Sensible defaults** - Everything preconfigured for typical use
3. **Progressive disclosure** - Simple → Advanced as needed
4. **Override only what you need** - Minimal YAML required

### 2.1 Minimal Configuration

**Bare minimum `prunarr.yaml`:**
```yaml
# Minimal viable config
admin:
  username: admin
  password: changeme  # Auto-hashed on first run

integrations:
  jellyfin:
    enabled: true
    url: http://jellyfin:8096
    api_key: your-key-here
  
  radarr:
    enabled: true
    url: http://radarr:7878
    api_key: your-key-here
  
  sonarr:
    enabled: true
    url: http://sonarr:8989
    api_key: your-key-here
  
  jellyseerr:
    enabled: false
    url: http://jellyseerr:5055
    api_key: ""
  
  jellystat:
    enabled: false
    url: http://jellystat:3000
    api_key: ""

# That's it! Everything below is optional with defaults
```

### 2.2 Full Configuration (with overrides)

```yaml
admin:
  username: admin
  password: changeme  # Plain-text auto-hashed to bcrypt on first load

# Optional app settings (all have defaults)
app:
  dry_run: true               # Default: true (safe by default)
  leaving_soon_days: 14       # Default: 14
  
# Optional sync settings (defaults shown)
sync:
  full_interval: 3600         # Default: 3600 (1 hour)
  incremental_interval: 900   # Default: 900 (15 minutes)
  auto_start: true            # Default: true

# Optional simple retention rules (defaults shown)
rules:
  movie_retention: 90d        # Default: 90d
  tv_retention: 120d          # Default: 120d

# Optional advanced rules (tag-based, episode limits)
advanced_rules:
  - name: Tag Cleanup
    type: tag
    enabled: true
    tag: demo-content
    retention: 7d
  
  - name: Episode Limit
    type: episode
    enabled: true
    tag: daily-shows
    max_episodes: 10
    max_age: 30d
  
  - name: User-Based Cleanup
    type: user
    enabled: true
    require_watched: false     # Default: false (delete after retention regardless of watch status)
    users:
      - username: trial_user
        retention: 7d
      - user_id: 123
        retention: 30d
        require_watched: true  # Per-user override: only delete if watched
      - email: guest@example.com
        retention: 14d

integrations:
  jellyfin:
    enabled: true
    url: http://jellyfin:8096
    api_key: abc123
    username: prunarr        # Optional: for collection management
    password: password       # Optional
    leaving_soon_type: MOVIES_AND_TV  # MOVIES, TV, MOVIES_AND_TV, NONE
  
  radarr:
    enabled: true
    url: http://radarr:7878
    api_key: abc123
  
  sonarr:
    enabled: true
    url: http://sonarr:8989
    api_key: abc123
  
  jellyseerr:
    enabled: false
    url: http://jellyseerr:5055
    api_key: ""
  
  jellystat:
    enabled: false
    url: http://jellystat:3000
    api_key: ""
```

### 2.3 Configuration Defaults Table

| Setting | Default | Description |
|---------|---------|-------------|
| `app.dry_run` | `true` | Safe mode - no actual deletions |
| `app.leaving_soon_days` | `14` | Days before deletion to show "Leaving Soon" |
| `sync.full_interval` | `3600` | Full sync interval (seconds) |
| `sync.incremental_interval` | `900` | Incremental sync interval (seconds) |
| `sync.auto_start` | `true` | Start sync scheduler on boot |
| `rules.movie_retention` | `90d` | Default movie retention period |
| `rules.tv_retention` | `120d` | Default TV show retention period |
| `server.port` | `8080` | HTTP server port |
| `server.host` | `0.0.0.0` | HTTP server bind address |

### 2.4 Configuration Validation

**On startup, Prunarr validates:**
- ✅ Admin credentials present
- ✅ At least one integration enabled
- ✅ Valid URLs for enabled integrations (parseable, non-empty)
- ✅ Valid duration formats (`30d`, `1h`, `90d`)
- ✅ API keys provided for enabled integrations
- ✅ Port ranges valid (1-65535)

**Validation errors fail-fast with clear messages:**
```
ERROR: Configuration validation failed
  - integrations.jellyfin.url: must be a valid URL (got: "not-a-url")
  - integrations.radarr.api_key: required when enabled=true
  - rules.movie_retention: invalid duration format "30 days" (use "30d")
```

### 2.5 Password Auto-Hashing

**Behavior:**
1. On first load, if `admin.password` is plain-text → auto-hash with bcrypt
2. Write hashed password back to `prunarr.yaml`
3. Log warning: `"Plain-text password detected and auto-hashed"`
4. Subsequent loads use the hashed password

**Example:**
```yaml
# Before first run:
admin:
  password: changeme

# After first run (auto-updated):
admin:
  password: $2a$12$LQv3c1yqBWVHxkd0LHAkCOYz6TtxMQJqhN8/LewY5GyYqW4kFqLue
```

### 2.6 Hot-Reload Strategy

**All configuration changes hot-reload automatically:**
- Uses `fsnotify` to watch `prunarr.yaml`
- On file change:
  1. Reload config from disk
  2. Validate new config
  3. Invalidate affected caches
  4. Re-initialize affected services
  5. Log reload event
- No restart required
- Minimal complexity (~50 lines of code)

---

## 3. Storage Architecture

### 3.1 File Structure
```
/app/
├── config/
│   └── prunarr.yaml            # Main configuration (editable via UI)
├── data/
│   ├── exclusions.json         # User "Keep" exclusions
│   └── jobs.json               # Job history
└── logs/
    └── prunarr.log             # Structured logs (JSON)
```

### 3.2 Exclusions File (exclusions.json)

```json
{
  "version": "1.0",
  "updated_at": "2025-11-02T10:30:00Z",
  "items": {
    "tt1234567": {
      "external_id": "tt1234567",
      "external_type": "imdb",
      "media_type": "movie",
      "title": "Favorite Movie (2023)",
      "excluded_at": "2025-11-02T10:30:00Z",
      "excluded_by": "admin",
      "reason": "User clicked Keep button"
    },
    "tt7654321": {
      "external_id": "tt7654321",
      "external_type": "tvdb",
      "media_type": "show",
      "title": "Favorite Show",
      "excluded_at": "2025-11-01T14:20:00Z",
      "excluded_by": "admin",
      "reason": "Manual exclusion"
    }
  }
}
```

### 3.3 Jobs History File (jobs.json)

```json
{
  "version": "1.0",
  "jobs": [
    {
      "id": "uuid-1234",
      "type": "full_sync",
      "status": "completed",
      "started_at": "2025-11-02T10:00:00Z",
      "completed_at": "2025-11-02T10:00:23Z",
      "duration_ms": 23000,
      "summary": {
        "movies_synced": 1234,
        "shows_synced": 567,
        "total_items": 1801,
        "errors": 0
      }
    }
  ]
}
```

---

## 4. Caching Architecture

### 4.1 Cache Backend: go-cache

```go
import "github.com/patrickmn/go-cache"

// Initialize with sensible defaults
cache := cache.New(
    5*time.Minute,  // default expiration
    10*time.Minute, // cleanup interval
)
```

### 4.2 Cache Keys & TTLs

```go
const (
    CacheKeyJellyfinLibrary    = "jellyfin:library:%s"      // TTL: 1h
    CacheKeyRadarrMovies       = "radarr:movies"            // TTL: 30m
    CacheKeyRadarrHistory      = "radarr:history:%d"        // TTL: 15m
    CacheKeySonarrShows        = "sonarr:shows"             // TTL: 30m
    CacheKeySonarrHistory      = "sonarr:history:%d"        // TTL: 15m
    CacheKeyJellyseerrRequests = "jellyseerr:requests"      // TTL: 15m
    CacheKeyJellystatWatch     = "jellystat:watch:%s"       // TTL: 5m
    CacheKeyRuleEvaluation     = "rule:eval:%s"             // TTL: until sync
    CacheKeyDeletionTimeline   = "timeline:deletion"        // TTL: 5m
    CacheKeyLeavingSoon        = "library:leaving_soon"     // TTL: 5m
)
```

### 4.3 Sync Strategy: Full + Incremental

**Full Sync** (every 1 hour, default):
1. Fetch complete library from Jellyfin
2. Fetch all movies/shows from Radarr/Sonarr
3. Fetch all requests from Jellyseerr (if enabled)
4. Clear all cache
5. Re-evaluate all rules
6. Recalculate deletion timeline

**Incremental Sync** (every 15 minutes, default):
1. Fetch recently added items from Radarr/Sonarr (using `since` parameter)
2. Fetch recent history entries
3. Update only changed items in cache
4. Re-evaluate rules for changed items only
5. Update deletion timeline cache

### 4.4 Cache Invalidation

| Event | Invalidation Strategy |
|-------|----------------------|
| Manual sync triggered | Clear all external API caches |
| Rule created/modified | Clear `rule:eval:*` and `timeline:*` |
| Media item excluded | Clear item cache + rule eval + timeline |
| Deletion executed | Clear all external API caches |
| Config file changed | Clear all caches, reload config |

---

## 5. REST API Endpoints

### 5.1 Authentication
```
POST   /api/auth/login          - Login and get JWT token
POST   /api/auth/refresh        - Refresh JWT token
POST   /api/auth/logout         - Logout (invalidate token)
GET    /api/auth/me             - Get current admin info
PUT    /api/auth/password       - Change password
```

### 5.2 Dashboard
```
GET    /api/dashboard/stats     - Overall system statistics
GET    /api/dashboard/health    - Health checks for all integrations
GET    /api/dashboard/activity  - Recent activity feed (from jobs)
GET    /api/dashboard/disk      - Disk space information
```

**Response Example** (`/api/dashboard/stats`):
```json
{
  "library": {
    "total_movies": 1234,
    "total_shows": 567,
    "total_episodes": 8901,
    "leaving_soon_count": 45,
    "scheduled_deletion_count": 12
  },
  "disk": {
    "free_space_gb": 450.5,
    "total_space_gb": 2000.0,
    "free_percent": 22.5
  },
  "last_sync": {
    "type": "incremental",
    "completed_at": "2025-11-02T10:15:02Z",
    "duration_ms": 2000,
    "items_synced": 15
  }
}
```

### 5.3 Integrations
```
GET    /api/integrations                - List all integrations
GET    /api/integrations/:type          - Get integration by type
PUT    /api/integrations/:type          - Update integration
POST   /api/integrations/:type/test     - Test connection
GET    /api/integrations/:type/health   - Health check
```

### 5.4 Rules
```
GET    /api/rules                - List all rules
PUT    /api/rules                - Update rules
GET    /api/rules/preview        - Preview what rules would match
GET    /api/rules/users          - List user-based cleanup rules
PUT    /api/rules/users          - Update user-based cleanup rules
```

### 5.5 Library
```
GET    /api/library/items                - List all media items (from cache)
GET    /api/library/items/:id            - Get single media item
GET    /api/library/leaving-soon         - Items in "leaving soon" window
POST   /api/library/items/:id/keep       - Exclude item from deletion
DELETE /api/library/items/:id/keep       - Remove exclusion
GET    /api/library/stats                - Library statistics
```

**Query Parameters for `/api/library/items`**:
```
?type=movie|show|season|episode
?status=leaving_soon|scheduled|excluded|safe
?sort=title|added_at|scheduled_deletion_at
?order=asc|desc
?page=1&limit=50
?search=query
```

### 5.6 Deletions
```
GET    /api/deletions/timeline              - Timeline view of scheduled deletions
GET    /api/deletions/schedule              - List deletion schedule
POST   /api/deletions/schedule/:id/cancel   - Cancel scheduled deletion
GET    /api/deletions/history               - Past deletion history
POST   /api/deletions/execute/:id           - Manually execute deletion
```

### 5.7 Sync
```
POST   /api/sync/full           - Trigger full sync
POST   /api/sync/incremental    - Trigger incremental sync
GET    /api/sync/status         - Get current sync job status
GET    /api/sync/jobs           - List recent sync jobs
```

### 5.8 Configuration
```
GET    /api/config              - Get all app configuration
PUT    /api/config              - Update app configuration
POST   /api/config/reload       - Reload config from file
```

### 5.9 Exclusions
```
GET    /api/exclusions          - List all excluded items
POST   /api/exclusions          - Add exclusion
DELETE /api/exclusions/:id      - Remove exclusion
```

---

## 6. Project Structure

```
prunarr/
├── cmd/
│   └── prunarr/
│       └── main.go                      # Application entry point
├── internal/
│   ├── api/
│   │   ├── handlers/
│   │   │   ├── auth.go
│   │   │   ├── dashboard.go
│   │   │   ├── integrations.go
│   │   │   ├── rules.go
│   │   │   ├── library.go
│   │   │   ├── deletions.go
│   │   │   ├── sync.go
│   │   │   ├── config.go
│   │   │   └── exclusions.go
│   │   ├── middleware/
│   │   │   ├── auth.go
│   │   │   ├── cors.go
│   │   │   ├── logging.go
│   │   │   └── recovery.go
│   │   └── router.go                    # Chi router setup
│   ├── cache/
│   │   ├── cache.go                     # Cache interface
│   │   └── memory.go                    # go-cache implementation
│   ├── clients/
│   │   ├── jellyfin/
│   │   │   ├── client.go
│   │   │   ├── library.go
│   │   │   ├── collections.go
│   │   │   └── types.go
│   │   ├── radarr/
│   │   │   ├── client.go
│   │   │   ├── movies.go
│   │   │   ├── history.go
│   │   │   └── types.go
│   │   ├── sonarr/
│   │   │   ├── client.go
│   │   │   ├── shows.go
│   │   │   ├── episodes.go
│   │   │   ├── history.go
│   │   │   └── types.go
│   │   ├── jellyseerr/
│   │   │   ├── client.go
│   │   │   ├── requests.go
│   │   │   └── types.go
│   │   └── jellystat/
│   │       ├── client.go
│   │       ├── watch.go
│   │       └── types.go
│   ├── config/
│   │   ├── config.go                    # Viper configuration loading
│   │   ├── types.go                     # Config structs
│   │   ├── defaults.go                  # Default values
│   │   ├── validation.go                # Config validation
│   │   └── watcher.go                   # File watcher for hot-reload
│   ├── storage/
│   │   ├── exclusions.go                # Exclusions file management
│   │   └── jobs.go                      # Jobs file management
│   ├── services/
│   │   ├── auth.go                      # Authentication service
│   │   ├── sync.go                      # Sync orchestration
│   │   ├── rules.go                     # Rule evaluation engine
│   │   ├── deletion.go                  # Deletion orchestration
│   │   ├── library.go                   # Library management
│   │   ├── collections.go               # Collection management
│   │   ├── timeline.go                  # Deletion timeline computation
│   │   └── user_cleanup.go              # User-based cleanup logic
│   └── utils/
│       ├── jwt.go
│       ├── logger.go
│       ├── filesystem.go
│       └── helpers.go
├── web/                                  # React frontend
│   ├── public/
│   ├── src/
│   │   ├── api/
│   │   │   └── client.ts                # API client with TanStack Query
│   │   ├── components/
│   │   │   ├── Layout.tsx
│   │   │   ├── MediaCard.tsx
│   │   │   ├── DeletionTimeline.tsx
│   │   │   └── CountdownTimer.tsx
│   │   ├── pages/
│   │   │   ├── Dashboard.tsx
│   │   │   ├── LeavingSoon.tsx
│   │   │   ├── Library.tsx
│   │   │   ├── Integrations.tsx
│   │   │   ├── Rules.tsx
│   │   │   ├── Configuration.tsx
│   │   │   └── Login.tsx
│   │   ├── hooks/
│   │   │   ├── useAuth.ts
│   │   │   └── useLibrary.ts
│   │   ├── stores/
│   │   │   └── authStore.ts
│   │   ├── types/
│   │   │   └── index.ts
│   │   ├── App.tsx
│   │   └── main.tsx
│   ├── package.json
│   ├── vite.config.ts
│   └── tsconfig.json
├── deployments/
│   ├── docker/
│   │   └── Dockerfile
│   └── docker-compose.yml
├── scripts/
│   ├── build.sh
│   └── setup.sh
├── .github/
│   └── workflows/
│       └── release.yml
├── go.mod
├── go.sum
├── README.md
└── LICENSE
```

---

## 7. User-Based Cleanup Feature

### 7.1 Overview

User-based cleanup automatically removes media content requested by specific users after a configurable retention period. This feature provides fine-grained control over content lifecycle based on:

1. **Who requested it** (via Jellyseerr)
2. **Whether they watched it** (via Jellystat, optional)

### 7.2 Configuration

**Simple user-based cleanup** (retention only):
```yaml
advanced_rules:
  - name: Trial User Cleanup
    type: user
    enabled: true
    require_watched: false  # Delete after retention period regardless of watch status
    users:
      - username: trial_user
        retention: 7d
      - user_id: 123
        retention: 30d
```

**Advanced with watch tracking** (requires Jellystat):
```yaml
advanced_rules:
  - name: User Cleanup with Watch Tracking
    type: user
    enabled: true
    require_watched: true   # Only delete if user has watched it
    users:
      - username: occasional_user
        retention: 14d
        require_watched: true  # Per-user override
      - email: guest@example.com
        retention: 7d
        require_watched: false  # Delete after 7d even if not watched
```

### 7.3 Matching Strategies

User matching supports multiple identifiers (in order of priority):

1. **user_id** (integer) - Most reliable, from Jellyseerr user ID
2. **username** (string) - Jellyseerr username (case-insensitive)
3. **email** (string) - User email address (case-insensitive)

**Example:**
```yaml
users:
  - user_id: 42              # Matches Jellyseerr user ID 42
    retention: 14d
  - username: john_doe       # Matches username "john_doe" (case-insensitive)
    retention: 30d
  - email: temp@example.com  # Matches email "temp@example.com"
    retention: 7d
```

### 7.4 Watch Tracking Logic

When `require_watched: true`, deletion only occurs if **BOTH** conditions are met:

1. **Retention period has passed** (e.g., 30 days since added)
2. **User has watched the content** (tracked via Jellystat)

**Behavior:**
```
Media Added: Jan 1
Retention: 30 days
Watched: Jan 15

If require_watched=true:
  - Jan 31: ✅ DELETE (30 days passed AND watched)

If require_watched=false:
  - Jan 31: ✅ DELETE (30 days passed, watch status ignored)
```

**Not watched behavior:**
```
Media Added: Jan 1
Retention: 30 days
Watched: Never

If require_watched=true:
  - Jan 31: ❌ KEEP (30 days passed BUT not watched)
  - Feb 15: ✅ DELETE (45 days passed AND watched on Feb 14)

If require_watched=false:
  - Jan 31: ✅ DELETE (30 days passed, watch status ignored)
```

### 7.5 Data Model

**LibraryItem enhancements:**
```go
type LibraryItem struct {
    // ... existing fields ...
    
    // Requester info (populated from Jellyseerr)
    RequestedByUserID   *int    `json:"requested_by_user_id,omitempty"`
    RequestedByUsername *string `json:"requested_by_username,omitempty"`
    RequestedByEmail    *string `json:"requested_by_email,omitempty"`
    
    // Watch tracking (populated from Jellystat)
    WatchedByUsers      []int   `json:"watched_by_users,omitempty"`  // User IDs who watched
    LastWatchedAt       *time.Time `json:"last_watched_at,omitempty"`
}
```

### 7.6 Rule Evaluation Algorithm

```
FOR EACH library item:
    IF no user-based rules enabled:
        SKIP
    
    // Step 1: Populate requester from Jellyseerr
    IF item.RequestedByUserID is NULL:
        CALL jellyseerr.PopulateRequester(item)
    
    IF item.RequestedByUserID is still NULL:
        SKIP  // Not requested via Jellyseerr
    
    // Step 2: Find matching user rule
    matching_rule = FIND rule WHERE (
        rule.user_id == item.RequestedByUserID OR
        rule.username == item.RequestedByUsername (case-insensitive) OR
        rule.email == item.RequestedByEmail (case-insensitive)
    )
    
    IF no matching_rule:
        SKIP
    
    // Step 3: Check retention period
    age = NOW - item.ImportedDate
    IF age < matching_rule.retention:
        SKIP  // Too recent
    
    // Step 4: Check watch requirement (if enabled)
    IF matching_rule.require_watched:
        IF item.WatchedByUsers is empty:
            CALL jellystat.PopulateWatchHistory(item)
        
        IF item.RequestedByUserID NOT IN item.WatchedByUsers:
            SKIP  // Required to watch but hasn't watched yet
    
    // Step 5: Schedule for deletion
    SCHEDULE_DELETE(item, reason="User-based cleanup", user=matching_rule)
```

### 7.7 API Endpoints

**User Rules Management:**
```
GET    /api/rules/users
Response:
{
  "rules": [
    {
      "name": "Trial User Cleanup",
      "type": "user",
      "enabled": true,
      "require_watched": false,
      "users": [
        {
          "username": "trial_user",
          "retention": "7d"
        }
      ]
    }
  ]
}

PUT    /api/rules/users
Body: Same as GET response (full replacement)
```

**User Watch Status:**
```
GET    /api/library/items/:id/watch-status
Response:
{
  "item_id": "tt1234567",
  "requested_by": {
    "user_id": 42,
    "username": "john_doe",
    "email": "john@example.com"
  },
  "watched_by": [
    {
      "user_id": 42,
      "watched_at": "2025-01-15T10:30:00Z"
    }
  ],
  "eligible_for_deletion": true,
  "reason": "Retention passed (30d) and user has watched"
}
```

### 7.8 Use Cases

1. **Trial/Temporary Users**
   - Delete content requested by trial users after 7 days
   - Ensure trial users don't permanently consume storage

2. **Watch-and-Delete Policy**
   - User requests content → retention 30 days
   - User watches content → eligible for deletion
   - User doesn't watch → keep indefinitely (encourages engagement)

3. **Tiered User Management**
   - Free tier: 14 days retention
   - Premium tier: 90 days retention
   - VIP tier: Excluded from user-based cleanup

4. **Inactive User Cleanup**
   - Occasional requesters: 30 days retention
   - Active requesters: 90 days retention (configured separately)

### 7.9 Dependencies

**Required:**
- Jellyseerr integration (for requester tracking)

**Optional but recommended:**
- Jellystat integration (for watch tracking with `require_watched: true`)

**Fallback behavior:**
- If Jellystat disabled and `require_watched: true` → treat as unwatched (keep indefinitely)
- If Jellyseerr disabled → user-based cleanup disabled

### 7.10 Configuration Validation

**On startup/config reload:**
```
✅ At least one user identifier (user_id, username, or email) per rule
✅ Valid retention duration format (e.g., "7d", "30d")
✅ If require_watched=true, Jellystat integration is configured
✅ No duplicate user identifiers across rules
✅ user_id is positive integer if provided

❌ Error examples:
  - "User rule missing all identifiers (user_id, username, email)"
  - "require_watched=true but Jellystat integration disabled"
  - "Invalid retention format '30 days' (use '30d')"
```

---

## 8. Implementation Phases

### Current Status Overview

**Backend Progress**: ~90% Complete ✅
- ✅ Complete REST API (12 endpoints)
- ✅ All service integrations (Jellyfin, Radarr, Sonarr, Jellyseerr, Jellystat)
- ✅ Sync engine with scheduler
- ✅ Rules engine with retention policies
- ✅ Deletion executor with dry-run
- ✅ Exclusions management with persistence through syncs
- ✅ Deletion reason generation with detailed explanations
- ✅ Job history tracking
- ✅ Authentication & authorization (with optional bypass)
- ✅ Configuration with hot-reload
- ⏳ User-based cleanup (pending)

**Testing**: 208 tests passing
- Handlers: 89.0% coverage
- Storage: 92.7% coverage  
- Services: 52.2% coverage
- Clients: 5.3% coverage

**Frontend Progress**: ~50% Complete ⏳
- ✅ React + Vite + shadcn/ui initialized
- ✅ Login page functional
- ✅ Dashboard with media statistics
- ✅ Library browser (Movies/TV Shows tabs)
- ✅ "Leaving Soon" view with countdown timers
- ✅ Deletion reason tooltips
- ✅ Exclusion management UI (Keep button)
- ✅ Type badges and visual indicators
- ✅ API client with TanStack Query
- ❌ Deletion timeline view (pending)
- ❌ Configuration editor (pending)
- ❌ Advanced rules UI (pending)

**Tools Available**:
- `make dev` - Start development server
- `./test-api.sh` - Automated API testing
- `config/prunarr.yaml.example` - Configuration template

---

## Implementation Phases (7 Weeks)

### Phase 1: Foundation ✅ COMPLETED
**Goal**: Basic backend + config + auth

**Backend**:
- [x] Project initialization
- [x] Config loading with Viper (with defaults)
- [x] Config validation
- [x] Password auto-hashing
- [x] File-based storage (exclusions, jobs)
- [x] go-cache integration
- [x] Chi router + middleware
- [x] JWT authentication
- [x] Health check endpoint
- [x] Structured logging (zerolog)
- [x] Hot-reload support
- [x] Security hardening

**Frontend**:
- [ ] React + Vite + shadcn/ui setup
- [ ] Login page
- [ ] Basic dashboard shell
- [ ] API client (TanStack Query)

**Deliverable**: ✅ App starts, login works, all Phase 1 API endpoints functional
**Status**: Backend complete with 100% unit test coverage for core modules

---

### Phase 2: Integrations & Sync ✅ COMPLETED
**Goal**: Connect to external services

**Backend**:
- [x] Jellyfin client (with comprehensive tests)
- [x] Radarr client (with comprehensive tests)
- [x] Sonarr client (with comprehensive tests)
- [x] Jellyseerr client (optional integration)
- [x] Jellystat client (optional integration)
- [x] Full sync service with media aggregation
- [x] Incremental sync service
- [x] Background scheduler with auto-start
- [x] Job history tracking with circular buffer
- [x] Complete API endpoints (sync, media, jobs, exclusions)

**Frontend**:
- [ ] Integrations page (CRUD)
- [ ] Health checks
- [ ] Sync trigger buttons
- [ ] Activity feed

**Deliverable**: ✅ All integrations working, sync engine operational, REST API complete
**Status**: Backend complete with 208 tests passing, 89% handler coverage, ready for live testing

---

### Phase 3: Rules & Deletion Logic ✅ COMPLETED
**Goal**: Core cleanup logic + deletion visibility

**Backend**:
- [x] Simple rule evaluation (movie/tv retention)
- [x] Advanced rule evaluation (tag-based, episode limits)
- [x] Deletion timeline computation
- [x] "Leaving Soon" calculator
- [x] Exclusion logic (add/remove/list)
- [x] Exclusion persistence through syncs (bug fix applied)
- [x] Deletion executor with batch operations
- [x] Dry-run enforcement (safe by default)
- [x] Watch history integration (Jellystat)
- [x] Request tracking (Jellyseerr)
- [x] Deletion reason generation with detailed explanations
- [x] `applyExclusions()` method to reapply exclusions during sync

**Frontend**:
- [x] Library browser with filters (basic implementation)
- [x] **"Leaving Soon" dashboard** with countdown timers
- [x] **Deletion reason tooltips** with info icons
- [ ] **Deletion timeline view** (grouped by date)
- [x] **"Keep" button** on media cards (exclusion functionality)
- [ ] Rules configuration page

**Deliverable**: ✅ Backend deletion logic complete, exclusions working correctly through syncs, deletion reasons implemented
**Status**: Backend complete with comprehensive testing and bug fixes applied. Basic frontend dashboard implemented with leaving-soon view and deletion reason tooltips.

---

### Phase 4: Advanced Features & Polish ⏳ IN PROGRESS
**Goal**: Feature parity + polish

**Backend**:
- [x] Jellyseerr client
- [x] Jellystat client
- [x] Advanced rules (tag-based, episode limits)
- [x] Config hot-reload
- [x] Optional authentication bypass for testing (`admin.disable_auth`)
- [ ] User-based cleanup with watch tracking
- [ ] Collection management

**Frontend**:
- [x] React + Vite + shadcn/ui setup
- [x] Login page with JWT integration
- [x] Dashboard with media statistics
- [x] Library browser with filters (Movies/TV Shows tabs)
- [x] "Leaving Soon" view with countdown timers
- [x] Deletion reason tooltips with info icons
- [x] "Keep" button functionality (Shield/ShieldOff icons)
- [x] Type badges (Movie/TV Show indicators)
- [x] API client with null safety
- [ ] Deletion timeline view (grouped by date)
- [ ] Configuration page (full YAML editor)
- [ ] Deletion history
- [ ] Statistics/charts
- [ ] User-based rules UI
- [ ] Mobile responsive design improvements
- [ ] Comprehensive error handling
- [ ] Loading states improvements

**Deliverable**: Production-ready UI + advanced backend features
**Status**: 
- Backend ~90% complete (user-based rules pending)
- Frontend ~50% complete (basic dashboard operational, advanced features pending)

---

## 9. Docker Deployment

### 9.1 Dockerfile

```dockerfile
# Build Frontend
FROM node:22-alpine AS frontend-builder
WORKDIR /build
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Build Backend
FROM golang:1.23-alpine AS backend-builder
WORKDIR /build
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend-builder /build/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -tags netgo \
    -o prunarr \
    ./cmd/prunarr

# Runtime
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
RUN addgroup -g 1000 prunarr && \
    adduser -D -u 1000 -G prunarr prunarr
WORKDIR /app
COPY --from=backend-builder /build/prunarr .
RUN mkdir -p /app/config /app/data /app/logs && \
    chown -R prunarr:prunarr /app
USER prunarr
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s \
    CMD wget -q --spider http://localhost:8080/health || exit 1
CMD ["./prunarr"]
```

### 9.2 Docker Compose

```yaml
version: "3.9"

services:
  prunarr:
    container_name: prunarr
    image: ghcr.io/yourname/prunarr:latest
    user: "1000:1000"
    volumes:
      - ./config:/app/config
      - ./data:/app/data
      - ./logs:/app/logs
      - /path/to/media:/media:ro
    environment:
      - SERVER_PORT=8080
      - JWT_SECRET=change-me-in-production
      - LOG_LEVEL=info
      - TZ=America/New_York
    ports:
      - "8080:8080"
    restart: unless-stopped
```

### 9.3 Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_PORT` | `8080` | HTTP server port |
| `SERVER_HOST` | `0.0.0.0` | HTTP bind address |
| `CONFIG_PATH` | `/app/config/prunarr.yaml` | Config file path |
| `DATA_PATH` | `/app/data` | Data directory |
| `LOG_LEVEL` | `info` | Log level (debug/info/warn/error) |
| `LOG_FORMAT` | `json` | Log format (json/text) |
| `JWT_SECRET` | *required* | JWT signing secret (32+ chars) |
| `JWT_EXPIRATION` | `24h` | JWT token expiration |
| `TZ` | `UTC` | Timezone |

---

## 10. Performance Targets

| Metric | Target | Expected |
|--------|--------|----------|
| Docker image size | <20MB | ~15MB |
| RAM usage (idle) | <40MB | ~30MB |
| RAM usage (sync 10k items) | <60MB | ~45MB |
| Startup time | <100ms | ~50ms |
| API response (cached) | <50ms | ~20ms |
| API response (uncached) | <200ms | ~100ms |
| Full sync (1000 movies) | <30s | ~20s |
| Incremental sync | <5s | ~2s |
| Rule evaluation (1000 items) | <1s | ~500ms |
| Config reload | <100ms | ~50ms |

---

## 11. Security Considerations

1. **JWT Secret**: Must be 32+ characters, randomly generated
2. **Password Hashing**: bcrypt with cost 12
3. **File Permissions**: Config 0600, data 0644
4. **Input Validation**: All user inputs sanitized
5. **Path Traversal**: Validate all filesystem operations
6. **API Keys**: Never log API keys (mask in logs)

---

## 12. Key Features

### Deletion Visibility
- **Timeline View**: See all scheduled deletions grouped by date
- **Countdown Timers**: Live countdown to deletion date
- **"Keep" Button**: One-click exclusion from deletion
- **"Leaving Soon" Dashboard**: Items entering deletion window

### Simple Configuration
- **Minimal YAML**: Just credentials + overrides
- **Sensible Defaults**: Works out-of-box for 90% of users
- **UI Editor**: Edit config directly in web UI
- **Hot-Reload**: Changes apply immediately

### Smart Caching
- **In-Memory**: Fast, no external dependencies
- **Incremental Sync**: Only fetch changed items
- **Auto-Invalidation**: Caches cleared intelligently

### User-Based Cleanup with Watch Tracking
- **Requester-Based**: Delete content based on who requested it
- **Watch Tracking**: Only delete after user has watched (optional)
- **Flexible Matching**: Match by user ID, username, or email
- **Tiered Policies**: Different retention periods for different users

---

## 13. Feature Parity with Janitorr v1

| Feature | Janitorr v1 | Prunarr |
|---------|-------------|---------|
| Media deletion by age | ✅ | ✅ Phase 3 |
| Tag-based deletion | ✅ | ✅ Phase 4 |
| Episode cleanup | ✅ | ✅ Phase 4 |
| **User-based cleanup** | ✅ (PR pending) | ✅ Phase 4 |
| **Watch-tracked deletion** | ❌ | ✅ NEW Phase 4 |
| Leaving Soon collections | ✅ | ✅ Phase 4 |
| Jellyfin support | ✅ | ✅ Phase 2 |
| Radarr integration | ✅ | ✅ Phase 2 |
| Sonarr integration | ✅ | ✅ Phase 2 |
| Jellyseerr integration | ✅ | ✅ Phase 4 |
| Jellystat tracking | ✅ | ✅ Phase 4 |
| Dry-run mode | ✅ | ✅ Phase 3 |
| Web UI | ✅ (read-only) | ✅ (full CRUD) |
| **Simple config** | ❌ | ✅ NEW |
| **Deletion timeline** | ❌ | ✅ NEW |
| **One-click "Keep"** | ❌ | ✅ NEW |
| **Hot-reload config** | ❌ | ✅ NEW |
| **API-first** | ❌ | ✅ NEW |

---

## 14. Recent Fixes & Improvements

### 14.1 Exclusion Persistence Fix (Nov 2, 2025)

**Problem**: 
Media items marked as excluded in `data/exclusions.json` were showing `"excluded": false` after sync operations. When syncing from Radarr/Sonarr, new `models.Media` structs were created with the default value `IsExcluded = false`, causing exclusion status to be lost.

**Solution**:
Added `applyExclusions()` method in `internal/services/sync.go` that:
1. Runs during `FullSync()` after all integrations sync but before retention rules
2. Iterates through all media items in the library
3. Checks each item's ID against the exclusions file using `IsExcluded(id)`
4. Updates the `IsExcluded` field accordingly
5. Logs the count of excluded items for debugging

**Files Modified**:
- `internal/services/sync.go` - Added `applyExclusions()` method (lines 572-595)
- Called in `FullSync()` at line 234, before `applyRetentionRules()`

**Testing**:
- ✅ Exclusions persist through multiple full syncs
- ✅ Adding/removing exclusions via API works correctly
- ✅ Status endpoint shows correct `excluded_count`
- ✅ Exclusions from file are reapplied after restart

### 14.2 Deletion Reason Tooltips (Nov 2, 2025)

**Problem**:
Users couldn't understand why media items were scheduled for deletion. The UI showed countdown timers but no explanation of the retention rules being applied.

**Solution**:
1. **Backend** - Added deletion reason generation:
   - Made `GenerateDeletionReason()` method public in `internal/services/rules.go` (line 132)
   - Generates human-readable explanations like: "This movie was last watched 95 days ago. The retention policy for movies is 90 days, meaning it will be deleted after that period of inactivity."
   - Populates `DeletionReason` field for all items with `daysUntilDue > 0` in `applyRetentionRules()`
   - Also populates reason in `GetLeavingSoon()` method

2. **Frontend** - Added info icon tooltips:
   - Added `DeletionReason` field to `MediaItem` TypeScript interface
   - Added Info icon from Lucide in `DashboardPage.tsx`
   - Displays reason on hover using native HTML title attribute
   - Shows next to countdown timer in "Leaving Soon" section

**Files Modified**:
- `internal/models/media.go` - Added `DeletionReason string` field
- `internal/services/rules.go` - Made `GenerateDeletionReason()` public
- `internal/services/sync.go` - Populates deletion reason in `applyRetentionRules()`
- `web/src/lib/types.ts` - Added `deletion_reason?` field
- `web/src/pages/DashboardPage.tsx` - Added Info icon with tooltip

**Benefits**:
- Users understand why items are being deleted
- Transparency in retention policy enforcement
- Better user experience with clear explanations

### 14.3 Optional Authentication Bypass (Nov 2, 2025)

**Problem**:
Testing and development required generating JWT tokens for every API request, making iteration slow and cumbersome.

**Solution**:
Added optional authentication bypass feature:
1. Added `DisableAuth bool` field to `AdminConfig` struct in `internal/config/types.go`
2. Updated `Auth()` middleware in `internal/api/middleware/auth.go` to check config and bypass when enabled
3. Added `disable_auth: true` option to test configuration file

**Usage**:
```yaml
admin:
  username: admin
  password: changeme
  disable_auth: true  # Bypass JWT authentication (development only)
```

**Security Notes**:
- ⚠️ Should NEVER be enabled in production
- Only for local development and testing
- Logged as DEBUG message when bypassed
- Config validation should warn if enabled

**Files Modified**:
- `internal/config/types.go` - Added `DisableAuth` field
- `internal/api/middleware/auth.go` - Added bypass logic
- `config/prunarr.test.yaml` - Example configuration

### 14.4 API Response Null Safety (Nov 2, 2025)

**Problem**:
Frontend was receiving undefined/null responses from API endpoints, causing runtime errors in the UI when trying to iterate over media items.

**Solution**:
Updated API client in `web/src/lib/api.ts` to provide default empty arrays and safe response handling:
```typescript
const response = await this.request<MediaListResponse>(`/media/movies?${query}`);
return {
  items: response.items || [],
  total: response.total || 0,
};
```

Applied to:
- `listMovies()`
- `listShows()` 
- `listLeavingSoon()`

**Benefits**:
- No more "Cannot iterate over undefined" errors
- Graceful handling of empty responses
- Consistent return types

---

## Summary

**Prunarr** is a complete rewrite focused on:

✅ **Simplicity** - Minimal config, sensible defaults  
✅ **Performance** - 15MB image, <40MB RAM, <50ms startup  
✅ **Visibility** - Timeline view, countdown timers, "Keep" button  
✅ **Modern** - React 19, Go 1.23, JWT auth, hot-reload  
✅ **Developer-friendly** - API-first, structured logs, clear validation  

**Next Steps**:
1. Review and approve this spec
2. Begin Phase 1 implementation
3. Initialize Go project structure
4. Build config loading with validation
