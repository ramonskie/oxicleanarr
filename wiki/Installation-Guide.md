# Installation Guide

This guide covers multiple installation methods for OxiCleanarr.

## Prerequisites

Before installing OxiCleanarr, ensure you have:

- Active *arr stack services (Radarr and/or Sonarr)
- Jellyfin instance
- **[OxiCleanarr Bridge Plugin](https://github.com/ramonskie/jellyfin-plugin-oxicleanarr)** installed in Jellyfin

> **⚠️ IMPORTANT:** The OxiCleanarr Bridge Plugin is **required** for Jellyfin integration. It provides file system operations (symlink management, directory operations) that Jellyfin's native API doesn't support.

## Install the Bridge Plugin First

### Via Plugin Repository (Recommended)

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

### Manual Installation

For manual installation from source or releases, see the [plugin repository](https://github.com/ramonskie/jellyfin-plugin-oxicleanarr).

## Option 1: Docker (Recommended)

### Pull the Image

```bash
docker pull ghcr.io/ramonskie/oxicleanarr:latest
```

**Available Tags:**
- `ghcr.io/ramonskie/oxicleanarr:latest` - Latest stable release
- `ghcr.io/ramonskie/oxicleanarr:v1.0.0` - Specific version
- `ghcr.io/ramonskie/oxicleanarr:1.0` - Major.minor version
- `ghcr.io/ramonskie/oxicleanarr:1` - Major version

### Run with Docker

```bash
docker run -d \
  --name oxicleanarr \
  -p 8080:8080 \
  -v /path/to/config:/app/config \
  -v /path/to/data:/app/data \
  -v /path/to/media:/data/media:ro \
  -e PUID=1000 \
  -e PGID=1000 \
  -e TZ=UTC \
  ghcr.io/ramonskie/oxicleanarr:latest
```

### Docker Compose

```yaml
version: '3.9'

services:
  oxicleanarr:
    container_name: oxicleanarr
    image: ghcr.io/ramonskie/oxicleanarr:latest
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=America/New_York
    volumes:
      - ./config:/app/config
      - ./data:/app/data
      - /path/to/media:/data/media:ro
    ports:
      - "8080:8080"
    restart: unless-stopped
```

See [Docker Deployment](Docker-Deployment) for detailed Docker setup instructions.

## Option 2: Build from Source

### Requirements

- Go 1.21+
- Node.js 18+ (for frontend)
- Git

### Build Steps

1. **Clone the repository:**
```bash
git clone https://github.com/ramonskie/oxicleanarr.git
cd oxicleanarr
```

2. **Build the application:**
```bash
# Build backend only
go build -o oxicleanarr cmd/oxicleanarr/main.go

# Or build with Makefile (builds frontend + backend)
make build
```

3. **Create configuration:**
```bash
mkdir -p config data
cp config/config.yaml.example config/config.yaml
```

4. **Edit configuration:**
```bash
nano config/config.yaml
```

Update with your service URLs and API keys (see [Configuration](Configuration) guide).

5. **Run OxiCleanarr:**
```bash
./oxicleanarr
```

The application will start on `http://0.0.0.0:8080` by default.

## Post-Installation Steps

### 1. Configure "Leaving Soon" Libraries (Optional)

If you want the "Leaving Soon" feature:

1. **Create directories on the Jellyfin server:**
```bash
mkdir -p /path/to/media/leaving-soon/movies
mkdir -p /path/to/media/leaving-soon/tv
```

2. **Add libraries in Jellyfin:**
   - Go to **Dashboard** → **Libraries** → **Add Media Library**
   - Create library "Leaving Soon - Movies" pointing to `/path/to/media/leaving-soon/movies`
   - Create library "Leaving Soon - TV" pointing to `/path/to/media/leaving-soon/tv`

### 2. Configure OxiCleanarr

Edit `config/config.yaml`:

```yaml
admin:
  username: admin
  password: changeme  # ⚠️ Change this!

integrations:
  jellyfin:
    enabled: true
    url: http://jellyfin:8096
    api_key: your-jellyfin-api-key-here
    symlink_library:
      enabled: true
      base_path: /path/to/media/leaving-soon
      movies_library_name: "Leaving Soon - Movies"
      tv_library_name: "Leaving Soon - TV"
  
  radarr:
    enabled: true
    url: http://radarr:7878
    api_key: your-radarr-api-key-here
  
  sonarr:
    enabled: true
    url: http://sonarr:8989
    api_key: your-sonarr-api-key-here

rules:
  movie_retention: 90d
  tv_retention: 120d

app:
  dry_run: true  # Start in safe mode
```

> **⚠️ Security Note:** Passwords are stored in plain text. Ensure the file has restricted permissions (`chmod 600 config/config.yaml`).

### 3. Access the Web UI

1. Open browser: `http://localhost:8080`
2. Login with your admin credentials
3. Check Dashboard for integration status
4. Trigger a manual sync to populate the library

## Verification

### Check Service Health

```bash
# Health check endpoint
curl http://localhost:8080/health

# Expected response:
# {"status":"ok","uptime":"5.803388477s","version":"1.0.0"}
```

### View Logs

**Docker:**
```bash
docker logs -f oxicleanarr
```

**Binary:**
Check stdout/stderr or logs directory

## Next Steps

- Read the [Quick Start](Quick-Start) guide
- Configure [Advanced Rules](Advanced-Rules) for fine-grained control
- Learn about the [REST API](API-Reference)
- Deploy to your NAS: [NAS Deployment Guide](NAS-Deployment)

## Upgrading

### Docker

```bash
# Pull latest image
docker pull ghcr.io/ramonskie/oxicleanarr:latest

# Recreate container
docker-compose down
docker-compose up -d
```

### From Source

```bash
cd oxicleanarr
git pull origin main
make build
./oxicleanarr
```

## Uninstalling

### Docker

```bash
docker-compose down
docker rmi ghcr.io/ramonskie/oxicleanarr:latest
rm -rf config data logs
```

### From Source

```bash
rm -rf oxicleanarr config data logs
```

## Troubleshooting

See the [Troubleshooting](Troubleshooting) page for common issues and solutions.
