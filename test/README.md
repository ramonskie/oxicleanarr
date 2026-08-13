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
    ├── leaving_soon_lifecycle_test.go # Leaving Soon plugin lifecycle tests
    ├── leaving_soon_constants.go     # Leaving Soon plugin test constants
    └── constants_test.go             # Shared test constants
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
- ✅ Leaving Soon plugin lifecycle tests (Phase 1 & Phase 2)

If the test fails, check the troubleshooting section below.

### 4. Run Individual Tests (Advanced)

You can run individual test components. The suite automatically handles infrastructure setup:

```bash
# Run only infrastructure setup (as a subtest)
go test -v ./test/integration/ -run TestIntegrationSuite/InfrastructureSetup

# Run lifecycle tests (auto-runs setup if needed)
go test -v ./test/integration/ -run TestIntegrationSuite/LeavingSoonPluginLifecycle
```

**How Auto-Setup Works**:
- When running the full suite, infrastructure is set up once and shared across all subtests
- When running a filtered subtest (e.g., `-run TestIntegrationSuite/LeavingSoonPluginLifecycle`), the suite detects that infrastructure setup was skipped and automatically runs it before executing the test
- This uses an `infrastructureReady` flag to track whether setup has completed in the current test session
- **Result**: You can run individual subtests without manual setup, while maintaining efficiency when running the full suite

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
   - Verify Radarr container running and API accessible
   - Create quality profile and root folder
   - Import test movies (7 movies total)
   - Verify OxiCleanarr container running and API accessible
   - Validate network connectivity between containers
   - Verify all integrations enabled in config
   - Verify data consistency across all services

4. **Leaving Soon plugin lifecycle** (`leaving_soon_lifecycle_test.go`):
   - Installs jellyfin-plugin-leaving-soon into the Jellyfin container
   - Phase 1: 7d retention → OxiCleanarr exposes leaving-soon items → plugin creates
     symlinks + a Jellyfin library
   - Phase 2: 0d retention → nothing leaving → plugin removes symlinks and the library
     (hide_when_empty + double refresh)

5. **Auto-Setup for Filtered Tests**:
   - When running the full suite, infrastructure is set up once and shared
   - When running a filtered subtest (e.g., `-run TestIntegrationSuite/LeavingSoonPluginLifecycle`), the suite automatically detects that infrastructure setup was skipped by Go's test runner
   - Implementation uses `infrastructureReady` flag (package-level variable)
   - Full suite: `InfrastructureSetup` sets flag to `true` → subsequent tests skip setup
   - Filtered run: Flag stays `false` → test wrapper runs setup automatically
   - This allows running individual tests without manual setup while maintaining efficiency

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
7. ✅ Test media directory scanned (7 movies found)

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
18. ✅ Data consistency validated (all services report 7 movies)
19. ✅ Infrastructure ready for lifecycle tests

---

## Technical Details: Test Suite Architecture

### The Problem with Go's `-run` Filter

When you run `go test -run TestIntegrationSuite/LeavingSoonPluginLifecycle`, Go's test runner:
1. Matches `TestIntegrationSuite` function and enters it
2. Evaluates subtest filter `/LeavingSoonPluginLifecycle`
3. **Skips** `t.Run("InfrastructureSetup", ...)` - doesn't match filter
4. **Runs** `t.Run("LeavingSoonPluginLifecycle", ...)` - matches filter

This causes LeavingSoonPluginLifecycle to run without infrastructure being set up first.

### The Solution: Auto-Setup Detection

The suite uses a package-level `infrastructureReady` flag to track setup state:

```go
// Package-level flag (shared across test functions)
var infrastructureReady = false

func TestIntegrationSuite(t *testing.T) {
    // Infrastructure subtest
    t.Run("InfrastructureSetup", func(t *testing.T) {
        testInfrastructureSetup(t)
        infrastructureReady = true  // Mark as ready
    })

    // Lifecycle subtest with auto-setup
    t.Run("LeavingSoonPluginLifecycle", func(t *testing.T) {
        // Check if setup was skipped by Go's -run filter
        if !infrastructureReady {
            t.Log("⚠️ Infrastructure not ready (filtered by -run), setting up now...")
            testInfrastructureSetup(t)
            infrastructureReady = true
        }
        testLeavingSoonLifecycle(t)  // Now safe to run
    })
}
```

### Behavior in Different Scenarios

**Scenario 1: Full Suite** (`-run TestIntegrationSuite`)
```
1. InfrastructureSetup runs → sets infrastructureReady = true
2. LeavingSoonPluginLifecycle checks flag → sees true → skips setup → uses existing infra
```
✅ Infrastructure built once, shared across all subtests

**Scenario 2: Filtered Subtest** (`-run TestIntegrationSuite/LeavingSoonPluginLifecycle`)
```
1. InfrastructureSetup skipped by Go (doesn't match filter)
2. LeavingSoonPluginLifecycle checks flag → sees false → runs setup → then runs tests
```
✅ Tests work correctly even when run individually

**Scenario 3: Infrastructure Only** (`-run TestIntegrationSuite/InfrastructureSetup`)
```
1. InfrastructureSetup runs → sets infrastructureReady = true
2. LeavingSoonPluginLifecycle skipped by Go (doesn't match filter)
```
✅ Only infrastructure validation runs

### Why This Works

- **Package-level scope**: The `infrastructureReady` flag persists across subtests within the same test session
- **Go's test runner behavior**: When a parent test function runs, all subtest wrappers execute (even if skipped), allowing our check to run
- **No environment variables**: Pure in-process state management
- **No TestMain complexity**: TestMain still manages Docker lifecycle, suite manages test dependencies

### Benefits

1. ✅ **Developer convenience**: Run any subtest directly without setup ceremony
2. ✅ **CI/CD efficiency**: Full suite shares infrastructure (no redundant rebuilds)
3. ✅ **Test isolation**: Each scenario gets the infrastructure it needs
4. ✅ **Maintainability**: Simple flag-based logic, easy to understand and debug

---

## Environment Variables

### Optional (for debugging)

- **`TEST_JELLYFIN_URL`** - Override Jellyfin URL (default: auto-detected)
- **`TEST_RADARR_URL`** - Override Radarr URL (default: auto-detected)
- **`TEST_OXICLEANARR_URL`** - Override OxiCleanarr URL (default: auto-detected)
- **`GITHUB_TOKEN`** - GitHub personal access token to avoid API rate limiting when downloading the Leaving Soon plugin

Example:
```bash
TEST_JELLYFIN_URL=http://localhost:8096 \
go test -v ./test/integration/ -run TestIntegrationSuite/InfrastructureSetup
```

### Using GITHUB_TOKEN to Avoid Rate Limiting

The tests automatically download the Leaving Soon plugin from GitHub. Without authentication, GitHub limits you to 60 API requests per hour per IP address. If you encounter rate limit errors:

```bash
# Create a GitHub personal access token (no scopes required for public repos)
# Visit: https://github.com/settings/tokens

# Set the token before running tests
export GITHUB_TOKEN=ghp_your_token_here
go test -v ./test/integration/ -run TestIntegrationSuite
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

#### Leaving Soon plugin install failed
**Error**: "Failed to install Leaving Soon plugin"

**Solution**: Check the GitHub release is reachable and the plugin zip is available:
1. Verify `https://api.github.com/repos/ramonskie/jellyfin-plugin-leaving-soon/releases/latest` returns a release with a `.zip` asset
2. Set `GITHUB_TOKEN` if hitting API rate limits
3. Check the Jellyfin container is running (`docker ps`)

#### Leaving Soon plugin sync not creating symlinks
**Error**: `WaitForContainerSymlinkCount` timeout

**Solution**: The plugin polls `GET /api/media/leaving-soon` on OxiCleanarr:
1. Verify OxiCleanarr returns items (send the `admin.api_key` as a Bearer token):
   ```bash
   curl -s -H "Authorization: Bearer test-api-key" http://localhost:9709/api/media/leaving-soon
   # Expect { "version": 1, "items": [ ... ] }
   ```
2. The test config sets `admin.api_key: test-api-key`, and the plugin installer writes
   that key into the plugin's `config.xml`, so the plugin's poll is authorized.
3. Check Jellyfin logs for plugin errors: `docker logs oxicleanarr-test-jellyfin | grep -i "leaving soon"`

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
go test -v ./test/integration/ -run TestIntegrationSuite/LeavingSoonPluginLifecycle
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

### Leaving Soon Plugin Lifecycle (Implemented)

See `test/integration/leaving_soon_lifecycle_test.go` — runs as the
`LeavingSoonPluginLifecycle` subtest in `TestIntegrationSuite`:

1. **Phase 1 - Symlink Creation**:
   - 7d retention → OxiCleanarr exposes leaving-soon items
   - Trigger plugin sync → verify symlinks created in the container's leaving-soon/movies dir
   - Verify the Jellyfin library was created

2. **Phase 2 - Symlink Cleanup**:
   - 0d retention → nothing leaving soon
   - Trigger plugin sync → verify symlinks removed
   - Verify the Jellyfin library deleted (hide_when_empty + double refresh)

3. **Potential future cases**:
   - Missing source files (symlink creation should skip)
   - Permission errors (graceful error handling)
   - Concurrent syncs (thread safety)
   - Provider outage (ForceEmptyAfterFailureCount behavior)

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

**Last Updated**: Aug 13, 2026  
**OxiCleanarr Version**: v1.3.0+  
**Test Framework**: Go testing package + Docker Compose
