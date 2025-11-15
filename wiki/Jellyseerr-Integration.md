# Jellyseerr Integration

This guide covers integrating OxiCleanarr with **Jellyseerr** to enable user-based advanced retention rules and request tracking.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [What is Jellyseerr?](#what-is-jellyseerr)
- [Installation](#installation)
- [Generating API Key](#generating-api-key)
- [Configuration](#configuration)
- [User-Based Rules](#user-based-rules)
- [Request Tracking](#request-tracking)
- [Testing Connection](#testing-connection)
- [Troubleshooting](#troubleshooting)
- [API Operations](#api-operations)

---

## Overview

Jellyseerr integration enables **user-based retention rules** where media can be automatically managed based on who requested it. This is particularly useful for multi-user environments where different users have different retention policies.

**Key Features:**

- **User identification** - Match Jellyseerr users to Jellyfin users
- **Request tracking** - Know who requested each piece of media
- **Per-user retention** - Apply different retention policies per user
- **Watch requirement** - Optionally only delete if user watched the content

**Related Pages:**
- [Advanced Rules](Advanced-Rules.md) - User-based rule configuration
- [Configuration](Configuration.md) - General configuration setup
- [Architecture](Architecture.md) - System design overview

---

## Prerequisites

Before configuring Jellyseerr integration:

1. **Jellyseerr installed**
   - Jellyseerr version 1.0+ recommended
   - Connected to your Jellyfin server

2. **Network connectivity**
   - OxiCleanarr can reach Jellyseerr URL
   - If using Docker: containers on same network

3. **Jellyfin authentication configured**
   - Jellyseerr must be connected to your Jellyfin server
   - Users must be synced between Jellyfin and Jellyseerr

4. **Administrative access**
   - Access to Jellyseerr web UI to generate API keys

---

## What is Jellyseerr?

**Jellyseerr** is a request management and media discovery platform for Jellyfin users. It allows users to:

- Browse and discover movies/TV shows
- Request new content for download
- Track request status (pending, approved, available)
- Manage user permissions and quotas

**OxiCleanarr Integration Use Case:**

When users request media through Jellyseerr, that request is tracked with user information (user ID, username, email). OxiCleanarr can use this data to apply **per-user retention policies**.

**Example Scenario:**

- User "Alice" has a 7-day retention policy
- User "Bob" has a 30-day retention policy
- When Alice requests "Inception", it's deleted 7 days after she watches it
- When Bob requests "The Matrix", it's kept for 30 days after he watches it

---

## Installation

### Installing Jellyseerr (Docker)

**Recommended Method:**

```bash
docker run -d \
  --name jellyseerr \
  -e TZ=America/New_York \
  -p 5055:5055 \
  -v /path/to/config:/app/config \
  fallenbagel/jellyseerr:latest
```

**Docker Compose:**

```yaml
version: '3.8'
services:
  jellyseerr:
    image: fallenbagel/jellyseerr:latest
    container_name: jellyseerr
    environment:
      - TZ=America/New_York
      - LOG_LEVEL=info
    ports:
      - "5055:5055"
    volumes:
      - ./jellyseerr/config:/app/config
    restart: unless-stopped
```

### First-Time Setup

1. **Access Jellyseerr:** Open `http://localhost:5055` in browser

2. **Connect to Jellyfin:**
   - Enter Jellyfin server URL (e.g., `http://jellyfin:8096`)
   - Enter Jellyfin admin username and password
   - Click **"Test"** then **"Continue"**

3. **Sync Libraries:**
   - Select which Jellyfin libraries to make available for requests
   - Click **"Continue"**

4. **Configure Radarr/Sonarr (Optional but Recommended):**
   - Add Radarr for movie requests
   - Add Sonarr for TV show requests
   - These are used for automatic downloading of requested media

5. **Finish Setup:**
   - Access Jellyseerr with Jellyfin credentials

**Related Page:**
- [Docker Deployment](Docker-Deployment.md) - Full stack Docker Compose setup

---

## Generating API Key

### Method 1: Via Web UI (Recommended)

1. **Login to Jellyseerr:**
   - Navigate to `http://localhost:5055`
   - Login with admin account

2. **Access Settings:**
   - Click **user icon** (top-right) → **Settings**
   - Navigate to **Settings** → **General**

3. **Locate API Key:**
   - Scroll to **"API Key"** section
   - Copy the API key (base64-encoded string, ~40-60 characters)

4. **Generate New Key (if needed):**
   - Click **"Regenerate API Key"**
   - Confirm regeneration
   - **Warning:** Invalidates previous key for all clients

### Method 2: Via Configuration File

If web UI access is unavailable:

```bash
# Locate Jellyseerr config file
cat /path/to/jellyseerr/config/settings.json

# Look for "apiKey" field
grep -i "apiKey" /path/to/jellyseerr/config/settings.json
```

**Example settings.json:**

```json
{
  "main": {
    "apiKey": "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXo="
  }
}
```

---

## Configuration

Add Jellyseerr integration to `config/config.yaml`:

```yaml
integrations:
  jellyseerr:
    enabled: true
    url: "http://localhost:5055"
    api_key: "YOUR_JELLYSEERR_API_KEY_HERE"
    timeout: "30s"  # Optional: API request timeout
```

**Docker Example:**

If Jellyseerr runs in Docker with internal networking:

```yaml
integrations:
  jellyseerr:
    enabled: true
    url: "http://jellyseerr:5055"  # Use container name
    api_key: "YOUR_JELLYSEERR_API_KEY_HERE"
    timeout: "60s"  # Increase for slow networks
```

**Configuration Options:**

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `enabled` | boolean | Yes | `false` | Enable Jellyseerr integration |
| `url` | string | Yes | - | Jellyseerr base URL (include `http://` or `https://`) |
| `api_key` | string | Yes | - | Jellyseerr API key from Settings → General |
| `timeout` | string | No | `30s` | API request timeout (duration format: `10s`, `1m`, etc.) |

---

## User-Based Rules

User-based retention rules allow different policies per user based on who requested the media.

### Configuration Example

```yaml
advanced_rules:
  - name: "Alice - Short Retention"
    type: "user"
    enabled: true
    users:
      - username: "alice"          # Jellyseerr username (case-insensitive)
        retention: "7d"             # 7 days after watch
        require_watched: true       # Only delete if watched
      
  - name: "Bob - Long Retention"
    type: "user"
    enabled: true
    users:
      - email: "bob@example.com"   # Can use email instead
        retention: "30d"            # 30 days after watch
        require_watched: true
      
  - name: "Power Users"
    type: "user"
    enabled: true
    users:
      - user_id: 42                # Most reliable: Jellyseerr user ID
        retention: "90d"
        require_watched: false     # Delete even if not watched
```

### User Matching Options

You can identify users using **any ONE** of the following (all are case-insensitive):

| Field | Type | Description | Example | Reliability |
|-------|------|-------------|---------|-------------|
| `user_id` | integer | Jellyseerr user ID | `42` | **Most reliable** (immutable) |
| `username` | string | Jellyseerr/Jellyfin username | `"alice"` | Good (can change) |
| `email` | string | User email address | `"bob@example.com"` | Good (can change) |

**Best Practice:** Use `user_id` for production environments (immutable and guaranteed unique).

### Finding User IDs

**Method 1: Jellyseerr Web UI**

1. Go to **Settings** → **Users**
2. Click on a user
3. Check browser URL: `http://localhost:5055/users/[USER_ID]`

**Method 2: API Request**

```bash
# Get all users
curl -X GET "http://localhost:5055/api/v1/user" \
  -H "X-Api-Key: YOUR_API_KEY"

# Response shows user IDs, usernames, emails
```

**Example Response:**

```json
{
  "results": [
    {
      "id": 1,
      "email": "admin@example.com",
      "username": "",
      "jellyfinUsername": "admin",
      "displayName": "Administrator"
    },
    {
      "id": 42,
      "email": "alice@example.com",
      "username": "",
      "jellyfinUsername": "alice",
      "displayName": "Alice"
    }
  ]
}
```

### User Rule Behavior

**How OxiCleanarr Matches Requests:**

1. **Fetch all requests** from Jellyseerr (with pagination)
2. **Filter by status** - Only "available" requests (status = 5)
3. **Extract user info** - User ID, username (Jellyfin username), email
4. **Match to rules** - Compare against `user_id`, `username`, or `email` in config
5. **Apply retention** - Calculate deletion date based on user's retention period

**Implementation Details:**

- User matching is **case-insensitive** for `username` and `email` - `types.go:174-180`
- Multiple identifiers can be provided (redundancy), only one needs to match
- Jellyseerr uses `jellyfinUsername` field for actual Jellyfin username - `types.go:178`

**Related Page:**
- [Advanced Rules](Advanced-Rules.md) - Complete guide to advanced retention rules

---

## Request Tracking

OxiCleanarr tracks media requests to determine who requested what content.

### Request Lifecycle

1. **User requests media** via Jellyseerr (e.g., "Inception")
2. **Jellyseerr creates request** with user info and media IDs (TMDB/TVDB)
3. **Radarr/Sonarr downloads** media automatically (if configured)
4. **Request status updates** to "available" when download completes
5. **OxiCleanarr syncs requests** periodically (15-minute cache)
6. **Retention rules applied** based on requesting user's policy

### Request Data Structure

OxiCleanarr fetches the following from Jellyseerr:

```json
{
  "id": 123,
  "type": "movie",
  "status": 5,
  "media": {
    "tmdbId": 27205,
    "tvdbId": 0
  },
  "requestedBy": {
    "id": 42,
    "email": "alice@example.com",
    "username": "",
    "jellyfinUsername": "alice",
    "displayName": "Alice"
  },
  "createdAt": "2025-01-15T10:00:00Z"
}
```

**Status Codes:**

| Status | Meaning | OxiCleanarr Action |
|--------|---------|-------------------|
| 1 | Pending approval | Ignored |
| 2 | Approved | Ignored (not yet downloaded) |
| 3 | Declined | Ignored |
| 4 | Downloading | Ignored |
| **5** | **Available** | **Used for retention rules** |

### Caching

OxiCleanarr caches Jellyseerr requests to reduce API load:

- **Cache TTL:** 15 minutes - `architecture.md:246`
- **Cache key:** `jellyseerr:requests`
- **Full refresh:** On every full sync job

---

## Testing Connection

### Using OxiCleanarr API

```bash
# Start OxiCleanarr
./oxicleanarr

# Check health endpoint (includes Jellyseerr connectivity)
curl -X GET http://localhost:8080/api/health
```

**Expected Response:**

```json
{
  "status": "healthy",
  "timestamp": "2025-01-15T10:00:00Z",
  "services": {
    "jellyseerr": "up",
    "jellyfin": "up",
    "radarr": "up"
  }
}
```

### Using curl Directly

```bash
# Test Jellyseerr API connectivity
curl -X GET "http://localhost:5055/api/v1/status" \
  -H "X-Api-Key: YOUR_JELLYSEERR_API_KEY"
```

**Expected Response:**

```json
{
  "version": "1.7.0",
  "updateAvailable": false
}
```

### Testing User Matching

```bash
# Get all requests from Jellyseerr
curl -X GET "http://localhost:5055/api/v1/request?take=50&skip=0" \
  -H "X-Api-Key: YOUR_API_KEY"

# Verify users in response match your config
```

---

## Troubleshooting

### Connection Failures

**Symptom:** `Error: making request: dial tcp: connection refused`

**Cause:** OxiCleanarr cannot reach Jellyseerr URL.

**Solutions:**

1. **Verify URL is correct:**
   ```bash
   curl http://localhost:5055/api/v1/status
   ```

2. **Check Docker networking:**
   ```bash
   docker network inspect oxicleanarr_network
   # Ensure both containers are on same network
   ```

3. **Check Jellyseerr is running:**
   ```bash
   docker ps | grep jellyseerr
   # OR
   systemctl status jellyseerr
   ```

### Authentication Failures

**Symptom:** `Error: unexpected status code: 401`

**Cause:** Invalid or missing API key.

**Solutions:**

1. **Verify API key in config.yaml:**
   ```bash
   grep -A 3 "jellyseerr:" config/config.yaml
   ```

2. **Regenerate API key** in Jellyseerr Settings → General

3. **Check for whitespace/newlines:**
   ```bash
   # API key should be base64-encoded string (no spaces)
   echo "$API_KEY" | wc -c
   ```

### User Matching Failures

**Symptom:** User-based rules not applying; media not being deleted.

**Cause:** Username/email in config doesn't match Jellyseerr user data.

**Solutions:**

1. **Check Jellyseerr username field:**
   ```bash
   # Fetch users from Jellyseerr
   curl -X GET "http://localhost:5055/api/v1/user" \
     -H "X-Api-Key: YOUR_KEY" | jq '.results[] | {id, email, jellyfinUsername, displayName}'
   ```

2. **Use `jellyfinUsername` field** (not `username` which is often empty)

3. **Verify case-insensitive matching:**
   ```yaml
   users:
     - username: "Alice"  # Matches "alice", "ALICE", "Alice"
   ```

4. **Use `user_id` instead** (most reliable):
   ```yaml
   users:
     - user_id: 42  # Always matches user ID 42
   ```

### Empty Requests

**Symptom:** OxiCleanarr reports 0 requests from Jellyseerr.

**Cause:** No requests with status "available" (5).

**Solutions:**

1. **Verify requests exist in Jellyseerr UI:**
   - Go to **Requests** → Check for "Available" status

2. **Test API directly:**
   ```bash
   curl -X GET "http://localhost:5055/api/v1/request" \
     -H "X-Api-Key: YOUR_KEY" | jq '.results[] | {id, status, type}'
   ```

3. **Check request status codes:**
   - Only status `5` (available) is used by OxiCleanarr
   - Pending/approved requests are ignored

**Related Page:**
- [Troubleshooting](Troubleshooting.md) - General troubleshooting guide

---

## API Operations

OxiCleanarr performs the following operations via Jellyseerr API:

### Jellyseerr API Endpoints

| Endpoint | Method | Purpose | Implementation |
|----------|--------|---------|----------------|
| `/api/v1/status` | GET | Health check / connectivity test | `jellyseerr.go:102` |
| `/api/v1/request` | GET | Fetch all requests (paginated) | `jellyseerr.go:39` |
| `/api/v1/user` | GET | Fetch all users (not yet used) | Not implemented |

### API Request Format

All requests include:

- **Header:** `X-Api-Key: YOUR_API_KEY`
- **Header:** `Accept: application/json` (for GET requests)
- **Timeout:** Configurable via `timeout` field (default: 30s)

### Pagination Handling

Jellyseerr uses **cursor-based pagination**:

```bash
# Page 1 (first 50 results)
GET /api/v1/request?take=50&skip=0

# Page 2 (next 50 results)
GET /api/v1/request?take=50&skip=50

# Continue until pages < current page
```

**Implementation:** `jellyseerr.go:39-99`

OxiCleanarr automatically fetches **all pages** and aggregates results.

### Example Request

```bash
curl -X GET "http://localhost:5055/api/v1/request?take=50&skip=0" \
  -H "X-Api-Key: YOUR_API_KEY" \
  -H "Accept: application/json"
```

**Example Response:**

```json
{
  "pageInfo": {
    "pages": 3,
    "pageSize": 50,
    "results": 123,
    "page": 1
  },
  "results": [
    {
      "id": 456,
      "type": "movie",
      "status": 5,
      "media": {
        "tmdbId": 27205,
        "tvdbId": 0
      },
      "requestedBy": {
        "id": 42,
        "email": "alice@example.com",
        "username": "",
        "jellyfinUsername": "alice",
        "displayName": "Alice"
      },
      "createdAt": "2025-01-15T10:00:00.000Z"
    }
  ]
}
```

**Related Page:**
- [API Reference](API-Reference.md) - OxiCleanarr API documentation

---

## Best Practices

1. **Use `user_id` for production** - More reliable than username/email
2. **Enable `require_watched`** - Prevents deleting unwatched content
3. **Test rules with dry-run mode** - Verify behavior before enabling deletion
4. **Monitor request sync** - Check logs for Jellyseerr sync errors
5. **Keep API key secure** - Store in environment variables or secret manager

---

## Next Steps

- **[Advanced Rules](Advanced-Rules.md)** - Configure user-based retention policies
- **[Jellystat Integration](Jellystat-Integration.md)** - Set up watch history tracking
- **[Configuration](Configuration.md)** - General configuration options
- **[Troubleshooting](Troubleshooting.md)** - Resolve common issues
