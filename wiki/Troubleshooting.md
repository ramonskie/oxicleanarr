# Troubleshooting

Common issues and solutions for OxiCleanarr.

## Installation Issues

### OxiCleanarr won't start

**Symptoms:**
- Container exits immediately
- Application crashes on startup

**Solutions:**

1. **Check configuration file exists**
   ```bash
   ls -la config/config.yaml
   ```

2. **Validate YAML syntax**
   ```bash
   # Use a YAML validator
   yamllint config/config.yaml
   ```

3. **Check Docker logs**
   ```bash
   docker logs oxicleanarr
   ```

4. **Verify file permissions**
   ```bash
   chmod 600 config/config.yaml
   chown 1000:1000 config/config.yaml
   ```

### Bridge Plugin not found

**Symptoms:**
- Error: "OxiCleanarr Bridge plugin not found"
- Symlink operations fail

**Solutions:**

1. **Verify plugin installed in Jellyfin**
   - Dashboard → Plugins
   - Look for "OxiCleanarr Bridge"
   - Should show as Active

2. **Restart Jellyfin after installation**
   ```bash
   docker restart jellyfin
   ```

3. **Check plugin repository URL**
   - Should be: `https://cdn.jsdelivr.net/gh/ramonskie/jellyfin-plugin-oxicleanarr@main/manifest.json`

4. **Manual plugin installation**
   - See [plugin repository](https://github.com/ramonskie/jellyfin-plugin-oxicleanarr)

## Configuration Issues

### "Configuration validation failed"

**Common errors and fixes:**

**Error: `invalid URL`**
```yaml
# ❌ Wrong
url: jellyfin:8096

# ✅ Correct
url: http://jellyfin:8096
```

**Error: `api_key required when enabled=true`**
```yaml
# ❌ Wrong
radarr:
  enabled: true
  url: http://radarr:7878
  # api_key missing!

# ✅ Correct
radarr:
  enabled: true
  url: http://radarr:7878
  api_key: your-api-key-here
```

**Error: `invalid duration format`**
```yaml
# ❌ Wrong
movie_retention: 30 days
movie_retention: 2 months

# ✅ Correct
movie_retention: 30d
movie_retention: 60d
```

### Can't login to web UI

**Solutions:**

1. **Verify credentials in config**
   ```yaml
   admin:
     username: admin
     password: changeme
   ```

2. **Check if password was auto-hashed**
   - If password looks like `$2a$12$...`, it's hashed
   - Use the original password you set before hashing

3. **Reset password**
   - Edit `config/config.yaml`
   - Set new plain-text password
   - Restart OxiCleanarr (will auto-hash on start)

4. **Disable auth for testing (development only)**
   ```yaml
   admin:
     disable_auth: true  # ⚠️ NEVER in production!
   ```

### Configuration not hot-reloading

**Solutions:**

1. **Check file watcher is working**
   ```bash
   # Edit config and watch logs
   docker logs -f oxicleanarr
   # Should see: "Configuration reloaded"
   ```

2. **Trigger manual reload**
   ```bash
   curl -X POST -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/config/reload
   ```

3. **Restart if needed**
   ```bash
   docker restart oxicleanarr
   ```

## Integration Issues

### Jellyfin integration failing

**Symptoms:**
- Red indicator on Dashboard
- Error: "Failed to connect to Jellyfin"

**Solutions:**

1. **Verify Jellyfin is running**
   ```bash
   curl http://jellyfin:8096/System/Info/Public
   ```

2. **Check API key is valid**
   - Jellyfin → Dashboard → API Keys
   - Verify key matches config

3. **Test from container**
   ```bash
   docker exec oxicleanarr curl http://jellyfin:8096/System/Info/Public
   ```

4. **Check network connectivity**
   - Ensure containers on same network
   - Check `network_mode` in docker-compose

### Radarr/Sonarr not syncing

**Symptoms:**
- No movies/shows showing in library
- Sync completes but 0 items

**Solutions:**

1. **Verify API key**
   - Settings → General → API Key
   - Must match config exactly

2. **Test API connection**
   ```bash
   curl -H "X-Api-Key: YOUR-API-KEY" \
     http://radarr:7878/api/v3/movie
   ```

3. **Check URL format**
   ```yaml
   # ✅ Correct
   url: http://radarr:7878
   
   # ❌ Wrong (trailing slash)
   url: http://radarr:7878/
   ```

4. **Trigger manual sync**
   - Dashboard → "Sync Now"
   - Watch logs for errors

### Jellyseerr/Jellystat not connecting

**Symptoms:**
- Optional integrations showing red
- Request/watch data not syncing

**Solutions:**

1. **Verify integration is enabled**
   ```yaml
   jellyseerr:
     enabled: true  # Must be true
   ```

2. **Check URL and API key**
   - Test with curl from container

3. **Review logs for specific errors**
   ```bash
   docker logs oxicleanarr 2>&1 | grep -i jellyseerr
   docker logs oxicleanarr 2>&1 | grep -i jellystat
   ```

## Symlink/Library Issues

### No symlinks created

**Symptoms:**
- "Leaving Soon" directories empty
- No errors in logs

**Solutions:**

1. **Verify symlink_library enabled**
   ```yaml
   jellyfin:
     symlink_library:
       enabled: true  # Must be true
   ```

2. **Check base_path exists and is writable**
   ```bash
   # Inside OxiCleanarr container
   docker exec oxicleanarr ls -la /data/media/leaving-soon/
   ```

3. **Verify items are actually "leaving soon"**
   - Check Timeline page
   - Items must be within `leaving_soon_days` window

4. **Trigger manual sync**
   - Dashboard → "Sync Now"
   - Check logs for symlink creation

### Jellyfin libraries empty

**Symptoms:**
- "Leaving Soon - Movies" library exists but shows 0 items
- Jellyfin can't see symlinks

**Solutions:**

1. **Verify Jellyfin has access to symlink directory**
   
   **If using recommended `base_path: /data/media/leaving-soon`:**
   ```yaml
   # Jellyfin docker-compose.yml
   volumes:
     - /volume1/data/media:/data/media:ro  # Includes leaving-soon/ ✅
   ```

   **If using separate directory `base_path: /app/leaving-soon`:**
   ```yaml
   # Jellyfin docker-compose.yml
   volumes:
     - /volume1/data/media:/data/media:ro
     - /volume3/docker/oxicleanarr/leaving-soon:/app/leaving-soon:ro  # Extra mount
   ```

2. **Test access from Jellyfin container**
   ```bash
   # Check symlinks visible
   docker exec jellyfin ls -la /data/media/leaving-soon/movies/
   
   # Check symlink targets accessible
   docker exec jellyfin ls -la /data/media/movies/
   ```

3. **Refresh Jellyfin library**
   - Jellyfin → Dashboard → Libraries
   - Click "Scan All Libraries"

4. **Recreate Virtual Folders**
   - Delete and recreate "Leaving Soon" libraries
   - Point to correct paths

### Permission denied errors

**Symptoms:**
- Error: "Permission denied" in logs
- Can't write to directories

**Solutions:**

1. **Check PUID/PGID**
   ```yaml
   environment:
     - PUID=1000  # Match your user ID
     - PGID=1000  # Match your group ID
   ```

2. **Fix directory ownership**
   ```bash
   sudo chown -R 1000:1000 /path/to/oxicleanarr/
   ```

3. **Add SELinux labels (Fedora/RHEL)**
   ```yaml
   volumes:
     - ./config:/app/config:z
     - ./data:/app/data:z
   ```

4. **Check directory mount type**
   ```yaml
   # ❌ Wrong - File mount prevents ownership changes
   volumes:
     - ./config.yaml:/app/config/config.yaml
   
   # ✅ Correct - Directory mount
   volumes:
     - ./config:/app/config
   ```

## Deletion Issues

### Items not being deleted

**Symptoms:**
- Retention period passed but item still exists
- Timeline shows items but no deletion

**Solutions:**

1. **Check dry_run mode**
   ```yaml
   app:
     dry_run: false           # Must be false
     enable_deletion: true    # Must be true
   ```

2. **Verify item not excluded**
   - Check for Shield icon in UI
   - Check `data/exclusions.json`

3. **Check retention rules**
   - Review Timeline page for deletion date
   - Verify rules match expectations

4. **Trigger manual sync**
   - Deletions occur during sync
   - Dashboard → "Sync Now"

### Items deleted unexpectedly

**Symptoms:**
- Media disappeared without warning
- Unexpected deletions

**Solutions:**

1. **Check job history**
   - Job History page
   - Review deletion details

2. **Review retention rules**
   - Check `rules` and `advanced_rules`
   - Verify retention periods

3. **Enable dry_run to prevent further deletions**
   ```yaml
   app:
     dry_run: true
   ```

4. **Add exclusions**
   - Use Shield button in UI
   - Or manually edit `data/exclusions.json`

## Performance Issues

### Slow sync operations

**Symptoms:**
- Sync takes very long (>5 minutes for 1000 items)
- High CPU/memory usage

**Solutions:**

1. **Increase sync intervals**
   ```yaml
   sync:
     full_interval: 7200        # 2 hours instead of 1
     incremental_interval: 1800  # 30 min instead of 15
   ```

2. **Check integration timeouts**
   ```yaml
   integrations:
     jellyfin:
       timeout: 60s  # Increase if needed
   ```

3. **Review logs for errors**
   - Multiple retries indicate networking issues
   - Check integration health

### High memory usage

**Symptoms:**
- Container using >100MB RAM
- Out of memory errors

**Solutions:**

1. **Reduce cache TTL** (future feature)
2. **Increase container memory limit**
   ```yaml
   deploy:
     resources:
       limits:
         memory: 128M
   ```

3. **Report issue** with library size and logs

## Data Issues

### Exclusions not persisting

**Symptoms:**
- Excluded items reset after sync
- Shield icon doesn't stay active

**Solution:**

This was a bug fixed in recent versions. Update to latest:

```bash
docker pull ghcr.io/ramonskie/oxicleanarr:latest
docker-compose up -d
```

### Job history missing

**Symptoms:**
- Job History page empty
- No sync records

**Solutions:**

1. **Check data directory**
   ```bash
   ls -la data/jobs.json
   ```

2. **Verify write permissions**
   ```bash
   docker exec oxicleanarr touch /app/data/test && \
   docker exec oxicleanarr rm /app/data/test
   ```

3. **Check logs for errors**
   ```bash
   docker logs oxicleanarr 2>&1 | grep -i "jobs.json"
   ```

## API Issues

### 401 Unauthorized errors

**Symptoms:**
- API returns 401
- "Token validation failed"

**Solutions:**

1. **Get fresh token**
   ```bash
   TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
     -H "Content-Type: application/json" \
     -d '{"username":"admin","password":"changeme"}' \
     | jq -r '.token')
   ```

2. **Check token in request**
   ```bash
   curl -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/sync/status
   ```

3. **Verify JWT secret not changed**
   - Changing JWT_SECRET invalidates all tokens
   - Login again if secret changed

### CORS errors in browser

**Symptoms:**
- Browser console shows CORS errors
- Frontend can't reach backend

**Solutions:**

1. **Access UI via same host as API**
   - Use `http://localhost:8080` not `http://127.0.0.1:8080`

2. **Check CORS middleware enabled**
   - Should be enabled by default
   - Report issue if not working

## Log Analysis

### Enable debug logging

```bash
export LOG_LEVEL=debug
export LOG_FORMAT=pretty
docker-compose up -d
```

### Useful log filters

```bash
# Connection errors
docker logs oxicleanarr 2>&1 | grep -i "error"
docker logs oxicleanarr 2>&1 | grep -i "fail"

# Sync operations
docker logs oxicleanarr 2>&1 | grep -i "sync"

# Symlink operations
docker logs oxicleanarr 2>&1 | grep -i "symlink"

# Rule evaluation
docker logs oxicleanarr 2>&1 | grep -i "retention"
docker logs oxicleanarr 2>&1 | grep -i "rule"
```

## Getting Help

If issues persist:

1. **Enable debug logging**
2. **Collect information:**
   - OxiCleanarr version
   - Configuration (redact API keys!)
   - Docker logs
   - Integration versions (Jellyfin, Radarr, Sonarr)

3. **Check existing issues:**
   - [GitHub Issues](https://github.com/ramonskie/oxicleanarr/issues)

4. **Open new issue with:**
   - Clear description of problem
   - Steps to reproduce
   - Logs and configuration
   - Screenshots if applicable

## Common Error Messages

### "Failed to load configuration"
- Config file missing or invalid YAML syntax
- Check file exists and is readable

### "At least one integration must be enabled"
- All integrations are `enabled: false`
- Enable at least Jellyfin, Radarr, or Sonarr

### "OxiCleanarr Bridge plugin not found"
- Plugin not installed in Jellyfin
- See [Installation Guide](Installation-Guide#install-the-bridge-plugin-first)

### "Permission denied: /app/data/jobs.json"
- Volume mount issue
- Check PUID/PGID and directory ownership

### "Invalid JWT token"
- Token expired (default 24h)
- Login again to get fresh token

### "Connection refused"
- Integration service not accessible
- Check URLs and network connectivity

## Related Documentation

- [Installation Guide](Installation-Guide)
- [Configuration](Configuration)
- [FAQ](FAQ)
- [Development Guide](Development-Guide)
