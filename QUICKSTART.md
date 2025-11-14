# OxiCleanarr Quick Start Guide

## Prerequisites

Before running OxiCleanarr, ensure you have:

1. **Running Services** (at least one):
   - Jellyfin server with API access
   - Radarr (for movie management)
   - Sonarr (for TV show management)
   - Jellyseerr (optional - for request tracking)
   - Jellystat (optional - for watch history and watched-based rules)

2. **API Keys**:
   - Jellyfin API key
   - Radarr API key
   - Sonarr API key

3. **OxiCleanarr Bridge Plugin**:
   - **Required** for "Leaving Soon" library functionality
   - Provides symlink management via HTTP API
   - See installation instructions below

## Installation

### Step 0: Install the OxiCleanarr Bridge Plugin

> **⚠️ REQUIRED**: This plugin is necessary for OxiCleanarr to create "Leaving Soon" libraries in Jellyfin.

1. Open Jellyfin → **Dashboard** → **Plugins** → **Repositories**
2. Click **"+"** to add a repository
3. Enter:
   - **Repository Name**: `OxiCleanarr Plugin Repository`
   - **Repository URL**: `https://cdn.jsdelivr.net/gh/ramonskie/jellyfin-plugin-oxicleanarr@main/manifest.json`
4. Click **Save**
5. Go to **Dashboard** → **Plugins** → **Catalog**
6. Find "OxiCleanarr Bridge" and click **Install**
7. Restart Jellyfin when prompted
8. Verify: **Dashboard** → **Plugins** → Confirm "OxiCleanarr Bridge" is listed and active

> **Manual Installation**: For manual installation, see the [plugin repository](https://github.com/ramonskie/jellyfin-plugin-oxicleanarr)

## Configuration

### Step 1: Create Configuration File

Copy the example configuration and edit it with your credentials:

```bash
cp config/config.yaml.example config/config.yaml
# Now edit config/config.yaml with your API keys
```

**⚠️ IMPORTANT:** Never commit `config/config.yaml` to version control - it contains sensitive API keys!

### Step 2: Configure "Leaving Soon" Library (Optional)

If you want the "Leaving Soon" feature, configure the symlink library paths:

1. **Create directories** on your Jellyfin server:
   ```bash
   mkdir -p /path/to/media/leaving-soon/movies
   mkdir -p /path/to/media/leaving-soon/tv
   ```

2. **Add libraries in Jellyfin**:
   - Go to **Dashboard** → **Libraries** → **Add Media Library**
   - Create library "Leaving Soon - Movies" pointing to `/path/to/media/leaving-soon/movies`
   - Create library "Leaving Soon - TV" pointing to `/path/to/media/leaving-soon/tv`
   - Set content type to Movies/Shows respectively

### Step 3: Edit Configuration

Edit `config/config.yaml` and update the following:

```yaml
admin:
  username: admin
  password: changeme  # ⚠️ CHANGE THIS! Plain text password

integrations:
  jellyfin:
    enabled: true
    url: http://YOUR-JELLYFIN-HOST:8096  # Update this
    api_key: YOUR-JELLYFIN-API-KEY       # Update this
    
    # Optional: Enable "Leaving Soon" library
    symlink_library:
      enabled: true
      base_path: /path/to/media/leaving-soon  # Path on Jellyfin server
      movies_library_name: "Leaving Soon - Movies"
      tv_library_name: "Leaving Soon - TV"
  
  radarr:
    enabled: true
    url: http://YOUR-RADARR-HOST:7878    # Update this
    api_key: YOUR-RADARR-API-KEY         # Update this
  
  sonarr:
    enabled: true
    url: http://YOUR-SONARR-HOST:8989    # Update this
    api_key: YOUR-SONARR-API-KEY         # Update this
  
  # Optional: Enable for user-based rules
  jellyseerr:
    enabled: false
    url: http://YOUR-JELLYSEERR-HOST:5055
    api_key: YOUR-JELLYSEERR-API-KEY
  
  # Optional: Enable for watched-based rules
  jellystat:
    enabled: false
    url: http://YOUR-JELLYSTAT-HOST:3000
    api_key: YOUR-JELLYSTAT-API-KEY
```

**⚠️ Security Warning:** 
- Passwords are stored in **plain text** in the config file
- Change the default password immediately!
- Protect the config file: `chmod 600 config/config.yaml`
- Use a strong password since it's not hashed

### Step 4: Optional Settings

Uncomment and modify these sections as needed:

```yaml
app:
  dry_run: true              # Set to true for testing (no actual deletions)
  leaving_soon_days: 14      # Days before considering media "leaving soon"

sync:
  full_interval: 3600        # Full sync every hour (in seconds)
  incremental_interval: 900  # Incremental sync every 15 minutes
  auto_start: true           # Start syncing immediately on startup

rules:
  movie_retention: 90d       # Keep movies for 90 days
  tv_retention: 120d         # Keep TV shows for 120 days

server:
  host: 0.0.0.0             # Listen on all interfaces
  port: 8080                 # HTTP port
```

### Step 5: Advanced Rules (Optional)

Configure advanced rules for fine-grained control. See [README.md](README.md#advanced-rules) for full documentation.

```yaml
advanced_rules:
  # Tag-based: Preserve kids content longer
  - name: Kids Content
    type: tag
    enabled: true
    tag: kids
    retention: 180d
  
  # User-based: Trial users get shorter retention (requires Jellyseerr)
  - name: Trial Users
    type: user
    enabled: true
    users:
      - email: guest@example.com
        retention: 7d
        require_watched: true
  
  # Watched-based: Auto-cleanup watched content (requires Jellystat)
  - name: Watched Cleanup
    type: watched
    enabled: true
    retention: 30d
    require_watched: true
```

## Running OxiCleanarr

### Option 1: Run Development Mode (Recommended for Testing)

```bash
make dev
```

Or directly:

```bash
go run cmd/oxicleanarr/main.go
```

### Option 2: Build and Run Binary

```bash
make build
./oxicleanarr
```

### Option 3: Build and Run in One Command

```bash
make run
```

## Environment Variables

You can override settings with environment variables:

```bash
# Logging
export LOG_LEVEL=debug          # Options: debug, info, warn, error
export LOG_FORMAT=pretty        # Options: json, pretty

# JWT
export JWT_SECRET=your-secret-key-min-32-chars
export JWT_EXPIRATION=24h

# Paths
export CONFIG_PATH=/path/to/config.yaml
export DATA_PATH=/path/to/data

# Run with environment variables
make dev
```

## Testing the API

### 1. Check Health

```bash
curl http://localhost:8080/health
```

Expected response:
```json
{
  "status": "ok",
  "version": "1.0.0",
  "uptime_seconds": 10
}
```

### 2. Login to Get JWT Token

```bash
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "changeme"
  }'
```

Expected response:
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**Save this token** - you'll need it for authenticated requests.

### 3. Get Sync Status

```bash
# Replace YOUR_TOKEN with the token from login
curl http://localhost:8080/api/sync/status \
  -H "Authorization: Bearer YOUR_TOKEN"
```

Expected response:
```json
{
  "running": true,
  "media_count": 0,
  "full_interval_seconds": 3600,
  "incr_interval_seconds": 900,
  "movies_count": 0,
  "tv_shows_count": 0,
  "excluded_count": 0
}
```

### 4. Trigger Manual Full Sync

```bash
curl -X POST http://localhost:8080/api/sync/full \
  -H "Authorization: Bearer YOUR_TOKEN"
```

Expected response:
```json
{
  "message": "Full sync started"
}
```

### 5. List Movies

```bash
curl http://localhost:8080/api/media/movies \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### 6. List TV Shows

```bash
curl http://localhost:8080/api/media/shows \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### 7. List Media Leaving Soon

```bash
curl http://localhost:8080/api/media/leaving-soon \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### 8. Get Recent Jobs

```bash
curl http://localhost:8080/api/jobs \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### 9. Get Latest Job

```bash
curl http://localhost:8080/api/jobs/latest \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### 10. Exclude Media from Deletion

```bash
# Replace MEDIA_ID with actual media ID (e.g., "radarr-123")
curl -X POST http://localhost:8080/api/media/MEDIA_ID/exclude \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "reason": "User favorite - do not delete"
  }'
```

### 11. Delete Media (Dry Run)

```bash
# With dry_run=true in config, this will only log what would be deleted
curl -X DELETE http://localhost:8080/api/media/MEDIA_ID?dry_run=true \
  -H "Authorization: Bearer YOUR_TOKEN"
```

## Testing Workflow

### Complete Live Test Scenario

1. **Start OxiCleanarr** with `dry_run: true` in config:
   ```bash
   make dev
   ```

2. **Login and get token**:
   ```bash
   TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
     -H "Content-Type: application/json" \
     -d '{"username":"admin","password":"changeme"}' \
     | jq -r '.token')
   
   echo "Token: $TOKEN"
   ```

3. **Trigger a sync** to pull data from your services:
   ```bash
   curl -X POST http://localhost:8080/api/sync/full \
     -H "Authorization: Bearer $TOKEN"
   ```

4. **Wait a few seconds**, then check job status:
   ```bash
   curl http://localhost:8080/api/jobs/latest \
     -H "Authorization: Bearer $TOKEN" | jq
   ```

5. **List your media**:
   ```bash
   # Movies
   curl http://localhost:8080/api/media/movies \
     -H "Authorization: Bearer $TOKEN" | jq
   
   # TV Shows
   curl http://localhost:8080/api/media/shows \
     -H "Authorization: Bearer $TOKEN" | jq
   ```

6. **Check what's leaving soon**:
   ```bash
   curl http://localhost:8080/api/media/leaving-soon \
     -H "Authorization: Bearer $TOKEN" | jq
   ```

7. **Test exclusion** (protect a movie from deletion):
   ```bash
   # Get first movie ID
   MOVIE_ID=$(curl -s http://localhost:8080/api/media/movies \
     -H "Authorization: Bearer $TOKEN" | jq -r '.[0].id')
   
   # Add exclusion
   curl -X POST http://localhost:8080/api/media/$MOVIE_ID/exclude \
     -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"reason":"Testing exclusion feature"}'
   ```

8. **Check sync status**:
   ```bash
   curl http://localhost:8080/api/sync/status \
     -H "Authorization: Bearer $TOKEN" | jq
   ```

### Testing Advanced Rules

#### Testing Watched-Based Rules

If you have Jellystat configured and want to test watched-based cleanup:

```bash
# 1. Enable watched rules in config
# advanced_rules:
#   - name: Watched Cleanup
#     type: watched
#     enabled: true
#     retention: 30d
#     require_watched: true

# 2. Sync to pull watch history from Jellystat
curl -X POST http://localhost:8080/api/sync/full \
  -H "Authorization: Bearer $TOKEN"

# 3. Check media with watch counts
curl http://localhost:8080/api/media/movies \
  -H "Authorization: Bearer $TOKEN" | jq '.[] | {title, watch_count, last_watched, delete_after}'

# 4. Verify that:
#    - Watched content has delete_after date = last_watched + retention
#    - Unwatched content (watch_count: 0) is NOT scheduled for deletion
```

#### Testing User-Based Rules

If you have Jellyseerr configured:

```bash
# 1. Enable user rules in config
# advanced_rules:
#   - name: Guest Users
#     type: user
#     enabled: true
#     users:
#       - email: guest@example.com
#         retention: 7d

# 2. Sync to pull request data from Jellyseerr
curl -X POST http://localhost:8080/api/sync/full \
  -H "Authorization: Bearer $TOKEN"

# 3. Check which user requested each item
curl http://localhost:8080/api/media/movies \
  -H "Authorization: Bearer $TOKEN" | jq '.[] | {title, requested_by, delete_after}'
```

## Troubleshooting

### Check Logs

Logs are output to stdout/stderr. Look for:

- **Errors** connecting to services (Jellyfin, Radarr, Sonarr)
- **Authentication failures** (check API keys)
- **Configuration issues** (YAML syntax, missing required fields)

### Common Issues

1. **"Failed to load configuration"**
   - Check `config/config.yaml` exists
   - Verify YAML syntax is correct
   - Ensure admin password is set

2. **"Failed to sync Radarr/Sonarr/Jellyfin"**
   - Verify service URLs are accessible
   - Check API keys are correct
   - Ensure services are running

3. **"Token validation failed"**
   - Your JWT token may have expired (default 24h)
   - Login again to get a new token

4. **"401 Unauthorized"**
   - Missing or invalid JWT token
   - Include `Authorization: Bearer YOUR_TOKEN` header

5. **"Leaving Soon library not working"**
   - Verify OxiCleanarr Bridge Plugin is installed and active in Jellyfin
   - Check Jellyfin logs for plugin errors
   - Verify `symlink_library.enabled: true` in config
   - Ensure `base_path` exists and is accessible by Jellyfin
   - Confirm library names match in both config and Jellyfin

6. **"Watched rules not working"**
   - Ensure Jellystat integration is enabled and configured
   - Verify Jellystat is tracking watch history
   - Check that `require_watched: true` is set if you want to protect unwatched content
   - Sync again to refresh watch history: `POST /api/sync/full`

### Enable Debug Logging

```bash
export LOG_LEVEL=debug
export LOG_FORMAT=pretty
make dev
```

## Data Storage

OxiCleanarr stores data in the `./data` directory:

- `exclusions.json` - List of excluded media items
- `jobs.json` - Job history (last 100 jobs)

These files are automatically created on first run.

## Next Steps

Once you've verified OxiCleanarr is working:

1. **Set `dry_run: false`** in config to enable actual deletions
2. **Configure retention rules** in the config file
3. **Set up automation** with proper sync intervals
4. **Monitor job history** to track cleanup operations
5. **Build a frontend** or integrate with your existing dashboard

## Safety Features

- **Dry Run Mode**: Test without making changes
- **Exclusions**: Protect specific media from deletion
- **Request Protection**: Won't delete requested items (if Jellyseerr is configured)
- **Retention Rules**: Configurable retention periods
- **Job History**: Audit trail of all operations

## Get Help

If you encounter issues:

1. Check the logs with `LOG_LEVEL=debug`
2. Verify your configuration
3. Test API endpoints individually
4. Review the test suite for usage examples

## Development

Run tests:
```bash
make test
```

Run tests with coverage:
```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

Format code:
```bash
make fmt
```
