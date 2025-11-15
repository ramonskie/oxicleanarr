# Configuration Guide

Complete reference for configuring OxiCleanarr.

## Configuration File Location

OxiCleanarr uses a YAML configuration file:

- **Default path**: `./config/config.yaml`
- **Override with env**: `CONFIG_PATH=/path/to/config.yaml`

The configuration file supports **hot-reloading** - changes are applied automatically without restarting.

## Minimal Configuration

The absolute minimum required configuration:

```yaml
admin:
  username: admin
  password: changeme

integrations:
  jellyfin:
    enabled: true
    url: http://jellyfin:8096
    api_key: your-key-here
  
  radarr:
    enabled: true
    url: http://radarr:7878
    api_key: your-key-here
  
  sonarr:
    enabled: true
    url: http://sonarr:8989
    api_key: your-key-here
```

Everything else has sensible defaults!

## Full Configuration Reference

### Admin Section

```yaml
admin:
  username: admin                 # Admin username for web UI
  password: changeme              # ⚠️ Plain text - change immediately!
  disable_auth: false             # Bypass JWT auth (dev only, NEVER in production)
```

**Security Notes:**
- Passwords are stored in **plain text** - protect this file!
- Set file permissions: `chmod 600 config/config.yaml`
- Use a strong password
- Never commit config to version control

### App Settings

```yaml
app:
  dry_run: true                   # Safe mode - no actual deletions
  enable_deletion: false          # Enable automatic deletion during sync
  leaving_soon_days: 14           # Days before deletion to show in "Leaving Soon"
```

**Deletion Safety:**
- Start with `dry_run: true` to test
- Set `enable_deletion: true` only when ready for real deletions
- Both must be configured for deletions to occur

### Sync Settings

```yaml
sync:
  full_interval: 3600             # Full sync interval (seconds) - default: 1 hour
  incremental_interval: 900       # Incremental sync interval (seconds) - default: 15 min
  auto_start: true                # Start sync scheduler on boot
```

**Sync Types:**
- **Full Sync**: Complete library refresh from all services
- **Incremental Sync**: Quick update of recent changes and watch history

### Server Settings

```yaml
server:
  host: 0.0.0.0                   # HTTP bind address
  port: 8080                      # HTTP port
  read_timeout: 30s               # Request read timeout
  write_timeout: 30s              # Response write timeout
  idle_timeout: 60s               # Keep-alive timeout
  shutdown_timeout: 30s           # Graceful shutdown timeout
```

### Retention Rules

```yaml
rules:
  movie_retention: 90d            # Keep movies for 90 days
  tv_retention: 120d              # Keep TV shows for 120 days
```

**Duration Format:**
- `30d` - 30 days
- `24h` - 24 hours
- `180d` - 180 days (6 months)
- `never` or `0d` - Disable standard retention (only advanced rules apply)

**Special Values:**
- `never` - Disables standard retention rules entirely
- `0d` - Same as `never`
- Use when you only want user-based or advanced rules

## Integration Configuration

### Jellyfin Integration

```yaml
integrations:
  jellyfin:
    enabled: true                 # Enable Jellyfin integration
    url: http://jellyfin:8096     # Jellyfin server URL
    api_key: your-key-here        # Jellyfin API key
    timeout: 30s                  # API timeout
    
    # Symlink Library (optional)
    symlink_library:
      enabled: true
      base_path: /data/media/leaving-soon
      movies_library_name: "Leaving Soon - Movies"
      tv_library_name: "Leaving Soon - TV Shows"
      hide_when_empty: true       # Auto-remove empty libraries
```

**Getting Jellyfin API Key:**
1. Jellyfin → Dashboard → API Keys
2. Click "+" to create new key
3. Name it "OxiCleanarr"
4. Copy the key

**Symlink Library Configuration:**
- See [Leaving Soon Library](Leaving-Soon-Library) for detailed setup
- Requires [OxiCleanarr Bridge Plugin](https://github.com/ramonskie/jellyfin-plugin-oxicleanarr)

### Radarr Integration

```yaml
integrations:
  radarr:
    enabled: true                 # Enable Radarr integration
    url: http://radarr:7878       # Radarr server URL
    api_key: your-key-here        # Radarr API key
    timeout: 30s                  # API timeout
```

**Getting Radarr API Key:**
1. Radarr → Settings → General
2. Security section
3. Copy "API Key"

### Sonarr Integration

```yaml
integrations:
  sonarr:
    enabled: true                 # Enable Sonarr integration
    url: http://sonarr:8989       # Sonarr server URL
    api_key: your-key-here        # Sonarr API key
    timeout: 30s                  # API timeout
```

**Getting Sonarr API Key:**
1. Sonarr → Settings → General
2. Security section
3. Copy "API Key"

### Jellyseerr Integration (Optional)

```yaml
integrations:
  jellyseerr:
    enabled: false                # Enable for request tracking
    url: http://jellyseerr:5055   # Jellyseerr server URL
    api_key: your-key-here        # Jellyseerr API key
    timeout: 30s                  # API timeout
```

**Required for:**
- User-based cleanup rules
- Request tracking
- "Requested by" information

**Getting Jellyseerr API Key:**
1. Jellyseerr → Settings → General
2. API Key section
3. Copy the key

### Jellystat Integration (Optional)

```yaml
integrations:
  jellystat:
    enabled: false                # Enable for watch history
    url: http://jellystat:3000    # Jellystat server URL
    api_key: your-key-here        # Jellystat API key
    timeout: 30s                  # API timeout
```

**Required for:**
- Watched-based cleanup rules
- Watch count tracking
- Last watched dates

**Getting Jellystat API Key:**
1. Jellystat → Settings → API
2. Generate or copy existing key

## Advanced Rules

See [Advanced Rules](Advanced-Rules) for complete documentation.

### Tag-Based Rules

```yaml
advanced_rules:
  - name: Kids Content
    type: tag
    enabled: true
    tag: kids
    retention: 180d              # Keep kids content for 6 months
```

### User-Based Rules

```yaml
advanced_rules:
  - name: Trial Users
    type: user
    enabled: true
    users:
      - user_id: 42
        retention: 7d
      - email: guest@example.com
        retention: 14d
        require_watched: true    # Only delete after watched
```

**Requires:** Jellyseerr integration enabled

### Watched-Based Rules

```yaml
advanced_rules:
  - name: Auto Clean Watched
    type: watched
    enabled: true
    retention: 30d               # Delete 30 days after last watch
    require_watched: true        # Only delete media that has been watched
```

**Requires:** Jellystat integration enabled

## Environment Variables

Override configuration with environment variables using the `OXICLEANARR_` prefix:

```bash
# Admin
export OXICLEANARR_ADMIN_USERNAME=myadmin
export OXICLEANARR_ADMIN_PASSWORD=mypassword

# Server
export OXICLEANARR_SERVER_PORT=9090
export OXICLEANARR_SERVER_HOST=0.0.0.0

# App
export OXICLEANARR_APP_DRY_RUN=false
export OXICLEANARR_APP_LEAVING_SOON_DAYS=7

# Integrations
export OXICLEANARR_INTEGRATIONS_JELLYFIN_URL=http://jellyfin:8096
export OXICLEANARR_INTEGRATIONS_JELLYFIN_API_KEY=your-key
export OXICLEANARR_INTEGRATIONS_RADARR_URL=http://radarr:7878
export OXICLEANARR_INTEGRATIONS_RADARR_API_KEY=your-key

# JWT
export JWT_SECRET=your-secret-key-min-32-chars
export JWT_EXPIRATION=24h

# Logging
export LOG_LEVEL=debug
export LOG_FORMAT=pretty

# Paths
export CONFIG_PATH=/custom/path/config.yaml
export DATA_PATH=/custom/path/data
```

## Configuration Validation

OxiCleanarr validates configuration on startup:

**Validates:**
- ✅ Admin credentials present
- ✅ At least one integration enabled
- ✅ Valid URLs for enabled integrations
- ✅ API keys provided for enabled integrations
- ✅ Valid duration formats (`30d`, `1h`, `never`)
- ✅ Port ranges valid (1-65535)

**Fails fast with clear messages:**
```
ERROR: Configuration validation failed
  - integrations.jellyfin.url: must be a valid URL (got: "not-a-url")
  - integrations.radarr.api_key: required when enabled=true
  - rules.movie_retention: invalid duration format "30 days" (use "30d")
```

## Hot-Reload

Configuration changes are automatically detected and applied:

1. Edit `config/config.yaml`
2. Save the file
3. OxiCleanarr detects change (via fsnotify)
4. Reloads and validates new config
5. Applies changes without restart

**Limitations:**
- Server host/port changes require restart
- JWT secret changes require restart

## Example Configurations

### Simple Home Setup

```yaml
admin:
  username: admin
  password: strong-password-here

integrations:
  jellyfin:
    enabled: true
    url: http://jellyfin:8096
    api_key: jellyfin-key
  
  radarr:
    enabled: true
    url: http://radarr:7878
    api_key: radarr-key
  
  sonarr:
    enabled: true
    url: http://sonarr:8989
    api_key: sonarr-key

rules:
  movie_retention: 90d
  tv_retention: 120d
```

### Advanced Setup with All Features

```yaml
admin:
  username: admin
  password: strong-password-here

app:
  dry_run: false
  enable_deletion: true
  leaving_soon_days: 14

sync:
  full_interval: 7200
  incremental_interval: 600
  auto_start: true

rules:
  movie_retention: 60d
  tv_retention: 90d

advanced_rules:
  - name: Keep Forever
    type: tag
    enabled: true
    tag: keep
    retention: never
  
  - name: Auto Clean Watched
    type: watched
    enabled: true
    retention: 30d
    require_watched: true
  
  - name: Guest Users
    type: user
    enabled: true
    users:
      - email: guest@example.com
        retention: 7d

integrations:
  jellyfin:
    enabled: true
    url: http://jellyfin:8096
    api_key: jellyfin-key
    symlink_library:
      enabled: true
      base_path: /data/media/leaving-soon
  
  radarr:
    enabled: true
    url: http://radarr:7878
    api_key: radarr-key
  
  sonarr:
    enabled: true
    url: http://sonarr:8989
    api_key: sonarr-key
  
  jellyseerr:
    enabled: true
    url: http://jellyseerr:5055
    api_key: jellyseerr-key
  
  jellystat:
    enabled: true
    url: http://jellystat:3000
    api_key: jellystat-key
```

## Best Practices

1. **Start with dry_run: true**
   - Test your configuration safely
   - Review scheduled deletions
   - Verify rules work as expected

2. **Use specific retention periods**
   - Don't use generic values
   - Consider your storage capacity
   - Match your viewing habits

3. **Protect your config**
   - `chmod 600 config/config.yaml`
   - Never commit to version control
   - Use strong passwords

4. **Enable optional integrations**
   - Jellyseerr for request tracking
   - Jellystat for watch history
   - Unlock advanced features

5. **Monitor initially**
   - Check logs regularly at first
   - Review Timeline page daily
   - Adjust rules as needed

## Troubleshooting

See [Troubleshooting](Troubleshooting) for help with configuration issues.

## Next Steps

- Learn about [Advanced Rules](Advanced-Rules)
- Set up [Leaving Soon Library](Leaving-Soon-Library)
- Review [API Reference](API-Reference)
