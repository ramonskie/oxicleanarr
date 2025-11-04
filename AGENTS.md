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
✅ Complete REST API (auth, sync, media, jobs, exclusions, deletion control)  
✅ All service integrations (Jellyfin, Radarr, Sonarr, Jellyseerr, Jellystat)  
✅ Sync engine with full/incremental scheduler  
✅ Rules engine with retention policies (standard, tag-based, episode-based, user-based)  
✅ Tag-based retention rules (fetch tags from Radarr/Sonarr, case-insensitive matching)  
✅ Deletion executor with dry-run mode  
✅ Manual deletion control with UI confirmation  
✅ Automatic deletion toggle (`enable_deletion` config)  
✅ Exclusions management with persistence  
✅ Job history tracking  
✅ React UI with Dashboard, Timeline, Library, Scheduled Deletions, Job History pages  
✅ Authentication & authorization (with optional bypass for testing)  
✅ Configuration with hot-reload  
✅ Deletion reason generation (including tag-based rules)  
✅ Jellyfin collections management ("Leaving Soon" collections)  
✅ Configuration & Advanced Rules management UI  
✅ Toast notifications for user feedback (Sonner)  
✅ Auto-sync on retention rule changes (immediate effect)  
✅ Scheduled Deletions UI displays correctly in all modes (dry-run and live)  
✅ Tag display on media cards across all pages  
✅ Tag filtering in Library page  
✅ Rule type badges showing which rule matched  

### What's Pending
⏳ User-based cleanup with watch tracking  
⏳ Mobile responsiveness polish  
⏳ Statistics/charts  
⏳ Comprehensive error handling  

### Testing Status
- **380 tests passing** (increased from 111 in Session 15)
- **Coverage**: Handlers 89.0%, Storage 92.7%, Services 58.3%+, Clients 5.8%

---

## Recent Work (Last Session - Nov 4, 2025, Session 16)

### Tag-Based Rules UI Enhancements - COMPLETED ✅

**Work Completed:**
- ✅ Added 17 comprehensive unit tests for tag-based rule evaluation
- ✅ Fixed 2 build errors (unused variables) and 2 test failures (incorrect expectations)
- ✅ Displayed tags on media cards across all three pages (Library, Scheduled Deletions, Timeline)
- ✅ Added tag filter to Library page with multi-select and clear functionality
- ✅ Added rule type badges showing which rule matched (Tag/User/Standard)
- ✅ All 380 test runs passing (was 111 test functions)

**Problem Solved:**
- No unit test coverage for tag-based rule logic
- Tags were stored in backend but not visible in UI
- No way to filter library by tags
- Users couldn't see which retention rule caused a deletion

**Solution Implemented:**
- Created comprehensive test suite in `internal/services/rules_test.go` with 17 test cases
- Fixed test expectations: Tag rules DO override requested status when advanced rules exist
- Added `tags?: string[]` to TypeScript interfaces (MediaItem, DeletionCandidate)
- Display tags as secondary badges below title on all media cards
- Tag filter with active state, sorted alphabetically, OR logic (match ANY selected tag)
- Created `getRuleType()` helper to parse deletion_reason strings
- Display rule type badges using shadcn/ui Badge component variants

**Files Modified:**
- `internal/services/rules_test.go` (+460 lines) - 17 new test cases
- `web/src/lib/types.ts` (+2 lines) - Tag fields
- `web/src/pages/LibraryPage.tsx` (+99 lines) - Tags display + filter + rule badges
- `web/src/pages/ScheduledDeletionsPage.tsx` (+33 lines) - Tags display + rule badges
- `web/src/pages/TimelinePage.tsx` (+9 lines) - Tags display

**Commits:**
1. `02818b2` - test: add comprehensive unit tests for tag-based rule evaluation
2. `2923395` - feat: display tags on media cards across all pages
3. `cb2a3f1` - feat: add tag filter to Library page
4. `33d5997` - feat: display rule type badges on media cards

**Testing Results:**
- ✅ All 380 test runs passing (17 new test cases added)
- ✅ Services test coverage increased
- ✅ Frontend built successfully (440.18 kB, gzipped: 130.93 kB)
- ✅ Tags display correctly on all pages
- ✅ Tag filter works with multi-select and clear
- ✅ Rule type badges show correct rule that matched

**Rule Type Detection:**
- Tag rules: `"tag rule 'NAME' (tag: TAG) retention expired (RETENTION)"`
- User rules: `"user rule 'NAME' retention expired (RETENTION)"`
- Standard rules: `"retention period expired (RETENTION)"` or `"within retention"`

**UI Features Added:**
- Tag badges on media cards (secondary variant, flex-wrap gap-1)
- Tag filter buttons above sort controls (toggleable, multi-select)
- "Clear filters" button when tags selected
- Rule type badges next to media type (Tag Rule: default, User Rule: secondary, Standard: outline)
- Filter resets pagination automatically

**Current State:**
- Running: Yes (PID: 287868)
- Tests passing: 380/380 ✅
- Known issues: None
- Tag-based rules: Fully implemented with UI ✅
- Session 16: COMPLETE ✅

**Next Session TODO:**
- [ ] User-based cleanup with watch tracking integration
- [ ] Mobile responsiveness improvements
- [ ] Statistics/charts for disk space trends
- [ ] Comprehensive error handling

---

## Previous Session: Nov 4, 2025 (Session 14)

### Scheduled Deletions UI Bug Fix - COMPLETED ✅

**Problem Identified:**
- Scheduled Deletions page was showing 0 items despite having 23-50 deletion candidates in job history
- Root cause: `would_delete` array in job summaries was only populated when `dry_run: true`
- Frontend ScheduledDeletionsPage always expects this array (`latestJob.summary?.would_delete || []`)
- With `dry_run: false`, the array was never populated, causing empty UI

**Solution Implemented:**
- Modified `internal/services/sync.go` lines 295-298
- Removed `dry_run` condition check before populating `would_delete` array
- Now always populates `would_delete` when deletion candidates exist
- Added clarifying comment explaining purpose in both modes

**Code Change:**
```go
// Before (Session 9):
if e.config.App.DryRun && len(wouldDelete) > 0 {
    job.Summary["would_delete"] = wouldDelete
}

// After (Session 14):
// Always add deletion candidates to job summary for UI display
// In dry-run mode, these are candidates that would be deleted
// Otherwise, these are candidates that will be deleted (if enable_deletion is true)
if len(wouldDelete) > 0 {
    job.Summary["would_delete"] = wouldDelete
}
```

**Files Modified:**
- `internal/services/sync.go` - Removed dry_run condition, always populate would_delete array (~4 lines changed)

**Commits:**
- `ae06c16` - fix: always populate would_delete in job summary for Scheduled Deletions UI

**Testing Results:**
- ✅ All 111 backend tests passing
- ✅ No regressions introduced
- ✅ Config verified safe mode (`dry_run: true`)
- ✅ Binary rebuilt and tested with real data

**Technical Discovery - Field Name Mapping:**
- Backend `Media.DeleteAfter` → JSON `deletion_date` → Frontend `MediaItem.deletion_date`
- Backend `Media.DaysUntilDue` → JSON `days_until_deletion` → Frontend `MediaItem.days_until_deletion`
- Job summary candidates use `delete_after` → Frontend `DeletionCandidate.delete_after`
- Overdue items (`now.After(media.DeleteAfter)`) go in `would_delete` array
- Future deletions (`DaysUntilDue > 0`) returned by `/api/media/leaving-soon` endpoint

**Current State:**
- Running: No (stopped after testing)
- Tests passing: 111/111 ✅
- Known issues: None
- Scheduled Deletions page: Fixed ✅

**Next Session TODO:**
- [ ] User-based cleanup with watch tracking
- [ ] Mobile responsiveness improvements
- [ ] Statistics/charts for disk space trends
- [ ] Comprehensive error handling

---

## Previous Session: Nov 3, 2025 (Session 13)

### Part 1: Toast Notifications UI - COMPLETED ✅

**Implementation**: Added modern toast notifications for user feedback
- Installed Sonner library (modern React toast notifications)
- Replaced placeholder use-toast hook with proper Sonner implementation
- Added Toaster component to App.tsx for rendering notifications
- Success and error toasts now display properly across all pages
- Positioned top-right with rich colors (green for success, red for errors)

**Problem Solved**:
- Configuration page save appeared to do nothing (no visual feedback)
- use-toast hook only logged to console for success messages
- Error messages used browser alert() which is poor UX
- Users had no indication that operations succeeded or failed

**Files Modified**:
- `web/src/hooks/use-toast.ts` - Implemented Sonner toast API (~9 lines changed)
- `web/src/App.tsx` - Added Toaster component (+2 lines)

**Commits**: `bf5f735`

### Part 2: Auto-Sync on Config Change - COMPLETED ✅

**Implementation**: Config updates now automatically trigger full sync when retention rules change
- ConfigHandler now accepts SyncEngine dependency injection
- Detects changes to `movie_retention`, `tv_retention`, or `advanced_rules`
- Triggers async full sync after config reload to re-evaluate media
- Eliminates need for manual sync after config changes

**Problem Solved**:
- User changed retention rules from default to `0d` but still saw 248 items scheduled for deletion
- Config hot-reload updated in-memory config but didn't re-run rules engine
- Required manual full sync via UI to see changes take effect
- Poor UX: changes appeared to not work immediately

**Solution**:
- Added retention change detection in `UpdateConfig()` handler
- Runs `h.syncEngine.FullSync(ctx)` asynchronously in goroutine
- Uses `context.Background()` to avoid blocking HTTP response
- Comprehensive logging for tracking sync triggers and completion
- Changes take effect within 1-2 seconds automatically

**Files Modified**:
- `internal/api/handlers/config.go` (+42 lines, -4 lines) - Detection & trigger logic
- `internal/api/router.go` (+1 line, -1 line) - Pass SyncEngine dependency

**Commits**: `c3f3118`

**Testing Results**:
- ✅ All 111 backend tests still passing
- ✅ Manual testing successful (retention 0d → 365d/180d → 0d)
- ✅ Auto-sync triggered within 1 second after config save
- ✅ Leaving-soon count updated correctly (0 → 129 → 0 items)
- ✅ Logs confirm sync trigger and completion

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
- **Flow**: Radarr/Sonarr → Jellyfin (watch data) → Jellyseerr (requests) → Apply exclusions → Apply rules → Sync collections

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
1. ~~**Configuration UI**~~ - ✅ COMPLETE (Session 10/11)
2. ~~**Advanced rules UI**~~ - ✅ COMPLETE (Session 10/11)
3. ~~**Collection management**~~ - ✅ COMPLETE (Session 7)
4. **User-based cleanup** - Implement watch tracking integration with rules engine

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

## Last Session: Nov 3, 2025 (Session 13 - UI Polish & Auto-Sync ✅)

### Part 1: Toast Notifications UI

**Work Completed:**
- ✅ Resumed from Session 12 summary (config YAML serialization was fixed)
- ✅ Identified missing toast notifications in UI (use-toast hook was placeholder)
- ✅ Installed Sonner library for modern toast notifications
- ✅ Implemented proper toast notifications using Sonner
- ✅ Added Toaster component to App.tsx for rendering toasts
- ✅ Fixed use-toast hook to use Sonner instead of console.log/alert

**Problem Fixed:**
- Configuration page save appeared to do nothing (no user feedback)
- use-toast hook only logged to console for success messages
- Error messages used browser alert() which is poor UX
- No toast component existed in the UI to render notifications

**Solution:**
- Installed `sonner` npm package (modern React toast library)
- Updated `use-toast.ts` to call `sonnerToast.success()` and `sonnerToast.error()`
- Added `<Toaster position="top-right" richColors />` to App.tsx
- Vite hot-reload automatically picked up changes (no restart needed)

**Files Modified & Committed:**
- `web/src/hooks/use-toast.ts` - Replaced console.log/alert with Sonner toast API (~9 lines changed)
- `web/src/App.tsx` - Added Toaster import and component (+2 lines)

**Commits:**
1. `bf5f735` - feat: add Sonner toast notifications for user feedback

### Part 2: Auto-Sync on Retention Rule Changes

**Work Completed:**
- ✅ Identified UX issue: config changes didn't take effect until manual sync
- ✅ Added SyncEngine dependency to ConfigHandler
- ✅ Implemented retention rule change detection
- ✅ Added async full sync trigger after config updates
- ✅ All 111 backend tests still passing

**Problem Fixed:**
- User changed retention rules from defaults to `0d` but still saw 248 items scheduled for deletion
- Config hot-reload updated in-memory config but didn't re-run rules engine
- Required manual full sync via UI to see changes take effect
- Poor UX: changes appeared to not work immediately

**Solution:**
- ConfigHandler now accepts `SyncEngine` dependency injection
- `UpdateConfig()` detects changes to `movie_retention`, `tv_retention`, or `advanced_rules`
- Triggers `h.syncEngine.FullSync(ctx)` asynchronously in goroutine after config reload
- Uses `context.Background()` to avoid blocking HTTP response
- Comprehensive logging tracks retention changes, sync triggers, and completion

**Files Modified & Committed:**
- `internal/api/handlers/config.go` (+42 lines, -4 lines) - Detection & trigger logic
- `internal/api/router.go` (+1 line, -1 line) - Pass SyncEngine dependency

**Commits:**
2. `c3f3118` - feat: auto-trigger full sync when retention rules change in config

**Current State:**
- Running: Yes (backend + frontend dev server)
- Tests passing: 111/111 ✅
- Known issues: None
- Toast notifications: Working ✅
- Auto-sync on config change: Working ✅
- Frontend build: 437.92 kB (gzipped: 130.28 kB)

**Testing Results:**
- ✅ Manual test: Changed retention from 0d → 365d/180d via Configuration page
- ✅ Auto-sync triggered within 1 second (logged: "Triggering full sync to re-apply retention rules")
- ✅ Leaving-soon count updated correctly: 0 → 129 items
- ✅ Manual test: Changed retention back to 0d
- ✅ Auto-sync triggered again, leaving-soon count: 129 → 0 items
- ✅ Full sync completes in ~1 second for 378 media items

**Next Session TODO:**
- [ ] Manual UI testing: Verify Configuration page shows toast + auto-sync behavior end-to-end
- [ ] Manual UI testing: Advanced Rules page (create/edit/delete rules)
- [ ] Consider Sessions 10-13 (Config UI + Rules UI + Toast + Auto-Sync) COMPLETE
- [ ] Move to next feature: mobile responsiveness or user-based cleanup
- [ ] Statistics/charts for disk space trends

**Key Lessons:**
- **Sonner**: Modern standard for React toast notifications, works great with shadcn/ui
- **Auto-sync**: Retention rule changes should trigger immediate re-evaluation for better UX
- **Async operations**: Use goroutines for sync to avoid blocking HTTP responses
- **Context**: Use `context.Background()` for background tasks, not request contexts

---

## Previous Session: Nov 3, 2025 (Session 12 - Config YAML Serialization Bug Fix ✅)

**Work Completed:**
- ✅ Resumed from Session 11 summary (identified YAML serialization bug)
- ✅ Fixed malformed YAML field names by adding `yaml` tags to all config structs
- ✅ Fixed directory path extraction using `filepath.Dir()` instead of hardcoded string trimming
- ✅ Added comprehensive debug logging for config write operations
- ✅ Added `GetPath()` function to retrieve current config file path
- ✅ Fixed ConfigurationPage `useEffect` hook for proper form initialization
- ✅ All 111 tests still passing
- ✅ Config update API now works end-to-end

**Problem Fixed:**
- Config update endpoint was writing malformed YAML with incorrect field names
- Example: `dryrun` instead of `dry_run`, `disableauth` instead of `disable_auth`
- Root cause: Missing `yaml` tags on Go struct fields (only had `mapstructure` and `json` tags)
- Impact: Config reload validation failures after API updates

**Solution:**
- Added `yaml:"field_name"` tags to all 14 config struct types (types.go)
- Used `yaml:",inline"` for embedded `BaseIntegrationConfig` structs
- Used `yaml:",omitempty"` for optional fields (advanced_rules, user rules)
- All fields now have triple tags: `mapstructure:"name" yaml:"name" json:"name"`

**Files Modified & Committed:**
- `internal/config/types.go` - Added `yaml` tags to all 14 config struct types (~60 lines changed)
- `internal/config/config.go` - Added `GetPath()` function (+5 lines)
- `internal/api/handlers/config.go` - Fixed directory handling, added debug logging (~23 lines changed)
- `web/src/pages/ConfigurationPage.tsx` - Fixed `useEffect` hook (changed from `useState` to `useEffect`)

**Commits:**
1. `e47330c` - fix: add YAML tags for proper config file serialization and improve write logging

**Current State:**
- Running: Yes (backend + frontend dev server)
- Tests passing: 111/111 ✅
- Known issues: None
- Config update: Working end-to-end ✅
- YAML serialization: Correct snake_case field names ✅

**Technical Details:**

**Config Handler Improvements:**
- `writeConfigToFile()` now uses `filepath.Dir(configPath)` to extract directory path
- Added 7 new log statements for debugging config write operations:
  - "About to write config to file" (with leaving_soon_days value)
  - "Writing config to file" (with path)
  - "Ensuring directory exists" (with dir path)
  - "Writing file" (with path and byte count)
  - "Config file written successfully"
  - "Write config completed successfully"
  - Error logs for marshal, mkdir, and write failures

**Verification Tests:**
- ✅ `GET /api/config` returns correct JSON with snake_case fields
- ✅ `PUT /api/config` updates file with correct YAML field names
- ✅ Updated config values reflected immediately via API
- ✅ No more validation errors on config reload
- ✅ All integration settings preserved correctly

**Next Session TODO:**
- [ ] Manual UI testing: Configuration page form (load/save)
- [ ] Manual UI testing: Advanced Rules page (create/edit/delete)
- [ ] Mobile responsiveness improvements
- [ ] Statistics/charts for disk space trends
- [ ] User-based cleanup with watch tracking

**Key Lesson:**
- Go structs used for YAML marshaling require explicit `yaml` tags
- Without `yaml` tags, Go uses field names directly (e.g., `DryRun` becomes `dryrun`)
- Triple tags pattern ensures correct serialization across all formats: `mapstructure:"x" yaml:"x" json:"x"`

---

## Previous Session: Nov 3, 2025 (Session 11 - Configuration & Rules Management UI ✅)

**Work Completed:**
- ✅ Resumed from Session 10 summary (Config & Rules UI implementation)
- ✅ Rebuilt backend with new handlers (config.go, rules.go)
- ✅ Verified all API endpoints working correctly
- ✅ Tested config GET/PUT, rules CRUD operations
- ✅ Verified frontend TypeScript compilation
- ✅ Committed Session 10 changes with comprehensive message
- ✅ All 111 tests still passing

**Files Modified & Committed:**
- `internal/api/handlers/config.go` - NEW (390 lines): Config view/update handler
- `internal/api/handlers/rules.go` - NEW (452 lines): Rules CRUD handler with validation
- `internal/api/router.go` (+11 routes): Config (2) and Rules (5) endpoints
- `web/src/pages/ConfigurationPage.tsx` - NEW (292 lines): App/sync/retention settings editor
- `web/src/pages/RulesPage.tsx` - NEW (611 lines): Advanced rules management with type-specific forms
- `web/src/lib/types.ts` (+120 lines): Config, AdvancedRule, UserRule interfaces
- `web/src/lib/api.ts` (+40 lines): 7 new methods (getConfig, updateConfig, listRules, etc.)
- `web/src/App.tsx` (+2 routes): /configuration and /rules
- `web/src/pages/DashboardPage.tsx` (+Configuration button): Navigate to config page
- `go.mod` (yaml.v3 moved from indirect to direct dependency)

**Commits:**
1. `60316fb` - feat: add configuration and advanced rules management UI

**Current State:**
- Running: Yes (backend + frontend dev server)
- Tests passing: 111/111 ✅
- Known issues: Config YAML serialization bug (fixed in Session 12)
- Total new code: ~1,951 lines (10 files changed)
- Backend endpoints: 11 new routes (6 config, 5 rules)
- Frontend pages: 2 new pages (Configuration, Rules)

**Feature Implementation:**

**Backend (Config Handler):**
- `GET /api/config` - Returns sanitized config (masks passwords/API keys as booleans)
- `PUT /api/config` - Updates config with validation and auto-reload
- Security: Shows `has_api_key`/`has_password` instead of actual secrets
- Creates config directory if missing
- Writes YAML with header comment

**Backend (Rules Handler):**
- `GET /api/rules` - List all advanced rules
- `POST /api/rules` - Create new rule with type validation
- `PUT /api/rules/{name}` - Update existing rule
- `DELETE /api/rules/{name}` - Delete rule
- `PATCH /api/rules/{name}/toggle` - Toggle enabled state (requires JSON body: `{"enabled": bool}`)
- Validation: Type (tag/episode/user), duplicate names, required fields per type

**Frontend (Configuration Page):**
- Application Settings: dry_run, enable_deletion, leaving_soon_days
- Sync Settings: full_interval, incremental_interval, auto_start
- Default Retention: movie_retention, tv_retention
- Real-time form updates with React state
- Save button with loading state
- Navigation to Advanced Rules page
- Toast notifications for success/error

**Frontend (Advanced Rules Page):**
- List all rules with enable/disable toggles
- Badge indicators (enabled/disabled, rule type)
- Edit/Delete buttons per rule
- "Add Rule" button with dialog
- Type selector: tag/episode/user
- Type-specific form fields:
  - Tag rules: tag, retention
  - Episode rules: max_episodes, max_age, require_watched
  - User rules: dynamic user list with add/remove
- Validation and error handling
- Empty state with helpful message

**API Testing Results:**
- ✅ GET /api/config returns sanitized config structure
- ✅ GET /api/rules lists existing rules
- ✅ POST /api/rules creates new rule (requires capitalized JSON keys: Name, Type, etc.)
- ✅ PATCH /api/rules/{name}/toggle requires JSON body: `{"enabled": true/false}`
- ✅ DELETE /api/rules/{name} removes rule and reloads config
- ✅ TypeScript compilation successful (no errors)
- ✅ Frontend build successful (404.18 kB)

---

## Previous Session: Nov 3, 2025 (Session 9 - Deletion Control Feature & Test Fix ✅)

**Work Completed:**
- ✅ Resumed from Session 8 (97 tests passing, Collection Manager complete)
- ✅ Implemented manual deletion control feature with "Execute Deletions" button
- ✅ Added automatic deletion toggle (`enable_deletion` config flag)
- ✅ Created comprehensive service and handler tests (14 new tests)
- ✅ Fixed test failure: response format for empty deletion execution
- ✅ All 111 tests passing (up from 97, +14 new tests)

**Files Modified & Committed:**
- `internal/config/types.go` - Added `EnableDeletion bool` field to `AppConfig`
- `internal/config/defaults.go` - Set default `EnableDeletion: false` (safe mode)
- `config/prunarr.yaml.example` - Documented new config with clear comments
- `internal/services/sync.go` (+47 lines) - Created `ExecuteDeletions()` and `CalculateDeletionInfo()`, updated `FullSync()`
- `internal/api/handlers/sync.go` (+59 lines) - Added `ExecuteDeletions()` handler for `POST /api/deletions/execute`
- `internal/api/router.go` (+1 line) - Added route for manual deletion endpoint
- `web/src/lib/api.ts` (+9 lines) - Added `executeDeletions(dryRun)` method
- `web/src/lib/types.ts` (+11 lines) - Added `DeletionExecutionResponse` interface
- `web/src/pages/ScheduledDeletionsPage.tsx` (+26 lines) - Added "Execute Deletions" button with confirmation dialog
- `internal/services/sync_test.go` (+186 lines) - 10 new service tests
- `internal/api/handlers/sync_test.go` (+64 lines) - 4 new handler tests

**Commits:**
1. `cf21b1b` - feat: add manual deletion control and automatic deletion toggle
2. `aff6e3d` - fix: correct response format for empty deletion execution and add comprehensive tests

**Current State:**
- Running: No
- Tests passing: 111/111 ✅
- Known issues: None
- Test coverage: Handlers 89.0%, Storage 92.7%, Services 58.3%, Clients 5.8%
- Total test functions: 106 (up from 97)

**Feature Implementation:**
- **Config default:** `enable_deletion: false` (manual-only mode by default)
- **Automatic deletion:** Requires both `enable_deletion: true` AND `dry_run: false`
- **Manual deletion:** Available via `POST /api/deletions/execute` endpoint
- **Safety layers:** Config flag + dry-run check + UI confirmation dialog
- **Job tracking:** New fields - `enable_deletion`, `deleted_count`, `deleted_items`

**Test Cases Added (14 total):**
1. Service Tests (10):
   - `TestSyncEngine_CalculateDeletionInfo` (4 subtests) - Overdue calculation logic
   - `TestSyncEngine_ExecuteDeletions` (3 subtests) - Deletion execution
   - `TestSyncEngine_FullSync_EnableDeletion` (2 subtests) - Config toggle behavior

2. Handler Tests (4):
   - `TestSyncHandler_ExecuteDeletions` - Endpoint behavior with dry-run, empty, and actual execution

**Next Session TODO:**
- [ ] Configuration UI page (edit prunarr.yaml via web)
- [ ] Advanced rules UI (user-based rules editor)
- [ ] Mobile responsiveness improvements
- [ ] Statistics/charts for disk space trends

---

## Previous Session: Nov 3, 2025 (Session 8 - Collection Manager Tests & Bug Fix ✅)

**Work Completed:**
- ✅ Task #1 COMPLETE: Added comprehensive unit tests for `JellyfinCollectionManager`
- ✅ Task #2 COMPLETE: Verified `hide_when_empty` behavior via unit tests
- ✅ Task #3 COMPLETE: Fixed TV show type bug (collections now properly include TV shows)
- ✅ Created 13 test cases covering all collection lifecycle scenarios
- ✅ Introduced `JellyfinCollectionClient` interface for better testability
- ✅ All 97 tests passing (up from 89, +8 new collection tests)
- ✅ Test coverage: services 57.1% → 58.3%

**Files Modified & Committed:**
- `internal/services/jellyfin_collections.go` - Added interface, fixed media type comparison (+12/-4 lines)
- `internal/services/jellyfin_collections_test.go` - NEW: 484 lines, 13 comprehensive test cases

**Commits:**
1. `cb0239b` - test: add comprehensive tests for Jellyfin collection manager and fix TV show type bug

**Current State:**
- Running: No
- Tests passing: 97/97 ✅
- Known issues: None
- Test coverage: Handlers 89.0%, Storage 92.7%, Services 58.3%, Clients 5.8%

**Test Cases Added:**
1. `TestNewJellyfinCollectionManager` - Constructor validation
2. `TestSyncCollections_Disabled` - Skip when disabled
3. `TestSyncCollections_CreateMovieCollection` - Create movie collections
4. `TestSyncCollections_CreateTVShowCollection` - Create TV show collections
5. `TestSyncCollections_SeparatesByType` - Movies and TV shows separated
6. `TestSyncCollections_SkipsExcludedItems` - Excluded items not added
7. `TestSyncCollections_SkipsItemsWithoutJellyfinID` - Missing IDs skipped
8. `TestSyncCollections_SkipsPastDeletionDates` - Past dates filtered
9. `TestSyncCollections_DeletesEmptyCollectionWithHideWhenEmpty` - Auto-delete empty
10. `TestSyncCollections_KeepsEmptyCollectionWithoutHideWhenEmpty` - Keep empty
11. `TestSyncCollections_UpdatesExistingCollection` - Update existing
12. `TestSyncCollections_DryRunMode` - Dry-run behavior
13. `TestSyncCollection_EmptyName` - Edge case handling

**Bug Fixed:**
- **Root Cause**: Collection manager checked `media.Type == "show"` but models use `MediaTypeTVShow = "tv_show"`
- **Impact**: TV shows were never added to collections (always 0 items)
- **Fix**: Changed to use `models.MediaTypeMovie` and `models.MediaTypeTVShow` constants
- **Verification**: Unit test `TestSyncCollections_CreateTVShowCollection` now passes

**Interface Design:**
- Created `JellyfinCollectionClient` interface with 4 methods:
  - `GetCollectionByName(ctx, name) (*JellyfinCollection, error)`
  - `CreateCollection(ctx, name, itemIDs, dryRun) (string, error)`
  - `AddItemsToCollection(ctx, collectionID, itemIDs, dryRun) error`
  - `DeleteCollection(ctx, collectionID, dryRun) error`
- Allows mock clients for unit testing
- Follows Go best practices for dependency injection

**Next Session TODO:**
- [ ] Configuration UI page (edit prunarr.yaml via web)
- [ ] Advanced rules UI (user-based rules editor)
- [ ] Mobile responsiveness improvements
- [ ] Statistics/charts for disk space trends

---

## Previous Session: Nov 3, 2025 (Session 7 - Jellyfin Collections Implementation ✅)

**Work Completed:**
- ✅ Implemented Jellyfin collections management feature
- ✅ Created `JellyfinCollectionManager` service
- ✅ Added collection CRUD methods to Jellyfin client
- ✅ Fixed URL encoding issues in Jellyfin API
- ✅ Integrated into main sync flow (runs after retention rules)
- ✅ Live tested with real Jellyfin instance
- ✅ Successfully created "Prunarr - Movies Leaving Soon" collection with 8 movies

**Files Modified & Committed:**
- `internal/services/jellyfin_collections.go` - NEW: 174 lines
- `internal/clients/jellyfin.go` - +231 lines (URL encoding, collection methods)
- `internal/clients/types.go` - +13 lines (collection types)
- `internal/config/types.go` - +20 lines (collection config)
- `internal/config/validation.go` - +16 lines (collection validation)
- `internal/services/sync.go` - +21 lines (integrate collection manager)
- `config/prunarr.yaml.example` - +10 lines (collection config docs)

**Commits:**
1. `54ded3f` - feat: add Jellyfin collections management for "Leaving Soon" items
2. `bed2d32` - docs: update AGENTS.md with session 7 summary

**Features Implemented:**
- Separate collections for movies and TV shows
- Configurable collection names
- `hide_when_empty: true` - Auto-delete collection when no items scheduled
- Debug logging for all collection operations
- Graceful error handling (doesn't fail entire sync)
- Filters: non-excluded items, with future deletion dates, with valid Jellyfin IDs

---

## Previous Session: Nov 3, 2025 (Session 6 - Semantic Date Labels)

**Work Completed:**
- ✅ Resumed from Session 5 (requester info feature was complete)
- ✅ Verified frontend properly handles missing requester data (no changes needed)
- ✅ Fixed zero date display bug (Go's `0001-01-01T00:00:00Z` showing as "Jan 1, 1")
- ✅ Implemented semantic date labels for better UX clarity
- ✅ Added context-aware `formatDate()` function to all three pages
- ✅ All 282 tests still passing

**Files Modified:**
- `web/src/pages/LibraryPage.tsx` - Added context parameter to formatDate(), semantic labels
- `web/src/pages/ScheduledDeletionsPage.tsx` - Added context parameter, semantic labels  
- `web/src/pages/TimelinePage.tsx` - Changed "Unknown" → "N/A" for deletion dates

**Commits:**
1. `08a28b7` - fix: handle zero date values (0001-01-01) in Library and Timeline pages
2. `6bd6305` - fix: use semantic date labels (N/A for deletions, Never/Unknown for watched)

**Current State:**
- Running: Yes (Prunarr + Frontend dev server)
- Tests passing: 282/282 ✅
- Known issues: None
- Media tracked: 378 items (255 movies, 123 TV shows)
- Items with zero dates: 84 movies with zero last_watched, 5 deletions with zero last_watched

**Date Label Semantics (User Clarity):**
- **"Never"** (watched context) - Item hasn't been watched yet
- **"N/A"** (deletion context) - No deletion scheduled (doesn't imply exclusion/exemption)
- **"Unknown"** (scheduled deletions) - Generic unknown value for watched dates
- **"Not scheduled"** (library page) - When deletion_date is null/undefined

**Implementation Details:**
- `formatDate(dateStr, context: 'watched' | 'deletion')` - Context-aware formatting
- Zero date detection: `getFullYear() <= 1970 && getMonth() === 0 && getDate() === 1`
- Library Page: Uses both contexts (watched for last_watched, deletion for deletion_date)
- Scheduled Deletions: Uses both contexts (deletion for delete_after, watched for last_watched)
- Timeline Page: Only uses deletion context (filters out zero dates entirely)

**Next Session TODO:**
- [ ] Configuration UI page (allow editing prunarr.yaml via web)
- [ ] Collection management for "Leaving Soon" in Jellyfin
- [ ] Advanced rules UI (user-based rules editor)
- [ ] Mobile responsiveness improvements
- [ ] Statistics/charts for disk space trends

---

## Previous Session: Nov 3, 2025 (Session 5 - Requester Info Feature Complete)

**Work Completed:**
- ✅ Completed requester information feature end-to-end
- ✅ Fixed Jellyseerr username resolution (DisplayName → JellyfinUsername fallback)
- ✅ Added requester fields to scheduled deletion candidates in job summaries
- ✅ Verified frontend displays requester info correctly on all three pages
- ✅ Removed Plex-related fields (Prunarr is Jellyfin-only)
- ✅ All 282 tests passing

**Files Modified:**
- `internal/clients/types.go` - Added DisplayName/JellyfinUsername, removed Plex fields
- `internal/services/sync.go` - Updated username resolution (lines 489-514), added requester fields to deletion candidates (lines 689-694)
- `README.md` - Changed "Jellyfin/Plex" → "Jellyfin"

**Commits:**
1. `d9fcab5` - fix: use DisplayName/JellyfinUsername for requester display
2. `f08b17a` - refactor: remove Plex-related fields and focus on Jellyfin
3. `1b4d30d` - feat: add requester info to scheduled deletion candidates
4. `5b835f0` - docs: update AGENTS.md with session 5 summary

**Key Implementation Details:**
- Username fallback chain: DisplayName → JellyfinUsername → Username
- Job summary includes 4 requester fields: `is_requested`, `requested_by_user_id`, `requested_by_username`, `requested_by_email`
- Frontend conditionally shows requester only when `is_requested == true`
- Jellyseerr API uses `displayName` field (not `username`) for display names

---

---

## Previous Session: Nov 3, 2025 (Session 4 - Client Logging)

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

## Previous Session: Nov 4, 2025 (Session 15 - Tag-Based Retention Rules ✅)

**Work Completed:**
- ✅ Implemented tag-based retention rules end-to-end
- ✅ Added Tags field to Media model
- ✅ Created GetTags() methods for Radarr and Sonarr clients
- ✅ Integrated tag fetching into sync operations
- ✅ Implemented evaluateTagBasedRules() with case-insensitive matching
- ✅ Fixed GenerateDeletionReason() to handle tag-based rules
- ✅ Live tested with real Radarr instance
- ✅ All 111 tests passing

**Feature Implemented:**
- Tag-based rules allow different retention policies based on media tags
- Highest priority after exclusions (before user-based and standard rules)
- Case-insensitive tag matching
- Fetches tags from Radarr/Sonarr APIs and maps IDs to names
- Deletion reasons properly formatted with tag rule information

**Files Modified & Committed:**
- `internal/models/media.go` (+1 line) - Added Tags field
- `internal/clients/types.go` (+14 lines) - Tag structs
- `internal/clients/radarr.go` (+31 lines) - GetTags() method
- `internal/clients/sonarr.go` (+31 lines) - GetTags() method
- `internal/services/sync.go` (+46 lines) - Tag fetching and population
- `internal/services/rules.go` (+168 lines) - Tag evaluation + deletion reason fix

**Commits:**
1. `c66eb99` - feat: implement tag-based retention rules

**Current State:**
- Running: Yes (PID: 287868)
- Tests passing: 111/111 ✅
- Known issues: None
- Tag-based rules: Fully implemented and tested ✅

---

**Last Updated**: Nov 4, 2025  
**Document Version**: 1.4
