# Jellystat Integration

This guide covers integrating OxiCleanarr with **Jellystat** to track watch history and enable watched-based retention rules.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [What is Jellystat?](#what-is-jellystat)
- [Installation](#installation)
- [Generating API Key](#generating-api-key)
- [Configuration](#configuration)
- [Watch History Tracking](#watch-history-tracking)
- [Watched-Based Rules](#watched-based-rules)
- [Testing Connection](#testing-connection)
- [Troubleshooting](#troubleshooting)
- [API Operations](#api-operations)

---

## Overview

Jellystat integration enables **watch history tracking** to determine if media has been watched by users. This is critical for retention rules that only delete media after it has been watched.

**Key Features:**

- **Watch history tracking** - Know when each user watched each piece of media
- **Completion detection** - Track partial vs. complete watches
- **User-specific watches** - Different users' watch history tracked separately
- **Watch-based deletion** - Only delete media after it's been watched

**Related Pages:**
- [Advanced Rules](Advanced-Rules.md) - Watched-based rule configuration
- [Configuration](Configuration.md) - General configuration setup
- [Architecture](Architecture.md) - System design overview

---

## Prerequisites

Before configuring Jellystat integration:

1. **Jellystat installed**
   - Version 1.0+ recommended
   - Connected to your Jellyfin server

2. **Network connectivity**
   - OxiCleanarr can reach Jellystat URL
   - If using Docker: containers on same network

3. **Jellyfin playback data**
   - Jellystat must have collected watch history from Jellyfin
   - Requires time for initial data collection (hours to days)

4. **Administrative access**
   - Access to Jellystat web UI to generate API keys

---

## What is Jellystat?

**Jellystat** is an analytics and statistics platform for Jellyfin. It provides:

- **Watch history tracking** - Who watched what, when, and for how long
- **User analytics** - Playback statistics per user
- **Library insights** - Most popular content, play counts, etc.
- **Dashboards** - Visual analytics for media consumption

**OxiCleanarr Integration Use Case:**

OxiCleanarr uses Jellystat's watch history to determine if media has been watched before applying retention rules. This prevents deletion of unwatched content.

**Example Scenario:**

- Rule: "Delete movies 7 days after watched"
- User watches "Inception" on January 1st
- Jellystat records the watch event
- OxiCleanarr schedules deletion for January 8th
- If unwatched, media is never deleted (unless another rule applies)

---

## Installation

### Installing Jellystat (Docker)

**Recommended Method:**

```bash
docker run -d \
  --name jellystat \
  -e TZ=America/New_York \
  -e JWT_SECRET="your-secret-here" \
  -p 3000:3000 \
  -v /path/to/data:/app/backend/backup-data \
  cyfershepard/jellystat:latest
```

**Docker Compose:**

```yaml
version: '3.8'
services:
  jellystat:
    image: cyfershepard/jellystat:latest
    container_name: jellystat
    environment:
      - TZ=America/New_York
      - JWT_SECRET=your-random-secret-key-here
    ports:
      - "3000:3000"
    volumes:
      - ./jellystat/data:/app/backend/backup-data
    restart: unless-stopped
```

### First-Time Setup

1. **Access Jellystat:** Open `http://localhost:3000` in browser

2. **Create Admin Account:**
   - Enter admin username and password
   - Click **"Create Account"**

3. **Connect to Jellyfin:**
   - Go to **Settings** → **Jellyfin Servers**
   - Click **"Add Server"**
   - Enter Jellyfin URL (e.g., `http://jellyfin:8096`)
   - Enter Jellyfin admin API key
   - Click **"Test Connection"** → **"Save"**

4. **Enable Data Collection:**
   - Go to **Settings** → **General**
   - Enable **"Activity Tracking"**
   - Set collection interval (recommended: 5 minutes)

5. **Wait for Data Collection:**
   - Jellystat needs time to collect watch history from Jellyfin
   - Initial collection can take hours depending on library size
   - Check **Dashboard** for activity data

**Related Page:**
- [Docker Deployment](Docker-Deployment.md) - Full stack Docker Compose setup

---

## Generating API Key

### Method 1: Via Web UI (Recommended)

1. **Login to Jellystat:**
   - Navigate to `http://localhost:3000`
   - Login with admin account

2. **Access API Settings:**
   - Click **user icon** (top-right) → **Settings**
   - Navigate to **API** or **API Keys** section

3. **Create API Key:**
   - Click **"Create API Key"**
   - Enter description (e.g., "OxiCleanarr")
   - Click **"Generate"**
   - Copy the generated key (alphanumeric string)

4. **Store Securely:**
   - Save to password manager or config file
   - **Warning:** Key is only shown once

### Method 2: Via Database (Advanced)

If web UI access is unavailable:

```bash
# Access Jellystat SQLite database
sqlite3 /path/to/jellystat/data/jellystat.db

# Query API keys
SELECT * FROM api_keys;

# Exit
.quit
```

**Note:** Database structure varies by Jellystat version. Check documentation.

---

## Configuration

Add Jellystat integration to `config/config.yaml`:

```yaml
integrations:
  jellystat:
    enabled: true
    url: "http://localhost:3000"
    api_key: "YOUR_JELLYSTAT_API_KEY_HERE"
    timeout: "30s"  # Optional: API request timeout
```

**Docker Example:**

If Jellystat runs in Docker with internal networking:

```yaml
integrations:
  jellystat:
    enabled: true
    url: "http://jellystat:3000"  # Use container name
    api_key: "YOUR_JELLYSTAT_API_KEY_HERE"
    timeout: "60s"  # Increase for slow networks or large history
```

**Configuration Options:**

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `enabled` | boolean | Yes | `false` | Enable Jellystat integration |
| `url` | string | Yes | - | Jellystat base URL (include `http://` or `https://`) |
| `api_key` | string | Yes | - | Jellystat API key from Settings → API |
| `timeout` | string | No | `30s` | API request timeout (duration format: `10s`, `1m`, etc.) |

---

## Watch History Tracking

Jellystat tracks detailed watch history for all Jellyfin users.

### Watch History Data Structure

OxiCleanarr fetches the following watch data from Jellystat:

```json
{
  "Id": "abc123",
  "UserId": "user-guid-here",
  "UserName": "alice",
  "NowPlayingItemId": "item-guid-here",
  "NowPlayingItemName": "Inception",
  "SeriesName": null,
  "EpisodeId": null,
  "SeasonId": null,
  "PlaybackDuration": 8880,
  "ActivityDateInserted": "2025-01-15T10:30:00Z"
}
```

**Key Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `NowPlayingItemId` | string | Jellyfin item GUID (matches Jellyfin media ID) |
| `NowPlayingItemName` | string | Movie/episode title |
| `SeriesName` | string | TV series name (null for movies) |
| `UserId` | string | Jellyfin user GUID |
| `UserName` | string | Jellyfin username |
| `PlaybackDuration` | int | Watch time in seconds |
| `ActivityDateInserted` | timestamp | Last watch timestamp |

### Watch Detection Logic

**OxiCleanarr determines "watched" status:**

1. **Fetch watch history** from Jellystat (all users, paginated)
2. **Match media IDs** - Compare Jellystat `NowPlayingItemId` with Jellyfin item ID
3. **Check playback duration** - Verify sufficient watch time (>80% recommended by Jellyfin)
4. **Track per user** - Different users' watches tracked separately
5. **Use latest watch date** - Most recent watch timestamp used for retention calculation

**Implementation Details:**

- Watch history cached for 5 minutes - `architecture.md:247`
- Pagination automatically handled - `jellystat.go:39-99`
- All users' history aggregated

### Partial vs. Complete Watches

**Jellyfin's "Watched" Criteria:**

Jellyfin marks content as "watched" when:
- User watches **≥80%** of total runtime (configurable in Jellyfin)
- User manually marks as watched

**OxiCleanarr Behavior:**

OxiCleanarr relies on **Jellyfin's "Played" flag** (via Jellyfin API) rather than calculating completion percentage from Jellystat data. This ensures consistency with Jellyfin's UI.

---

## Watched-Based Rules

Watched-based retention rules only delete media **after** it has been watched.

### Simple Watched Rules

```yaml
rules:
  movie_retention: "7d"   # Delete movies 7 days after watched
  tv_retention: "14d"     # Delete TV shows 14 days after watched
```

**Behavior:**

- If media is watched → deletion scheduled for 7/14 days later
- If media is unwatched → never deleted (unless other rules apply)

### Advanced Watched Rules

```yaml
advanced_rules:
  - name: "Quick Cleanup - Watched Only"
    type: "tag"
    enabled: true
    tag: "quick-watch"
    retention: "3d"
    require_watched: true  # Only delete if watched

  - name: "User-Based Watched Rule"
    type: "user"
    enabled: true
    users:
      - username: "alice"
        retention: "7d"
        require_watched: true  # Alice's media deleted only after she watches
```

**`require_watched` Behavior:**

| Value | Behavior |
|-------|----------|
| `true` | Delete only if media has been watched by **any user** |
| `false` | Delete regardless of watch status (time-based only) |
| Omitted | Same as `false` (delete regardless of watch status) |

### Episode-Based Rules (TV Shows)

For TV series, watched tracking works at the **series level**:

```yaml
advanced_rules:
  - name: "TV Episode Cleanup"
    type: "episode"
    enabled: true
    max_episodes: 5         # Keep max 5 episodes per series
    max_age: "30d"          # Delete episodes older than 30 days
    require_watched: true   # Only delete watched episodes
```

**Behavior:**

- Keeps 5 most recent episodes per series
- Older episodes deleted **only if watched**
- Unwatched episodes always kept (unless exceeding `max_episodes`)

**Related Page:**
- [Advanced Rules](Advanced-Rules.md) - Complete guide to watched-based rules

---

## Testing Connection

### Using OxiCleanarr API

```bash
# Start OxiCleanarr
./oxicleanarr

# Check health endpoint (includes Jellystat connectivity)
curl -X GET http://localhost:8080/api/health
```

**Expected Response:**

```json
{
  "status": "healthy",
  "timestamp": "2025-01-15T10:00:00Z",
  "services": {
    "jellystat": "up",
    "jellyfin": "up",
    "radarr": "up"
  }
}
```

### Using curl Directly

```bash
# Test Jellystat API connectivity
curl -X GET "http://localhost:3000/api/getLibraries" \
  -H "x-api-token: YOUR_JELLYSTAT_API_KEY"
```

**Expected Response:**

```json
[
  {
    "Id": "lib-guid-here",
    "Name": "Movies",
    "ItemCount": 150
  }
]
```

### Testing Watch History

```bash
# Fetch watch history (first page)
curl -X GET "http://localhost:3000/api/getHistory?page=1&size=10" \
  -H "x-api-token: YOUR_API_KEY"

# Verify response contains watch events
```

**Expected Response:**

```json
{
  "current_page": 1,
  "pages": 50,
  "size": 10,
  "results": [
    {
      "Id": "event-id",
      "UserId": "user-guid",
      "UserName": "alice",
      "NowPlayingItemId": "item-guid",
      "NowPlayingItemName": "Inception",
      "PlaybackDuration": 8880,
      "ActivityDateInserted": "2025-01-15T10:30:00Z"
    }
  ]
}
```

---

## Troubleshooting

### Connection Failures

**Symptom:** `Error: making request: dial tcp: connection refused`

**Cause:** OxiCleanarr cannot reach Jellystat URL.

**Solutions:**

1. **Verify URL is correct:**
   ```bash
   curl http://localhost:3000/api/getLibraries
   ```

2. **Check Docker networking:**
   ```bash
   docker network inspect oxicleanarr_network
   # Ensure both containers are on same network
   ```

3. **Check Jellystat is running:**
   ```bash
   docker ps | grep jellystat
   # OR
   systemctl status jellystat
   ```

### Authentication Failures

**Symptom:** `Error: unexpected status code: 401`

**Cause:** Invalid or missing API key.

**Solutions:**

1. **Verify API key in config.yaml:**
   ```bash
   grep -A 3 "jellystat:" config/config.yaml
   ```

2. **Regenerate API key** in Jellystat Settings → API

3. **Check header name:**
   - Jellystat uses `x-api-token` header (lowercase, hyphenated)
   - Different from Jellyfin's `X-Emby-Token` or Radarr's `X-Api-Key`

### Empty Watch History

**Symptom:** OxiCleanarr reports 0 watch events from Jellystat.

**Cause:** Jellystat hasn't collected watch history yet, or activity tracking disabled.

**Solutions:**

1. **Wait for data collection:**
   - Jellystat collects data periodically (default: 5 minutes)
   - Initial collection can take time

2. **Check Jellystat dashboard:**
   - Open `http://localhost:3000`
   - Navigate to **Dashboard** → Verify activity data exists

3. **Verify activity tracking enabled:**
   - Go to **Settings** → **General**
   - Enable **"Activity Tracking"**

4. **Check Jellyfin connection:**
   - Go to **Settings** → **Jellyfin Servers**
   - Test connection to Jellyfin server

### Watch Status Mismatch

**Symptom:** Jellyfin shows media as watched, but OxiCleanarr treats it as unwatched.

**Cause:** Jellystat data not synced with Jellyfin, or using different watch criteria.

**Solutions:**

1. **Force Jellystat sync:**
   - Jellystat Settings → **Sync Now** button

2. **Check Jellyfin playback threshold:**
   - Jellyfin Settings → Playback → "Mark played at"
   - Default: 90% (media marked watched at 90% completion)

3. **Wait for cache expiry:**
   - OxiCleanarr caches watch history for 5 minutes
   - Wait 5 minutes after watch event

4. **Verify media IDs match:**
   ```bash
   # Get Jellyfin item ID
   curl -X GET "http://jellyfin:8096/Users/{userId}/Items/{itemId}" \
     -H "X-Emby-Token: YOUR_KEY"
   
   # Check if ID appears in Jellystat history
   curl -X GET "http://jellystat:3000/api/getHistory" \
     -H "x-api-token: YOUR_KEY" | grep "item-guid"
   ```

**Related Page:**
- [Troubleshooting](Troubleshooting.md) - General troubleshooting guide

---

## API Operations

OxiCleanarr performs the following operations via Jellystat API:

### Jellystat API Endpoints

| Endpoint | Method | Purpose | Implementation |
|----------|--------|---------|----------------|
| `/api/getLibraries` | GET | Health check / connectivity test | `jellystat.go:103` |
| `/api/getHistory` | GET | Fetch watch history (paginated) | `jellystat.go:39` |

### API Request Format

All requests include:

- **Header:** `x-api-token: YOUR_API_KEY` (lowercase, hyphenated)
- **Header:** `Accept: application/json` (for GET requests)
- **Timeout:** Configurable via `timeout` field (default: 30s)

### Pagination Handling

Jellystat uses **page-based pagination**:

```bash
# Page 1 (first 100 results)
GET /api/getHistory?page=1&size=100

# Page 2 (next 100 results)
GET /api/getHistory?page=2&size=100

# Continue until current_page >= pages
```

**Implementation:** `jellystat.go:39-99`

OxiCleanarr automatically fetches **all pages** (100 results per page) and aggregates watch history.

### Example Request

```bash
curl -X GET "http://localhost:3000/api/getHistory?page=1&size=100" \
  -H "x-api-token: YOUR_API_KEY" \
  -H "Accept: application/json"
```

**Example Response:**

```json
{
  "current_page": 1,
  "pages": 10,
  "size": 100,
  "results": [
    {
      "Id": "event-123",
      "UserId": "abc-123-def-456",
      "UserName": "alice",
      "NowPlayingItemId": "item-789-ghi-012",
      "NowPlayingItemName": "Inception",
      "SeriesName": null,
      "EpisodeId": null,
      "SeasonId": null,
      "PlaybackDuration": 8880,
      "ActivityDateInserted": "2025-01-15T10:30:00.000Z"
    }
  ]
}
```

**Related Page:**
- [API Reference](API-Reference.md) - OxiCleanarr API documentation

---

## Performance Considerations

### Large Watch Histories

Jellystat can accumulate **millions of watch events** over time. OxiCleanarr optimizes performance:

1. **Pagination:** Fetches 100 events per page (configurable in code)
2. **Caching:** Watch history cached for 5 minutes (reduces API load)
3. **Incremental sync:** Only full sync fetches entire history

**Monitoring:**

```bash
# Check Jellystat response time
time curl -X GET "http://localhost:3000/api/getHistory?page=1&size=100" \
  -H "x-api-token: YOUR_KEY"

# Check OxiCleanarr logs for slow queries
docker logs oxicleanarr 2>&1 | grep -i "jellystat"
```

### Database Size

Jellystat stores watch history in SQLite database:

```bash
# Check Jellystat database size
du -sh /path/to/jellystat/data/jellystat.db

# Vacuum database to reclaim space (run periodically)
sqlite3 /path/to/jellystat/data/jellystat.db "VACUUM;"
```

**Recommendation:** Purge old watch history in Jellystat settings if database grows too large.

---

## Best Practices

1. **Enable activity tracking immediately** - Start collecting watch history ASAP
2. **Use `require_watched: true`** - Prevent accidental deletion of unwatched content
3. **Monitor Jellystat sync** - Ensure data is up-to-date with Jellyfin
4. **Test with dry-run mode** - Verify watch detection works before enabling deletion
5. **Set appropriate cache TTL** - Balance freshness vs. API load (default 5 minutes is good)

---

## Data Privacy Considerations

Jellystat tracks **detailed user activity**:

- Who watched what, when, and for how long
- Playback history for all users
- User preferences and viewing patterns

**Recommendations:**

1. **Inform users** - Disclose that watch history is being tracked
2. **Secure API keys** - Protect Jellystat API access
3. **Limit retention** - Consider purging old watch history in Jellystat
4. **GDPR compliance** - Allow users to request data deletion if required

**Related Page:**
- [Security](Security.md) - API key security and access control

---

## Alternative: Jellyfin API Watch Data

OxiCleanarr can also fetch watch status **directly from Jellyfin API** (without Jellystat):

**Jellyfin UserData API:**

```bash
# Get user's watched status for item
curl -X GET "http://jellyfin:8096/Users/{userId}/Items/{itemId}" \
  -H "X-Emby-Token: YOUR_KEY"
```

**Response includes:**

```json
{
  "UserData": {
    "PlayCount": 3,
    "LastPlayedDate": "2025-01-15T10:30:00Z",
    "Played": true
  }
}
```

**Comparison:**

| Feature | Jellyfin API | Jellystat API |
|---------|--------------|---------------|
| Watch status | ✅ `Played` flag | ✅ Watch events |
| Watch timestamp | ✅ `LastPlayedDate` | ✅ `ActivityDateInserted` |
| Watch duration | ❌ Not available | ✅ `PlaybackDuration` |
| Historical watches | ❌ Last watch only | ✅ All watches |
| API calls required | Many (per-item) | Few (bulk fetch) |

**OxiCleanarr uses both:**
- **Jellyfin API** for current watch status (`Played` flag)
- **Jellystat API** for detailed watch history (future feature)

---

## Next Steps

- **[Advanced Rules](Advanced-Rules.md)** - Configure watched-based retention policies
- **[Jellyseerr Integration](Jellyseerr-Integration.md)** - Set up user request tracking
- **[Configuration](Configuration.md)** - General configuration options
- **[Troubleshooting](Troubleshooting.md)** - Resolve common issues
