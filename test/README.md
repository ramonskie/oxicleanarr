# OxiCleanarr Integration Tests

This directory contains integration tests for OxiCleanarr that validate end-to-end functionality with real service containers (Jellyfin, Radarr, Sonarr).

---

## Directory Structure

```
test/
├── README.md                          # This file
├── assets/                            # Test resources
│   ├── docker-compose.yml            # Service containers (Jellyfin, Radarr, OxiCleanarr)
│   ├── config/
│   │   └── config.yaml               # Test configuration (empty API keys by design)
│   └── test-media/
│       └── movies/                   # 7 sample movies with .mkv + .nfo files
│           ├── Fight Club (1999)/
│           ├── Forrest Gump (1994)/
│           ├── Inception (2010)/
│           ├── Interstellar (2014)/
│           ├── Pulp Fiction (1994)/
│           ├── Schindler's List (1993)/
│           └── The Dark Knight (2008)/
└── integration/                       # Integration test code
    ├── helpers.go                    # HTTP helpers, container detection, config updates
    ├── jellyfin_setup.go             # Jellyfin user/library/plugin setup functions
    ├── radarr_setup_test.go          # Radarr quality profiles, root folders, movie import
    ├── setup_test.go                 # 21-step infrastructure validation test
    └── symlink_lifecycle_test.go     # Symlink library lifecycle tests (to be implemented)
```

---

## Prerequisites

### Required Software
- **Docker** (20.10+) and **Docker Compose** (v2.0+)
- **Go** (1.23+)
- **Make** (optional, for convenience commands)

### Required Docker Images
The tests will automatically pull these images:
- `jellyfin/jellyfin:latest` - Media server
- `linuxserver/radarr:latest` - Movie management
- `oxicleanarr:latest` - OxiCleanarr (must be built locally first)

---

## Quick Start

### 1. Build OxiCleanarr Docker Image

Before running integration tests, build the OxiCleanarr Docker image:

```bash
# From project root
docker build -t oxicleanarr:latest .
```

Verify the image exists:
```bash
docker images | grep oxicleanarr
# Should show: oxicleanarr   latest   <image-id>   <time>   ~19MB
```

### 2. Start Test Containers

```bash
cd test/assets
docker-compose up -d
```

Wait for containers to be ready (~30-60 seconds):
```bash
docker-compose ps
# All containers should show "Up" status
```

### 3. Run Integration Test Suite

The tests now use a **Test Suite Pattern** where both infrastructure and lifecycle tests run together as subtests. This ensures proper environment sharing and cleanup:

```bash
# From project root - Run complete test suite (recommended)
go test -v ./test/integration/ -run TestIntegrationSuite
```

**Expected Output**: 
- ✅ Infrastructure setup (21 validation steps)
- ✅ Symlink lifecycle tests (Phase 1 & Phase 2)

If the test fails, check the troubleshooting section below.

### 4. Run Individual Tests (Advanced)

You can run individual test components, but note that lifecycle tests depend on infrastructure being set up first:

```bash
# Run only infrastructure setup (as a subtest)
go test -v ./test/integration/ -run TestIntegrationSuite/InfrastructureSetup

# Run lifecycle tests (requires infrastructure to be already set up)
go test -v ./test/integration/ -run TestIntegrationSuite/SymlinkLifecycle
```

**⚠️ Important**: Running `TestIntegrationSuite/SymlinkLifecycle` standalone will fail because it depends on infrastructure being set up first. Always run the full suite with `TestIntegrationSuite` for reliable results.

---

## Integration Test Workflow

### How the Tests Work

1. **Container Detection**:
   - Tests detect if running inside OxiCleanarr container (checks for `/app/config/config.yaml`)
   - If inside container: Use internal Docker network URLs (e.g., `http://jellyfin:8096`)
   - If on host: Use localhost URLs (e.g., `http://localhost:8096`)

2. **API Key Discovery**:
   - Test config (`test/assets/config/config.yaml`) has empty API keys by design (security)
   - Tests query Jellyfin/Radarr APIs to discover their API keys at runtime
   - `UpdateConfigAPIKeys()` writes discovered keys to config file
   - OxiCleanarr reloads config and uses correct keys for sync

3. **Infrastructure Setup** (21 steps in `setup_test.go`):
   - Verify Jellyfin container running and API accessible
   - Create test user and generate API key
   - Create media library and scan for movies
   - Verify OxiCleanarr Bridge plugin installed and active
   - Verify plugin API endpoint functional
   - Verify Radarr container running and API accessible
   - Create quality profile and root folder
   - Import test movies (7 movies total)
   - Verify OxiCleanarr container running and API accessible
   - Validate network connectivity between containers
   - Verify all integrations enabled in config
   - Verify data consistency across all services

4. **Lifecycle Testing** (to be implemented in `symlink_lifecycle_test.go`):
   - Test symlink creation when media scheduled for deletion
   - Test symlink cleanup when retention rules change
   - Test Jellyfin library creation/deletion via plugin API
   - Test edge cases: missing files, permission errors, concurrent syncs

---

## Infrastructure Validation Steps

The `TestInfrastructureSetup` function performs these 21 validation steps:

### Jellyfin Setup (Steps 1-7)
1. ✅ Jellyfin container running and reachable
2. ✅ Jellyfin public API accessible
3. ✅ Admin credentials work
4. ✅ Test user 'testuser' created successfully
5. ✅ Test user API key generated
6. ✅ Test media library created
7a. ✅ Test media directory scanned (7 movies found)
7b. ✅ OxiCleanarr Bridge plugin verified (version, Active status)
7c. ✅ OxiCleanarr Bridge plugin API endpoint functional

### Radarr Setup (Steps 8-12)
8. ✅ Radarr container running and reachable
9. ✅ Radarr API accessible
10. ✅ Quality profile created
11. ✅ Root folder configured
12. ✅ Test movies imported (7 total)

### OxiCleanarr Setup (Steps 13-15)
13. ✅ OxiCleanarr container running and reachable
14. ✅ OxiCleanarr API accessible
15. ✅ OxiCleanarr config valid

### Integration Validation (Steps 16-21)
16. ✅ Network connectivity validated (container IPs)
17. ✅ All integrations enabled in config
18. ✅ Symlink library feature enabled
19. ✅ Leaving-soon base path configured
20. ✅ Data consistency validated (all services report 7 movies)
21. ✅ Infrastructure ready for lifecycle tests

---

## Environment Variables

### Optional (for debugging)

- **`TEST_JELLYFIN_URL`** - Override Jellyfin URL (default: auto-detected)
- **`TEST_RADARR_URL`** - Override Radarr URL (default: auto-detected)
- **`TEST_OXICLEANARR_URL`** - Override OxiCleanarr URL (default: auto-detected)

Example:
```bash
TEST_JELLYFIN_URL=http://localhost:8096 \
go test -v ./test/integration/ -run TestIntegrationSuite/InfrastructureSetup
```

---

## Configuration

### Test Config File

Location: `test/assets/config/config.yaml`

**Important**: API keys are empty by design (security best practice)
- Keys are discovered at runtime from running containers
- `UpdateConfigAPIKeys()` populates keys dynamically during tests
- Never commit real API keys to version control

Example structure:
```yaml
admin:
  username: admin
  password: admin123
  disable_auth: false

integrations:
  jellyfin:
    enabled: true
    url: http://jellyfin:8096
    api_key: ""  # Populated at runtime

  radarr:
    enabled: true
    url: http://radarr:7878
    api_key: ""  # Populated at runtime

  # ... other integrations
```

### Docker Compose Configuration

Location: `test/assets/docker-compose.yml`

Key settings:
- **Network**: `oxicleanarr-test` (bridge network for container communication)
- **Volumes**: `./test-media:/data/media` (shared test media)
- **Ports**: Exposed for host access (Jellyfin: 8096, Radarr: 7878, OxiCleanarr: 8080)

---

## Troubleshooting

### Container Issues

#### Containers won't start
```bash
cd test/assets
docker-compose down -v  # Remove containers and volumes
docker-compose up -d    # Restart fresh
docker-compose logs     # Check for errors
```

#### Port conflicts
If ports 8096, 7878, or 8080 are in use:
```bash
# Check what's using the port
sudo lsof -i :8096
sudo lsof -i :7878
sudo lsof -i :8080

# Stop conflicting services or modify docker-compose.yml ports
```

#### OxiCleanarr image not found
```bash
# Build the image first
docker build -t oxicleanarr:latest .

# Verify it exists
docker images | grep oxicleanarr
```

### Test Failures

#### Step 7b: Plugin verification failed
**Error**: "OxiCleanarr Bridge plugin not found in Jellyfin"

**Solution**: Install the plugin manually:
1. Open Jellyfin UI: http://localhost:8096
2. Go to Dashboard → Plugins → Catalog
3. Search for "OxiCleanarr Bridge"
4. Install and restart Jellyfin
5. Verify status shows "Active"

#### Step 7c: Plugin API endpoint not functional
**Error**: "Plugin API endpoint returned non-200 status"

**Solution**:
```bash
# Test the endpoint manually
curl -H "X-Emby-Token: <api-key>" http://localhost:8096/api/oxicleanarr/status

# If 404: Plugin not fully loaded, restart Jellyfin
docker-compose restart jellyfin

# Wait 30 seconds and retry test
```

#### Step 12: Movies not imported
**Error**: "Expected 7 movies in Radarr, got 0"

**Solution**:
```bash
# Check if media files are accessible
docker exec radarr ls -la /data/media/movies/

# Check Radarr logs
docker logs radarr | grep -i error

# Manually trigger import in Radarr UI
open http://localhost:7878
# Go to Settings → Media Management → Import Lists → Manual Import
```

#### Step 20: Data consistency validation failed
**Error**: "Services report different movie counts"

**Solution**:
```bash
# Check each service's count
curl http://localhost:7878/api/v3/movie?apiKey=<key> | jq '. | length'  # Radarr
curl http://localhost:8096/Items?apiKey=<key>&IncludeItemTypes=Movie | jq '.TotalRecordCount'  # Jellyfin
curl http://localhost:8080/api/media/movies | jq '.items | length'  # OxiCleanarr

# If counts differ, trigger manual sync
curl -X POST http://localhost:8080/api/sync/full
```

### Network Issues

#### Container-to-container communication fails
```bash
# Test network connectivity
docker exec oxicleanarr ping jellyfin
docker exec oxicleanarr ping radarr

# Verify all on same network
docker network inspect oxicleanarr-test

# Check container DNS resolution
docker exec oxicleanarr nslookup jellyfin
```

#### Host-to-container communication fails
```bash
# Verify ports are exposed
docker-compose ps

# Test each service
curl http://localhost:8096/health  # Jellyfin
curl http://localhost:7878/ping    # Radarr
curl http://localhost:8080/api/health  # OxiCleanarr
```

---

## Running Tests in Different Modes

### Run All Integration Tests
```bash
go test -v ./test/integration/
```

### Run Specific Test
```bash
go test -v ./test/integration/ -run TestIntegrationSuite/InfrastructureSetup
go test -v ./test/integration/ -run TestIntegrationSuite/SymlinkLifecycle
```

### Run with Verbose Output
```bash
go test -v ./test/integration/ -run TestIntegrationSuite/InfrastructureSetup 2>&1 | tee test-output.log
```

### Run with Race Detection
```bash
go test -v -race ./test/integration/
```

### Run with Coverage
```bash
go test -v -cover ./test/integration/
```

---

## Cleaning Up

### Stop Containers (Keep Data)
```bash
cd test/assets
docker-compose down
```

### Stop Containers and Remove Volumes (Fresh Start)
```bash
cd test/assets
docker-compose down -v
```

### Remove Test Images
```bash
docker rmi oxicleanarr:latest
docker rmi jellyfin/jellyfin:latest
docker rmi linuxserver/radarr:latest
```

### Clean Up All Test Resources
```bash
cd test/assets
docker-compose down -v
docker network rm oxicleanarr-test 2>/dev/null || true
rm -f config/config.yaml  # Regenerated on next run
```

---

## Adding New Tests

### Test File Structure

Integration tests should follow this pattern:

```go
package integration_test

import (
    "testing"
)

func TestMyFeature(t *testing.T) {
    // Detect environment (inside container vs host)
    jellyfinURL, radarrURL, oxiURL := detectEnvironment()

    // Test setup
    // ...

    // Test cases
    t.Run("SubTest1", func(t *testing.T) {
        // Test logic
    })

    t.Run("SubTest2", func(t *testing.T) {
        // Test logic
    })
}
```

### Helper Functions Available

See `test/integration/helpers.go` for available helpers:

- **HTTP Helpers**: `httpGet()`, `httpPost()`, `httpDelete()`
- **Container Detection**: `isRunningInContainer()`, `detectEnvironment()`
- **Config Management**: `UpdateConfigAPIKeys()`, `ReadConfig()`, `WriteConfig()`
- **Service Queries**: `GetRadarrMovieCount()`, `GetJellyfinMovieCount()`
- **Waiters**: `waitForContainer()`, `waitForSync()`

### Best Practices

1. **Always skip when flag not set**:
   ```go
   if !isIntegrationTest() {
       t.Skip("...")
   }
   ```

2. **Use subtests for clarity**:
   ```go
   t.Run("descriptive_name", func(t *testing.T) { ... })
   ```

3. **Clean up after tests**:
   ```go
   defer cleanupTestData(t)
   ```

4. **Use fatal for setup failures**:
   ```go
   if err != nil {
       t.Fatalf("Setup failed: %v", err)
   }
   ```

5. **Use error for test failures**:
   ```go
   if got != want {
       t.Errorf("Got %v, want %v", got, want)
   }
   ```

---

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Integration Tests

on: [push, pull_request]

jobs:
  integration:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.23'
      
      - name: Build Docker image
        run: docker build -t oxicleanarr:latest .
      
      - name: Start test containers
        run: |
          cd test/assets
          docker-compose up -d
          sleep 60  # Wait for services to be ready
      
      - name: Run integration tests
        run: go test -v ./test/integration/
      
      - name: Cleanup
        if: always()
        run: |
          cd test/assets
          docker-compose down -v
```

---

## Future Test Scenarios

### Symlink Lifecycle Tests (To Be Implemented)

Planned test cases in `test/integration/symlink_lifecycle_test.go`:

1. **Test Symlink Creation**:
   - Set low retention (immediate deletion)
   - Trigger sync
   - Verify symlinks created in `/data/media/leaving-soon/movies/`
   - Verify Jellyfin library appears via plugin API

2. **Test Symlink Cleanup**:
   - Set high retention (no deletions)
   - Trigger sync
   - Verify symlinks removed
   - Verify Jellyfin library removed (if `hide_when_empty: true`)

3. **Test Edge Cases**:
   - Missing source files (symlink creation should skip)
   - Permission errors (graceful error handling)
   - Concurrent syncs (thread safety)
   - Plugin API failures (fallback behavior)

4. **Test Configuration Changes**:
   - Change `base_path` (symlinks recreated in new location)
   - Toggle `enabled: false` (symlinks cleaned up)
   - Change `hide_when_empty` (library visibility)

---

## Support

### Getting Help

If you encounter issues not covered in this guide:

1. **Check logs**:
   ```bash
   docker-compose logs jellyfin
   docker-compose logs radarr
   docker-compose logs oxicleanarr
   ```

2. **Check container health**:
   ```bash
   docker-compose ps
   docker exec jellyfin curl http://localhost:8096/health
   docker exec radarr curl http://localhost:7878/ping
   ```

3. **Enable debug logging**:
   Edit `test/assets/config/config.yaml`:
   ```yaml
   app:
     log_level: debug
   ```

4. **Report issues**:
   - GitHub Issues: https://github.com/sst/opencode
   - Include: error messages, logs, test output, docker-compose ps output

---

## Contributing

When contributing integration tests:

1. Follow existing test patterns (see `setup_test.go` as reference)
2. Document new test scenarios in this README
3. Ensure tests pass locally before submitting PR
4. Add troubleshooting tips for common failures
5. Update helper functions if needed (helpers.go)

---

**Last Updated**: Nov 9, 2025  
**OxiCleanarr Version**: v1.3.0+  
**Test Framework**: Go testing package + Docker Compose
