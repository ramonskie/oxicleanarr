# Prunarr AI Agent Context

This document provides essential context for AI coding agents working on the Prunarr project. It serves as a quick reference to understand the project state, active work, and how to resume development.

---

## Project Overview

**Prunarr** is a lightweight media cleanup automation tool for the *arr stack (Sonarr, Radarr, Jellyfin). It provides intelligent retention policies, deletion visibility, and a modern web UI.

**Tech Stack:**
- **Backend**: Go 1.23+ with Chi router, Viper config, zerolog logging
- **Frontend**: React 19, Vite 6, shadcn/ui, TanStack Query, Zustand
- **Storage**: File-based (YAML config + JSON data)
- **Cache**: go-cache (in-memory)

---

## Current Project Status

### Overall Progress
- **Backend**: ~90% complete ✅
- **Frontend**: ~80% complete ✅
- **Phase**: 4 (Advanced Features & Polish) - IN PROGRESS

### What's Working
✅ Complete REST API (auth, sync, media, jobs, exclusions)  
✅ All service integrations (Jellyfin, Radarr, Sonarr, Jellyseerr, Jellystat)  
✅ Sync engine with full/incremental scheduler  
✅ Rules engine with retention policies  
✅ Deletion executor with dry-run mode  
✅ Exclusions management with persistence  
✅ Job history tracking  
✅ React UI with Dashboard, Timeline, Library, Scheduled Deletions, Job History pages  
✅ Authentication & authorization (with optional bypass for testing)  
✅ Configuration with hot-reload  
✅ Deletion reason generation  

### What's Pending
⏳ User-based cleanup with watch tracking  
⏳ Collection management (Jellyfin "Leaving Soon" collections)  
⏳ Configuration editor UI  
⏳ Advanced rules UI  
⏳ Mobile responsiveness polish  
⏳ Statistics/charts  
⏳ Comprehensive error handling  

### Testing Status
- **282 tests passing** (89 test functions total)
- **Coverage**: Handlers 89.0%, Storage 92.7%, Services 57.1%, Clients 8.2%

---

## Recent Work (Last Session - Nov 3, 2025)

### 1. Jellyfin Fallback Removal
**Problem**: Media entries were being created from Jellyfin with incorrect file sizes, causing phantom entries and confusion about data source.

**Solution**: Removed fallback logic in `internal/services/sync.go`:
- Radarr/Sonarr are now the SOLE source of truth for media library
- Jellyfin ONLY updates watch data (play counts, last watched dates)
- Deleted ~46 lines of fallback code

**Commits**: `483cb62`

### 2. UI Formatting Improvements
**Problem**: Scheduled Deletions page showed invalid dates and "0.00 GB" for unknown file sizes.

**Solution**: Enhanced formatting in `web/src/pages/ScheduledDeletionsPage.tsx`:
- Filter out zero/invalid dates (display "Unknown")
- Handle zero byte file sizes (display "Unknown")

**Commits**: `1574ca3`

---

## Key Architecture Decisions

### Data Source Hierarchy
1. **Media Library**: Radarr (movies) + Sonarr (TV shows) = SOLE source of truth
2. **Watch Data**: Jellyfin updates existing entries with play counts and timestamps
3. **Requests**: Jellyseerr tracks who requested what
4. **Watch History**: Jellystat provides detailed watch tracking per user

### Sync Strategy
- **Full Sync** (every 1 hour): Complete library refresh from all integrations
- **Incremental Sync** (every 15 minutes): Update changed items only
- **Exclusions**: Applied after sync but before retention rules
- **Flow**: Radarr/Sonarr → Jellyfin (watch data) → Jellyseerr (requests) → Apply exclusions → Apply rules

### File Structure
```
/app/
├── config/
│   └── prunarr.yaml          # Main configuration (hot-reload enabled)
├── data/
│   ├── exclusions.json       # User "Keep" exclusions
│   └── jobs.json             # Job history (circular buffer)
└── logs/
    └── prunarr.log           # Structured JSON logs
```

---

## Important Code Locations

### Backend Core
- **Sync Engine**: `internal/services/sync.go` - Orchestrates all integrations
- **Rules Engine**: `internal/services/rules.go` - Evaluates retention policies
- **Exclusions**: `internal/storage/exclusions.go` - Manages "Keep" functionality
- **Jobs**: `internal/storage/jobs.go` - Tracks sync/deletion history
- **API Handlers**: `internal/api/handlers/` - REST endpoints
- **Clients**: `internal/clients/` - External service integrations

### Frontend Core
- **API Client**: `web/src/lib/api.ts` - TanStack Query integration
- **Auth Store**: `web/src/store/auth.ts` - Zustand auth state
- **Pages**: `web/src/pages/` - Main UI views
- **Types**: `web/src/lib/types.ts` - TypeScript interfaces

### Configuration
- **Example**: `config/prunarr.yaml.example` - Template with defaults
- **Test Config**: `config/prunarr.test.yaml` - Testing configuration
- **Validation**: `internal/config/validation.go` - Config checks

---

## Development Workflow

### Running the Application
```bash
# Start development (backend + frontend)
make dev

# Build production binary
make build

# Run tests
make test

# Test API endpoints
./test-api.sh
```

### Testing Against Real Services
```bash
# Use test config with real Jellyfin/Radarr/Sonarr
./prunarr-test --config config/prunarr.test.yaml

# Check logs
tail -f /tmp/prunarr-debug.log

# Access UI
open http://localhost:8080
```

### Common Tasks
1. **Add new API endpoint**: Create handler in `internal/api/handlers/`, add route to `internal/api/router.go`
2. **Add new integration**: Create client in `internal/clients/`, integrate into `internal/services/sync.go`
3. **Add new UI page**: Create component in `web/src/pages/`, add route to `web/src/App.tsx`
4. **Modify sync logic**: Edit `internal/services/sync.go`, ensure `applyExclusions()` runs before `applyRetentionRules()`

---

## Known Issues & Gotchas

### Backend
1. **Exclusions must persist through syncs**: Always call `applyExclusions()` in `FullSync()` before `applyRetentionRules()`
2. **Source of truth**: Never create media entries from Jellyfin - only Radarr/Sonarr
3. **Zero values**: Go's zero time (`0001-01-01`) requires special handling in JSON responses
4. **File sizes**: Only trust `SizeOnDisk` from Radarr/Sonarr, not Jellyfin

### Frontend
1. **Null safety**: Always provide default empty arrays in API responses (`response.items || []`)
2. **Date validation**: Filter out zero dates before displaying
3. **File size display**: Handle zero bytes gracefully ("Unknown" instead of "0.00 GB")
4. **Auth bypass**: Only use `admin.disable_auth: true` for local development

---

## Testing Strategy

### Backend Tests
- **Unit tests**: `*_test.go` files alongside source
- **Coverage targets**: >80% for handlers, >90% for storage
- **Run**: `go test ./... -v -cover`

### Frontend Tests
- **Manual testing**: Use test-api.sh to verify endpoints
- **Browser testing**: Check all pages in dev mode
- **Run dev**: `cd web && npm run dev`

### Integration Tests
- **Real services**: Test against actual Jellyfin/Radarr/Sonarr
- **Debug logging**: Set `LOG_LEVEL=debug` in config
- **Dry-run first**: Always test with `app.dry_run: true`

---

## Configuration Examples

### Minimal Config
```yaml
admin:
  username: admin
  password: changeme

integrations:
  jellyfin:
    enabled: true
    url: http://jellyfin:8096
    api_key: your-key
  radarr:
    enabled: true
    url: http://radarr:7878
    api_key: your-key
  sonarr:
    enabled: true
    url: http://sonarr:8989
    api_key: your-key
```

### Development Config (Auth Bypass)
```yaml
admin:
  username: admin
  password: changeme
  disable_auth: true  # Skip JWT for testing

app:
  dry_run: true       # Safe mode

sync:
  auto_start: false   # Manual sync only
```

---

## API Endpoint Reference

### Authentication
- `POST /api/auth/login` - Get JWT token
- `GET /api/auth/me` - Current user info

### Dashboard
- `GET /api/dashboard/stats` - System statistics
- `GET /api/dashboard/health` - Integration health

### Media
- `GET /api/media/movies` - List all movies
- `GET /api/media/shows` - List all TV shows
- `GET /api/media/leaving-soon` - Items in deletion window
- `GET /api/media/:id` - Single media item
- `POST /api/media/:id/exclude` - Add to exclusions ("Keep")
- `DELETE /api/media/:id/exclude` - Remove from exclusions

### Sync
- `POST /api/sync/full` - Trigger full sync
- `POST /api/sync/incremental` - Trigger incremental sync
- `GET /api/sync/status` - Current sync status

### Jobs
- `GET /api/jobs` - Recent job history
- `GET /api/jobs/:id` - Single job details

### Exclusions
- `GET /api/exclusions` - List all exclusions
- `POST /api/exclusions` - Add exclusion
- `DELETE /api/exclusions/:id` - Remove exclusion

---

## Debug Tips

### Backend Debugging
```bash
# Enable debug logging
export LOG_LEVEL=debug

# Run with config flag
./prunarr --config config/prunarr.test.yaml

# Check logs
tail -f /tmp/prunarr-debug.log | jq
```

### Frontend Debugging
```bash
# Check API responses
curl http://localhost:8080/api/dashboard/stats | jq

# Test with auth disabled (config: disable_auth: true)
curl http://localhost:8080/api/media/movies | jq

# Check browser console for errors
```

### Common Errors
1. **"Unauthorized"**: Check JWT token or enable `disable_auth`
2. **"Media not found"**: Verify Radarr/Sonarr sync completed
3. **Empty library**: Check integration URLs and API keys
4. **Exclusions not persisting**: Ensure `applyExclusions()` is called in sync

---

## Next Steps (Priority Order)

### High Priority
1. **User-based cleanup** - Implement requester tracking from Jellyseerr
2. **Configuration UI** - Allow editing `prunarr.yaml` via web interface
3. **Collection management** - Create "Leaving Soon" collections in Jellyfin
4. **Advanced rules UI** - Tag-based rules, episode limits

### Medium Priority
5. **Mobile responsiveness** - Improve UI on small screens
6. **Statistics/charts** - Visualize disk space trends, deletion history
7. **Error handling** - Better error messages and recovery
8. **Loading states** - Improve UI feedback during operations

### Low Priority
9. **Docker optimization** - Reduce image size further
10. **Performance tuning** - Cache optimization, response times
11. **Documentation** - API docs, user guide
12. **E2E tests** - Automated testing suite

---

## Important Notes for AI Agents

### When Resuming Work
1. **Read this file first** to understand current state
2. **Check PRUNARR_SPEC.md** for detailed architecture
3. **Review recent commits** (`git log --oneline -10`)
4. **Check test coverage** (`make test`)

### When Making Changes
1. **Always run tests** after backend changes
2. **Preserve exclusions** - Never break `applyExclusions()` logic
3. **Maintain data source hierarchy** - Radarr/Sonarr = truth, Jellyfin = watch data only
4. **Update this file** when completing major features
5. **Document in PRUNARR_SPEC.md** when fixing bugs or adding features

### When Debugging
1. **Enable debug logging** (`LOG_LEVEL=debug`)
2. **Use test config** with real services
3. **Check job history** to see sync results
4. **Verify API responses** with curl or test-api.sh

---

## Quick Reference

### File Locations
- Config: `config/prunarr.yaml`
- Data: `data/exclusions.json`, `data/jobs.json`
- Logs: `/tmp/prunarr-debug.log` (test mode)
- Binary: `./prunarr` or `./prunarr-test`

### Ports
- HTTP Server: `8080` (default)
- Frontend Dev: `5173` (Vite)

### Key Commands
- `make dev` - Start everything
- `make build` - Build binary
- `make test` - Run tests
- `./test-api.sh` - Test endpoints

### Environment Variables
- `LOG_LEVEL` - debug/info/warn/error
- `CONFIG_PATH` - Path to config file
- `SERVER_PORT` - HTTP port (default: 8080)

---

## Session Summary Template

When ending a session, update this section with:

### Last Session: [DATE]

**Work Completed:**
- [ ] Feature/fix description
- [ ] Files modified
- [ ] Commits made
- [ ] Tests added/updated

**Current State:**
- Running: Yes/No
- Tests passing: X/X
- Known issues: List

**Next Session TODO:**
- [ ] Next task
- [ ] Priority item
- [ ] Follow-up needed

---

## Last Session: Nov 3, 2025 (Session 4 - Client Logging)

**Work Completed:**
- ✅ Added consistent structured logging to Jellyseerr client
- ✅ Added consistent structured logging to Jellystat client
- ✅ Implemented logging patterns matching existing clients (Jellyfin, Radarr, Sonarr)
- ✅ Added debug logs for API requests and responses
- ✅ Added pagination progress logging
- ✅ Added error logs with context for failures
- ✅ All tests passing (282/282)

**Files Modified:**
- `internal/clients/jellyseerr.go` - Added zerolog logging (+46 lines)
- `internal/clients/jellystat.go` - Added zerolog logging (+46 lines)

**Commits:**
1. `e55870e` - feat: add consistent logging to Jellyseerr and Jellystat clients

**Current State:**
- Running: No (code changes only)
- Tests passing: 282/282 ✅
- Known issues: None

**Logging Improvements:**
- Debug level: API requests, responses, counts, pagination progress
- Error level: Failed requests, connection issues, unexpected status codes
- Success confirmations: Ping operations, data fetch completions
- All six clients now have consistent logging patterns

**Next Session TODO:**
- [ ] Test logging output with `LOG_LEVEL=debug` in production environment
- [ ] Configuration UI page (allow editing prunarr.yaml via web)
- [ ] Collection management for "Leaving Soon" in Jellyfin
- [ ] Advanced rules UI (user-based rules editor)
- [ ] Mobile responsiveness improvements

---

## Previous Session: Nov 3, 2025 (Session 3 - Jellyseerr & Jellystat Tests)

**Work Completed:**
- ✅ Resumed from previous session (committed deletion reason tests)
- ✅ Refactored client file structure (split optional.go into separate files)
- ✅ Created comprehensive Jellyseerr client tests
- ✅ Created comprehensive Jellystat client tests
- ✅ Added integration and unit tests following existing patterns
- ✅ Test coverage increased for clients module

**Files Modified:**
- `internal/clients/jellyseerr.go` - Extracted from optional.go (106 lines)
- `internal/clients/jellystat.go` - Extracted from optional.go (108 lines)
- `internal/clients/jellyseerr_test.go` - NEW: 273 lines, 10 test cases
- `internal/clients/jellystat_test.go` - NEW: 352 lines, 10 test cases
- Deleted: `internal/clients/optional.go` (refactored into separate files)

**Commits:**
1. `f734151` - test: add comprehensive deletion reason generation tests (13 tests)
2. `26eb686` - refactor: split optional clients into separate files
3. `ac4605e` - test: add comprehensive tests for Jellyseerr and Jellystat clients

**Current State:**
- Running: No (tests only)
- Tests passing: 282/282 ✅ (up from 269, +13 new tests)
- Known issues: None

**Test Coverage Changes:**
- Services: 52.3% → 57.1% (+4.8%) - from deletion reason tests
- Clients: 4.9% → 8.2% (+3.3%) - from new client tests
- Total test functions: 89 (previously 76, +13)

**Key Changes:**
- Client file structure now matches pattern: each client has its own file + test file
- Jellyseerr tests: Ping, GetRequests, pagination, request types, requester validation
- Jellystat tests: Ping, GetHistory, pagination, user activity, playback duration
- All tests follow integration + unit test pattern from existing clients
- Integration tests require `PRUNARR_INTEGRATION_TEST=1` environment variable

**Next Session TODO:**
- [ ] Configuration UI page (allow editing prunarr.yaml via web)
- [ ] Collection management for "Leaving Soon" in Jellyfin
- [ ] Advanced rules UI (user-based rules editor)
- [ ] Mobile responsiveness improvements
- [ ] Consider adding more client tests (e.g., error handling, retry logic)

---

## Previous Session: Nov 3, 2025 (Session 2 - Simplified User Rules)

**Work Completed:**
- ✅ Simplified user-based rules configuration (single identifier per user)
- ✅ Added comprehensive documentation and examples
- ✅ Enhanced code comments in UserRule struct
- ✅ Created validation tests (61 new tests added)
- ✅ Created config loading test for simplified rules
- ✅ Updated PRUNARR_SPEC.md with clarified matching strategy
- ✅ Updated config/prunarr.yaml.example with clear examples

**Commits:**
- `14f1d7d` - feat: add config hot-reload support for retention rules
- `d68b990` - feat: improve deletion reasons and add Jellystat watch tracking

---

**Last Updated**: Nov 3, 2025  
**Document Version**: 1.2
