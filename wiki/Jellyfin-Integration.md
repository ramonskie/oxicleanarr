# Jellyfin Integration

Complete guide for integrating OxiCleanarr with Jellyfin.

**Related pages:**
- [Installation Guide](Installation-Guide.md) - Initial setup
- [Configuration](Configuration.md) - Jellyfin configuration options
- [Leaving Soon Library](Leaving-Soon-Library.md) - Symlink library feature
- [Troubleshooting](Troubleshooting.md) - Common issues

---

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Installing the Bridge Plugin](#installing-the-bridge-plugin)
- [Generating API Keys](#generating-api-keys)
- [Configuring OxiCleanarr](#configuring-oxicleanarr)
- [Setting Up Leaving Soon Libraries](#setting-up-leaving-soon-libraries)
- [Testing the Integration](#testing-the-integration)
- [Troubleshooting](#troubleshooting)

---

## Overview

OxiCleanarr integrates with Jellyfin to:

1. **Fetch media library data**: Retrieve all movies and TV shows from Jellyfin
2. **Track watch history**: Monitor which media has been watched and by whom
3. **Manage "Leaving Soon" libraries**: Create symlink libraries to preview content scheduled for deletion
4. **Execute deletions**: Remove media from Jellyfin when retention expires

### Integration Architecture

```
OxiCleanarr ←→ Jellyfin API (native)
            ←→ OxiCleanarr Bridge Plugin (file operations)
```

- **Jellyfin API**: Used for media queries, watch history, and metadata
- **Bridge Plugin**: Required for file system operations (symlink management, directory creation)

---

## Prerequisites

- **Jellyfin 10.8+** installed and running
- **Admin access** to Jellyfin dashboard
- **Network connectivity** between OxiCleanarr and Jellyfin
- **Shared file system access** (if using symlink libraries)

---

## Installing the Bridge Plugin

The **OxiCleanarr Bridge Plugin** is **required** for full integration. It provides file system operations that Jellyfin's native API doesn't support.

### Why is the Plugin Required?

Jellyfin's native API doesn't expose file system operations for security reasons. The Bridge Plugin safely extends Jellyfin to:

- Create symlinks for "Leaving Soon" libraries
- Create/delete directories
- List symlink contents
- Validate file paths

**Without this plugin**, OxiCleanarr can still:
- Fetch media from Jellyfin
- Track watch history
- Trigger deletions via Radarr/Sonarr

**But cannot:**
- Create "Leaving Soon" libraries
- Preview content before deletion
- Manage symlink lifecycle

### Installation Methods

#### Method 1: Plugin Repository (Recommended)

1. Open Jellyfin → **Dashboard** → **Plugins** → **Repositories**

2. Click the **"+"** button to add a repository

3. Enter the following details:
   - **Repository Name**: `OxiCleanarr Plugin Repository`
   - **Repository URL**: `https://cdn.jsdelivr.net/gh/ramonskie/jellyfin-plugin-oxicleanarr@main/manifest.json`

4. Click **Save**

5. Navigate to **Dashboard** → **Plugins** → **Catalog**

6. Find **"OxiCleanarr Bridge"** in the catalog

7. Click **Install**

8. **Restart Jellyfin** when prompted

9. Verify installation: **Dashboard → Plugins → My Plugins** → Look for "OxiCleanarr Bridge"

#### Method 2: Manual Installation

1. Download the latest plugin DLL from:
   ```
   https://github.com/ramonskie/jellyfin-plugin-oxicleanarr/releases/latest
   ```

2. Locate your Jellyfin plugins directory:
   - **Linux**: `/var/lib/jellyfin/plugins/`
   - **Windows**: `C:\ProgramData\Jellyfin\Server\plugins\`
   - **Docker**: `/config/plugins/` (inside container)

3. Create the plugin directory:
   ```bash
   mkdir -p /var/lib/jellyfin/plugins/OxiCleanarr_1.0.0.0/
   ```

4. Copy the DLL to the directory:
   ```bash
   cp OxiCleanarr.dll /var/lib/jellyfin/plugins/OxiCleanarr_1.0.0.0/
   ```

5. **Restart Jellyfin**

6. Verify in **Dashboard → Plugins → My Plugins**

### Verifying Plugin Installation

#### Method 1: Dashboard Check

1. Go to **Dashboard → Plugins → My Plugins**
2. Look for "OxiCleanarr Bridge" in the installed plugins list
3. Check that status is "Active"

#### Method 2: API Check

```bash
curl http://jellyfin:8096/api/oxicleanarr/status
```

**Expected Response:**
```json
{
  "status": "ok",
  "plugin_version": "1.0.0",
  "jellyfin_version": "10.8.13"
}
```

**If plugin is not installed:**
```
404 Not Found
```

---

## Generating API Keys

OxiCleanarr needs an API key to authenticate with Jellyfin.

### Step-by-Step

1. Log in to Jellyfin as an **administrator**

2. Navigate to **Dashboard** → **API Keys**

3. Click **"+"** to create a new API key

4. Enter:
   - **App Name**: `OxiCleanarr`
   - **Description**: `Media cleanup automation` (optional)

5. Click **OK**

6. **Copy the generated API key** immediately
   - You won't be able to view it again
   - It looks like: `a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6`

7. Store the API key securely for configuration

### Testing the API Key

```bash
# Replace with your Jellyfin URL and API key
curl -H "X-Emby-Token: YOUR_API_KEY" \
  http://jellyfin:8096/Users
```

**Expected:** JSON list of users  
**If unauthorized:** `401 Unauthorized` (invalid API key)

---

## Configuring OxiCleanarr

### Basic Configuration

Edit `config/config.yaml`:

```yaml
integrations:
  jellyfin:
    enabled: true
    url: http://jellyfin:8096
    api_key: a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6
    timeout: 30s
```

**Configuration Options:**

| Setting | Required | Default | Description |
|---------|----------|---------|-------------|
| `enabled` | Yes | `false` | Enable Jellyfin integration |
| `url` | Yes | - | Jellyfin base URL (include port) |
| `api_key` | Yes | - | Jellyfin API key (from API Keys section) |
| `timeout` | No | `30s` | HTTP request timeout |
| `symlink_library` | No | (disabled) | Leaving Soon library configuration |

### URL Format

- **Include protocol**: `http://` or `https://`
- **Include port** (if not 80/443): `:8096`
- **No trailing slash**: `http://jellyfin:8096` ✅ not `http://jellyfin:8096/` ❌

**Valid examples:**
```yaml
url: http://jellyfin:8096           # Docker container name
url: http://192.168.1.100:8096      # IP address
url: https://jellyfin.example.com   # Domain name (HTTPS)
url: http://localhost:8096          # Local instance
```

### Testing the Configuration

1. Start OxiCleanarr:
   ```bash
   ./oxicleanarr
   ```

2. Check logs for Jellyfin connection:
   ```
   INFO Jellyfin client initialized url=http://jellyfin:8096
   INFO Testing Jellyfin connectivity...
   INFO Jellyfin connectivity test successful
   ```

3. Trigger a manual sync:
   ```bash
   curl -X POST -H "Authorization: Bearer TOKEN" \
     http://localhost:8080/api/sync/full
   ```

4. Check logs for media sync:
   ```
   INFO Starting full sync
   INFO Syncing Jellyfin media...
   INFO Found 500 movies and 300 TV shows
   INFO Full sync completed duration=15s
   ```

---

## Setting Up Leaving Soon Libraries

The "Leaving Soon" feature creates special Jellyfin libraries with symlinks to media scheduled for deletion, allowing users to preview and "keep" content before it's removed.

### Requirements

1. **OxiCleanarr Bridge Plugin** installed in Jellyfin
2. **Shared file system** between Jellyfin and OxiCleanarr
3. **Write permissions** to create symlinks and directories

### Architecture

```
Original Media:
  /media/movies/Inception (2010)/Inception.mkv

Symlink Library:
  /media/leaving-soon/movies/Inception (2010)/Inception.mkv → (symlink)
                                                             ↓
  /media/movies/Inception (2010)/Inception.mkv
```

### Path Mapping

**Critical:** Paths must be accessible from **both** OxiCleanarr and Jellyfin:

| Service | Path Perspective |
|---------|------------------|
| **Radarr** | `/movies/Inception (2010)/Inception.mkv` |
| **Jellyfin** | `/data/movies/Inception (2010)/Inception.mkv` |
| **OxiCleanarr** | Converts Radarr → Jellyfin paths automatically |

OxiCleanarr uses **path mapping** to translate between services.

### Configuration

Edit `config/config.yaml`:

```yaml
integrations:
  jellyfin:
    enabled: true
    url: http://jellyfin:8096
    api_key: your-api-key
    
    symlink_library:
      enabled: true
      library_name: "Leaving Soon"
      library_path: "/data/media/leaving-soon"
      retention_window_days: 14
      path_mappings:
        - radarr_path: "/movies"
          jellyfin_path: "/data/movies"
        - radarr_path: "/tv"
          jellyfin_path: "/data/tv"
```

**Path Mapping Example:**

```yaml
path_mappings:
  # Movies
  - radarr_path: "/movies"
    jellyfin_path: "/data/movies"
  
  # TV Shows
  - radarr_path: "/tv"
    jellyfin_path: "/data/tv"
  
  # Multiple mount points
  - radarr_path: "/mnt/media1/movies"
    jellyfin_path: "/media/movies"
  - radarr_path: "/mnt/media2/movies"
    jellyfin_path: "/media2/movies"
```

**How it works:**

1. Radarr reports: `/movies/Inception (2010)/Inception.mkv`
2. OxiCleanarr translates to: `/data/movies/Inception (2010)/Inception.mkv`
3. Creates symlink at: `/data/media/leaving-soon/movies/Inception (2010)/Inception.mkv`
4. Jellyfin scans and displays the symlink in "Leaving Soon" library

### Creating the Leaving Soon Library in Jellyfin

1. **Create the directory** (on Jellyfin host):
   ```bash
   mkdir -p /data/media/leaving-soon/movies
   mkdir -p /data/media/leaving-soon/tv
   ```

2. In Jellyfin, navigate to **Dashboard → Libraries**

3. Click **"+ Add Media Library"**

4. Select:
   - **Content type**: Movies
   - **Display name**: `Leaving Soon - Movies`
   - **Folders**: `/data/media/leaving-soon/movies`

5. Click **OK**

6. Repeat for TV shows:
   - **Content type**: Shows
   - **Display name**: `Leaving Soon - TV`
   - **Folders**: `/data/media/leaving-soon/tv`

7. **Disable metadata download**:
   - Edit each library → **Library Options**
   - Uncheck "Download images"
   - Uncheck "Enable chapter image extraction"
   - Reason: Symlinks reference existing media, no need to re-download metadata

### Testing Symlink Creation

1. Trigger a full sync:
   ```bash
   curl -X POST -H "Authorization: Bearer TOKEN" \
     http://localhost:8080/api/sync/full
   ```

2. Check logs:
   ```
   INFO Creating symlink library for leaving soon media
   INFO Created 12 symlinks for movies
   INFO Created 8 symlinks for TV shows
   INFO Symlink library updated successfully
   ```

3. Verify in Jellyfin:
   - Navigate to "Leaving Soon - Movies" library
   - Should see movies scheduled for deletion
   - Countdown timers show days remaining

4. Check file system:
   ```bash
   ls -l /data/media/leaving-soon/movies/
   ```
   
   **Expected:** Symlinks pointing to original media files

---

## Testing the Integration

### Connectivity Test

```bash
# Health check (no auth required)
curl http://localhost:8080/health

# Login
TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"changeme"}' | jq -r '.token')

# Trigger full sync
curl -X POST -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/sync/full

# Check sync status
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/sync/status | jq
```

### Expected Log Output

```
INFO Jellyfin client initialized url=http://jellyfin:8096
INFO Testing Jellyfin connectivity...
INFO Jellyfin connectivity test successful
INFO OxiCleanarr Bridge Plugin detected version=1.0.0
INFO Starting full sync
INFO Syncing Jellyfin media...
INFO Found 500 movies
INFO Found 300 TV shows
INFO Creating symlink library...
INFO Created 12 symlinks for leaving soon media
INFO Full sync completed duration=15s media_count=800
```

---

## Troubleshooting

### Plugin Not Detected

**Symptom:** Logs show "OxiCleanarr Bridge Plugin not detected"

**Causes:**
1. Plugin not installed
2. Jellyfin not restarted after plugin installation
3. Wrong Jellyfin URL in config

**Solutions:**
```bash
# Test plugin endpoint
curl http://jellyfin:8096/api/oxicleanarr/status

# Expected: {"status":"ok","plugin_version":"1.0.0"}
# If 404: Plugin not installed or Jellyfin not restarted
```

### Connection Refused

**Symptom:** `connection refused` or `dial tcp: connect: connection refused`

**Causes:**
1. Jellyfin not running
2. Incorrect URL or port
3. Network issues (firewall, Docker network)

**Solutions:**
```bash
# Test Jellyfin directly
curl http://jellyfin:8096/health

# Check Docker network (if using Docker)
docker network inspect bridge

# Verify Jellyfin container is running
docker ps | grep jellyfin
```

### Authentication Errors

**Symptom:** `401 Unauthorized` or `Invalid API key`

**Causes:**
1. Incorrect API key
2. API key revoked or expired
3. Typo in configuration

**Solutions:**
```bash
# Test API key manually
curl -H "X-Emby-Token: YOUR_API_KEY" \
  http://jellyfin:8096/Users

# Expected: JSON array of users
# If 401: API key is invalid

# Regenerate API key in Jellyfin:
# Dashboard → API Keys → Delete old key → Create new key
```

### Symlink Creation Failed

**Symptom:** `Failed to create symlink` or `Permission denied`

**Causes:**
1. OxiCleanarr lacks write permissions
2. Directory doesn't exist
3. File system doesn't support symlinks (e.g., NTFS without admin)
4. Path mapping incorrect

**Solutions:**
```bash
# Test permissions
touch /data/media/leaving-soon/test.txt
ln -s /data/movies/test.mkv /data/media/leaving-soon/test.mkv

# Check directory exists
ls -ld /data/media/leaving-soon/movies

# Verify path mappings in logs
grep "Path mapping" /var/log/oxicleanarr.log
```

### Path Mapping Issues

**Symptom:** Symlinks created but point to wrong location

**Causes:**
1. Path mapping not configured correctly
2. Multiple mount points not accounted for
3. Trailing slashes in paths

**Solutions:**
```yaml
# Bad (trailing slashes)
path_mappings:
  - radarr_path: "/movies/"
    jellyfin_path: "/data/movies/"

# Good (no trailing slashes)
path_mappings:
  - radarr_path: "/movies"
    jellyfin_path: "/data/movies"
```

### Leaving Soon Library Empty

**Symptom:** Library exists but shows no media

**Causes:**
1. No media scheduled for deletion
2. Symlinks not created
3. Jellyfin hasn't scanned the library
4. Path permissions incorrect

**Solutions:**
```bash
# Check if any media is leaving soon
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/media/leaving-soon | jq

# Manually trigger library scan in Jellyfin
Dashboard → Libraries → Scan All Libraries

# Check symlink directory
ls -la /data/media/leaving-soon/movies/
```

---

## Advanced Configuration

### Read-Only Mode

Disable symlink library creation (read-only integration):

```yaml
integrations:
  jellyfin:
    enabled: true
    url: http://jellyfin:8096
    api_key: your-api-key
    symlink_library:
      enabled: false  # Disable symlink creation
```

### Custom Library Names

```yaml
integrations:
  jellyfin:
    symlink_library:
      enabled: true
      library_name: "Cleanup Preview"  # Custom name
      library_path: "/data/cleanup"
```

### Multiple Jellyfin Instances

OxiCleanarr currently supports **one Jellyfin instance** at a time. For multiple instances, run separate OxiCleanarr containers.

---

## Plugin API Reference

The OxiCleanarr Bridge Plugin exposes these endpoints:

### GET /api/oxicleanarr/status

Check plugin status (no authentication required).

**Response:**
```json
{
  "status": "ok",
  "plugin_version": "1.0.0",
  "jellyfin_version": "10.8.13"
}
```

### POST /api/oxicleanarr/symlinks/add

Create symlinks (requires API key).

**Headers:**
```
X-Emby-Token: YOUR_API_KEY
```

**Request:**
```json
{
  "items": [
    {
      "sourcePath": "/data/movies/Inception (2010)/Inception.mkv",
      "targetDirectory": "/data/leaving-soon/movies/Inception (2010)"
    }
  ]
}
```

**Response:**
```json
{
  "success": true,
  "created": 1,
  "failed": 0
}
```

### POST /api/oxicleanarr/symlinks/remove

Remove symlinks (requires API key).

**Request:**
```json
{
  "symlinkPaths": [
    "/data/leaving-soon/movies/Inception (2010)/Inception.mkv"
  ]
}
```

### GET /api/oxicleanarr/symlinks/list

List symlinks in a directory (requires API key).

**Query Parameters:**
- `directory`: Directory path to list

**Response:**
```json
{
  "symlinks": [
    {
      "path": "/data/leaving-soon/movies/Inception (2010)/Inception.mkv",
      "target": "/data/movies/Inception (2010)/Inception.mkv",
      "valid": true
    }
  ]
}
```

---

## See Also

- [Installation Guide](Installation-Guide.md) - Initial setup
- [Configuration](Configuration.md) - Full configuration reference
- [Leaving Soon Library](Leaving-Soon-Library.md) - Symlink library details
- [Troubleshooting](Troubleshooting.md) - Common issues
- [API Reference](API-Reference.md) - REST API documentation
