# Frequently Asked Questions

Common questions about OxiCleanarr.

## General Questions

### What is OxiCleanarr?

OxiCleanarr is a lightweight media cleanup automation tool for the *arr stack (Radarr, Sonarr) with Jellyfin integration. It automatically identifies and handles media that should be cleaned up based on configurable retention rules.

### Why "OxiCleanarr"?

Just like OxiClean tackles tough stains with the power endorsed by Billy Mays, OxiCleanarr tackles your unwatched media backlog! **"But wait, there's more!"**

### Is it safe to use?

Yes! OxiCleanarr has multiple safety features:

- **Dry-run mode** (enabled by default) - No actual deletions
- **Exclusions** - Manually protect specific media from deletion
- **Timeline view** - See exactly what will be deleted and when
- **"Leaving Soon" libraries** - Preview content before deletion
- **Detailed logging** - Audit trail of all operations

Start with `dry_run: true` to test safely.

### How is it different from Janitorr?

OxiCleanarr is inspired by Janitorr but with key improvements:

- **Simpler configuration** - Sensible defaults, minimal YAML
- **Better visibility** - Timeline view, countdown timers, deletion reasons
- **Modern tech stack** - Go backend + React 19 frontend
- **API-first** - Full REST API for automation
- **Hot-reload** - Config changes without restart
- **Lightweight** - 15MB image, <40MB RAM

See [Architecture](Architecture) for technical details.

### Was this really built with AI?

Yes! ~90% of this project was created with AI assistance using [OpenCode](https://opencode.ai/) and Claude 3.5 Sonnet. It's a testament to what's possible with modern AI coding tools.

## Installation & Setup

### Do I need the Bridge Plugin?

**Yes**, the [OxiCleanarr Bridge Plugin](https://github.com/ramonskie/jellyfin-plugin-oxicleanarr) is **required** for Jellyfin integration. It provides file system operations (symlink management) that Jellyfin's native API doesn't support.

Without it, OxiCleanarr cannot create "Leaving Soon" libraries.

### What are the minimum requirements?

- Docker (recommended) or Go 1.21+
- Active Radarr and/or Sonarr instance
- Jellyfin instance with Bridge Plugin
- ~40MB RAM
- ~100MB disk space

### Can I run without Docker?

Yes! You can build from source:

```bash
git clone https://github.com/ramonskie/oxicleanarr.git
cd oxicleanarr
make build
./oxicleanarr
```

See [Installation Guide](Installation-Guide#option-2-build-from-source).

### Do I need both Radarr AND Sonarr?

No, you can use either one or both. At least one must be enabled.

### Do I need Jellyseerr and Jellystat?

No, these are **optional** integrations:

- **Jellyseerr** - Required for user-based cleanup rules
- **Jellystat** - Required for watched-based cleanup rules and watch tracking

Standard retention rules work without them.

## Configuration

### Where is the config file?

Default location: `./config/config.yaml`

Override with: `CONFIG_PATH=/path/to/config.yaml`

### How do I get API keys?

**Jellyfin:**
1. Dashboard → API Keys
2. Click "+" → Name it "OxiCleanarr"
3. Copy the key

**Radarr/Sonarr:**
1. Settings → General
2. Security section
3. Copy "API Key"

**Jellyseerr:**
1. Settings → General
2. API Key section
3. Copy the key

**Jellystat:**
1. Settings → API
2. Generate or copy existing key

### Can I use environment variables?

Yes! All config can be overridden with `OXICLEANARR_` prefix:

```bash
export OXICLEANARR_ADMIN_USERNAME=myadmin
export OXICLEANARR_ADMIN_PASSWORD=mypassword
export OXICLEANARR_INTEGRATIONS_JELLYFIN_URL=http://jellyfin:8096
```

See [Configuration](Configuration#environment-variables).

### Does the config hot-reload?

Yes! OxiCleanarr automatically detects changes to `config.yaml` and reloads without restart.

Some settings (like server host/port) require restart.

### Why is my password stored in plain text?

For simplicity and transparency. Protect your config file:

```bash
chmod 600 config/config.yaml
```

Never commit config to version control. Use a strong password.

## Features

### What are "Leaving Soon" libraries?

Special Jellyfin libraries that show media scheduled for deletion. They use symlinks to preview content before it's deleted, giving users a chance to exclude items they want to keep.

See [Leaving Soon Library](Leaving-Soon-Library) guide.

### How does the deletion timeline work?

The Timeline view shows a visual calendar of scheduled deletions, grouped by date. It helps you:

- See what's being deleted and when
- Understand deletion reasons
- Plan storage cleanup
- Take action to prevent deletion

See [Deletion Timeline](Deletion-Timeline) page.

### What are exclusions?

Exclusions are items you've marked to "Keep" forever. Click the Shield icon on any media item to exclude it from deletion. Excluded items are stored in `data/exclusions.json` and persist through syncs.

### How do advanced rules work?

Advanced rules provide fine-grained control:

- **Tag-based** - Different retention for tagged content (e.g., kids, premium)
- **User-based** - Different retention per user (e.g., trial users, guests)
- **Watched-based** - Auto-cleanup after content is watched

Rules are evaluated in priority order. See [Advanced Rules](Advanced-Rules).

## Safety & Deletion

### Will it delete my media immediately?

No! By default:

- `dry_run: true` - No deletions occur
- `enable_deletion: false` - Auto-deletion disabled

You must explicitly enable deletions:

```yaml
app:
  dry_run: false
  enable_deletion: true
```

### How can I test safely?

1. Keep `dry_run: true` in config
2. Run a sync
3. Check Timeline and Scheduled Deletions pages
4. Review what would be deleted
5. Adjust rules as needed
6. Only disable dry_run when satisfied

### Can I undo a deletion?

No, deletions are permanent. OxiCleanarr deletes from both Radarr/Sonarr AND Jellyfin. Always:

- Test with dry_run first
- Review Timeline before enabling deletions
- Use exclusions for important content
- Keep backups of important media

### What if I accidentally delete something?

You'll need to re-download it:

1. Re-add to Radarr/Sonarr
2. Download again
3. Exclude it: Click Shield icon to prevent future deletion

Prevention is key - use dry_run mode and exclusions.

### How do I protect specific media?

Click the **Shield icon** on any media item in the UI. This adds it to exclusions and it will never be deleted.

Or manually edit `data/exclusions.json`.

## Operations

### How often does it sync?

Default intervals:

- **Full sync**: Every hour (3600 seconds)
- **Incremental sync**: Every 15 minutes (900 seconds)

Customize in config:

```yaml
sync:
  full_interval: 7200      # 2 hours
  incremental_interval: 1800  # 30 minutes
```

### What's the difference between full and incremental sync?

**Full Sync:**
- Complete library refresh
- Fetches all media from all services
- Re-evaluates all rules
- Takes longer (~20-30s for 1000 items)

**Incremental Sync:**
- Quick update of recent changes
- Fetches only recent watch history
- Updates changed items only
- Fast (~2-5s)

### Can I trigger syncs manually?

Yes! Two ways:

1. **Web UI**: Dashboard → "Sync Now" button
2. **API**: `POST /api/sync/full` or `POST /api/sync/incremental`

### How do I monitor operations?

- **Dashboard** - Overall status and recent activity
- **Timeline** - Scheduled deletions by date
- **Job History** - All sync operations with details
- **Logs** - Docker logs or log files

### Where is data stored?

```
./config/         # Configuration
./data/           # Runtime data
  ├── exclusions.json  # User exclusions
  └── jobs.json        # Job history
./logs/           # Application logs (if configured)
```

## Performance

### How much resources does it use?

Typical usage:

- **RAM**: <40MB idle, <60MB during sync
- **CPU**: Minimal (only during sync)
- **Disk**: ~100MB (binary + data)
- **Network**: Minimal API calls

### Can it handle large libraries?

Yes! Designed to support 10,000+ media items with:

- Efficient caching
- Incremental syncs
- Low memory footprint

### How fast are API responses?

Target performance:

- Cached requests: <50ms
- Uncached requests: <200ms
- Startup time: <50ms

## Integrations

### Which services are supported?

**Required (at least one):**
- Radarr
- Sonarr

**Always required:**
- Jellyfin (with Bridge Plugin)

**Optional:**
- Jellyseerr (for user-based rules)
- Jellystat (for watched-based rules)

### Can I use Plex instead of Jellyfin?

Not currently. OxiCleanarr is designed specifically for Jellyfin and requires the Bridge Plugin for file operations.

### Does it work with other *arr apps?

Currently only Radarr and Sonarr. Other *arr apps (Lidarr, Readarr, etc.) are not supported.

### Can I use custom Jellyfin libraries?

Yes, you can customize library names:

```yaml
jellyfin:
  symlink_library:
    movies_library_name: "Custom Movies"
    tv_library_name: "Custom TV"
```

## Troubleshooting

### Why aren't items being deleted?

Check:

1. `dry_run: false` and `enable_deletion: true`
2. Retention period has passed
3. Item not excluded (Shield icon)
4. Deletion occurs during sync - trigger one

### Why is my library empty?

Check:

1. Integration connections (Dashboard)
2. API keys are correct
3. Trigger manual sync
4. Review logs for errors

### Why aren't symlinks working?

Check:

1. Bridge Plugin installed and active
2. `symlink_library.enabled: true`
3. Jellyfin has access to symlink paths
4. Items are actually "leaving soon"

See [Troubleshooting](Troubleshooting) for detailed help.

### How do I enable debug logging?

```bash
export LOG_LEVEL=debug
export LOG_FORMAT=pretty
docker-compose up -d
```

## API & Development

### Is there an API?

Yes! Full REST API with JWT authentication. See [API Reference](API-Reference).

Example:
```bash
# Get token
TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"changeme"}' \
  | jq -r '.token')

# Use API
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/media/movies
```

### Can I contribute?

Yes! Contributions welcome. See [Development Guide](Development-Guide).

Project uses:
- Backend: Go 1.21+
- Frontend: React 19 + Vite + TypeScript
- Build: Make, Docker

### How do I report bugs?

1. Check [existing issues](https://github.com/ramonskie/oxicleanarr/issues)
2. Open new issue with:
   - Clear description
   - Steps to reproduce
   - Logs (redact API keys!)
   - Configuration (redact API keys!)

### Where's the source code?

GitHub: [ramonskie/oxicleanarr](https://github.com/ramonskie/oxicleanarr)

### What's the license?

MIT License - free and open source.

## Advanced Topics

### Can I run multiple instances?

Yes, but:
- Use separate config/data directories
- Use different ports
- Be careful with competing deletions

### Can I customize the frontend?

Yes! The frontend is React + TypeScript. See [Development Guide](Development-Guide) for building from source.

### Can I extend with plugins?

Not currently. The codebase is modular but doesn't have a plugin system. Consider contributing features directly.

### Can I use this in production?

Yes! But:

- Test thoroughly with `dry_run: true` first
- Monitor closely initially
- Have backups of important media
- Use exclusions for irreplaceable content
- Start with conservative retention periods

### How do I backup OxiCleanarr?

Backup these directories:
- `./config/` - Configuration
- `./data/` - Exclusions and job history

Your actual media is in Radarr/Sonarr - backup those separately.

## Still Have Questions?

- Check the [Troubleshooting](Troubleshooting) guide
- Review the [Installation Guide](Installation-Guide)
- Read the [Configuration](Configuration) reference
- Browse [GitHub Issues](https://github.com/ramonskie/oxicleanarr/issues)
- Open a new issue for help
