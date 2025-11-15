# Quick Start Guide

Get OxiCleanarr up and running in 15 minutes.

## Before You Start

Ensure you have:

1. ✅ Docker installed (recommended) or Go 1.21+
2. ✅ Jellyfin, Radarr, and/or Sonarr running
3. ✅ API keys for your services
4. ✅ **OxiCleanarr Bridge Plugin** installed in Jellyfin

> **Missing the plugin?** See [Installation Guide](Installation-Guide#install-the-bridge-plugin-first)

## Step 1: Create Configuration

```bash
mkdir -p oxicleanarr/{config,data}
cd oxicleanarr
```

Create `config/config.yaml`:

```yaml
admin:
  username: admin
  password: changeme  # ⚠️ Change this!

integrations:
  jellyfin:
    enabled: true
    url: http://jellyfin:8096
    api_key: YOUR_JELLYFIN_API_KEY
  
  radarr:
    enabled: true
    url: http://radarr:7878
    api_key: YOUR_RADARR_API_KEY
  
  sonarr:
    enabled: true
    url: http://sonarr:8989
    api_key: YOUR_SONARR_API_KEY

# Everything below is optional - these are the defaults
app:
  dry_run: true  # Safe mode - no deletions
  leaving_soon_days: 14

rules:
  movie_retention: 90d
  tv_retention: 120d
```

## Step 2: Run OxiCleanarr

### Docker (Recommended)

```bash
docker run -d \
  --name oxicleanarr \
  -p 8080:8080 \
  -v $(pwd)/config:/app/config \
  -v $(pwd)/data:/app/data \
  ghcr.io/ramonskie/oxicleanarr:latest
```

### Docker Compose

```yaml
version: '3.9'

services:
  oxicleanarr:
    image: ghcr.io/ramonskie/oxicleanarr:latest
    container_name: oxicleanarr
    ports:
      - "8080:8080"
    volumes:
      - ./config:/app/config
      - ./data:/app/data
    restart: unless-stopped
```

```bash
docker-compose up -d
```

## Step 3: Access the Web UI

1. Open browser: **http://localhost:8080**
2. Login with: `admin` / `changeme` (or your password)
3. You'll see the Dashboard

## Step 4: Trigger Your First Sync

1. On the Dashboard, click **"Sync Now"** button
2. Wait a few seconds for sync to complete
3. Refresh the page - you should see:
   - Movie count
   - TV show count
   - Items "leaving soon"

## Step 5: Explore the Features

### Dashboard
- View media statistics
- See items leaving soon with countdown timers
- Monitor sync status

### Timeline View
- Visual calendar of scheduled deletions
- Grouped by date
- See what's being deleted and when

### Library Browser
- Browse all your media
- Filter by type (Movies/TV Shows)
- Search by title or year
- Sort by various fields

### Scheduled Deletions
- See all items that would be deleted
- Understand deletion reasons
- Take action to prevent deletion

## Step 6: Exclude Items from Deletion

When viewing media items, click the **Shield** icon to exclude them from deletion. This is your "Keep" button.

**Excluded items will never be deleted**, even if they exceed retention periods.

## Testing the API

### Get JWT Token

```bash
# Login and get token
TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"changeme"}' \
  | jq -r '.token')

echo "Token: $TOKEN"
```

### Check Sync Status

```bash
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/sync/status | jq
```

### List Movies

```bash
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/media/movies | jq
```

### List Items Leaving Soon

```bash
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/media/leaving-soon | jq
```

## Safety Features

OxiCleanarr starts in **safe mode** by default:

- ✅ `dry_run: true` - No actual deletions occur
- ✅ All operations are simulated
- ✅ You can test rules and timelines without risk
- ✅ Media files are never touched

To enable real deletions (after testing):

```yaml
app:
  dry_run: false
  enable_deletion: true
```

## Common First-Time Tasks

### 1. Verify Integration Health

Dashboard → Check integration status indicators (should all be green)

### 2. Adjust Retention Rules

Edit `config/config.yaml`:

```yaml
rules:
  movie_retention: 60d   # Keep movies for 60 days
  tv_retention: 180d     # Keep TV shows for 180 days
```

Configuration hot-reloads automatically!

### 3. Enable "Leaving Soon" Libraries

See [Leaving Soon Library](Leaving-Soon-Library) guide.

### 4. Set Up Advanced Rules

See [Advanced Rules](Advanced-Rules) for:
- Tag-based retention
- User-based cleanup
- Watched-based deletion

## Next Steps

- Read the full [Configuration](Configuration) guide
- Learn about [Advanced Rules](Advanced-Rules)
- Deploy to production: [Docker Deployment](Docker-Deployment)
- Review [API Reference](API-Reference) for automation

## Troubleshooting

### Can't login?
- Verify username/password in config
- Check Docker logs: `docker logs oxicleanarr`

### No media showing?
- Verify API keys are correct
- Check integration URLs are accessible
- Trigger manual sync: Dashboard → "Sync Now"

### Integrations showing red?
- Verify services are running
- Check URLs and API keys
- Review logs for connection errors

See [Troubleshooting](Troubleshooting) for more help.

## Getting Help

- Check the [FAQ](FAQ)
- Review existing [GitHub Issues](https://github.com/ramonskie/oxicleanarr/issues)
- Open a new issue with logs and configuration (redact API keys!)
