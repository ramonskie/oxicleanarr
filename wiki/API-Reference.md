# API Reference

Complete REST API documentation for OxiCleanarr.

**Related pages:**
- [Architecture](Architecture.md) - System architecture and design
- [Configuration](Configuration.md) - API endpoint configuration
- [Advanced Rules](Advanced-Rules.md) - Rules management API

---

## Table of Contents

- [Authentication](#authentication)
- [Health Check](#health-check)
- [Media Endpoints](#media-endpoints)
- [Sync Endpoints](#sync-endpoints)
- [Jobs Endpoints](#jobs-endpoints)
- [Configuration Endpoints](#configuration-endpoints)
- [Rules Endpoints](#rules-endpoints)
- [Error Responses](#error-responses)

---

## Base URL

All API endpoints are served at:

```
http://<host>:<port>/api
```

Default: `http://localhost:8080/api`

---

## Authentication

OxiCleanarr uses JWT (JSON Web Token) authentication for all protected API endpoints.

### POST /api/auth/login

Authenticate and receive a JWT token.

**Request:**
```json
{
  "username": "admin",
  "password": "your-password"
}
```

**Response (200 OK):**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**Error (401 Unauthorized):**
```json
{
  "error": "Invalid username or password"
}
```

### Using the Token

Include the JWT token in the `Authorization` header for all protected endpoints:

```
Authorization: Bearer <token>
```

**Example:**
```bash
curl -H "Authorization: Bearer eyJhbGci..." \
  http://localhost:8080/api/media/movies
```

### Token Expiration

- Default expiration: 24 hours
- Configurable via `server.jwt_expiration` in `config.yaml`
- When expired, re-authenticate to receive a new token

### Disabling Authentication

To disable authentication (NOT recommended for production):

```yaml
admin:
  disable_auth: true
```

---

## Health Check

### GET /health

Check service health status. Does not require authentication.

**Response (200 OK):**
```json
{
  "status": "ok",
  "uptime": "2h30m15s",
  "version": "1.0.0"
}
```

---

## Media Endpoints

All media endpoints require authentication.

### GET /api/media/movies

List all movies in the media library.

**Query Parameters:**
- `sort_by` (optional): Field to sort by (`title`, `added_at`, `delete_after`)
- `order` (optional): Sort order (`asc`, `desc`)
- `status` (optional): Filter by status (`all`, `leaving_soon`, `excluded`)

**Response (200 OK):**
```json
{
  "items": [
    {
      "id": "radarr-123",
      "title": "Inception",
      "type": "movie",
      "added_at": "2024-01-15T10:30:00Z",
      "delete_after": "2024-04-15T10:30:00Z",
      "days_until_due": 15,
      "is_excluded": false,
      "exclusion_reason": "",
      "jellyfin_match_status": "matched",
      "path": "/movies/Inception (2010)/Inception (2010).mkv",
      "tags": ["action", "premium"],
      "watch_history": {
        "user_123": {
          "last_watched": "2024-03-01T20:00:00Z",
          "play_count": 3
        }
      }
    }
  ],
  "total": 150
}
```

### GET /api/media/shows

List all TV shows in the media library.

**Query Parameters:** Same as `/api/media/movies`

**Response:** Same structure as movies, but `type` is `tv_show`

### GET /api/media/leaving-soon

List media items in the "leaving soon" window (within configured threshold days).

**Response (200 OK):**
```json
{
  "items": [
    {
      "id": "radarr-456",
      "title": "Interstellar",
      "type": "movie",
      "delete_after": "2024-04-01T10:30:00Z",
      "days_until_due": 3,
      "is_excluded": false
    }
  ],
  "total": 12
}
```

### GET /api/media/unmatched

List media items with Jellyfin matching issues (not found or metadata mismatch).

**Response (200 OK):**
```json
{
  "items": [
    {
      "id": "radarr-789",
      "title": "Unknown Movie",
      "jellyfin_match_status": "not_found",
      "path": "/movies/Unknown Movie (2020)/movie.mkv"
    }
  ],
  "total": 5
}
```

### GET /api/media/{id}

Get details for a specific media item.

**Path Parameters:**
- `id`: Media item ID (e.g., `radarr-123`, `sonarr-456`)

**Response (200 OK):** Single media object (same structure as list responses)

**Error (404 Not Found):**
```json
{
  "error": "Media not found"
}
```

### POST /api/media/{id}/exclude

Add a media item to the exclusion list (prevent deletion).

**Path Parameters:**
- `id`: Media item ID

**Request (optional):**
```json
{
  "reason": "Keep forever - family favorite"
}
```

**Response (200 OK):**
```json
{
  "success": true,
  "message": "Exclusion added"
}
```

### DELETE /api/media/{id}/exclude

Remove a media item from the exclusion list.

**Path Parameters:**
- `id`: Media item ID

**Response (200 OK):**
```json
{
  "success": true,
  "message": "Exclusion removed"
}
```

### DELETE /api/media/{id}

Delete a media item from all systems (Radarr/Sonarr, Jellyfin).

**Path Parameters:**
- `id`: Media item ID

**Query Parameters:**
- `dry_run` (optional): Set to `true` to preview deletion without executing

**Response (200 OK):**
```json
{
  "success": true,
  "message": "Media deleted successfully",
  "dry_run": false
}
```

**Dry Run Response:**
```json
{
  "success": true,
  "message": "Dry run: Media would be deleted",
  "dry_run": true
}
```

---

## Sync Endpoints

All sync endpoints require authentication.

### POST /api/sync/full

Trigger a full synchronization with all external services (Jellyfin, Radarr, Sonarr, etc.).

**Response (202 Accepted):**
```json
{
  "success": true,
  "message": "Full sync started"
}
```

**Note:** Sync runs asynchronously. Check `/api/sync/status` for progress.

### POST /api/sync/incremental

Trigger an incremental sync (only changes since last sync).

**Response (202 Accepted):**
```json
{
  "success": true,
  "message": "Incremental sync started"
}
```

### GET /api/sync/status

Get current sync status and statistics.

**Response (200 OK):**
```json
{
  "is_syncing": true,
  "last_sync_at": "2024-03-28T15:30:00Z",
  "last_sync_type": "full",
  "last_sync_duration": "45s",
  "media_count": {
    "total": 500,
    "movies": 300,
    "tv_shows": 200
  },
  "scheduled_for_deletion": 15,
  "excluded_count": 12,
  "next_full_sync": "2024-03-29T00:00:00Z",
  "next_incremental_sync": "2024-03-28T16:00:00Z"
}
```

---

## Jobs Endpoints

All jobs endpoints require authentication.

### GET /api/jobs

List all job execution history.

**Response (200 OK):**
```json
{
  "jobs": [
    {
      "id": "job-2024-03-28-15-30-00",
      "type": "full_sync",
      "status": "completed",
      "started_at": "2024-03-28T15:30:00Z",
      "completed_at": "2024-03-28T15:30:45Z",
      "duration": "45s",
      "items_processed": 500,
      "items_added": 5,
      "items_deleted": 3,
      "errors": []
    }
  ],
  "total": 150
}
```

### GET /api/jobs/latest

Get the most recent job execution.

**Response (200 OK):** Single job object (same structure as list responses)

**Error (404 Not Found):**
```json
{
  "error": "No jobs found"
}
```

### GET /api/jobs/{id}

Get details for a specific job.

**Path Parameters:**
- `id`: Job ID (e.g., `job-2024-03-28-15-30-00`)

**Response (200 OK):** Single job object with detailed logs

---

## Configuration Endpoints

All configuration endpoints require authentication.

### GET /api/config

Get current configuration (sanitized - API keys and passwords hidden).

**Response (200 OK):**
```json
{
  "admin": {
    "username": "admin",
    "disable_auth": false
  },
  "app": {
    "leaving_soon_days": 7,
    "data_dir": "./data"
  },
  "sync": {
    "full_interval": 1440,
    "incremental_interval": 60
  },
  "rules": {
    "movie_retention": "90d",
    "tv_retention": "60d",
    "delete_immediately": false
  },
  "server": {
    "host": "0.0.0.0",
    "port": 8080,
    "jwt_secret": "***HIDDEN***",
    "jwt_expiration": 86400
  },
  "integrations": {
    "jellyfin": {
      "enabled": true,
      "url": "http://jellyfin:8096",
      "has_api_key": true,
      "timeout": "30s",
      "symlink_library": {
        "enabled": true,
        "library_name": "Leaving Soon",
        "library_path": "/media/leaving-soon"
      }
    },
    "radarr": {
      "enabled": true,
      "url": "http://radarr:7878",
      "has_api_key": true,
      "timeout": "30s"
    },
    "sonarr": {
      "enabled": true,
      "url": "http://sonarr:8989",
      "has_api_key": true,
      "timeout": "30s"
    }
  },
  "advanced_rules": [
    {
      "name": "Premium Content",
      "type": "tag",
      "enabled": true,
      "tag": "premium",
      "retention": "365d"
    }
  ]
}
```

### PUT /api/config

Update configuration. All fields are optional (partial updates supported).

**Request:**
```json
{
  "app": {
    "leaving_soon_days": 14
  },
  "rules": {
    "movie_retention": "120d"
  },
  "integrations": {
    "jellyfin": {
      "api_key": "new-api-key"
    }
  }
}
```

**Response (200 OK):**
```json
{
  "message": "Configuration updated successfully"
}
```

**Error (400 Bad Request):**
```json
{
  "error": "Validation error: movie_retention must be a valid duration (e.g., 90d, 3m, 1y)"
}
```

**Notes:**
- Configuration is written to `config.yaml` and hot-reloaded
- Changing retention rules triggers re-evaluation of existing media
- Changing sync intervals restarts the sync scheduler

---

## Rules Endpoints

All rules endpoints require authentication.

### GET /api/rules

List all advanced rules.

**Response (200 OK):**
```json
{
  "rules": [
    {
      "name": "Premium Content",
      "type": "tag",
      "enabled": true,
      "tag": "premium",
      "retention": "365d"
    },
    {
      "name": "Keep Last 5 Episodes",
      "type": "episode",
      "enabled": true,
      "max_episodes": 5,
      "require_watched": true
    }
  ]
}
```

### POST /api/rules

Create a new advanced rule.

**Request (Tag-Based Rule):**
```json
{
  "name": "Archive Content",
  "type": "tag",
  "enabled": true,
  "tag": "archive",
  "retention": "1825d"
}
```

**Request (Episode-Based Rule):**
```json
{
  "name": "Recent Episodes Only",
  "type": "episode",
  "enabled": true,
  "max_episodes": 10,
  "max_age": "30d",
  "require_watched": false
}
```

**Request (User-Based Rule):**
```json
{
  "name": "Family Members Retention",
  "type": "user",
  "enabled": true,
  "users": [
    {
      "username": "alice",
      "retention": "180d"
    },
    {
      "email": "bob@example.com",
      "retention": "90d"
    }
  ]
}
```

**Response (201 Created):** Rule object

**Error (409 Conflict):**
```json
{
  "error": "Rule with this name already exists"
}
```

### PUT /api/rules/{name}

Update an existing rule.

**Path Parameters:**
- `name`: Rule name (URL-encoded)

**Request:** Same structure as POST (all fields required)

**Response (200 OK):** Updated rule object

### DELETE /api/rules/{name}

Delete a rule.

**Path Parameters:**
- `name`: Rule name (URL-encoded)

**Response (200 OK):**
```json
{
  "message": "Rule deleted successfully"
}
```

### PATCH /api/rules/{name}/toggle

Enable or disable a rule without modifying other settings.

**Path Parameters:**
- `name`: Rule name (URL-encoded)

**Request:**
```json
{
  "enabled": false
}
```

**Response (200 OK):** Updated rule object

---

## Deletion Endpoints

### POST /api/deletions/execute

Execute scheduled deletions for all overdue media.

**Query Parameters:**
- `dry_run` (optional): Set to `true` to preview deletions without executing

**Response (200 OK - Actual Deletion):**
```json
{
  "success": true,
  "scheduled_count": 15,
  "deleted_count": 12,
  "failed_count": 3,
  "message": "Deletion execution completed",
  "deleted_items": [
    {
      "id": "radarr-123",
      "title": "Old Movie",
      "type": "movie"
    }
  ]
}
```

**Response (200 OK - Dry Run):**
```json
{
  "success": true,
  "scheduled_count": 15,
  "dry_run": true,
  "message": "Dry-run preview: No deletions performed",
  "candidates": [
    {
      "id": "radarr-123",
      "title": "Old Movie",
      "delete_after": "2024-03-20T10:00:00Z",
      "days_overdue": 8
    }
  ]
}
```

---

## Error Responses

All endpoints may return the following error responses:

### 400 Bad Request
```json
{
  "error": "Invalid request body"
}
```

### 401 Unauthorized
```json
{
  "error": "Invalid or expired token"
}
```

### 404 Not Found
```json
{
  "error": "Resource not found"
}
```

### 409 Conflict
```json
{
  "error": "Resource already exists"
}
```

### 500 Internal Server Error
```json
{
  "error": "Internal server error"
}
```

---

## Rate Limiting

Currently, OxiCleanarr does not implement rate limiting. For production deployments, consider using a reverse proxy (e.g., nginx) with rate limiting enabled.

---

## API Versioning

The current API version is **v1** (implicit - no version in URL).

Future versions will be explicitly versioned: `/api/v2/...`

---

## CORS Support

CORS is enabled for all origins by default. Configure CORS settings in the router initialization if needed.

**Allowed Methods:** GET, POST, PUT, DELETE, OPTIONS  
**Allowed Headers:** Accept, Authorization, Content-Type, X-CSRF-Token  
**Credentials:** Supported  
**Max Age:** 300 seconds

---

## Timeouts

- **Request Timeout:** 60 seconds (global)
- **Integration Timeouts:** Configurable per service (default: 30s)

Long-running operations (sync, deletions) run asynchronously and return immediately with a 202 Accepted response.

---

## Examples

### Complete Workflow Example

```bash
# 1. Login
TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"changeme"}' | jq -r '.token')

# 2. Trigger full sync
curl -X POST http://localhost:8080/api/sync/full \
  -H "Authorization: Bearer $TOKEN"

# 3. Check sync status
curl http://localhost:8080/api/sync/status \
  -H "Authorization: Bearer $TOKEN" | jq

# 4. List leaving soon items
curl http://localhost:8080/api/media/leaving-soon \
  -H "Authorization: Bearer $TOKEN" | jq

# 5. Exclude a media item
curl -X POST http://localhost:8080/api/media/radarr-123/exclude \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"reason":"Family favorite"}'

# 6. Preview deletions (dry run)
curl -X POST "http://localhost:8080/api/deletions/execute?dry_run=true" \
  -H "Authorization: Bearer $TOKEN" | jq

# 7. Execute deletions
curl -X POST http://localhost:8080/api/deletions/execute \
  -H "Authorization: Bearer $TOKEN" | jq
```

---

## See Also

- [Architecture](Architecture.md) - API design and data flow
- [Configuration](Configuration.md) - Configure API endpoints and integrations
- [Advanced Rules](Advanced-Rules.md) - Advanced rules API usage
- [Troubleshooting](Troubleshooting.md) - API troubleshooting
