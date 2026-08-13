# OxiCleanarr NAS Deployment Guide

## Prerequisites Check

### 1. Install Leaving Soon Plugin in Jellyfin (optional)

> **ℹ️ OPTIONAL**: Only needed for the "Leaving Soon" library feature. The standalone
> [jellyfin-plugin-leaving-soon](https://github.com/ramonskie/jellyfin-plugin-leaving-soon)
> plugin polls OxiCleanarr's `GET /api/media/leaving-soon` endpoint and manages the
> symlink libraries inside Jellyfin itself.

**Install via Plugin Repository (Recommended):**
1. Open Jellyfin → **Dashboard** → **Plugins** → **Repositories**
2. Click **"+"** to add a repository
3. Enter:
   - **Repository Name**: `Leaving Soon Plugin Repository`
   - **Repository URL**: `https://cdn.jsdelivr.net/gh/ramonskie/jellyfin-plugin-leaving-soon@main/manifest.json`
4. Click **Save**
5. Go to **Dashboard** → **Plugins** → **Catalog**
6. Find "Leaving Soon" and click **Install**
7. Restart Jellyfin when prompted
8. Configure the plugin (Settings → Leaving Soon): add an `oxicleanarr` provider pointing
   at your OxiCleanarr server, and set a base path that Jellyfin can write to

> **Manual Installation**: See the [plugin repository](https://github.com/ramonskie/jellyfin-plugin-leaving-soon) for manual installation steps.

### 2. Verify NAS Setup

Run these commands on your NAS to verify the setup:

```bash
# 1. Check media structure
ls -la /volume1/data/media/ | head -10

# 2. Verify you can write to the media directory
touch /volume1/data/media/test-oxicleanarr.txt && rm /volume1/data/media/test-oxicleanarr.txt && echo "Write access OK"

# 3. Check existing containers
docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
```

## Path Mapping Explained

**The Key Principle:** Mount only what you need. More restrictive mounts = better security.

**Example Setup (adjust paths to match YOUR system):**
- **Radarr/Sonarr** see movies at: `/data/media/movies/Movie Name (2020)/movie.mkv`
- **Jellyfin** sees same file at: `/data/media/movies/Movie Name (2020)/movie.mkv`
- **OxiCleanarr** will see it at: `/data/media/movies/Movie Name (2020)/movie.mkv`

**Symlinks are created by the plugin (running inside Jellyfin):**
- **On host (NAS)**: `/volume3/docker/leaving-soon/movies/Movie Name (2020).mkv`
- **Jellyfin container sees**: `/app/leaving-soon/movies/Movie Name (2020).mkv` (mount the same host dir)
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

### Step 1: Create OxiCleanarr Directory Structure

```bash
# SSH into your NAS, then:
sudo mkdir -p /volume3/docker/oxicleanarr/config
sudo mkdir -p /volume3/docker/oxicleanarr/data
sudo mkdir -p /volume3/docker/oxicleanarr/logs
sudo chown -R 1027:65536 /volume3/docker/oxicleanarr
```

**IMPORTANT**: Note we create a `config` **directory** (not just placing the file directly). This is critical for Docker ownership changes to work correctly.

### Step 2: Create OxiCleanarr Config File

```bash
# Create config file INSIDE the config directory
sudo nano /volume3/docker/oxicleanarr/config/config.yaml
```

Paste this content (replace API keys):

```yaml
admin:
  username: admin
  password: changeme  # ⚠️ CHANGE THIS! Bcrypt hashes supported
  disable_auth: false  # Set true to skip login (NOT recommended for production)

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
```

> **Note:** "Leaving Soon" symlink libraries are managed by jellyfin-plugin-leaving-soon
> inside Jellyfin (see Step 5). There is no `symlink_library` config in OxiCleanarr.

Save and exit (Ctrl+X, Y, Enter)

**⚠️ Security Warning:**
- Change "changeme" to a strong password
- Protect the file: `sudo chmod 600 /volume3/docker/oxicleanarr/config/config.yaml`
- Only the NAS admin user should be able to read this file
- With authentication enabled (`disable_auth: false`), set the `JWT_SECRET` environment variable (min 32 chars) in the container, or OxiCleanarr will refuse to start

### Step 3: Pull OxiCleanarr Docker Image

**Option A: Pull from GitHub Container Registry (Recommended)**

```bash
# Pull the latest stable release
docker pull ghcr.io/ramonskie/oxicleanarr:latest

# Or pull a specific version
docker pull ghcr.io/ramonskie/oxicleanarr:v1.0.0

# Verify image downloaded
docker images | grep oxicleanarr
# Should show: ghcr.io/ramonskie/oxicleanarr  latest  <image-id>  <size>
```

**Available Tags:**
- `ghcr.io/ramonskie/oxicleanarr:latest` - Latest stable release
- `ghcr.io/ramonskie/oxicleanarr:v1.0.0` - Specific version (e.g., v1.0.0)
- `ghcr.io/ramonskie/oxicleanarr:1.0` - Major.minor version
- `ghcr.io/ramonskie/oxicleanarr:1` - Major version only

**Option B: Build from Source (if you need custom modifications)**

```bash
# On dev machine:
cd /path/to/oxicleanarr
docker build -t oxicleanarr:latest .

# Option B1: Save and transfer to NAS
docker save oxicleanarr:latest | gzip > oxicleanarr-latest.tar.gz
scp oxicleanarr-latest.tar.gz admin@your-nas:/volume3/docker/

# On NAS (SSH in):
docker load < /volume3/docker/oxicleanarr-latest.tar.gz
```

> **Note:** Building from source takes 5-10 minutes. The multi-stage Dockerfile builds the React frontend, Go backend, and combines them in a minimal Alpine runtime image.

### Step 4: Create Docker Compose File for OxiCleanarr

```bash
sudo nano /volume3/docker/oxicleanarr/docker-compose.yml
```

Paste:

```yaml
version: '3.8'

services:
  oxicleanarr:
    image: ghcr.io/ramonskie/oxicleanarr:latest
    container_name: oxicleanarr
    environment:
      - PUID=1027
      - PGID=65536
      - TZ=Europe/Amsterdam
      - UMASK=022
      # REQUIRED unless admin.disable_auth is true in config.yaml
      # Generate with: openssl rand -base64 48
      - JWT_SECRET=change-me-to-a-long-random-string
      # Optional: token lifetime (default: 24h)
      - JWT_EXPIRATION=24h
    volumes:
      # NOTE: Use :z flag on SELinux systems (Fedora, RHEL, CentOS)
      # Synology/QNAP typically don't need :z flag
      # IMPORTANT: Mount directories, not individual files!
      - /volume3/docker/oxicleanarr/config:/app/config:z
      - /volume3/docker/oxicleanarr/data:/app/data:z
      - /volume3/docker/oxicleanarr/logs:/app/logs:z
      
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

> **Note:** The old `leaving-soon` volume mount is no longer needed on the OxiCleanarr
> container — symlink management moved into jellyfin-plugin-leaving-soon, which runs
> inside the Jellyfin container.

### Step 5: Configure the Leaving Soon Plugin in Jellyfin

The plugin (running inside Jellyfin) creates the symlink directories and virtual
folders itself. You only need to give Jellyfin a writable directory to host them.

**RECOMMENDED:** reuse the existing media mount — the plugin writes symlinks under
`/data/media/leaving-soon/` on the Jellyfin container:

**How it works:**
- The plugin creates: `/data/media/leaving-soon/movies/Red Dawn (2012).mkv` → `/data/media/movies/Red Dawn (2012)/file.mkv`
- Jellyfin Virtual Folder points to: `/data/media/leaving-soon/movies/`
- Jellyfin can read both the symlink AND follow it to the real file (same mount!)

**Verify Jellyfin has a writable media mount** (edit your jellyfin docker-compose.yml if needed):

```yaml
volumes:
  - /volume3/docker/jellyfin:/config
  - /volume1/data/media:/data/media:rw  # writable, so the plugin can create symlinks
```

Then configure the plugin in Jellyfin (**Dashboard → Plugins → Leaving Soon → Settings**):
- **Base Path**: `/data/media/leaving-soon`
- **Library names**: e.g. "Leaving Soon - Movies", "Leaving Soon - TV"
- **Provider**: add an `oxicleanarr` provider with URL `http://<oxicleanarr-ip>:8080`
- **Sync Interval**: how often to poll OxiCleanarr

> **⚠️ Auth:** the plugin polls OxiCleanarr without a user login. Set `admin.api_key` in
> OxiCleanarr's config to a static key and enter it as the provider's API key in the
> plugin settings — the key is accepted on every protected endpoint. Alternatively run
> with `admin.disable_auth: true`, which disables the login for the whole app.

**Alternative approach** (dedicated directory, requires an extra Jellyfin mount):

```yaml
volumes:
  - /volume3/docker/jellyfin:/config
  - /volume1/data/media:/data/media:ro                          # Access actual files
  - /volume3/docker/leaving-soon:/app/leaving-soon:rw           # Plugin's symlink dir
```

If you changed anything, recreate Jellyfin container:
```bash
cd /path/to/jellyfin/compose
docker-compose up -d
```

### Step 6: Start OxiCleanarr

```bash
cd /volume3/docker/oxicleanarr
docker-compose up -d
```

### Step 7: Verify Startup

```bash
# Check logs
docker logs -f oxicleanarr

# Should see:
# - "Starting OxiCleanarr v1.0.0"
# - "Configuration loaded"
# - "HTTP server listening on :8080"
```

### Step 8: Access Web UI & Test

1. Open browser: `http://your-nas-ip:8080`
2. Login with: `admin` / `changeme` (if `disable_auth: true`, you'll land on the dashboard directly — no login prompt)
3. Check Dashboard for:
   - Integration health (all green)
   - Media count (should show your library)
4. Trigger manual sync: Dashboard → "Sync Now"
5. Check Timeline page for items scheduled for deletion

### Step 9: Verify Symlinks & Libraries (created by the plugin)

The Leaving Soon plugin creates the symlink directories and virtual folders inside
Jellyfin after a sync. To verify:

```bash
# Check the symlink dir the plugin writes to (adjust to the base path you configured)
ls -la /volume1/data/media/leaving-soon/   # If using the recommended /data/media/leaving-soon
# Should see movies/ and tv/ subdirectories once items are scheduled

# Check symlinks point to real files
ls -la /volume1/data/media/leaving-soon/movies/ | head -5
file /volume1/data/media/leaving-soon/movies/* | head -3
```

### Step 10: Verify Jellyfin Libraries Created

1. Open Jellyfin web UI
2. Click hamburger menu → Libraries
3. You should see the libraries configured in the plugin (e.g. "Leaving Soon - Movies",
   "Leaving Soon - TV Shows")
4. Click into each library - should show scheduled items

### Step 11: Check Plugin Logs

```bash
docker logs jellyfin 2>&1 | grep -i "leaving soon"
```

Look for plugin sync lines (e.g. "Reconciling leaving-soon libraries").

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
  - /volume3/docker/oxicleanarr/config:/app/config:z
  - /volume3/docker/oxicleanarr/data:/app/data:z
  - /volume3/docker/oxicleanarr/logs:/app/logs:z
  - /volume1/data:/data:ro  # Read-only mounts don't need :z
```

**Note:** Synology and QNAP NAS systems typically don't use SELinux, so the `:z` flag is optional but harmless.

### Problem: "Permission denied" on config/data files

If you see errors like:
```
chmod: /app/config/config.yaml: Operation not permitted
open /app/data/jobs.json: permission denied
```

**Root Cause:** Mounting individual **files** (instead of directories) prevents Docker from changing ownership.

**Solution:** Always mount **directories**, not individual files:

```yaml
# ❌ WRONG - File mount (causes permission errors)
volumes:
  - /volume3/docker/oxicleanarr/config.yaml:/app/config/config.yaml

# ✅ CORRECT - Directory mount (allows ownership changes)
volumes:
  - /volume3/docker/oxicleanarr/config:/app/config
```

**Fix existing deployment:**
```bash
# Move config file into config directory
mkdir -p /volume3/docker/oxicleanarr/config
mv /volume3/docker/oxicleanarr/config.yaml /volume3/docker/oxicleanarr/config/
sudo chown -R 1027:65536 /volume3/docker/oxicleanarr

# Update docker-compose.yml to use directory mount
# Then recreate container:
docker-compose up -d --force-recreate
```

### Problem: No symlinks created (Leaving Soon)

The symlinks are created by jellyfin-plugin-leaving-soon inside Jellyfin — not by
OxiCleanarr. Check in this order:

1. **OxiCleanarr exposes data**: run a sync, then verify the endpoint returns items
   (send the `admin.api_key` as a Bearer token):
   ```bash
   curl -s -H "Authorization: Bearer YOUR_API_KEY" http://localhost:8080/api/media/leaving-soon | jq
   # Expect { "version": 1, "items": [ ... ] } with at least one item
   ```
2. **Plugin installed & configured**: Dashboard → Plugins → Leaving Soon → Settings —
   confirm the `oxicleanarr` provider URL points at OxiCleanarr and is enabled.
3. **Plugin can write**: verify the base path exists and is writable by Jellyfin:
   ```bash
   ls -la /volume1/data/media/leaving-soon/
   # Should be owned by your PUID:PGID (default 1027:65536)
   # If not, fix it:
   sudo chown -R 1027:65536 /volume1/data/media/leaving-soon
   ```

### Problem: Jellyfin libraries not created

```bash
# Check that OxiCleanarr is reachable from Jellyfin (provider URL in plugin settings)
docker exec jellyfin curl -s http://<oxicleanarr-ip>:8080/health | jq
```

### Problem: Jellyfin libraries empty or not showing items

**Symptoms:**
- "Leaving Soon - Movies" library exists but shows 0 items
- Jellyfin can't see the symlinks the plugin created

**Root Cause:** Jellyfin doesn't have access to the symlink directory.

**Solution depends on the plugin's `BasePath` setting:**

**If using `/data/media/leaving-soon` (recommended):**
```yaml
# Jellyfin docker-compose.yml - Only needs ONE mount (writable!)
volumes:
  - /volume3/docker/jellyfin:/config
  - /volume1/data/media:/data/media:rw  # Includes /data/media/leaving-soon/ ✅
```

**If using a dedicated directory (e.g. `/app/leaving-soon`):**
```yaml
# Jellyfin docker-compose.yml - Needs TWO mounts
volumes:
  - /volume3/docker/jellyfin:/config
  - /volume1/data/media:/data/media:ro                          # Actual media files
  - /volume3/docker/leaving-soon:/app/leaving-soon:rw           # Plugin's symlink dir
```

**Verify mounts are working:**

**If using `/data/media/leaving-soon` (recommended):**
```bash
# From host: Check symlinks exist
ls -la /volume1/data/media/leaving-soon/movies/

# From Jellyfin container: Check if it can see symlinks
docker exec jellyfin ls -la /data/media/leaving-soon/movies/

# Should show the same files! Same mount.
```

**If using a dedicated directory:**
```bash
# From host: Check symlinks exist
ls -la /volume3/docker/leaving-soon/movies/

# From Jellyfin container: Check if it can see symlinks
docker exec jellyfin ls -la /app/leaving-soon/movies/

# If second command shows "No such file or directory", add the mount and restart Jellyfin
docker-compose restart jellyfin
```

**How symlinks work:**
1. The plugin creates symlink: `/data/media/leaving-soon/movies/Movie.mkv` → `/data/media/movies/Movie/file.mkv`
2. Jellyfin Virtual Folder points to: `/data/media/leaving-soon/movies/`
3. Jellyfin reads the symlink file and follows it to the real file
4. **Jellyfin only needs one mount** (the `/data/media` mount) to access both!

### Problem: Path mismatch errors

```bash
# Verify paths inside containers
docker exec oxicleanarr ls -la /data/media/movies/ | head -5
docker exec jellyfin ls -la /data/media/movies/ | head -5
docker exec radarr ls -la /data/media/movies/ | head -5

# All should show the same files
```

## What to Share With Me for Testing

Once you've deployed, share:

1. **Startup logs:**
   ```bash
   docker logs oxicleanarr | head -50
   ```

2. **Symlink directory contents:**
   ```bash
   ls -laR /volume1/data/media/leaving-soon/
   ```
   (or whatever base path you configured for the Leaving Soon plugin)

3. **Integration status from API:**
   ```bash
   curl -s http://localhost:8080/api/dashboard/health | jq
   ```

4. **Any errors from logs:**
   ```bash
   docker logs oxicleanarr 2>&1 | grep -i error
   docker logs oxicleanarr 2>&1 | grep -i fail
   ```

## Safety Notes

- ✅ Config has `dry_run: true` - no actual deletions
- ✅ Media volumes mounted read-only (`:ro`) - cannot modify originals
- ✅ Only the plugin's symlink directory is read-write (in the Jellyfin container)
- ✅ Symlinks are safe - deleting them doesn't delete source files
