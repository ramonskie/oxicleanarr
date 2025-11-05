# Prunarr NAS Deployment Guide

## Prerequisites Check

Run these commands on your NAS to verify the setup:

```bash
# 1. Check media structure
ls -la /volume1/data/media/ | head -10

# 2. Verify you can write to the media directory
touch /volume1/data/media/test-prunarr.txt && rm /volume1/data/media/test-prunarr.txt && echo "Write access OK"

# 3. Check existing containers
docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
```

## Path Mapping Explained

**The Key Principle:** Mount only what you need. More restrictive mounts = better security.

**Example Setup (adjust paths to match YOUR system):**
- **Radarr/Sonarr** see movies at: `/data/media/movies/Movie Name (2020)/movie.mkv`
- **Jellyfin** sees same file at: `/data/media/movies/Movie Name (2020)/movie.mkv`
- **Prunarr** will see it at: `/data/media/movies/Movie Name (2020)/movie.mkv`

**Symlinks will be created:**
- **On host (NAS)**: `/volume3/docker/prunarr/leaving-soon/movies/Movie Name (2020).mkv`
- **Prunarr container sees**: `/app/leaving-soon/movies/Movie Name (2020).mkv`
- **Jellyfin container should see**: `/app/leaving-soon/movies/Movie Name (2020).mkv` (mount same host dir)
- **Symlink target**: → `/data/media/movies/Movie Name (2020)/movie.mkv`

**Common Media Path Patterns:**

| Your Host Structure | Recommended Mount | Container Sees | Notes |
|---------------------|-------------------|----------------|-------|
| `/volume1/data/media/movies/` | `/volume1/data/media:/data/media:ro` | `/data/media/movies/` | ✅ Most restrictive |
| `/volume1/media/movies/` | `/volume1/media:/media:ro` | `/media/movies/` | ✅ Simple structure |
| `/mnt/storage/media/` | `/mnt/storage/media:/media:ro` | `/media/movies/` | ✅ Custom mount point |
| `/volume1/data/` | `/volume1/data:/data:ro` | `/data/media/movies/` | ⚠️ Exposes ALL of /data |

**Rule:** Mount the most specific parent directory that contains your media files.

## Step-by-Step Deployment

### Step 1: Create Prunarr Directory Structure

```bash
# SSH into your NAS, then:
sudo mkdir -p /volume3/docker/prunarr/config
sudo mkdir -p /volume3/docker/prunarr/data
sudo mkdir -p /volume3/docker/prunarr/logs
sudo mkdir -p /volume3/docker/prunarr/leaving-soon
sudo chown -R 1027:65536 /volume3/docker/prunarr
```

**IMPORTANT**: Note we create a `config` **directory** (not just placing the file directly). This is critical for Docker ownership changes to work correctly.

### Step 2: Create Prunarr Config File

```bash
# Create config file INSIDE the config directory
sudo nano /volume3/docker/prunarr/config/prunarr.yaml
```

Paste this content (replace API keys):

```yaml
admin:
  username: admin
  password: changeme

app:
  dry_run: true                   # KEEP THIS TRUE FOR TESTING
  enable_deletion: false
  leaving_soon_days: 14

integrations:
  jellyfin:
    enabled: true
    url: http://jellyfin:8096
    api_key: YOUR_JELLYFIN_API_KEY    # Replace this
  
  radarr:
    enabled: true
    url: http://radarr:7878
    api_key: YOUR_RADARR_API_KEY      # Replace this
  
  sonarr:
    enabled: true
    url: http://sonarr:8989
    api_key: YOUR_SONARR_API_KEY      # Replace this
  
  jellystat:
    enabled: true
    url: http://jellystat:3000
    api_key: YOUR_JELLYSTAT_API_KEY   # Replace this (if you use it)

sync:
  full_interval: 3600
  incremental_interval: 900
  auto_start: true

rules:
  movie_retention: 90d
  tv_retention: 120d

server:
  host: 0.0.0.0
  port: 8080

symlink_library:
  enabled: true
  base_path: /data/media/leaving-soon  # RECOMMENDED: Inside existing media mount
  movies_library_name: "Leaving Soon - Movies"
  tv_library_name: "Leaving Soon - TV Shows"
```

Save and exit (Ctrl+X, Y, Enter)

### Step 3: Build Prunarr Docker Image

Prunarr uses a **multi-stage Dockerfile** that:
1. Builds the React frontend (web UI)
2. Builds the Go backend binary
3. Combines both in a minimal Alpine runtime image

**Option A: Build on NAS (if you have the source code)**

```bash
cd /path/to/prunarr/source
docker build -t prunarr:latest .

# Build will take 5-10 minutes
# You'll see 3 stages: frontend-builder, backend-builder, runtime
```

**Option B: Build on dev machine, export, import on NAS (Recommended)**

```bash
# On dev machine:
cd /path/to/prunarr
docker build -t prunarr:latest .
docker save prunarr:latest | gzip > prunarr-latest.tar.gz

# Copy to NAS (replace with your NAS IP/hostname)
scp prunarr-latest.tar.gz admin@your-nas:/volume3/docker/

# On NAS (SSH in):
docker load < /volume3/docker/prunarr-latest.tar.gz

# Verify image loaded
docker images | grep prunarr
# Should show: prunarr  latest  <image-id>  <size>
```

**Option C: Use docker-compose build (simplest if source is on NAS)**

```bash
# If you cloned the repo to your NAS:
cd /volume3/docker/prunarr/source
docker-compose -f docker-compose.nas.yml build

# This will use the Dockerfile in the repo root
```

### Step 4: Create Docker Compose File for Prunarr

```bash
sudo nano /volume3/docker/prunarr/docker-compose.yml
```

Paste:

```yaml
version: '3.8'

services:
  prunarr:
    image: prunarr:latest
    container_name: prunarr
    environment:
      - PUID=1027
      - PGID=65536
      - TZ=Europe/Amsterdam
      - UMASK=022
    volumes:
      # NOTE: Use :z flag on SELinux systems (Fedora, RHEL, CentOS)
      # Synology/QNAP typically don't need :z flag
      # IMPORTANT: Mount directories, not individual files!
      - /volume3/docker/prunarr/config:/app/config:z
      - /volume3/docker/prunarr/data:/app/data:z
      - /volume3/docker/prunarr/logs:/app/logs:z
      - /volume3/docker/prunarr/leaving-soon:/app/leaving-soon:z
      
      # Media paths - MUST match your Radarr/Sonarr/Jellyfin configuration
      # Mount ONLY the media directory (more restrictive = more secure)
      # Adjust these paths to match YOUR system structure:
      - /volume1/data/media:/data/media:ro  # Recommended: Only expose media files
      # Alternative patterns:
      # - /volume1/media:/media:ro          # If media is at /volume1/media/
      # - /mnt/storage/media:/media:ro      # Custom storage location
    ports:
      - 8080:8080
    network_mode: synobridge
    security_opt:
      - no-new-privileges:true
    restart: always
```

### Step 5: Verify Jellyfin Can Access Symlinks

**RECOMMENDED APPROACH:** If you configured `base_path: /data/media/leaving-soon`, **no changes needed!**

Jellyfin already has the `/data/media` mount, which includes the `leaving-soon` subdirectory where Prunarr creates symlinks.

**How it works:**
- Prunarr creates: `/data/media/leaving-soon/movies/Red Dawn (2012).mkv` → `/data/media/movies/Red Dawn (2012)/file.mkv`
- Jellyfin Virtual Folder points to: `/data/media/leaving-soon/movies/`
- Jellyfin can read both the symlink AND follow it to the real file (same mount!)

**Verify Jellyfin has the media mount** (edit your jellyfin docker-compose.yml if needed):

```yaml
volumes:
  - /volume3/docker/jellyfin:/config
  - /volume1/data/media:/data/media:ro  # This gives access to both media files AND symlinks
```

**Alternative approach** (if you used `base_path: /app/leaving-soon` instead):

```yaml
volumes:
  - /volume3/docker/jellyfin:/config
  - /volume1/data/media:/data/media:ro                          # Access actual files
  - /volume3/docker/prunarr/leaving-soon:/app/leaving-soon:ro  # Access symlinks (extra mount)
```

If you changed anything, recreate Jellyfin container:
```bash
cd /path/to/jellyfin/compose
docker-compose up -d
```

### Step 6: Start Prunarr

```bash
cd /volume3/docker/prunarr
docker-compose up -d
```

### Step 7: Verify Startup

```bash
# Check logs
docker logs -f prunarr

# Should see:
# - "Starting Prunarr v1.0.0"
# - "Configuration loaded"
# - "HTTP server listening on :8080"
```

### Step 8: Access Web UI & Test

1. Open browser: `http://your-nas-ip:8080`
2. Login with: `admin` / `changeme`
3. Check Dashboard for:
   - Integration health (all green)
   - Media count (should show your library)
4. Trigger manual sync: Dashboard → "Sync Now"
5. Check Timeline page for items scheduled for deletion

### Step 9: Verify Symlinks Created

```bash
# Check symlink directories exist (adjust path based on your config)
ls -la /volume1/data/media/leaving-soon/   # If using recommended base_path
# OR
ls -la /volume3/docker/prunarr/leaving-soon/  # If using separate directory

# Should see:
# drwxr-xr-x movies/
# drwxr-xr-x tv/

# Check symlink contents
ls -la /volume3/docker/prunarr/leaving-soon/movies/ | head -5
ls -la /volume3/docker/prunarr/leaving-soon/tv/ | head -5

# Verify symlinks point to real files
file /volume3/docker/prunarr/leaving-soon/movies/* | head -3
```

### Step 10: Verify Jellyfin Libraries Created

1. Open Jellyfin web UI
2. Click hamburger menu → Libraries
3. You should see two new libraries:
   - "Leaving Soon - Movies"
   - "Leaving Soon - TV Shows"
4. Click into each library - should show scheduled items

### Step 11: Check Prunarr Logs

```bash
docker logs prunarr 2>&1 | grep -i symlink
docker logs prunarr 2>&1 | grep -i "virtual folder"
```

Look for:
- `"Syncing symlink libraries"`
- `"Created virtual folder: Leaving Soon - Movies"`
- `"Created X symlinks for movies"`
- `"Symlink library sync completed"`

## Troubleshooting

### Problem: Permission denied errors (SELinux systems)

If you're running on **Fedora, RHEL, CentOS, or other SELinux-enabled systems**, you may see permission errors:

```bash
# Check SELinux status
getenforce
# If it shows "Enforcing", you need to add :z flags to volume mounts
```

**Solution:** Add `:z` flag to all read-write volume mounts in your `docker-compose.yml`:

```yaml
volumes:
  - /volume3/docker/prunarr/config:/app/config:z
  - /volume3/docker/prunarr/data:/app/data:z
  - /volume3/docker/prunarr/logs:/app/logs:z
  - /volume3/docker/prunarr/leaving-soon:/app/leaving-soon:z
  - /volume1/data:/data:ro  # Read-only mounts don't need :z
```

**Note:** Synology and QNAP NAS systems typically don't use SELinux, so the `:z` flag is optional but harmless.

### Problem: "Permission denied" on config/data files

If you see errors like:
```
chmod: /app/config/prunarr.yaml: Operation not permitted
open /app/data/jobs.json: permission denied
```

**Root Cause:** Mounting individual **files** (instead of directories) prevents Docker from changing ownership.

**Solution:** Always mount **directories**, not individual files:

```yaml
# ❌ WRONG - File mount (causes permission errors)
volumes:
  - /volume3/docker/prunarr/prunarr.yaml:/app/config/prunarr.yaml

# ✅ CORRECT - Directory mount (allows ownership changes)
volumes:
  - /volume3/docker/prunarr/config:/app/config
```

**Fix existing deployment:**
```bash
# Move config file into config directory
mkdir -p /volume3/docker/prunarr/config
mv /volume3/docker/prunarr/prunarr.yaml /volume3/docker/prunarr/config/
sudo chown -R 1027:65536 /volume3/docker/prunarr

# Update docker-compose.yml to use directory mount
# Then recreate container:
docker-compose up -d --force-recreate
```

### Problem: No symlinks created

```bash
# Check if symlink directory exists and has correct permissions
# (Adjust path based on your base_path config)

# If using base_path: /data/media/leaving-soon (recommended)
ls -la /volume1/data/media/leaving-soon/

# If using base_path: /app/leaving-soon (separate directory)
ls -la /volume3/docker/prunarr/leaving-soon/

# Should be owned by your PUID:PGID (default 1027:65536)
# If not, fix it:
sudo chown -R 1027:65536 /volume1/data/media/leaving-soon
```

### Problem: Jellyfin libraries not created

```bash
# Check if Prunarr can reach Jellyfin API
docker exec prunarr curl -s http://jellyfin:8096/System/Info/Public | jq
```

### Problem: Jellyfin libraries empty or not showing items

**Symptoms:**
- "Leaving Soon - Movies" library exists but shows 0 items
- Jellyfin can't see the symlinks Prunarr created

**Root Cause:** Jellyfin doesn't have access to the symlink directory.

**Solution depends on your `base_path` setting:**

**If using `base_path: /data/media/leaving-soon` (recommended):**
```yaml
# Jellyfin docker-compose.yml - Only needs ONE mount!
volumes:
  - /volume3/docker/jellyfin:/config
  - /volume1/data/media:/data/media:ro  # Already includes /data/media/leaving-soon/ ✅
```

**If using `base_path: /app/leaving-soon` (separate directory):**
```yaml
# Jellyfin docker-compose.yml - Needs TWO mounts
volumes:
  - /volume3/docker/jellyfin:/config
  - /volume1/data/media:/data/media:ro                          # Actual media files
  - /volume3/docker/prunarr/leaving-soon:/app/leaving-soon:ro  # Symlinks (extra mount)
```

**Verify mounts are working:**

**If using `base_path: /data/media/leaving-soon` (recommended):**
```bash
# From host: Check symlinks exist
ls -la /volume1/data/media/leaving-soon/movies/

# From Jellyfin container: Check if it can see symlinks
docker exec jellyfin ls -la /data/media/leaving-soon/movies/

# Should show the same files! Both containers share the same mount.
```

**If using `base_path: /app/leaving-soon` (separate directory):**
```bash
# From host: Check symlinks exist
ls -la /volume3/docker/prunarr/leaving-soon/movies/

# From Jellyfin container: Check if it can see symlinks
docker exec jellyfin ls -la /app/leaving-soon/movies/

# If second command shows "No such file or directory", add the mount and restart Jellyfin
docker-compose restart jellyfin
```

**How symlinks work:**
1. Prunarr creates symlink: `/data/media/leaving-soon/movies/Movie.mkv` → `/data/media/movies/Movie/file.mkv`
2. Jellyfin Virtual Folder points to: `/data/media/leaving-soon/movies/`
3. Jellyfin reads the symlink file and follows it to the real file
4. **Jellyfin only needs one mount** (the `/data/media` mount) to access both!

### Problem: Path mismatch errors

```bash
# Verify paths inside containers
docker exec prunarr ls -la /data/media/movies/ | head -5
docker exec jellyfin ls -la /data/media/movies/ | head -5
docker exec radarr ls -la /data/media/movies/ | head -5

# All should show the same files
```

## What to Share With Me for Testing

Once you've deployed, share:

1. **Startup logs:**
   ```bash
   docker logs prunarr | head -50
   ```

2. **Symlink directory contents:**
   ```bash
   ls -laR /volume3/docker/prunarr/leaving-soon/
   ```

3. **Integration status from API:**
   ```bash
   curl -s http://localhost:8080/api/dashboard/health | jq
   ```

4. **Any errors from logs:**
   ```bash
   docker logs prunarr 2>&1 | grep -i error
   docker logs prunarr 2>&1 | grep -i fail
   ```

## Safety Notes

- ✅ Config has `dry_run: true` - no actual deletions
- ✅ Media volumes mounted read-only (`:ro`) - cannot modify originals
- ✅ Only symlink directory is read-write
- ✅ Symlinks are safe - deleting them doesn't delete source files
