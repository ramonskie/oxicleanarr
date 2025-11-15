# Leaving Soon Library

The **Leaving Soon Library** is OxiCleanarr's flagship feature that provides users with visibility into media scheduled for deletion. Instead of content silently disappearing, users see items entering a "leaving soon" window with countdown timers, giving them time to watch or exclude content before it's removed.

## Overview

The Leaving Soon Library uses **symlinks** to create virtual Jellyfin libraries that show media approaching its retention deadline. This approach:

- ✅ **Non-destructive** - Symlinks are pointers; deleting them doesn't affect original files
- ✅ **Low overhead** - Symlinks use minimal disk space (~100 bytes each)
- ✅ **Real-time updates** - Automatically refreshes as retention windows change
- ✅ **User-friendly** - Appears as regular Jellyfin libraries with full metadata

## How It Works

### 1. Retention Window

Content enters the "Leaving Soon" window based on the `leaving_soon_days` configuration:

```yaml
app:
  leaving_soon_days: 14  # Show items 14 days before deletion
```

**Example Timeline:**
```
Added: Jan 1
Retention: 90 days (movie_retention: 90d)
Delete After: Apr 1
Leaving Soon Window: Mar 18 - Apr 1 (last 14 days)
```

### 2. Symlink Creation

When media enters the leaving soon window, OxiCleanarr:

1. Creates symlink directories (if they don't exist):
   ```
   /path/to/leaving-soon/
   ├── movies/
   └── tv/
   ```

2. Creates symlinks pointing to original media files:
   ```bash
   # Movies: Link to movie file
   /leaving-soon/movies/Inception (2010).mkv 
     → /data/media/movies/Inception (2010)/Inception (2010).mkv
   
   # TV Shows: Link to entire show directory
   /leaving-soon/tv/Breaking Bad/
     → /data/media/tv/Breaking Bad/
   ```

3. Instructs Jellyfin to scan the symlink libraries

### 3. Jellyfin Virtual Folders

OxiCleanarr automatically creates two Jellyfin libraries:

- **Leaving Soon - Movies** → Points to `/leaving-soon/movies/`
- **Leaving Soon - TV Shows** → Points to `/leaving-soon/tv/`

These appear in Jellyfin like any other library, with full poster art, metadata, and playback support.

### 4. Automatic Cleanup

As media crosses retention thresholds:

- **Exits window** (user watched it, exclusion added) → Symlink removed
- **Deleted** → Symlink removed, original file deleted from Radarr/Sonarr
- **Window changes** → Symlinks updated on next sync

## Configuration

### Basic Setup

```yaml
integrations:
  jellyfin:
    enabled: true
    url: http://jellyfin:8096
    api_key: your-api-key-here
    symlink_library:
      enabled: true                           # Enable symlink libraries
      base_path: /data/media/leaving-soon    # Symlink directory
      movies_library_name: "Leaving Soon - Movies"
      tv_library_name: "Leaving Soon - TV Shows"

app:
  leaving_soon_days: 14  # Countdown window (days before deletion)
```

### Path Considerations

**Option 1: Inside existing media mount (recommended)**
```yaml
symlink_library:
  base_path: /data/media/leaving-soon
```

**Why recommended:**
- Jellyfin already has `/data/media` mounted
- Symlinks can reference files in same mount
- No additional Jellyfin configuration required

**Option 2: Separate directory**
```yaml
symlink_library:
  base_path: /app/leaving-soon
```

**Requires:**
- Additional mount in Jellyfin docker-compose:
  ```yaml
  volumes:
    - /path/to/leaving-soon:/app/leaving-soon:ro
  ```

### Docker Configuration

**OxiCleanarr container:**
```yaml
services:
  oxicleanarr:
    volumes:
      # Media files (read-only)
      - /volume1/data/media:/data/media:ro
      
      # Symlink directory (read-write, OxiCleanarr creates symlinks here)
      - /volume1/data/media/leaving-soon:/data/media/leaving-soon
```

**Jellyfin container:**
```yaml
services:
  jellyfin:
    volumes:
      # Media files + symlink directory (single mount covers both)
      - /volume1/data/media:/data/media:ro
```

## Lifecycle Example

### Day 1: Movie Added
```
Movie: "Inception (2010)"
Added: Jan 1
Retention: 90d
Delete After: Apr 1
Status: Safe (90 days remaining)
```

**Leaving Soon Library:** Empty (not in window yet)

### Day 77: Enters Leaving Soon Window
```
Date: Mar 18
Days Until Deletion: 14
Status: Leaving Soon
```

**OxiCleanarr Actions:**
1. Creates symlink: `/leaving-soon/movies/Inception (2010).mkv`
2. Tells Jellyfin to scan library
3. Movie appears in "Leaving Soon - Movies"

**User Sees:**
- Dashboard: "5 items leaving soon"
- Jellyfin Library: "Leaving Soon - Movies" shows Inception with full metadata
- Countdown timer: "14 days remaining"

### Day 85: User Watches Movie
```
Date: Mar 26
Days Until Deletion: 6
Last Watched: Mar 26
```

**Status:** Still in leaving soon window
**UI Updates:** Countdown now shows "6 days remaining"

### Day 88: User Clicks "Keep"
```
Date: Mar 29
Action: Exclusion added
```

**OxiCleanarr Actions:**
1. Adds exclusion to `data/exclusions.json`
2. Removes symlink (no longer scheduled for deletion)
3. Tells Jellyfin to refresh library

**User Sees:**
- Movie removed from "Leaving Soon - Movies"
- Dashboard: "4 items leaving soon"
- Library Browser: Movie shows "Excluded" badge

### Day 91: Window Expires (if not excluded)
```
Date: Apr 1
Retention Expired: Yes
Dry Run: true
```

**OxiCleanarr Actions (Dry Run):**
1. Logs: "Would delete: Inception (2010)"
2. Keeps symlink (dry run doesn't delete)
3. Job history shows deletion candidate

**OxiCleanarr Actions (Dry Run Disabled):**
1. Deletes movie from Radarr (removes file)
2. Removes symlink
3. Removes movie from Jellyfin library
4. Job history logs deletion

## Features

### Countdown Timers

The web UI shows real-time countdown timers:

```
🎬 Inception (2010)
   ⏱️ 6 days, 14 hours remaining
   📊 Retention expired (90d)
   🛡️ Keep
```

### Deletion Reasons

Hover over the info icon to see why content is scheduled for deletion:

```
"This movie was last watched 95 days ago. 
The retention policy for movies is 90 days, 
meaning it will be deleted after that period 
of inactivity."
```

### Keep Button

One-click exclusion from deletion:

```
🛡️ Keep  →  Adds exclusion
           →  Removes from Leaving Soon Library
           →  Protects indefinitely
```

### Filtering

Library browser allows filtering by status:

- **All Media** - Entire library
- **Leaving Soon** - Items in deletion window
- **Excluded** - Protected from deletion
- **Scheduled** - Past retention, pending deletion

## Sync Behavior

### Full Sync (Default: Every 1 hour)

1. Fetch all media from Radarr/Sonarr/Jellyfin
2. Apply retention rules
3. Identify items in leaving soon window
4. Create/update symlinks
5. Remove stale symlinks
6. Trigger Jellyfin library scan

### Incremental Sync (Default: Every 15 minutes)

1. Fetch recently changed media
2. Update watch history from Jellyfin
3. Recalculate leaving soon window
4. Update symlinks if needed

### Performance

- Creating 100 symlinks: ~1 second
- Jellyfin scan (100 items): ~5-10 seconds
- Total overhead: Negligible

## Troubleshooting

### Problem: Leaving Soon Libraries Empty

**Check 1: Are items actually in the window?**
```bash
# Check Timeline page in web UI
# Should show items grouped by deletion date
```

**Check 2: Do symlinks exist?**
```bash
ls -la /data/media/leaving-soon/movies/
ls -la /data/media/leaving-soon/tv/

# Should show symlinks like:
# lrwxrwxrwx  1 user group  56 Nov 15 10:30 Movie.mkv -> /data/media/movies/Movie/file.mkv
```

**Check 3: Can Jellyfin access symlinks?**
```bash
# From Jellyfin container
docker exec jellyfin ls -la /data/media/leaving-soon/movies/

# Should show same files as host
```

**Check 4: Check OxiCleanarr logs**
```bash
docker logs oxicleanarr 2>&1 | grep -i symlink
docker logs oxicleanarr 2>&1 | grep -i "leaving soon"
```

### Problem: Symlinks Don't Point to Real Files

**Symptoms:**
- Jellyfin shows items but can't play them
- "File not found" errors when playing

**Root Cause:** Path mismatch between OxiCleanarr and Jellyfin

**Solution:** Ensure paths are consistent across containers:

```yaml
# Both containers must see files at same path
oxicleanarr:
  volumes:
    - /volume1/data/media:/data/media:ro

jellyfin:
  volumes:
    - /volume1/data/media:/data/media:ro
```

**Verify:**
```bash
# On host
ls -la /volume1/data/media/movies/Inception\ (2010)/

# In OxiCleanarr container
docker exec oxicleanarr ls -la /data/media/movies/Inception\ (2010)/

# In Jellyfin container
docker exec jellyfin ls -la /data/media/movies/Inception\ (2010)/

# All three should show identical output
```

### Problem: Symlinks Not Created

**Check 1: OxiCleanarr Bridge Plugin installed?**
```bash
# Check Jellyfin Dashboard → Plugins
# Should show "OxiCleanarr Bridge" active
```

**Check 2: Permissions correct?**
```bash
# Symlink directory must be writable
ls -la /data/media/leaving-soon/

# Should be owned by OxiCleanarr user (PUID:PGID)
# Fix if needed:
sudo chown -R 1027:65536 /data/media/leaving-soon/
```

**Check 3: Feature enabled?**
```yaml
# Check config.yaml
integrations:
  jellyfin:
    symlink_library:
      enabled: true  # Must be true
```

### Problem: Items Stay in Leaving Soon After Exclusion

**Expected Behavior:** Adding exclusion removes symlink immediately

**Check 1: Trigger manual sync**
```bash
# Web UI → Dashboard → "Sync Now"
# Or via API:
curl -X POST -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/sync/incremental
```

**Check 2: Verify exclusion was saved**
```bash
cat /app/data/exclusions.json | jq
# Should show item in "items" object
```

**Check 3: Check logs**
```bash
docker logs oxicleanarr 2>&1 | tail -50
# Look for "Applied X exclusions to library"
```

## API Endpoints

### Get Leaving Soon Items

**GET** `/api/media/leaving-soon`

Response:
```json
{
  "media": [
    {
      "id": "radarr-123",
      "type": "movie",
      "title": "Inception",
      "year": 2010,
      "delete_after": "2024-04-01T00:00:00Z",
      "days_until_due": 6,
      "deletion_reason": "Retention period expired (90d)"
    }
  ],
  "total": 5
}
```

### Trigger Symlink Refresh

**POST** `/api/sync/incremental`

Forces immediate symlink library update.

## Best Practices

### 1. Test with Dry Run First

```yaml
app:
  dry_run: true  # Keep enabled until confident
```

Observe leaving soon behavior for 1-2 weeks before enabling deletions.

### 2. Adjust Window Based on Usage

```yaml
# For active users who check daily
app:
  leaving_soon_days: 7

# For casual users who check weekly
app:
  leaving_soon_days: 21
```

### 3. Use Appropriate Retention Periods

```yaml
rules:
  movie_retention: 90d    # 3 months for movies
  tv_retention: 120d      # 4 months for TV shows
```

Balance between storage cost and user satisfaction.

### 4. Monitor Job History

Check **Job History** page regularly to see:
- How many items enter/exit leaving soon window
- Which items are being deleted
- Whether users are using the Keep button

### 5. Consider Advanced Rules

Use tag-based rules to protect important content:

```yaml
advanced_rules:
  - name: Keep Collections
    type: tag
    enabled: true
    tag: collection
    retention: never  # Never delete
```

## Related Pages

- [Deletion Timeline](Deletion-Timeline.md) - Understand the full deletion lifecycle
- [Configuration](Configuration.md) - Complete configuration reference
- [Advanced Rules](Advanced-Rules.md) - Fine-grained retention control
- [Jellyfin Integration](Jellyfin-Integration.md) - Jellyfin setup and troubleshooting
- [Docker Deployment](Docker-Deployment.md) - Container configuration
