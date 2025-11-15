# Radarr/Sonarr Integration

This guide covers integrating OxiCleanarr with **Radarr** (for movies) and **Sonarr** (for TV shows) to enable automated media lifecycle management and deletion capabilities.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Radarr Setup](#radarr-setup)
  - [Generating API Key](#generating-api-key-radarr)
  - [Configuration](#configuration-radarr)
  - [Testing Connection](#testing-connection-radarr)
- [Sonarr Setup](#sonarr-setup)
  - [Generating API Key](#generating-api-key-sonarr)
  - [Configuration](#configuration-sonarr)
  - [Testing Connection](#testing-connection-sonarr)
- [Tag Management](#tag-management)
- [Path Configuration](#path-configuration)
- [Advanced Features](#advanced-features)
- [Troubleshooting](#troubleshooting)
- [API Operations](#api-operations)

---

## Overview

OxiCleanarr integrates with Radarr and Sonarr to:

- **Fetch media library information** (movies, TV shows, episodes)
- **Delete media items** from management and optionally remove files
- **Retrieve metadata** (tags, quality profiles, download history)
- **Enforce retention policies** through automated cleanup

Both integrations use the **Arr API v3** standard and communicate via HTTP REST APIs.

**Related Pages:**
- [Configuration](Configuration.md) - General configuration setup
- [Advanced Rules](Advanced-Rules.md) - Tag-based retention rules
- [Jellyfin Integration](Jellyfin-Integration.md) - Setting up Jellyfin
- [Architecture](Architecture.md) - System design overview

---

## Prerequisites

Before configuring Radarr/Sonarr integration:

1. **Install and configure Radarr/Sonarr**
   - Radarr v3+ or Sonarr v3+ (API v3 required)
   - Services must be accessible via HTTP/HTTPS

2. **Network connectivity**
   - OxiCleanarr can reach Radarr/Sonarr URL
   - If using Docker: containers on same network or host networking

3. **Administrative access**
   - Access to Radarr/Sonarr web UI to generate API keys

---

## Radarr Setup

### Generating API Key (Radarr)

1. **Access Radarr Settings:**
   - Open Radarr web UI (e.g., `http://localhost:7878`)
   - Navigate to **Settings** → **General**

2. **Show Advanced Settings:**
   - Click **Show Advanced** at the top of the page

3. **Locate API Key:**
   - Scroll to the **Security** section
   - Copy the **API Key** value (32-character alphanumeric string)

4. **Generate New Key (Optional):**
   - Click the **Regenerate** button next to API Key
   - **Warning:** Regenerating invalidates all existing API clients

### Configuration (Radarr)

Add to `config/config.yaml`:

```yaml
integrations:
  radarr:
    enabled: true
    url: "http://localhost:7878"
    api_key: "YOUR_RADARR_API_KEY_HERE"
    timeout: "30s"  # Optional: API request timeout
```

**Docker Example:**

If Radarr runs in Docker with internal networking:

```yaml
integrations:
  radarr:
    enabled: true
    url: "http://radarr:7878"  # Use container name
    api_key: "YOUR_RADARR_API_KEY_HERE"
    timeout: "60s"  # Increase for slow networks
```

**Configuration Options:**

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `enabled` | boolean | Yes | `false` | Enable Radarr integration |
| `url` | string | Yes | - | Radarr base URL (include `http://` or `https://`) |
| `api_key` | string | Yes | - | Radarr API key from Settings → General |
| `timeout` | string | No | `30s` | API request timeout (duration format: `10s`, `1m`, etc.) |

### Testing Connection (Radarr)

**Using OxiCleanarr API:**

```bash
# Start OxiCleanarr
./oxicleanarr

# Check health endpoint (includes Radarr connectivity)
curl -X GET http://localhost:8080/api/health
```

Expected response:

```json
{
  "status": "healthy",
  "timestamp": "2025-01-15T10:00:00Z",
  "services": {
    "radarr": "up",
    "jellyfin": "up"
  }
}
```

**Using curl directly:**

```bash
# Test Radarr API connectivity
curl -X GET "http://localhost:7878/api/v3/system/status" \
  -H "X-Api-Key: YOUR_RADARR_API_KEY"
```

Expected response:

```json
{
  "version": "4.3.2.6857",
  "buildTime": "2023-12-15T00:00:00Z",
  "isDebug": false,
  "isProduction": true
}
```

---

## Sonarr Setup

### Generating API Key (Sonarr)

1. **Access Sonarr Settings:**
   - Open Sonarr web UI (e.g., `http://localhost:8989`)
   - Navigate to **Settings** → **General**

2. **Show Advanced Settings:**
   - Click **Show Advanced** at the top of the page

3. **Locate API Key:**
   - Scroll to the **Security** section
   - Copy the **API Key** value (32-character alphanumeric string)

4. **Generate New Key (Optional):**
   - Click the **Regenerate** button next to API Key
   - **Warning:** Regenerating invalidates all existing API clients

### Configuration (Sonarr)

Add to `config/config.yaml`:

```yaml
integrations:
  sonarr:
    enabled: true
    url: "http://localhost:8989"
    api_key: "YOUR_SONARR_API_KEY_HERE"
    timeout: "30s"  # Optional: API request timeout
```

**Docker Example:**

```yaml
integrations:
  sonarr:
    enabled: true
    url: "http://sonarr:8989"  # Use container name
    api_key: "YOUR_SONARR_API_KEY_HERE"
    timeout: "60s"
```

**Configuration Options:**

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `enabled` | boolean | Yes | `false` | Enable Sonarr integration |
| `url` | string | Yes | - | Sonarr base URL (include `http://` or `https://`) |
| `api_key` | string | Yes | - | Sonarr API key from Settings → General |
| `timeout` | string | No | `30s` | API request timeout (duration format: `10s`, `1m`, etc.) |

### Testing Connection (Sonarr)

**Using OxiCleanarr API:**

```bash
# Check health endpoint (includes Sonarr connectivity)
curl -X GET http://localhost:8080/api/health
```

Expected response:

```json
{
  "status": "healthy",
  "timestamp": "2025-01-15T10:00:00Z",
  "services": {
    "sonarr": "up",
    "radarr": "up",
    "jellyfin": "up"
  }
}
```

**Using curl directly:**

```bash
# Test Sonarr API connectivity
curl -X GET "http://localhost:8989/api/v3/system/status" \
  -H "X-Api-Key: YOUR_SONARR_API_KEY"
```

---

## Tag Management

OxiCleanarr uses **tags** to apply retention policies to specific media items. Tags must exist in Radarr/Sonarr before being referenced in advanced rules.

### Creating Tags in Radarr

1. **Navigate to Settings:**
   - Open Radarr → **Settings** → **Indexers** (or any settings page)
   - Scroll to **Tags** section at the bottom

2. **Add New Tag:**
   - Click **+** button
   - Enter tag name (e.g., `short-retention`, `long-retention`)
   - Click **Save**

3. **Apply Tag to Movies:**
   - Go to **Movies** → Select movie → **Edit**
   - In **Tags** field, select your tag
   - Click **Save**

### Creating Tags in Sonarr

1. **Navigate to Settings:**
   - Open Sonarr → **Settings** → **Indexers** (or any settings page)
   - Scroll to **Tags** section at the bottom

2. **Add New Tag:**
   - Click **+** button
   - Enter tag name (e.g., `kids-shows`, `binge-watch`)
   - Click **Save**

3. **Apply Tag to Series:**
   - Go to **Series** → Select series → **Edit**
   - In **Tags** field, select your tag
   - Click **Save**

### Using Tags in Advanced Rules

Configure tag-based retention rules in `config/config.yaml`:

```yaml
advanced_rules:
  - name: "Short Retention Movies"
    type: "tag"
    enabled: true
    tag: "short-retention"
    retention: "7d"

  - name: "Kids TV Shows"
    type: "tag"
    enabled: true
    tag: "kids-shows"
    retention: "30d"
```

**Related Page:**
- [Advanced Rules](Advanced-Rules.md) - Complete guide to advanced retention rules

---

## Path Configuration

Proper path configuration is **critical** for OxiCleanarr to correctly identify and manage media files across Radarr, Sonarr, and Jellyfin.

### Understanding Path Mapping

OxiCleanarr needs to correlate paths between:

1. **Radarr/Sonarr** - Reports file paths where media is stored
2. **Jellyfin** - Accesses files at (potentially) different mount points
3. **OxiCleanarr Bridge Plugin** - Performs file operations on Jellyfin server

**Example Scenario:**

| Service | Path | Notes |
|---------|------|-------|
| Radarr | `/data/movies/Inception (2010)/Inception.mkv` | Radarr's root folder |
| Jellyfin | `/media/movies/Inception (2010)/Inception.mkv` | Jellyfin's library path |
| Plugin | `/media/movies/Inception (2010)/Inception.mkv` | Same as Jellyfin (runs on same server) |

In this case, paths differ between Radarr (`/data/movies`) and Jellyfin (`/media/movies`).

### Path Mapping Configuration

**Option 1: Use Consistent Paths (Recommended)**

Configure Radarr/Sonarr and Jellyfin to use **identical paths**:

- **Docker:** Mount volumes with same paths in all containers
  ```yaml
  volumes:
    - /mnt/media/movies:/data/movies  # Same path for all services
  ```

**Option 2: Path Translation (Future Feature)**

Path mapping/translation is not yet implemented. For now, ensure all services use identical paths.

**Related Pages:**
- [Jellyfin Integration](Jellyfin-Integration.md) - Jellyfin path configuration
- [Docker Deployment](Docker-Deployment.md) - Volume mounting best practices

---

## Advanced Features

### Quality Profiles

Radarr/Sonarr quality profiles are **not currently used** by OxiCleanarr but may be incorporated in future releases for:

- Retention based on quality (e.g., keep 4K longer than 1080p)
- Deletion priority (remove lower quality first)

### Download History

OxiCleanarr can fetch download history from Radarr/Sonarr to:

- Track when media was added
- Identify re-downloads or upgrades
- Audit deletion decisions

This feature is **available via API** but not yet exposed in the UI.

### Multiple Instances

OxiCleanarr **does not support multiple Radarr/Sonarr instances** in the current release. Only one instance of each can be configured.

**Workaround:**
- Run multiple OxiCleanarr instances with separate configuration files
- Use reverse proxy to consolidate access

---

## Troubleshooting

### Connection Failures

**Symptom:** `Error: making request: dial tcp: connection refused`

**Cause:** OxiCleanarr cannot reach Radarr/Sonarr URL.

**Solutions:**

1. **Verify URL is correct:**
   ```bash
   # Test connectivity
   curl http://localhost:7878/api/v3/system/status
   ```

2. **Check Docker networking:**
   ```bash
   # If using Docker, ensure containers are on same network
   docker network ls
   docker network inspect <network_name>
   ```

3. **Check firewall rules:**
   ```bash
   # Ensure port is open
   sudo ufw status
   ```

4. **Verify Radarr/Sonarr is running:**
   ```bash
   # Check service status
   systemctl status radarr
   systemctl status sonarr
   ```

### Authentication Failures

**Symptom:** `Error: unexpected status code: 401`

**Cause:** Invalid or missing API key.

**Solutions:**

1. **Verify API key in config.yaml:**
   ```bash
   grep api_key config/config.yaml
   ```

2. **Copy API key again from Radarr/Sonarr Settings → General**

3. **Check for whitespace/newlines in key:**
   ```bash
   # API key should be exactly 32 characters
   echo -n "YOUR_KEY" | wc -c
   ```

4. **Restart OxiCleanarr after changing config:**
   ```bash
   # Config changes require restart
   ./oxicleanarr
   ```

### Empty Media List

**Symptom:** OxiCleanarr reports 0 movies/series from Radarr/Sonarr.

**Cause:** Radarr/Sonarr has no media, or API returns empty response.

**Solutions:**

1. **Verify media exists in Radarr/Sonarr UI:**
   - Open Radarr/Sonarr web UI
   - Navigate to **Movies** or **Series**
   - Confirm media is listed

2. **Test API directly:**
   ```bash
   # Radarr: Fetch all movies
   curl -X GET "http://localhost:7878/api/v3/movie" \
     -H "X-Api-Key: YOUR_KEY"

   # Sonarr: Fetch all series
   curl -X GET "http://localhost:8989/api/v3/series" \
     -H "X-Api-Key: YOUR_KEY"
   ```

3. **Check OxiCleanarr logs for errors:**
   ```bash
   ./oxicleanarr 2>&1 | grep -i "radarr\|sonarr"
   ```

### Tag Not Found

**Symptom:** Advanced rules don't apply; logs show "tag not found".

**Cause:** Tag referenced in `advanced_rules` doesn't exist in Radarr/Sonarr.

**Solutions:**

1. **Create tag in Radarr/Sonarr** (see [Tag Management](#tag-management))

2. **Verify tag name matches exactly** (case-sensitive):
   ```yaml
   tag: "short-retention"  # Must match Radarr/Sonarr tag name exactly
   ```

3. **List all tags via API:**
   ```bash
   # Radarr
   curl -X GET "http://localhost:7878/api/v3/tag" \
     -H "X-Api-Key: YOUR_KEY"

   # Sonarr
   curl -X GET "http://localhost:8989/api/v3/tag" \
     -H "X-Api-Key: YOUR_KEY"
   ```

### Deletion Failures

**Symptom:** `Error: unexpected status code: 404` when deleting media.

**Cause:** Movie/series ID doesn't exist in Radarr/Sonarr.

**Solutions:**

1. **Media may have been manually deleted from Radarr/Sonarr**

2. **Check if media still exists:**
   ```bash
   # Radarr: Get movie by ID
   curl -X GET "http://localhost:7878/api/v3/movie/123" \
     -H "X-Api-Key: YOUR_KEY"

   # Sonarr: Get series by ID
   curl -X GET "http://localhost:8989/api/v3/series/456" \
     -H "X-Api-Key: YOUR_KEY"
   ```

3. **Run full sync to refresh OxiCleanarr's cache:**
   ```bash
   curl -X POST http://localhost:8080/api/sync/full
   ```

**Related Page:**
- [Troubleshooting](Troubleshooting.md) - General troubleshooting guide

---

## API Operations

OxiCleanarr performs the following operations via Radarr/Sonarr APIs:

### Radarr API Endpoints

| Endpoint | Method | Purpose | Implementation |
|----------|--------|---------|----------------|
| `/api/v3/system/status` | GET | Health check / connectivity test | `radarr.go:187` |
| `/api/v3/movie` | GET | Fetch all movies | `radarr.go:39` |
| `/api/v3/movie/{id}` | GET | Fetch single movie by ID | `radarr.go:71` |
| `/api/v3/movie/{id}` | DELETE | Delete movie and optionally files | `radarr.go:101` |
| `/api/v3/history/movie` | GET | Fetch download history for movie | `radarr.go:126` |
| `/api/v3/tag` | GET | Fetch all tags | `radarr.go:156` |

### Sonarr API Endpoints

| Endpoint | Method | Purpose | Implementation |
|----------|--------|---------|----------------|
| `/api/v3/system/status` | GET | Health check / connectivity test | `sonarr.go:187` |
| `/api/v3/series` | GET | Fetch all TV series | `sonarr.go:39` |
| `/api/v3/series/{id}` | GET | Fetch single series by ID | `sonarr.go:71` |
| `/api/v3/series/{id}` | DELETE | Delete series and optionally files | `sonarr.go:101` |
| `/api/v3/episode` | GET | Fetch episodes for series | `sonarr.go:126` |
| `/api/v3/tag` | GET | Fetch all tags | `sonarr.go:156` |

### API Request Format

All requests include:

- **Header:** `X-Api-Key: YOUR_API_KEY`
- **Header:** `Accept: application/json` (for GET requests)
- **Timeout:** Configurable via `timeout` field (default: 30s)

**Example Request:**

```bash
curl -X GET "http://localhost:7878/api/v3/movie" \
  -H "X-Api-Key: YOUR_API_KEY" \
  -H "Accept: application/json"
```

**Example Response (Radarr Movies):**

```json
[
  {
    "id": 1,
    "title": "Inception",
    "year": 2010,
    "path": "/data/movies/Inception (2010)",
    "monitored": true,
    "hasFile": true,
    "tags": [1, 2]
  }
]
```

**Related Page:**
- [API Reference](API-Reference.md) - OxiCleanarr API documentation

---

## Next Steps

- **[Advanced Rules](Advanced-Rules.md)** - Configure tag-based retention policies
- **[Jellyfin Integration](Jellyfin-Integration.md)** - Set up Jellyfin and symlink libraries
- **[Deletion Timeline](Deletion-Timeline.md)** - Understand the deletion lifecycle
- **[Troubleshooting](Troubleshooting.md)** - Resolve common issues
