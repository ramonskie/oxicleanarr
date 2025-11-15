# Docker Deployment

This guide covers Docker deployment for OxiCleanarr, including image details, docker-compose configurations, and platform-specific setups.

## Quick Start

```bash
# Pull the latest image
docker pull ghcr.io/ramonskie/oxicleanarr:latest

# Create directory structure
mkdir -p oxicleanarr/{config,data,logs}

# Create config file
nano oxicleanarr/config/config.yaml

# Run container
docker run -d \
  --name oxicleanarr \
  -p 8080:8080 \
  -v $(pwd)/oxicleanarr/config:/app/config \
  -v $(pwd)/oxicleanarr/data:/app/data \
  -v /path/to/media:/data/media:ro \
  ghcr.io/ramonskie/oxicleanarr:latest
```

## Docker Image

### Available Tags

| Tag | Description | Use Case |
|-----|-------------|----------|
| `ghcr.io/ramonskie/oxicleanarr:latest` | Latest stable release | Production |
| `ghcr.io/ramonskie/oxicleanarr:v1.0.0` | Specific version | Version pinning |
| `ghcr.io/ramonskie/oxicleanarr:1.0` | Major.minor version | Feature stability |
| `ghcr.io/ramonskie/oxicleanarr:1` | Major version only | Major version tracking |

### Image Details

- **Base:** Alpine Linux (minimal, secure)
- **Size:** ~15-20 MB compressed
- **Architecture:** amd64, arm64, arm/v7
- **User:** Non-root (UID 1000 by default)
- **Ports:** 8080 (HTTP)

## Docker Compose

### Basic Setup

```yaml
version: '3.8'

services:
  oxicleanarr:
    image: ghcr.io/ramonskie/oxicleanarr:latest
    container_name: oxicleanarr
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=America/New_York
    volumes:
      - ./config:/app/config
      - ./data:/app/data
      - ./logs:/app/logs
      - /path/to/media:/data/media:ro
    ports:
      - "8080:8080"
    restart: unless-stopped
```

### Full Stack Setup

Complete setup with Jellyfin, Radarr, Sonarr:

```yaml
version: '3.8'

networks:
  media:
    driver: bridge

services:
  # OxiCleanarr
  oxicleanarr:
    image: ghcr.io/ramonskie/oxicleanarr:latest
    container_name: oxicleanarr
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=America/New_York
    volumes:
      - ./oxicleanarr/config:/app/config
      - ./oxicleanarr/data:/app/data
      - ./oxicleanarr/logs:/app/logs
      - /mnt/storage/media:/data/media:ro
    ports:
      - "8080:8080"
    networks:
      - media
    restart: unless-stopped
    depends_on:
      - jellyfin
      - radarr
      - sonarr

  # Jellyfin
  jellyfin:
    image: jellyfin/jellyfin:latest
    container_name: jellyfin
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=America/New_York
    volumes:
      - ./jellyfin/config:/config
      - ./jellyfin/cache:/cache
      - /mnt/storage/media:/data/media:ro
    ports:
      - "8096:8096"
    networks:
      - media
    restart: unless-stopped

  # Radarr
  radarr:
    image: linuxserver/radarr:latest
    container_name: radarr
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=America/New_York
    volumes:
      - ./radarr/config:/config
      - /mnt/storage/media:/data/media
    ports:
      - "7878:7878"
    networks:
      - media
    restart: unless-stopped

  # Sonarr
  sonarr:
    image: linuxserver/sonarr:latest
    container_name: sonarr
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=America/New_York
    volumes:
      - ./sonarr/config:/config
      - /mnt/storage/media:/data/media
    ports:
      - "8989:8989"
    networks:
      - media
    restart: unless-stopped
```

### With Optional Services

Include Jellyseerr and Jellystat:

```yaml
  # Jellyseerr (Optional)
  jellyseerr:
    image: fallenbagel/jellyseerr:latest
    container_name: jellyseerr
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=America/New_York
    volumes:
      - ./jellyseerr/config:/app/config
    ports:
      - "5055:5055"
    networks:
      - media
    restart: unless-stopped

  # Jellystat (Optional)
  jellystat:
    image: cyfershepard/jellystat:latest
    container_name: jellystat
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=America/New_York
    volumes:
      - ./jellystat/data:/app/data
    ports:
      - "3000:3000"
    networks:
      - media
    restart: unless-stopped
```

## Environment Variables

### Core Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PUID` | `1000` | User ID for file ownership |
| `PGID` | `1000` | Group ID for file ownership |
| `TZ` | `UTC` | Timezone (e.g., `America/New_York`) |
| `UMASK` | `022` | File creation mask |

### Application Variables

These override config.yaml settings:

| Variable | Default | Description |
|----------|---------|-------------|
| `OXICLEANARR_SERVER_PORT` | `8080` | HTTP server port |
| `OXICLEANARR_APP_DRY_RUN` | `true` | Dry-run mode |
| `OXICLEANARR_LOG_LEVEL` | `info` | Log level (debug/info/warn/error) |

**Example:**
```yaml
environment:
  - PUID=1000
  - PGID=1000
  - TZ=America/New_York
  - OXICLEANARR_APP_DRY_RUN=false
  - OXICLEANARR_LOG_LEVEL=debug
```

## Volume Mounts

### Required Volumes

```yaml
volumes:
  # Configuration directory (read-write)
  - ./config:/app/config

  # Data directory (read-write)
  - ./data:/app/data

  # Media files (read-only recommended)
  - /path/to/media:/data/media:ro
```

### Optional Volumes

```yaml
volumes:
  # Logs directory
  - ./logs:/app/logs

  # Leaving Soon symlinks (if using separate directory)
  - ./leaving-soon:/app/leaving-soon
```

### Volume Permissions

Ensure directories have correct ownership:

```bash
# Set ownership to match PUID:PGID
sudo chown -R 1000:1000 oxicleanarr/

# Set permissions
chmod 755 oxicleanarr/config
chmod 755 oxicleanarr/data
chmod 644 oxicleanarr/config/config.yaml
```

## Network Configuration

### Bridge Network (Default)

Simple setup with default Docker bridge:

```yaml
services:
  oxicleanarr:
    ports:
      - "8080:8080"
```

Services communicate via container names (e.g., `http://radarr:7878`).

### Custom Bridge Network

Better isolation and DNS resolution:

```yaml
networks:
  media:
    driver: bridge

services:
  oxicleanarr:
    networks:
      - media
    ports:
      - "8080:8080"
```

### Host Network (Not Recommended)

Use only if you have network issues:

```yaml
services:
  oxicleanarr:
    network_mode: host
```

**Drawbacks:**
- Less secure
- Port conflicts possible
- Loses container isolation

### NAS-Specific Networks

**Synology:**
```yaml
services:
  oxicleanarr:
    network_mode: synobridge
```

**QNAP:**
```yaml
services:
  oxicleanarr:
    network_mode: qnet-dhcp-eth0
```

## Path Mapping

### Critical: Consistent Paths

**All containers must see media at the same path.**

**Bad Example (Don't Do This):**
```yaml
# ❌ Radarr sees: /movies/Movie.mkv
radarr:
  volumes:
    - /mnt/storage/media/movies:/movies

# ❌ OxiCleanarr sees: /data/media/movies/Movie.mkv
oxicleanarr:
  volumes:
    - /mnt/storage/media:/data/media:ro

# ❌ Result: Path mismatch! OxiCleanarr can't find files.
```

**Good Example (Do This):**
```yaml
# ✅ Radarr sees: /data/media/movies/Movie.mkv
radarr:
  volumes:
    - /mnt/storage/media:/data/media

# ✅ OxiCleanarr sees: /data/media/movies/Movie.mkv
oxicleanarr:
  volumes:
    - /mnt/storage/media:/data/media:ro

# ✅ Jellyfin sees: /data/media/movies/Movie.mkv
jellyfin:
  volumes:
    - /mnt/storage/media:/data/media:ro

# ✅ Result: All containers use same path structure!
```

### Path Mapping Examples

| Host Path | Container Path | Notes |
|-----------|---------------|-------|
| `/volume1/data/media` | `/data/media` | Synology standard |
| `/share/media` | `/data/media` | QNAP standard |
| `/mnt/storage/media` | `/data/media` | Unraid standard |
| `/srv/media` | `/data/media` | Linux server |

**Rule:** Pick ONE container path structure and use it everywhere.

## Platform-Specific Setup

### Synology NAS

```yaml
version: '3.8'

services:
  oxicleanarr:
    image: ghcr.io/ramonskie/oxicleanarr:latest
    container_name: oxicleanarr
    environment:
      - PUID=1027        # Synology default
      - PGID=65536       # Synology "users" group
      - TZ=Europe/Amsterdam
    volumes:
      - /volume1/docker/oxicleanarr/config:/app/config
      - /volume1/docker/oxicleanarr/data:/app/data
      - /volume1/data/media:/data/media:ro
    ports:
      - "8080:8080"
    network_mode: synobridge
    restart: always
```

**Notes:**
- Use `synobridge` network mode
- PUID `1027` is Synology admin user
- PGID `65536` is "users" group

### QNAP NAS

```yaml
version: '3.8'

services:
  oxicleanarr:
    image: ghcr.io/ramonskie/oxicleanarr:latest
    container_name: oxicleanarr
    environment:
      - PUID=1000
      - PGID=100         # QNAP "everyone" group
      - TZ=America/New_York
    volumes:
      - /share/Container/oxicleanarr/config:/app/config
      - /share/Container/oxicleanarr/data:/app/data
      - /share/media:/data/media:ro
    ports:
      - "8080:8080"
    network_mode: qnet-dhcp-eth0
    restart: always
```

### Unraid

```yaml
version: '3.8'

services:
  oxicleanarr:
    image: ghcr.io/ramonskie/oxicleanarr:latest
    container_name: oxicleanarr
    environment:
      - PUID=99
      - PGID=100
      - TZ=America/New_York
    volumes:
      - /mnt/user/appdata/oxicleanarr/config:/app/config
      - /mnt/user/appdata/oxicleanarr/data:/app/data
      - /mnt/user/media:/data/media:ro
    ports:
      - "8080:8080"
    restart: unless-stopped
```

### Ubuntu/Debian Server

```yaml
version: '3.8'

services:
  oxicleanarr:
    image: ghcr.io/ramonskie/oxicleanarr:latest
    container_name: oxicleanarr
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=America/New_York
    volumes:
      - ./oxicleanarr/config:/app/config
      - ./oxicleanarr/data:/app/data
      - /srv/media:/data/media:ro
    ports:
      - "8080:8080"
    restart: unless-stopped
```

## Security Hardening

### Read-Only Media Volumes

Always mount media as read-only:

```yaml
volumes:
  - /path/to/media:/data/media:ro  # :ro = read-only
```

**Why:** Prevents accidental file corruption or deletion from OxiCleanarr bugs.

### Non-Root User

Image runs as non-root user (UID 1000) by default:

```dockerfile
# Built into image
USER oxicleanarr
```

### Security Options

Add additional security constraints:

```yaml
services:
  oxicleanarr:
    security_opt:
      - no-new-privileges:true
    cap_drop:
      - ALL
    cap_add:
      - CHOWN
      - SETGID
      - SETUID
```

### SELinux (Fedora/RHEL/CentOS)

Add `:z` flag to volumes:

```yaml
volumes:
  - ./config:/app/config:z
  - ./data:/app/data:z
  - /path/to/media:/data/media:ro  # Read-only doesn't need :z
```

## Resource Limits

### Memory Limits

```yaml
services:
  oxicleanarr:
    mem_limit: 128m
    mem_reservation: 64m
```

**Typical usage:**
- Idle: 30-40 MB
- Sync (10k items): 50-60 MB
- Peak: ~80 MB

### CPU Limits

```yaml
services:
  oxicleanarr:
    cpus: 0.5  # 50% of one CPU core
```

OxiCleanarr is CPU-light (mostly waiting on API calls).

## Health Checks

### Built-in Health Check

Image includes health check:

```dockerfile
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s \
  CMD wget -q --spider http://localhost:8080/health || exit 1
```

### Custom Health Check

Override if needed:

```yaml
services:
  oxicleanarr:
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 10s
```

## Logging

### Log Driver

```yaml
services:
  oxicleanarr:
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
```

### View Logs

```bash
# Follow logs
docker logs -f oxicleanarr

# Last 100 lines
docker logs --tail 100 oxicleanarr

# With timestamps
docker logs -t oxicleanarr

# Since specific time
docker logs --since 2024-11-15T10:00:00 oxicleanarr
```

### Structured Logging

OxiCleanarr outputs JSON logs by default:

```bash
# Pretty-print with jq
docker logs oxicleanarr 2>&1 | jq

# Filter by level
docker logs oxicleanarr 2>&1 | jq 'select(.level=="error")'

# Filter by message
docker logs oxicleanarr 2>&1 | jq 'select(.message | contains("sync"))'
```

## Updates and Maintenance

### Updating

```bash
# Pull latest image
docker pull ghcr.io/ramonskie/oxicleanarr:latest

# Recreate container
docker-compose up -d --force-recreate oxicleanarr

# Or with plain Docker
docker stop oxicleanarr
docker rm oxicleanarr
docker run -d ... ghcr.io/ramonskie/oxicleanarr:latest
```

### Backup

```bash
# Backup config and data
tar -czf oxicleanarr-backup-$(date +%F).tar.gz \
  oxicleanarr/config \
  oxicleanarr/data

# Restore
tar -xzf oxicleanarr-backup-2024-11-15.tar.gz
```

### Database Maintenance

OxiCleanarr uses JSON files (no database):

```bash
# View exclusions
cat oxicleanarr/data/exclusions.json | jq

# View job history
cat oxicleanarr/data/jobs.json | jq
```

## Troubleshooting

### Container Won't Start

```bash
# Check logs
docker logs oxicleanarr

# Common issues:
# - Port 8080 already in use
# - Config file syntax error
# - Volume permission denied
```

### Permission Errors

```bash
# Fix ownership
sudo chown -R 1000:1000 oxicleanarr/

# Check from inside container
docker exec oxicleanarr ls -la /app/config
docker exec oxicleanarr ls -la /app/data
```

### Network Issues

```bash
# Test container networking
docker exec oxicleanarr ping -c 3 jellyfin
docker exec oxicleanarr ping -c 3 radarr
docker exec oxicleanarr ping -c 3 sonarr

# Test external connectivity
docker exec oxicleanarr wget -O- http://jellyfin:8096/health
```

### Path Issues

```bash
# Verify media mount
docker exec oxicleanarr ls -la /data/media/movies | head -10

# Compare with Radarr
docker exec radarr ls -la /data/media/movies | head -10

# Should show identical output
```

## Related Pages

- [Installation Guide](Installation-Guide.md) - Initial setup steps
- [Quick Start](Quick-Start.md) - Fast deployment guide
- [NAS Deployment](../NAS_DEPLOYMENT.md) - Detailed NAS setup
- [Troubleshooting](Troubleshooting.md) - Common issues
- [Configuration](Configuration.md) - Config file reference
