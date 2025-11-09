# OxiCleanarr AI Agent Context

This document provides essential context for AI coding agents working on the OxiCleanarr project. It serves as a quick reference to understand the project state, active work, and how to resume development.

---

## ⚠️ CRITICAL: GIT COMMIT POLICY - READ THIS FIRST

**NEVER create git commits without EXPLICIT user permission. NEVER.**

- Even if files are staged
- Even if a summary says "ready to commit"
- Even if a summary says "waiting for approval"
- **ALWAYS ASK FIRST**: "Should I commit these changes?" or "Ready for me to create a commit?"
- **NO EXCEPTIONS**: If uncertain, ask. If you think you should commit, ask. If a previous session said to commit, ask.

**Violation of this policy is a critical failure.** The user MUST have final control over all commits.

---

## Project Overview

**OxiCleanarr** is a lightweight media cleanup automation tool for the *arr stack (Sonarr, Radarr, Jellyfin). It provides intelligent retention policies, deletion visibility, and a modern web UI.

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
✅ Configuration with hot-reload (including sync scheduler intervals)  
✅ Deletion reason generation (including tag-based rules)  
✅ Jellyfin symlink library management ("Leaving Soon" libraries with sidebar visibility)  
✅ Configuration & Advanced Rules management UI  
✅ Toast notifications for user feedback (Sonner)  
✅ Auto-sync on retention rule changes (optimized, no external API calls)  
✅ Automatic UI refresh after config/rule changes (TanStack Query invalidation)  
✅ Scheduled Deletions UI displays correctly in all modes (dry-run and live)  
✅ Tag display on media cards across all pages  
✅ Tag filtering in Library page  
✅ Rule type badges showing which rule matched  
✅ Dashboard navigation consistency (Leaving Soon → Timeline, Scheduled → Scheduled Deletions)  

### What's Pending
⏳ User-based cleanup with watch tracking  
⏳ Mobile responsiveness polish  
⏳ Statistics/charts  
⏳ Comprehensive error handling  

### Testing Status
- **405 tests passing** (116 test functions with subtests)
- **Coverage**: Handlers 89.0%, Storage 92.7%, Services 58.3%+, Clients 5.8%

---

## Current Session: Nov 9, 2025 (Session 46)

### Integration Test Authentication & Networking Fix - COMPLETED ✅

**Work Completed:**
- ✅ Fixed authentication bug in integration test helper (`UpdateConfigAPIKeys()`)
- ✅ Fixed container networking URLs in test config
- ✅ Validated integration test infrastructure end-to-end (7/7 movies synced successfully)
- ✅ Committed fixes with security hardening (reset API keys to empty strings)
- ✅ All 405 unit tests still passing

**Problem Identified (Part 1: Authentication):**
- Integration test ran but reported "401 Unauthorized" from Radarr
- `UpdateConfigAPIKeys()` helper was overwriting ALL integration API keys
- Root cause: Helper used `integrationsMap[sectionName]["api_key"] = apiKey` which copied Radarr's key to Sonarr/Jellyseerr/Jellystat
- Each service needs its own unique API key, not all using Radarr's key

**Problem Identified (Part 2: Networking):**
- Test config used short URLs like `http://localhost:8096` for Jellyfin
- Docker containers can't resolve `localhost` - need full container names
- OxiCleanarr container couldn't reach Jellyfin/Radarr services
- URLs should be `http://oxicleanarr-test-jellyfin:8096` (from docker-compose.yml service names)

**Solution Implemented (Part 1):**
1. **Fixed API Key Assignment Logic** (`test/integration/helpers.go` +7 lines):
   - Added check: Skip sections that are also integration names (sonarr, jellyseerr, jellystat)
   - Only update the intended integration's API key
   - Preserves sibling integration keys during update
   - Code change at line 647-653:
     ```go
     // Skip this section if it's a sibling integration (not the one we're updating)
     if sectionName == "sonarr" || sectionName == "jellyseerr" || sectionName == "jellystat" {
         continue
     }
     ```

**Solution Implemented (Part 2):**
1. **Updated Container URLs** (`test/assets/config/config.yaml` 11 lines changed):
   - Changed `http://localhost:8096` → `http://oxicleanarr-test-jellyfin:8096`
   - Changed `http://localhost:7878` → `http://oxicleanarr-test-radarr:7878`
   - Changed `http://localhost:8989` → `http://oxicleanarr-test-sonarr:8989`
   - Changed `http://localhost:5055` → `http://oxicleanarr-test-jellyseerr:5055`
   - Changed `http://localhost:3001` → `http://oxicleanarr-test-jellystat:3001`
   - All URLs now match service names from `test/assets/docker-compose.yml`

2. **Security Hardening**:
   - Reset all API keys to empty strings in config file
   - Keys populated at runtime by test helper (as designed)
   - No credentials committed to repository

**Files Modified & Committed:**
- `test/integration/helpers.go` (+7 lines) - Fixed API key assignment logic
- `test/assets/config/config.yaml` (11 lines changed) - Container URLs + empty API keys

**Commits:**
1. `9d15b59` - fix: correct integration test authentication and container networking

**Current State:**
- Running: No (infrastructure validated, ready for lifecycle tests)
- Tests passing: 405/405 unit tests ✅
- Integration test validation: 7/7 movies synced successfully ✅
- Known issues: None
- Git status: Clean (all fixes committed)

**Validation Results (from Session 46 test run):**
- ✅ All 21 infrastructure steps passed
- ✅ OxiCleanarr Bridge plugin verified (version 3.2.1.0, Active)
- ✅ Plugin API endpoint functional (`/api/oxicleanarr/status`)
- ✅ Data consistency validated: Radarr (7) = Jellyfin (7) = OxiCleanarr (7)
- ✅ Test user created with API key
- ✅ Media library scanned (7 movies)
- ✅ Network connectivity: 172.25.0.x subnet
- ✅ Symlink library feature enabled

**Next Session TODO:**
- [ ] Begin Session 47: Implement symlink lifecycle tests
- [ ] Test scenarios in `test/integration/symlink_lifecycle_test.go`:
  - Create symlinks for scheduled deletions
  - Update retention rules
  - Verify symlink cleanup
  - Validate OxiCleanarr Bridge plugin API integration
- [ ] End-to-end test: OxiCleanarr sync → plugin creates symlinks → Jellyfin library updates

**Key Lessons:**
1. **Helper function scope**: Must check for sibling sections to avoid overwriting unrelated config
2. **Docker networking**: Containers use service names from docker-compose.yml, not `localhost`
3. **Container name format**: `{project}-{service}` where project = directory name or COMPOSE_PROJECT_NAME
4. **URL consistency**: All integration URLs must use Docker service names for inter-container communication
5. **Security**: Always reset credentials in committed config files (populate at runtime)
6. **Test validation order**: Authentication before API calls, networking before data sync
7. **Integration test debugging**: Check both code logic (helpers) and config values (URLs/keys)

---

## Previous Session: Nov 9, 2025 (Session 44)

### Integration Test Environment Variable Removal & Documentation Fixes - COMPLETED ✅

**Work Completed:**
- ✅ Removed `OXICLEANARR_INTEGRATION_TEST=1` environment variable requirement from all integration tests
- ✅ Tests now run by default without special flag (simpler workflow)
- ✅ Created comprehensive `test/README.md` documentation (610 lines)
- ✅ Fixed 7 incorrect test name references in README (5 TestSetup + 2 TestSymlink)
- ✅ All 405 unit tests still passing

**Problem Identified:**
- Integration tests required `OXICLEANARR_INTEGRATION_TEST=1` env var to run (from Session 42)
- Added friction to development workflow (extra flag required)
- Documentation referenced wrong test function names:
  - 5 instances of `TestSetup` (should be `TestInfrastructure`)
  - 2 instances of `TestSymlink` (should be `TestSymlinkLifecycle`)
- Test patterns `-run TestSetup` and `-run TestSymlink` wouldn't match actual function names

**Solution Implemented:**
1. **Removed Skip Checks** (3 test files):
   - Deleted environment variable checks from `setup_test.go`
   - Deleted environment variable checks from `radarr_setup_test.go`
   - Deleted environment variable checks from `symlink_lifecycle_test.go`
   - Tests now execute immediately without flag requirement

2. **Created Comprehensive Documentation** (`test/README.md`):
   - Complete test infrastructure overview (21-step validation)
   - Docker setup instructions with docker-compose.yml
   - Test assets documentation (config, 7 sample movies)
   - Usage examples for running tests
   - Troubleshooting section for common issues
   - Development workflow and cleanup instructions

3. **Fixed Test Name References**:
   - Changed 5 instances: `TestSetup` → `TestInfrastructure`
   - Changed 2 instances: `TestSymlink` → `TestSymlinkLifecycle`
   - All `-run` patterns now match actual function names

**Files Modified (Not Yet Committed):**
- `test/integration/setup_test.go` (-4 lines) - Removed skip check
- `test/integration/radarr_setup_test.go` (-4 lines) - Removed skip check
- `test/integration/symlink_lifecycle_test.go` (-4 lines) - Removed skip check
- `test/README.md` - NEW (610 lines) - Comprehensive integration test documentation
- `AGENTS.md` - This session summary

**Commits:**
- Pending user approval (all 5 files ready to commit)

**Current State:**
- Running: No (documentation and minor code changes)
- Tests passing: 405/405 unit tests ✅
- Integration tests: Ready to run without environment variable ✅
- Documentation: Complete with correct test names ✅
- Known issues: None

**Workflow Improvement:**
```bash
# OLD: Required environment variable
OXICLEANARR_INTEGRATION_TEST=1 go test -v ./test/integration/ -run TestInfrastructure

# NEW: Direct execution (simpler)
go test -v ./test/integration/ -run TestInfrastructure
```

**Documentation Highlights:**
- 21-step infrastructure validation checklist
- Docker container setup with health checks
- Test media library (7 sample movies with .nfo files)
- Plugin verification (OxiCleanarr Bridge v3.2.1.0)
- Data consistency validation across all services
- Troubleshooting guide for common Docker issues

**Next Session TODO:**
- [ ] Implement symlink lifecycle tests (placeholder from Session 42)
- [ ] Test scenarios: create symlinks, update retention, verify cleanup
- [ ] Validate OxiCleanarr Bridge plugin API integration
- [ ] End-to-end test: OxiCleanarr sync → plugin creates symlinks → Jellyfin library updates

**Key Lessons:**
1. **Simplicity wins**: Tests should run by default without special configuration
2. **Documentation accuracy**: Test names in docs must match actual function names
3. **Pattern matching**: Go test `-run` uses regex matching on function names
4. **Environment variables**: Only use env vars for optional features, not core functionality
5. **Developer experience**: Remove friction from test execution workflow

---

## Previous Session: Nov 9, 2025 (Session 42)

### Integration Test Infrastructure with Complete Validation - COMPLETED ✅

**Work Completed:**
- ✅ Committed comprehensive integration test infrastructure from Sessions 41-42
- ✅ Changed plugin verification from warning to fatal error (Part 1)
- ✅ Added plugin API endpoint verification (Part 2)
- ✅ Added data consistency validation across all services (Part 3)
- ✅ Cleaned up old integration-test/ directory (replaced by test/integration/)
- ✅ All 405 unit tests still passing
- ✅ Integration test skips without OXICLEANARR_INTEGRATION_TEST=1 flag
- ✅ 4 commits created

**Problem Identified (Part 1):**
- OxiCleanarr Bridge plugin verification was non-fatal (logged warnings)
- No point running symlink lifecycle tests if plugin missing/inactive
- Better to fail early with clear error than get confusing failures later

**Problem Identified (Part 2):**
- Plugin installation check only verified plugin was installed (via `/Plugins` endpoint)
- Didn't verify plugin's custom API endpoint was functional (the actual API OxiCleanarr will use)
- Tests could pass plugin check but fail during symlink operations due to broken API

**Problem Identified (Part 3):**
- Infrastructure test didn't validate data consistency across services
- No verification that Radarr, Jellyfin, and OxiCleanarr all see the same movie count
- Could have mismatched data (e.g., Radarr has 7 movies but Jellyfin only indexed 5)
- Better to verify complete data pipeline before running lifecycle tests

**Solution Implemented (Part 1):**
1. **Made Plugin Check Fatal** (`jellyfin_setup.go` lines 820-832):
   - Removed "non-fatal" language from error messages
   - Missing plugin now returns: `"OxiCleanarr Bridge plugin not found in Jellyfin - required for symlink integration tests"`
   - Inactive status returns: `"plugin found but status is '%s' (expected 'Active')"`

2. **Updated Test Failure Handling** (`setup_test.go` lines 97-101):
   - Changed from: `t.Logf("⚠️ Plugin verification warning (non-fatal): %v", err)`
   - Changed to: `t.Fatalf("❌ Plugin verification failed: %v", err)`
   - Now stops test execution immediately if plugin unavailable

**Solution Implemented (Part 2):**
1. **Added API Endpoint Verification** (`jellyfin_setup.go` after line 833, ~37 lines):
   - Created `VerifyOxiCleanarrPluginAPI(t, jellyfinURL, apiKey)` function
   - Calls `GET /api/oxicleanarr/status` endpoint (the actual API OxiCleanarr will use)
   - Verifies 200 OK response and parses JSON response
   - Returns fatal error if endpoint is not functional

2. **Integrated into Setup Test** (`setup_test.go` line ~102, +5 lines):
   - Added new **Step 7c**: "Verifying OxiCleanarr plugin API endpoint..."
   - Calls `VerifyOxiCleanarrPluginAPI()` after Step 7b (plugin installation check)
   - Uses `t.Fatalf()` for fatal error (stops test if API not functional)

**Solution Implemented (Part 3):**
1. **Added Count Query Helpers** (`helpers.go` lines 770-832, +63 lines):
   - Created `GetRadarrMovieCount(t, radarrURL, apiKey)` - Queries `/api/v3/movie` endpoint
   - Created `GetJellyfinMovieCount(t, jellyfinURL, apiKey, libraryID)` - Queries `/Items` with filters
   - Both functions return actual counts from service APIs

2. **Added Data Consistency Step** (`setup_test.go` lines 200-225, +26 lines):
   - Added **Step 20**: "Validating data consistency across all services..."
   - Queries all 3 services: Radarr, Jellyfin, OxiCleanarr
   - Validates all report same count (7 movies expected)
   - Fails with descriptive error if any mismatch detected
   - Success: "All services report 7 movies"

3. **Enhanced Summary Output** (`setup_test.go` lines 230-233):
   - Summary now displays actual validated counts from all services
   - Added line: "Data consistency: All 3 services validated with matching counts"
   - Test now has 21 total steps (was 20 steps before Session 42)

**Files Modified & Committed:**
- `test/integration/jellyfin_setup.go` (~87 lines changed total) - Fatal error messages + API verification
- `test/integration/helpers.go` (+63 lines) - Count query helpers for validation
- `test/integration/setup_test.go` (~41 lines changed total) - Fatal test failure + API verification + data consistency
- `test/assets/` - 21 new files (config, docker-compose, 7 test movies)
- `test/integration/` - 5 Go test files (helpers, setup, Radarr, lifecycle)
- Deleted: `integration-test/` directory (4 old shell scripts)

**Commits:**
1. `99bb2fa` - feat: add comprehensive integration test infrastructure with fatal plugin verification
2. `a8e31ad` - chore: remove old integration-test directory (replaced by test/integration/)
3. `1a1daab` - test: add OxiCleanarr plugin API endpoint verification to integration tests
4. `8a3646e` - test: add plugin API verification and data consistency validation to integration tests

**Current State:**
- Running: No (test infrastructure ready)
- Tests passing: 405/405 unit tests ✅
- Integration tests: Skip without flag (as designed)
- Known issues: None
- Net change: +3,048 lines added, -1,101 lines removed
- Infrastructure validation: 21 steps (plugin API + data consistency)

**Integration Test Infrastructure Added:**
- **test/assets/docker-compose.yml** - Jellyfin, Radarr, OxiCleanarr containers
- **test/assets/config/config.yaml** - Test configuration
- **test/assets/test-media/movies/** - 7 sample movies with .mkv + .nfo files
- **test/integration/helpers.go** (819 lines) - HTTP helpers, container detection
- **test/integration/jellyfin_setup.go** (869 lines) - User creation, library setup, **plugin verification**
- **test/integration/radarr_setup_test.go** (612 lines) - Quality profiles, movie import
- **test/integration/setup_test.go** (263 lines) - 20-step infrastructure validation
- **test/integration/symlink_lifecycle_test.go** (207 lines) - Placeholder for Session 43

**20-Step Infrastructure Test Validates:**
1. ✅ Jellyfin container running and reachable
2. ✅ Jellyfin public API accessible
3. ✅ Admin credentials work
4. ✅ Test user 'testuser' created successfully
5. ✅ Test user API key generated
6. ✅ Test media library created
7a. ✅ Test media directory scanned (7 movies found)
7b. ✅ **OxiCleanarr Bridge plugin verified (version 3.2.1.0, Active)** ← NEW STEP
8. ✅ Radarr container running and reachable
9. ✅ Radarr API accessible
10. ✅ Quality profile created
11. ✅ Root folder configured
12. ✅ Test movies imported (7 total)
13. ✅ OxiCleanarr container running and reachable
14. ✅ OxiCleanarr API accessible
15. ✅ OxiCleanarr config valid
16. ✅ Network connectivity validated (172.25.0.x)
17. ✅ All integrations enabled
18. ✅ Symlink library feature enabled
19. ✅ Leaving-soon base path configured
20. ✅ Infrastructure ready for lifecycle tests

**Next Session TODO:**
- [ ] Implement symlink lifecycle tests in `test/integration/symlink_lifecycle_test.go`
- [ ] Test scenarios: create symlinks, update retention, verify cleanup
- [ ] Validate OxiCleanarr Bridge plugin API integration
- [ ] End-to-end test: OxiCleanarr sync → plugin creates symlinks → Jellyfin library updates

**Key Lessons:**
1. **Fatal vs non-fatal**: Critical dependencies should fail tests immediately
2. **Early failure**: Better to fail at setup than during actual test execution
3. **Clear errors**: "Required for symlink integration tests" explains why it's fatal
4. **Test infrastructure**: 2,985 lines of setup enables comprehensive integration testing
5. **Docker-based testing**: Real containers ensure accurate behavior validation

---

## Current Session: Nov 9, 2025 (Session 43)

### Part 1: .gitignore Update - COMPLETED ✅

**Work Completed:**
- ✅ Resumed from Session 42 Part 3 completion
- ✅ Verified all 4 commits from Session 42 were successfully created
- ✅ Updated .gitignore to exclude oxicleanarr binary and mise.toml
- ✅ Committed .gitignore changes

**Problem Identified:**
- `oxicleanarr` binary and `mise.toml` showing as uncommitted in git status
- These are local development files that shouldn't be tracked
- .gitignore needed updating to exclude them

**Solution Implemented:**
- Added `/oxicleanarr` to .gitignore (binary file)
- Added `/mise.toml` to .gitignore (mise dev tool config)

**Files Modified & Committed:**
- `.gitignore` (+2 lines) - Added oxicleanarr binary and mise.toml

**Commits:**
1. `0b6c812` - chore: update .gitignore for oxicleanarr binary and add mise.toml

### Part 2: Test Config API Key Sanitization - COMPLETED ✅

**Work Completed:**
- ✅ Identified security issue: real API keys in test config
- ✅ Used git filter-branch to remove file from all 145 commits
- ✅ Created sanitized config with empty API key placeholders
- ✅ Committed clean version for integration tests
- ✅ All 405 unit tests still passing

**Security Issue Identified:**
- `test/assets/config/config.yaml` contained real API keys in git history
- Keys exposed: Jellyfin, Radarr, Sonarr, Jellyseerr, Jellystat (5 services)
- File was part of integration test infrastructure added in Session 42
- Keys committed in 6 different commits

**Solution Implemented:**
1. **Git History Cleanup:**
   - Used `git filter-branch --index-filter` to remove file from all commits
   - Verified removal: File shows `rm 'test/assets/config/config.yaml'` in history
   - Backup refs created at `.git/refs/original/` (filter-branch safety)
   - File completely purged from repository history ✅

2. **Created Sanitized Config:**
   - All `api_key` fields set to empty strings: `""`
   - Added comments: `# Set dynamically by integration tests`
   - Structure matches integration test requirements
   - Safe for public repository (no credentials exposed)

**Files Modified & Committed:**
- `test/assets/config/config.yaml` (+57 lines) - Sanitized config with placeholder keys

**Commits:**
2. `7970ec4` - test: add sanitized integration test config with placeholder API keys

**Security Note:**
- User should rotate exposed API keys as precaution
- 5 keys were exposed (some shared between services):
  - Radarr/Sonarr/Jellyseerr/Jellystat: `96a09523b3654d56abc553ba01e3b5e7`
  - Jellyfin: `e02c3f1b90434bfcb6104f47843318cd`

**Current State:**
- Running: No (documentation tasks only)
- Tests passing: 405/405 unit tests ✅
- Integration tests: Infrastructure complete, ready for lifecycle tests
- Known issues: None
- Git history: Cleaned of sensitive data ✅

**Integration Test Compatibility:**
- Tests expect config at `test/assets/config/config.yaml` ✅
- Tests call `UpdateConfigAPIKeys()` to populate keys at runtime ✅
- Config structure matches helper function expectations ✅

**Next Session TODO:**
- [ ] Begin Session 44: Implement symlink lifecycle tests
- [ ] Test scenarios: create symlinks, update retention, verify cleanup
- [ ] Validate OxiCleanarr Bridge plugin API integration
- [ ] End-to-end test: OxiCleanarr sync → plugin creates symlinks → Jellyfin library updates

**Key Lessons:**
1. **Git filter-branch**: Effective for removing sensitive data from history
2. **Test configs**: Always use placeholder values, populate at runtime
3. **Integration tests**: Should never commit real credentials
4. **Security**: Even test credentials should be rotated after exposure
5. **.gitignore importance**: Always verify development files are excluded

---

## Current Session: Nov 9, 2025 (Session 44)

### Integration Test Environment Variable Removal & Documentation Fixes - COMPLETED ✅

**Work Completed:**
- ✅ Removed `OXICLEANARR_INTEGRATION_TEST=1` environment variable requirement from all integration tests
- ✅ Tests now run by default without special flag (simpler workflow)
- ✅ Created comprehensive `test/README.md` documentation (610 lines)
- ✅ Fixed 7 incorrect test name references in README (5 TestSetup + 2 TestSymlink)
- ✅ All 405 unit tests still passing

**Problem Identified:**
- Integration tests required `OXICLEANARR_INTEGRATION_TEST=1` env var to run (from Session 42)
- Added friction to development workflow (extra flag required)
- Documentation referenced wrong test function names:
  - 5 instances of `TestSetup` (should be `TestInfrastructure`)
  - 2 instances of `TestSymlink` (should be `TestSymlinkLifecycle`)
- Test patterns `-run TestSetup` and `-run TestSymlink` wouldn't match actual function names

**Solution Implemented:**
1. **Removed Skip Checks** (3 test files):
   - Deleted environment variable checks from `setup_test.go`
   - Deleted environment variable checks from `radarr_setup_test.go`
   - Deleted environment variable checks from `symlink_lifecycle_test.go`
   - Tests now execute immediately without flag requirement

2. **Created Comprehensive Documentation** (`test/README.md`):
   - Complete test infrastructure overview (21-step validation)
   - Docker setup instructions with docker-compose.yml
   - Test assets documentation (config, 7 sample movies)
   - Usage examples for running tests
   - Troubleshooting section for common issues
   - Development workflow and cleanup instructions

3. **Fixed Test Name References**:
   - Changed 5 instances: `TestSetup` → `TestInfrastructure`
   - Changed 2 instances: `TestSymlink` → `TestSymlinkLifecycle`
   - All `-run` patterns now match actual function names

**Files Modified (Not Yet Committed):**
- `test/integration/setup_test.go` (-4 lines) - Removed skip check
- `test/integration/radarr_setup_test.go` (-4 lines) - Removed skip check
- `test/integration/symlink_lifecycle_test.go` (-4 lines) - Removed skip check
- `test/README.md` - NEW (610 lines) - Comprehensive integration test documentation
- `AGENTS.md` - This session summary

**Commits:**
- Pending user approval (all 5 files ready to commit)

**Current State:**
- Running: No (documentation and minor code changes)
- Tests passing: 405/405 unit tests ✅
- Integration tests: Ready to run without environment variable ✅
- Documentation: Complete with correct test names ✅
- Known issues: None

**Workflow Improvement:**
```bash
# OLD: Required environment variable
OXICLEANARR_INTEGRATION_TEST=1 go test -v ./test/integration/ -run TestInfrastructure

# NEW: Direct execution (simpler)
go test -v ./test/integration/ -run TestInfrastructure
```

**Documentation Highlights:**
- 21-step infrastructure validation checklist
- Docker container setup with health checks
- Test media library (7 sample movies with .nfo files)
- Plugin verification (OxiCleanarr Bridge v3.2.1.0)
- Data consistency validation across all services
- Troubleshooting guide for common Docker issues

**Next Session TODO:**
- [ ] Implement symlink lifecycle tests (placeholder from Session 42)
- [ ] Test scenarios: create symlinks, update retention, verify cleanup
- [ ] Validate OxiCleanarr Bridge plugin API integration
- [ ] End-to-end test: OxiCleanarr sync → plugin creates symlinks → Jellyfin library updates

**Key Lessons:**
1. **Simplicity wins**: Tests should run by default without special configuration
2. **Documentation accuracy**: Test names in docs must match actual function names
3. **Pattern matching**: Go test `-run` uses regex matching on function names
4. **Environment variables**: Only use env vars for optional features, not core functionality
5. **Developer experience**: Remove friction from test execution workflow

---

## Previous Session: Nov 8, 2025 (Session 40)

### Symlink Tracking Logic Fix - COMPLETED ✅

**Work Completed:**
- ✅ Fixed mock plugin implementation to use correct `PluginSymlinkItem` fields
- ✅ Fixed symlink tracking to only include successfully created symlinks
- ✅ Added verification logic to query filesystem after creation in live mode
- ✅ Implemented mode-aware tracking (dry-run vs live with proper verification)
- ✅ All 405 tests passing (up from 394, subtests expanded)
- ✅ 1 commit created

**Problem Identified:**
- Mock plugin `AddSymlinks()` used non-existent `item.Source` and `item.Target` fields
- Service tracked ALL symlinks in `pendingSymlinks` before creation attempt
- When plugin skipped items (missing source files), they remained in `currentSymlinks` map
- Cleanup would try to remove symlinks that never existed (incorrect tracking)
- Root cause: No verification of what actually got created vs what was attempted

**Root Cause:**
- Service immediately tracked symlinks in `currentSymlinks` map at lines 420-421 (old code)
- Plugin API only returns aggregate counts (created/skipped/failed), not which items succeeded
- If plugin skipped items, they were still tracked as existing symlinks
- This caused incorrect cleanup behavior and stale symlink references

**Solution Implemented:**
1. **Fixed Mock Plugin** (`symlink_library_test.go`):
   - Changed `item.Source` → `item.TargetDirectory` (correct field)
   - Changed `item.Target` → `item.Path` (correct field)
   - Used `filepath.Glob()` to find media files in target directory
   - Created proper symlinks with accurate count tracking

2. **Added Pending Tracking** (`symlink_library.go` line 370):
   - Created `pendingSymlinks` map to store path→name mappings temporarily
   - Changed line 421 from immediate tracking to pending storage

3. **Added Verification Logic** (`symlink_library.go` lines 473-503):
   - **Dry-run mode**: Track all pending symlinks (show what would be created)
   - **Live mode with creations**: Query `ListSymlinks()` to verify actual filesystem state
   - **Live mode no creations**: Track nothing
   - **Verification failure fallback**: Track all pending (safer than tracking nothing)

**Files Modified & Committed:**
- `internal/services/symlink_library.go` (+135 lines, -92 lines) - Fixed tracking logic
- `internal/services/symlink_library_test.go` (+52 lines) - Fixed mock plugin

**Commits:**
1. `aa8d2e9` - fix: correct symlink tracking to only include successfully created symlinks

**Current State:**
- Running: No (implementation complete)
- Tests passing: 405/405 ✅ (all 5 packages, subtests expanded)
- Known issues: None
- Build: Pending rebuild
- Docker: Not yet published (Session 39 image still latest)

**Key Implementation Detail:**
- Plugin API returns only aggregate counts, not which specific items succeeded
- Must query `ListSymlinks()` after creation to determine actual filesystem state
- This ensures cleanup only removes symlinks that actually exist
- Dry-run mode doesn't need verification since nothing is created

**Testing Results:**
- ✅ `TestCreateSymlinks/creates_symlinks_successfully` - Live mode with verification
- ✅ `TestCreateSymlinks/dry-run_mode_does_not_create_symlinks` - Tracks all pending
- ✅ `TestCreateSymlinks/skips_missing_source_files` - Plugin skips correctly, tracking accurate
- ✅ All other tests passing (auth, config, handlers, storage, clients)

**Next Session TODO:**
- [ ] Rebuild binary to verify changes work end-to-end
- [ ] Manual integration testing with real Jellyfin instance
- [ ] Consider Docker release (v1.3.1 or v1.4.0) if testing successful
- [ ] User-based cleanup with watch tracking integration
- [ ] Mobile responsiveness improvements

**Key Lessons:**
1. **Plugin API limitations**: Aggregate counts require post-creation verification
2. **Filesystem state verification**: Can't trust counts alone, must query actual state
3. **Map tracking purpose**: `currentSymlinks` used by cleanup, must be accurate
4. **Mock test importance**: Proper mock implementation caught the tracking bug
5. **Mode-aware logic**: Dry-run and live mode have different verification needs

---

## Previous Session: Nov 5, 2025 (Session 39)

### Jellyfin Symlink Library Cleanup Fix - COMPLETED ✅

**Work Completed:**
- ✅ Fixed symlink cleanup when libraries become empty
- ✅ Fixed Jellyfin dashboard refresh after library deletion
- ✅ Added 4-step cleanup process for empty libraries
- ✅ All tests passing (394 test runs with subtests)
- ✅ 1 commit created

**Problem Identified:**
- User reported two issues when disabling retention rules:
  1. Empty "Leaving Soon - Movies" library still visible in Jellyfin dashboard after deletion
     - Library correctly removed from Admin > Libraries settings
     - Dashboard only updated after Jellyfin restart
  2. Symlink files not cleaned up when library becomes empty
     - Example: `/data/media/leaving-soon/movies/Red Dawn (2012)` symlink remained
     - Logs showed `item_count=0` and library deleted from Jellyfin

**Root Cause:**
- In `internal/services/symlink_library.go` lines 152-210:
  - When `len(items) == 0` and `hide_when_empty: true`:
    - ✅ Code deleted Jellyfin virtual folder (line 190)
    - ✅ Returned early at line 201
    - ❌ **Never reached `cleanupSymlinks()` call** (line 229)
    - ❌ **Never triggered `RefreshLibrary()` call** (line 240)
- Early return prevented both filesystem cleanup AND Jellyfin UI refresh

**Solution Implemented:**
1. **Step 1: Clean up symlinks BEFORE deletion** (lines 159-172):
   - Call `cleanupSymlinks()` with empty map (removes all symlinks)
   - Graceful error handling (warnings, no sync failure)
   
2. **Step 2: Check and delete virtual folder** (lines 174-196):
   - Existing logic preserved, improved logging
   
3. **Step 3: Track deletion status** (lines 198-220):
   - Use `libraryDeleted` boolean instead of early return
   - Allows subsequent cleanup steps to execute
   
4. **Step 4: Trigger Jellyfin refresh** (lines 222-233):
   - Call `RefreshLibrary()` after successful deletion
   - Updates dashboard without Jellyfin restart
   - Graceful error handling for refresh failures

**Files Modified & Committed:**
- `internal/services/symlink_library.go` (+27 lines, -18 lines) - Complete cleanup fix

**Commits:**
1. `3b870e9` - fix: cleanup symlinks and refresh Jellyfin when empty library is removed
2. `a23b4e0` - docs: add Session 39 summary - Jellyfin symlink library cleanup fix

**Docker Release:**
- ✅ Built and pushed to Docker Hub: ramonskie/oxicleanarr:latest
- Image digest: sha256:02a3118c7b93260ae8a2bcdd374d1b641ec7546b5abeb0f5439f127025beabae
- Image size: 19.2 MB
- Image ID: 37cc430a0c89

**Current State:**
- Running: No (implementation complete)
- Tests passing: 394/394 ✅ (all 5 packages)
- Known issues: None
- Build: Successful (./oxicleanarr binary ready)
- Docker: Published to Docker Hub ✅
- Ready for user testing ✅

**Expected Behavior After Fix:**
When retention rules disabled → sync completes:
1. ✅ Symlinks removed from `/data/media/leaving-soon/movies/`
2. ✅ Virtual folder deleted from Jellyfin
3. ✅ `POST /Library/Refresh` triggered
4. ✅ **Dashboard updates immediately** (no restart needed)

**Key Benefits:**
- **Filesystem hygiene**: Orphaned symlinks no longer accumulate
- **UI consistency**: Dashboard reflects library state without restart
- **Graceful degradation**: Warnings for errors, sync continues
- **Improved logging**: Clear 4-step process for troubleshooting

**How Users Can Test:**
```bash
# Pull latest image with fix
docker pull ramonskie/oxicleanarr:latest

# Or update existing deployment
docker-compose pull oxicleanarr
docker-compose up -d oxicleanarr
```

**Next Session TODO:**
- [ ] User verification: Test with live Jellyfin instance
- [ ] Verify symlinks cleaned up when library empty
- [ ] Verify dashboard updates without Jellyfin restart
- [ ] Consider version tag (v1.4.0) if user confirms fix works
- [ ] User-based cleanup with watch tracking integration
- [ ] Mobile responsiveness improvements

---

## Previous Session: Nov 5, 2025 (Session 38)

### Config Validation Bug Fix - COMPLETED ✅

**Work Completed:**
- ✅ Fixed validation to skip admin credentials when `disable_auth: true`
- ✅ Added 7 comprehensive test cases for auth validation combinations
- ✅ Added debug logging for admin config lifecycle troubleshooting
- ✅ Added debug logging for empty symlink library deletion
- ✅ All tests passing (394 test runs with subtests)
- ✅ 3 commits created (validation fix, debug logging x2)

**Problem Identified:**
- User reported validation errors when updating advanced rules via UI:
  ```
  Failed to reload configuration | error=Configuration validation failed:
    - admin.username: required
    - admin.password: required
    - integrations: at least one integration must be enabled
  ```
- Root cause: `internal/config/validation.go` lines 38-50 **always validated admin credentials**
- Impact: Users with `disable_auth: true` couldn't update config via UI
- Bug only affected auth-disabled mode (not caught during normal testing)

**Solution Implemented:**
1. **Validation Fix** (`validation.go` lines 38-51):
   - Wrapped admin credential checks in `!cfg.Admin.DisableAuth` condition
   - Empty username/password now allowed when `disable_auth: true`
   - Normal validation enforced when `disable_auth: false`
   
2. **Comprehensive Tests** (`validation_test.go` +72 lines):
   - 7 test cases covering all combinations:
     - `disable_auth=true` + empty credentials → PASS ✅
     - `disable_auth=true` + username only → PASS ✅
     - `disable_auth=true` + password only → PASS ✅
     - `disable_auth=true` + both credentials → PASS ✅
     - `disable_auth=false` + empty username → FAIL ✅
     - `disable_auth=false` + empty password → FAIL ✅
     - `disable_auth=false` + both credentials → PASS ✅

3. **Debug Logging Added** (Session 37 carryover):
   - Admin config logging in `config.Load()` (after unmarshal, after defaults)
   - Admin config logging in config handler (before marshal, YAML preview)
   - Empty library deletion detailed logging in `symlink_library.go`
   - Helps troubleshoot validation and library sync issues

**Files Modified & Committed:**
- `internal/config/validation.go` (+3 lines) - Skip validation when disable_auth
- `internal/config/validation_test.go` (+72 lines) - 7 auth validation test cases
- `internal/config/config.go` (+14 lines) - Admin config debug logging
- `internal/api/handlers/config.go` (+14 lines) - Marshal debug logging
- `internal/services/symlink_library.go` (+18 lines) - Empty library deletion logging

**Commits:**
1. `f943849` - fix: skip admin credential validation when disable_auth is true
2. `2839c18` - debug: add admin config logging for troubleshooting validation issues
3. `8fbf0a1` - debug: add detailed logging for empty library deletion

**Current State:**
- Running: No (implementation complete)
- Tests passing: 394/394 ✅ (292 subtests across 5 packages)
- Known issues: None
- Build: Successful (./oxicleanarr binary ready)
- Ready for user testing with `disable_auth: true` config

**Key Benefits:**
- **Auth-disabled mode works**: Users can now update config/rules via UI without authentication
- **Better validation logic**: Respects authentication mode setting
- **Test coverage**: All validation scenarios covered
- **Debug tooling**: Comprehensive logging for future troubleshooting

**Next Session TODO:**
- [ ] User verification: Test advanced rule updates with `disable_auth: true`
- [ ] Consider Docker release (v1.3.1 or v1.4.0) if user confirms fix works
- [ ] Live environment testing with user's Jellyfin instance
- [ ] User-based cleanup with watch tracking integration
- [ ] Mobile responsiveness improvements

---

## Previous Session: Nov 5, 2025 (Session 37)

### `hide_when_empty` Feature for Symlink Libraries - COMPLETED ✅

**Work Completed:**
- ✅ Added `HideWhenEmpty bool` field to `SymlinkLibraryConfig` (default: true)
- ✅ Implemented automatic deletion of empty symlink libraries from Jellyfin sidebar
- ✅ Updated `syncLibrary()` to detect empty libraries and delete them
- ✅ Added 5 comprehensive unit tests covering all edge cases
- ✅ All 394 tests passing

**Problem Identified:**
- User had empty "Leaving Soon - TV Shows" library visible in Jellyfin sidebar
- No TV shows scheduled for deletion, making the library pointless
- Cluttered sidebar with empty sections reduces UX quality
- Should match behavior from Collections feature (Session 20)

**Solution Implemented:**
1. **Config Structure** (`types.go`, `defaults.go`):
   - Added `HideWhenEmpty bool` field (default: true for better UX)
   - Users can set to `false` to keep empty libraries visible
   - Backward compatible with existing configs

2. **Core Logic** (`symlink_library.go` lines 145-193):
   - Check if library is empty AND `hide_when_empty: true`
   - Query Jellyfin for existing virtual folders
   - Delete library if it exists and is empty
   - Early return to skip normal sync operations
   - Graceful error handling (doesn't fail entire sync)

3. **Comprehensive Testing** (5 new test cases):
   - Delete when hide_when_empty is true
   - Keep library when hide_when_empty is false
   - Library lifecycle transitions (items → empty → deleted)
   - Dry-run mode respects flag but doesn't delete
   - Handling non-existent libraries gracefully

**Files Modified & Committed:**
- `internal/config/types.go` (+1 line) - Added HideWhenEmpty field
- `internal/config/defaults.go` (+7 lines) - Set default to true
- `config/config.yaml.example` (+3 lines) - Documentation
- `internal/services/symlink_library.go` (+42 lines) - Deletion logic
- `internal/services/symlink_library_test.go` (+224 lines) - Unit tests

**Commits:**
1. `edaebcb` - feat: add hide_when_empty option for symlink libraries

**Current State:**
- Running: No (implementation complete)
- Tests passing: 394/394 ✅
- Known issues: None
- Live testing: Pending user availability

**Key Benefits:**
- **Cleaner UI**: Empty libraries automatically removed from Jellyfin sidebar
- **Per-library control**: Movies and TV shows evaluated independently
- **Consistent behavior**: Matches Collections `hide_when_empty` pattern
- **User choice**: Can be disabled via config if desired

**Next Session TODO:**
- [ ] Live environment testing with user's Jellyfin instance
- [ ] Verify empty library deletion and sidebar updates
- [ ] Consider Docker release (v1.3.1 or v1.4.0) if testing successful
- [ ] User-based cleanup with watch tracking integration
- [ ] Mobile responsiveness improvements

---

## Previous Session: Nov 5, 2025 (Session 36)

### Sync Scheduler Hot-Reload - COMPLETED ✅

**Work Completed:**
- ✅ Added `RestartScheduler()` method to dynamically recreate tickers with new intervals
- ✅ Fixed config reading to use `config.Get()` instead of stale struct pointers
- ✅ Implemented interval change detection in config handler
- ✅ Fixed test failures by adding `config.SetTestConfig()` calls
- ✅ All 394 tests passing

**Problem Identified:**
- Sync scheduler only read intervals once at startup via `time.NewTicker()`
- Config hot-reload updated in-memory values but didn't recreate tickers
- **Root cause**: `config.Reload()` creates NEW config struct, but `SyncEngine` held pointer to OLD struct
- Changing intervals required full application restart (poor UX)

**Root Cause:**
- `Start()` method used `e.config` (stale pointer) instead of `config.Get()` (fresh values)
- Go's `time.Ticker` cannot be updated dynamically - must Stop() and recreate
- No mechanism to detect interval changes and restart scheduler

**Solution Implemented:**
1. **Added `RestartScheduler()` method** (`sync.go` lines 159-194):
   - Stops existing scheduler safely
   - Waits 100ms for cleanup
   - Recreates `stopChan` (closed by Stop())
   - Calls `Start()` with new config values
   
2. **Fixed config reading** in `Start()` and `RestartScheduler()`:
   - Changed `e.config` → `config.Get()` to always read fresh values
   - Ensures new intervals used after hot-reload
   
3. **Added interval change detection** (`handlers/config.go` lines 399-414):
   - Captures old `full_interval` and `incremental_interval` before update
   - Compares with new values after config reload
   - Triggers async `RestartScheduler()` if changed
   - Skips restart when `auto_start: false` (manual mode)

**Files Modified & Committed:**
- `internal/services/sync.go` (+44 lines, -3 lines) - Added RestartScheduler(), fixed config.Get() usage
- `internal/api/handlers/config.go` (+15 lines) - Interval change detection and restart trigger
- `internal/services/sync_test.go` (+3 lines) - Fixed test to set global config
- `internal/api/handlers/sync_test.go` (+3 lines) - Fixed test helper to set global config

**Commits:**
1. `0151ba5` - feat: add hot-reload support for sync scheduler intervals

**Current State:**
- Running: Yes (backend PID varies per session)
- Tests passing: 394/394 ✅
- Known issues: None
- Scheduler hot-reload: Fully working ✅

**Testing Results:**
- ✅ Interval changes (300→600→900→1200 seconds) applied correctly
- ✅ Logs confirm new intervals used after config update
- ✅ Auto-start disabled mode handled correctly (no restart attempted)
- ✅ Rapid successive changes handled gracefully
- ✅ Test suite fixed with `config.SetTestConfig()` pattern

**Key Lessons:**
1. **Config pointer invalidation**: `Reload()` creates new struct, invalidating old pointers
2. **Solution pattern**: Always use `config.Get()` for hot-reload support, never store config pointers
3. **Ticker limitation**: No way to update ticker intervals - must Stop() and recreate
4. **Channel recreation**: `stopChan` must be recreated after `Stop()` closes it
5. **Test requirement**: Tests using global config must call `config.SetTestConfig()` first
6. **Async restart**: Run scheduler restart in goroutine to avoid blocking HTTP response

**Docker Hub Publication:**
- ✅ Published v1.3.0 to Docker Hub: ramonskie/oxicleanarr:v1.3.0 and :latest
- Image digest: sha256:43f8dcffceac3e1ff4ec09c1db9e3c9f95c56b43d2302c13e119621d191c70f7
- Image size: 19.2 MB (same as v1.2.0)
- Git tag created: v1.3.0
- Commit: 59a8414 (docs: add Session 36 summary)
- Image ID: b9dba909a45a

**Next Session TODO:**
- [ ] User-based cleanup with watch tracking integration
- [ ] Mobile responsiveness improvements
- [ ] Statistics/charts for disk space trends
- [ ] Consider adding UI indicator when scheduler restarts (optional UX improvement)

---

## Previous Session: Nov 5, 2025 (Session 33)

### Symlink Mount Simplification - COMPLETED ✅

**Work Completed:**
- ✅ Simplified symlink library setup by reusing existing `/data/media` mount
- ✅ Updated documentation to recommend single-mount approach as primary option
- ✅ Clarified why both approaches work and when to use each
- ✅ All 394 tests still passing

**Problem Identified:**
- User asked: "Why can't we reuse Jellyfin's existing `/data/media` mount instead of adding `/app/leaving-soon`?"
- Documentation (Session 32) showed separate mount as only approach
- User correctly identified this added unnecessary complexity

**Root Cause:**
- Documentation assumed separate directory was required
- Didn't consider that symlinks and targets in same filesystem is simpler
- Jellyfin already has `/data/media` mount that could include symlink subdirectory

**Solution Implemented:**
1. **Changed recommended approach** to `base_path: /data/media/leaving-soon`
2. **Updated all documentation** to show recommended + alternative approaches
3. **Simplified Jellyfin setup** - no extra mount needed!
4. **Benefits explained** - fewer mounts, simpler config, easier troubleshooting

**Recommended Config Structure** (simplified):
```yaml
integrations:
  jellyfin:
    enabled: true
    url: http://jellyfin:8096
    api_key: your-key
    symlink_library:
      enabled: true
      base_path: /data/media/leaving-soon  # Inside existing media mount!
```

**Docker Setup** (simplified):
```yaml
oxicleanarr:
  volumes:
    - /volume1/data/media:/data/media  # Creates symlinks at /data/media/leaving-soon/

jellyfin:
  volumes:
    - /volume1/data/media:/data/media:ro  # Already has access to symlinks! ✅
```

**Why This Is Better:**
- ✅ **Simpler**: One mount instead of two for Jellyfin
- ✅ **No extra config**: Jellyfin already has access
- ✅ **More reliable**: Symlinks and targets in same filesystem
- ✅ **Standard pattern**: Similar to how Radarr/Sonarr organize media
- ✅ **Easier debugging**: One mount to check, not two

**Alternative Approach** (still documented):
- `base_path: /app/leaving-soon` for clean separation
- Requires extra Jellyfin mount: `/volume3/docker/oxicleanarr/leaving-soon:/app/leaving-soon:ro`
- Use case: Want clear isolation of OxiCleanarr-managed content

**Files Modified & Committed:**
- `config/config.yaml.example` (+12 lines, -13 lines) - Show recommended approach first
- `NAS_DEPLOYMENT.md` (+48 lines, -41 lines) - Rewrite Step 5 to verify existing mount
- `docker-compose.nas.yml` (+10 lines, -6 lines) - Remove separate leaving-soon mount

**Commits:**
1. `876a27d` - docs: recommend reusing media mount for symlinks (simpler setup)

**Current State:**
- Running: No (documentation changes only)
- Tests passing: 394/394 ✅
- Known issues: None
- Documentation: Simplified and improved ✅
- Session 33: COMPLETE ✅

**User's Next Steps:**
- [ ] Deploy OxiCleanarr with `base_path: /data/media/leaving-soon` config
- [ ] Verify Jellyfin can see symlinks with existing `/data/media` mount
- [ ] Test "Leaving Soon" libraries appear in Jellyfin sidebar
- [ ] Confirm files playable (symlinks work end-to-end)

**Key Lessons:**
1. **Question assumptions**: User correctly challenged "why do we need this?"
2. **Simpler is better**: Reusing existing mounts reduces complexity
3. **Same filesystem**: Symlinks work best when source/target in same mount
4. **Document alternatives**: Show recommended approach + advanced options
5. **Listen to users**: They often spot unnecessary complexity we missed
6. **Credit where due**: User identified the optimization opportunity

---

## Previous Session: Nov 5, 2025 (Session 32)

### Documentation Fixes for Symlink Library & Docker Mounts - COMPLETED ✅

**Work Completed:**
- ✅ Fixed example config documentation (moved symlink_library to correct location)
- ✅ Added file vs directory mount troubleshooting to NAS_DEPLOYMENT.md
- ✅ Updated docker-compose example to use directory mounts
- ✅ All 394 tests still passing

**Problem Identified:**
- Example config (`config.yaml.example` lines 58-62) showed `symlink_library` at **root level**
- Actual code structure (`types.go` line 70) has it **inside** `integrations.jellyfin`
- User copied wrong structure from example, causing config parsing failures
- User hit permission errors mounting individual files instead of directories

**Root Causes:**
1. **Documentation drift**: Example config didn't match code structure
2. **File mount limitation**: Docker can't change ownership of bind-mounted individual files
3. **Synology user GID confusion**: User initially used wrong group (100 vs 65536)

**Solution Implemented:**
1. **Moved symlink_library documentation** to correct location (under `integrations.jellyfin`)
2. **Updated example config** with correct YAML structure and container paths (`/app/leaving-soon`)
3. **Added troubleshooting section** for file vs directory mount permission errors
4. **Updated docker-compose example**:
   - Changed from: `/volume3/docker/oxicleanarr/config.yaml:/app/config/config.yaml`
   - Changed to: `/volume3/docker/oxicleanarr/config:/app/config`
   - Added all directories: config, data, logs, leaving-soon
5. **Documented proper setup**: Create `config/` directory first, place file inside it

**Correct Config Structure** (confirmed from code):
```yaml
integrations:
  jellyfin:
    enabled: true
    url: http://jellyfin:8096
    api_key: your-key
    symlink_library:           # CORRECT: nested under jellyfin
      enabled: true
      base_path: /app/leaving-soon
```

**Files Modified & Committed:**
- `config/config.yaml.example` (+21 lines, -21 lines) - Moved symlink docs to correct location
- `NAS_DEPLOYMENT.md` (+38 lines, -6 lines) - Added file mount troubleshooting

**Commits:**
1. `9e4160b` - docs: fix symlink_library config location and add file mount warning

**Current State:**
- Running: No (documentation fix only)
- Tests passing: 394/394 ✅
- Known issues: None
- Documentation: Corrected ✅
- Docker Hub: v1.2.0 published (Session 31)

**User's Deployment Status:**
- ✅ Container starts without permission errors (after fixing mounts)
- ✅ Sync completes successfully (252 movies, 121 TV shows)
- ✅ Web UI accessible
- ⚠️ Retention = 0d (immediate deletion when dry_run disabled)
- ⚠️ Red Dawn movie missing Jellyfin ID (not imported yet)
- 🔄 User testing symlink library feature next

**Next Session TODO:**
- [ ] Help user test symlink library creation with corrected config structure
- [ ] Help user add Jellyfin volume mount for leaving-soon directory access
- [ ] Verify Jellyfin library creation works end-to-end
- [ ] Consider default retention values (explicit safe defaults like 90d?)
- [ ] User-based cleanup with watch tracking
- [ ] Mobile responsiveness improvements

**Key Lessons:**
1. **Documentation sync**: Always verify example configs match actual code structure
2. **File mounts block ownership**: Docker cannot `chown` bind-mounted individual files
3. **Directory structure**: Create `config/` directory first, never mount single files
4. **Config nesting**: Use `git grep "SymlinkLibrary"` to verify struct embedding in code
5. **Synology groups**: NAS users typically use group 65536 (users), not 100 (root/admin)
6. **Container networking**: `network_mode: synobridge` allows container name resolution on Synology
7. **Volume mount order**: Always test directory mounts before file mounts to avoid permission issues
8. **Config UI side effects**: Saving config may normalize URLs (IPs → container names)

---

## Previous Session: Nov 5, 2025 (Session 31)

### Docker PUID/PGID Simplification & SELinux Support - COMPLETED ✅

**Work Completed:**
- ✅ Identified and fixed SELinux bind mount write permission issue
- ✅ Simplified Docker PUID/PGID implementation (removed usermod/groupmod complexity)
- ✅ Removed shadow package dependency (image size reduced 31.6 MB → 19.2 MB, -39%)
- ✅ Added ownership fix loop in entrypoint for bind-mounted directories
- ✅ Documented SELinux `:z` flag requirement for Fedora/RHEL/CentOS
- ✅ Updated docker-compose.nas.yml with PUID/PGID examples and SELinux notes
- ✅ Updated NAS_DEPLOYMENT.md with SELinux troubleshooting section
- ✅ All 394 tests still passing

**Problem Solved:**
- Docker containers couldn't write to bind-mounted volumes on SELinux systems (Fedora, RHEL, CentOS)
- Previous v1.1.0 approach used complex usermod/groupmod logic from LinuxServer.io
- Unnecessarily large image size (31.6 MB) due to shadow package
- Root cause: SELinux was in Enforcing mode, blocking container writes without proper labels

**Solution Implemented:**
1. **SELinux Fix**: Documented `:z` flag requirement for volume mounts
   - Tells Docker to relabel volumes with `container_file_t` context
   - Required on Fedora, RHEL, CentOS (SELinux Enforcing mode)
   - Optional but harmless on Synology/QNAP (no SELinux)

2. **Simplified Entrypoint** (19 lines, down from 18 in v1.1.0):
   - No user creation at build time (removed `adduser`/`addgroup`)
   - No `usermod`/`groupmod` commands (removed shadow package)
   - Uses `su-exec "$PUID:$PGID"` directly to drop privileges
   - Ownership fix loop: checks `/app/config`, `/app/data`, `/app/logs`
   - Only runs `chown` when current ownership differs from target

3. **Dockerfile Improvements**:
   - Removed `shadow` package (saves 2.5 MB)
   - Removed user creation at build time
   - Container starts as root, entrypoint fixes ownership, then drops to PUID:PGID
   - Cleaner, simpler, smaller image

**Testing Results:**
- ✅ Tested on Fedora 43 with SELinux Enforcing mode
- ✅ Custom PUID=1027/PGID=65536 works correctly (Synology defaults)
- ✅ All directories writable with `:z` flag
- ✅ Files created with correct ownership on host
- ✅ Image size: 19.2 MB (vs 31.6 MB in v1.1.0, -39% reduction)

**Files Modified & Committed:**
- `Dockerfile` (-10 lines, +4 lines) - Removed shadow package and user creation
- `docker-entrypoint.sh` (19 lines total) - Simplified approach without usermod/groupmod
- `docker-compose.nas.yml` (+11 lines) - Added PUID/PGID env vars and SELinux notes
- `NAS_DEPLOYMENT.md` (+30 lines) - Added SELinux troubleshooting section

**Commits:**
1. `d52aed8` - feat: simplify Docker PUID/PGID implementation and add SELinux support

**Docker Hub Publication:**
- ✅ Published v1.2.0 to Docker Hub: ramonskie/oxicleanarr:v1.2.0 and :latest
- Image digest: sha256:d6eb302040ad97c38df4294d885d2b3ed62760562b20ff3b4cc1c88023214f24
- Git tag created: v1.2.0
- Commit: 42a231e (docs: add Session 31 summary)
- Both tags point to same image: b5bfcc674a53

**Version Comparison:**
- v1.0.0: 29.1 MB (base production image, no PUID/PGID)
- v1.1.0: 31.6 MB (+2.5 MB, usermod/groupmod approach with shadow package)
- v1.2.0: 19.2 MB (-12.4 MB, simplified approach, -39% vs v1.1.0)

**Current State:**
- Running: No (implementation complete, Docker Hub published ✅)
- Tests passing: 394/394 ✅
- Docker image: Published ramonskie/oxicleanarr:v1.2.0 (19.2 MB)
- Known issues: None
- Production ready: Yes ✅

**Usage Example (with SELinux):**
```yaml
services:
  oxicleanarr:
    image: ramonskie/oxicleanarr:v1.2.0
    environment:
      - PUID=1027        # Your NAS user ID
      - PGID=65536       # Your NAS group ID
      - TZ=Europe/Amsterdam
    volumes:
      # :z flag required for SELinux systems (Fedora, RHEL, CentOS)
      - /volume3/docker/oxicleanarr/config.yaml:/app/config/config.yaml:z
      - /volume3/docker/oxicleanarr/data:/app/data:z
      - /volume1/data:/data:ro
    ports:
      - 8080:8080
```

**Next Session TODO:**
- [ ] Test deployment on actual NAS system (Synology/QNAP) with v1.2.0
- [ ] Verify symlink library feature works with custom PUID/PGID
- [ ] Validate Jellyfin integration with symlinked libraries
- [ ] Update main README.md with Docker quick start guide (v1.2.0)
- [ ] Consider GitHub release notes (if remote repo exists)
- [ ] User-based cleanup with watch tracking
- [ ] Mobile responsiveness improvements

**Key Lessons:**
1. **SELinux matters**: Always test on SELinux systems (Fedora/RHEL) for production Docker images
2. **`:z` flag**: Essential for bind mounts on SELinux, harmless on other systems
3. **Simpler is better**: Direct `su-exec` approach cleaner than usermod/groupmod complexity
4. **Image size optimization**: Removing shadow package saved 39% image size (31.6 MB → 19.2 MB)
5. **Container security context**: `container_file_t` label allows container writes on SELinux
6. **Build-time vs runtime**: No need to create users at build time for PUID/PGID flexibility
7. **Ownership checks**: Only fix ownership when it actually differs (performance optimization)
8. **Docker layer caching**: Alpine base layer reused across versions = fast builds
9. **Semantic versioning**: v1.2.0 = minor feature (simplified approach) + patch (SELinux fix)

---

## Previous Session: Nov 4, 2025 (Session 30)

### Docker Container v1.1.0 Published - COMPLETED ✅

**Work Completed:**
- ✅ Built new Docker image with PUID/PGID support (v1.1.0)
- ✅ Published to Docker Hub: ramonskie/oxicleanarr:latest and ramonskie/oxicleanarr:v1.1.0
- ✅ Created git tag v1.1.0 with release message
- ✅ Tested published image with custom and default PUID/PGID values
- ✅ Verified entrypoint script works correctly in published image
- ✅ All 394 tests still passing

**Problem Solved:**
- Need to publish PUID/PGID support to Docker Hub for production use
- Users deploying on NAS systems need the new v1.1.0 image
- Version tagging ensures users can pin to stable releases

**Solution Implemented:**
- Multi-stage Docker build completed successfully
- Published two tags: `latest` (rolling) and `v1.1.0` (pinned version)
- Image digest: sha256:02095854512dfc7c58b67c864f9ea923e1e9a6756530864dde47a557e6506cfa
- Image size: 31.6 MB (2.5 MB increase from v1.0.0 due to shadow package)
- Git tag created locally: v1.1.0

**Docker Hub Publication:**
- Repository: ramonskie/oxicleanarr
- Tags published:
  - `latest` - Updated to v1.1.0 (rolling release)
  - `v1.1.0` - New versioned tag (stable release)
- Both tags point to same image: 11f0df2138a5

**Testing Results:**
- ✅ Custom PUID=1500/PGID=1500: Works correctly, shows "Setting ownership to 1500:1500..."
- ✅ Default PUID=1000/PGID=1000: Works correctly, shows "usermod: no changes" (efficient)
- ✅ Container runs as non-root after entrypoint initialization
- ✅ Entrypoint verified: `/docker-entrypoint.sh`

**Version Comparison:**
- v1.0.0: 29.1 MB (base production image)
- v1.1.0: 31.6 MB (+2.5 MB for shadow package, PUID/PGID support)

**Files Modified & Committed:**
- None (building and publishing existing code from Session 29)

**Commits:**
- Git tag `v1.1.0` created on commit `8ef2d43`
- Tag message: "Release v1.1.0: Docker PUID/PGID support for NAS compatibility"
- Note: No remote configured, tag exists locally only

**Current State:**
- Running: No (Docker testing complete)
- Tests passing: 394/394 ✅
- Docker Hub: Published ✅
- Git tag: Created locally ✅
- Production ready: Yes ✅

**Usage Example:**
```yaml
services:
  oxicleanarr:
    image: ramonskie/oxicleanarr:latest  # or v1.1.0
    environment:
      - PUID=1027        # Your NAS user ID
      - PGID=65536       # Your NAS group ID
      - TZ=Europe/Amsterdam
    volumes:
      - /volume3/docker/oxicleanarr/config.yaml:/app/config/config.yaml
      - /volume3/docker/oxicleanarr/data:/app/data
      - /volume1/data:/data:ro
    ports:
      - 8080:8080
```

**Next Session TODO:**
- [ ] Test deployment on actual NAS system (Synology/QNAP)
- [ ] Verify symlink library feature works with custom PUID/PGID
- [ ] Validate Jellyfin integration with symlinked libraries
- [ ] Update main README.md with Docker quick start guide
- [ ] Consider GitHub release notes (if remote repo exists)
- [ ] User-based cleanup with watch tracking
- [ ] Mobile responsiveness improvements

**Key Lessons:**
1. **Docker layer caching**: All layers cached from Session 29 build = fast rebuilds
2. **Image size trade-off**: +2.5 MB for NAS compatibility is acceptable
3. **Version tagging**: Use semantic versioning (v1.1.0 = minor feature addition)
4. **Tag strategy**: Both `latest` (rolling) and `vX.Y.Z` (pinned) for flexibility
5. **Testing published images**: Always verify published image works before announcing
6. **Production readiness**: PUID/PGID support makes OxiCleanarr NAS-ready

---

## Previous Session: Nov 4, 2025 (Session 29)

### Docker PUID/PGID Support - COMPLETED ✅

**Work Completed:**
- ✅ Created docker-entrypoint.sh script for dynamic user/group ID management
- ✅ Modified Dockerfile to use entrypoint wrapper instead of direct binary execution
- ✅ Added shadow package for usermod/groupmod commands
- ✅ Implemented PUID/PGID environment variable support (defaults to 1000:1000)
- ✅ Simplified entrypoint using usermod/groupmod approach (Linuxserver.io pattern)
- ✅ Automatic ownership fixes only when IDs change from defaults
- ✅ Enhanced NAS_DEPLOYMENT.md with improved Docker build instructions
- ✅ Tested successfully with custom user IDs (1001:1001) and defaults (1000:1000)
- ✅ All 394 tests still passing

**Problem Solved:**
- Docker containers need to match host system file permissions for media access
- NAS systems (Synology, QNAP) use custom user/group IDs (e.g., 1027:65536)
- Fixed permissions required for symlink creation and media file access
- Previous implementation ran as fixed UID/GID 1000, causing permission errors

**Solution Implemented:**
- Created `/docker-entrypoint.sh` script (18 lines, simplified from initial 34-line version)
- Container starts as root to allow user/group modification
- Uses `usermod -o` and `groupmod -o` to change IDs (allows duplicate IDs)
- Only runs ownership fix when IDs differ from defaults (performance optimization)
- Switches to oxicleanarr user via `su-exec` before starting application
- **Simplified approach**: No user deletion/recreation (more reliable and cleaner)

**Dockerfile Changes:**
- Line 45: Added `shadow` package for usermod/groupmod commands
- Line 64-65: Copy entrypoint script and make executable
- Line 72-73: Removed `USER oxicleanarr` directive (must start as root)
- Line 90: Changed ENTRYPOINT to `/docker-entrypoint.sh`
- Line 91: Changed CMD to pass oxicleanarr binary and args to entrypoint

**Entrypoint Script Features (Simplified):**
```sh
#!/bin/sh
set -e
PUID=${PUID:-1000}
PGID=${PGID:-1000}
groupmod -o -g "$PGID" oxicleanarr
usermod -o -u "$PUID" oxicleanarr
if [ "$PUID" != "1000" ] || [ "$PGID" != "1000" ]; then
    echo "Setting ownership to $PUID:$PGID..."
    chown -R oxicleanarr:oxicleanarr /app
fi
exec su-exec oxicleanarr "$@"
```

**Key Design Decision:**
- Initial implementation used complex `deluser`/`delgroup`/`adduser`/`addgroup` logic
- **Simplified to Linuxserver.io approach**: Use `usermod -o` and `groupmod -o`
- Advantages: More reliable, cleaner code, fewer potential errors
- The `-o` flag allows duplicate UIDs/GIDs which is exactly what we need

**Files Modified & Committed:**
- `Dockerfile` (+18 lines, -10 lines) - Entrypoint integration, package additions
- `docker-entrypoint.sh` - NEW (18 lines) - PUID/PGID management script (simplified)
- `NAS_DEPLOYMENT.md` (+35 lines, -4 lines) - Enhanced Docker build docs

**Commits:**
1. `ec2c14f` - feat: add PUID/PGID support for Docker container user management (amended with simplified version)

**Current State:**
- Running: No (Docker image built for testing)
- Tests passing: 394/394 ✅
- Known issues: None
- Docker image: Builds successfully (~XX MB)
- PUID/PGID: Tested and working ✅

**Usage Example:**
```yaml
# docker-compose.yml
services:
  oxicleanarr:
    image: oxicleanarr:latest
    environment:
      - PUID=1027        # Your NAS user ID
      - PGID=65536       # Your NAS group ID
      - TZ=Europe/Amsterdam
    volumes:
      - /volume3/docker/oxicleanarr/config.yaml:/app/config/config.yaml
      - /volume3/docker/oxicleanarr/data:/app/data
      - /volume1/data:/data:ro
```

**Testing Results:**
- ✅ Container starts with default PUID=1000, PGID=1000
- ✅ User/group recreation works with custom IDs (1001:1001 tested)
- ✅ Directory ownership updated correctly after ID changes
- ✅ Application runs as non-root user after entrypoint setup
- ✅ All functionality preserved (394 tests passing)

**Key Benefits:**
1. **NAS Compatibility**: Works with Synology, QNAP, UnRAID custom user IDs
2. **Flexible Permissions**: Match host system ownership for media access
3. **Security**: Runs as non-root user after initialization
4. **Zero Config**: Defaults to 1000:1000 for standard Docker setups
5. **Dynamic**: No image rebuild required to change user IDs

**Next Session TODO:**
- [ ] Test Docker deployment on actual NAS system
- [ ] Verify symlink creation works with custom PUID/PGID
- [ ] Validate Jellyfin library access through symlinks
- [ ] Update main README.md with Docker deployment instructions
- [ ] Consider adding UMASK environment variable support
- [ ] User-based cleanup with watch tracking
- [ ] Mobile responsiveness improvements

**Key Lessons:**
1. **User management order**: Must delete user before deleting group (Alpine Linux)
2. **Error suppression**: Use `2>/dev/null || true` for idempotent operations
3. **Root requirement**: Container must start as root to modify user/group IDs
4. **su-exec vs gosu**: Alpine uses su-exec (lighter weight, same functionality)
5. **Ownership timing**: Fix directory ownership AFTER user/group recreation
6. **Docker permissions**: setgroups errors in unprivileged mode are expected/harmless
7. **Entrypoint design**: Entrypoint wraps application, CMD provides default args

---

## Previous Session: Nov 4, 2025 (Session 28)

### Symlink Library Manager Implementation - COMPLETED ✅

**Work Completed:**
- ✅ Implemented complete SymlinkLibraryManager service (384 lines)
- ✅ Created Jellyfin Virtual Folder API methods (GET, CREATE, DELETE)
- ✅ Integrated symlink library sync into FullSync workflow
- ✅ Updated configuration structures and validation
- ✅ Replaced Collections config with SymlinkLibrary in example config
- ✅ Deleted old collection files (jellyfin_collections.go + test file)
- ✅ Added comprehensive unit tests (13 test cases, 661 lines)
- ✅ Fixed bugs discovered during testing (JellyfinID validation, source file checks)
- ✅ All 394 tests passing (381 existing + 13 new)
- ✅ Binary builds successfully (14MB)

**Decision Reversal:**
- Session 27: Decided to keep Collections for v1.0 (defer symlinks to v2.0)
- Session 28: **Implemented symlink libraries immediately** (better UX wins)
- Rationale: Better sidebar visibility outweighs setup complexity

**Implementation Details:**

**New SymlinkLibraryManager Service:**
- `SyncLibraries()` - Main orchestration (called after retention rules in FullSync)
- `filterScheduledMedia()` - Separate movies/TV with future deletion dates
- `syncLibrary()` - Sync individual library (create folder, symlinks, cleanup)
- `ensureVirtualFolder()` - Manage Jellyfin libraries via Virtual Folder API
- `ensureDirectory()` - Create symlink base directories
- `createSymlinks()` - Create filesystem symlinks to actual media files
- `cleanupSymlinks()` - Remove stale symlinks for unscheduled items
- `generateSymlinkName()` - Safe filename generation from metadata
- Full dry-run support throughout all operations

**Jellyfin Client Enhancements:**
- `GetVirtualFolders()` - List existing libraries
- `CreateVirtualFolder(name, collectionType, paths, dryRun)` - Create library
- `DeleteVirtualFolder(name, dryRun)` - Remove library
- All methods respect dry-run mode

**Configuration Changes:**
- NEW: `SymlinkLibraryConfig` struct (enabled, base_path, movies, tv_shows)
- Deleted: `JellyfinCollectionsConfig` (replaced entirely)
- Validation: Check base_path writability, library names, collection types
- Example config: Added Docker volume mapping examples and path translation docs

**Sync Integration:**
- Added `symlinkLibraryManager` field to SyncEngine
- Initialize when `SymlinkLibrary.Enabled` is true
- Call `SyncLibraries()` in FullSync after `applyRetentionRules()`
- Thread-safe: Copy media library with RLock before async operations

**Files Modified & Committed:**
- `internal/services/symlink_library.go` - NEW (384 lines)
- `internal/clients/jellyfin.go` - Virtual Folder API methods (+208 lines)
- `internal/clients/types.go` - VirtualFolder structs (+32 lines)
- `internal/config/types.go` - SymlinkLibraryConfig (+27 lines)
- `internal/config/validation.go` - Symlink validation (+34 lines)
- `internal/services/sync.go` - Integration (+42 lines)
- `internal/api/handlers/config.go` - Config handler updates (+38 lines)
- `config/config.yaml.example` - Symlink config docs (+28 lines)
- DELETED: `internal/services/jellyfin_collections.go` (-194 lines)
- DELETED: `internal/services/jellyfin_collections_test.go` (-531 lines)

**Commits:**
1. `492cd6b` - feat: replace Collections with Symlink Library Manager for better visibility
2. `da211f5` - test: add comprehensive unit tests for SymlinkLibraryManager

**Current State:**
- Running: No (implementation complete, manual testing pending)
- Tests passing: 394/394 ✅ (381 existing + 13 new)
- Build: ✅ Successful (oxicleanarr-symlink binary 14MB)
- Known issues: None
- Net change: +313 lines (1,243 added, 930 deleted)

**Advantages Over Collections:**
1. **Better Visibility** - Libraries appear in Jellyfin sidebar (not buried in Collections)
2. **User-Friendly** - More intuitive "Leaving Soon" section
3. **Proven Approach** - Janitorr uses this successfully
4. **Native Feel** - Works like regular Jellyfin libraries
5. **Cleaner Separation** - Movies and TV shows in separate libraries

**Requirements for Deployment:**
- Docker volume mapping for symlink directories required
- Both OxiCleanarr and Jellyfin must see same media paths
- Symlink base directory must be writable by OxiCleanarr container
- See `config/config.yaml.example` for Docker Compose setup

**Unit Tests Added (13 test cases):**
- `TestNewSymlinkLibraryManager` - Constructor validation
- `TestFilterScheduledMedia` - 6 subtests for media filtering (types, exclusions, dates, IDs)
- `TestGenerateSymlinkName` - 2 subtests for safe filename generation
- `TestCreateSymlinks` - 3 subtests for symlink creation (success, dry-run, missing files)
- `TestCleanupSymlinks` - 2 subtests for stale symlink removal
- `TestEnsureVirtualFolder` - 4 subtests for Jellyfin library management
- `TestSyncLibraries_Integration` - End-to-end integration test

**Bug Fixes Discovered via Testing:**
- Fixed missing `JellyfinID` validation in `filterScheduledMedia()` (line 112-114)
- Fixed source file existence check in `createSymlinks()` (line 272-277)
- Fixed symlink tracking to only include successfully created symlinks (line 322)

**Next Session TODO:**
- [ ] Manual testing with real Jellyfin instance (requires Docker setup)
- [ ] Verify symlink creation works correctly with real files
- [ ] Test Virtual Folder creation/deletion/updates with Jellyfin API
- [ ] Validate path translation in Docker environment
- [ ] Test edge cases: permission issues, concurrent syncs
- [ ] Update README/documentation with setup instructions
- [ ] Consider adding error recovery and retry logic if needed

**Key Lessons:**
1. **Bold decisions**: Sometimes the "defer to v2.0" choice should be reconsidered
2. **UX over complexity**: Better user experience justifies implementation complexity
3. **Code replacement**: Replacing 725 lines of old code with 1,045 lines of new code (384 implementation + 661 tests)
4. **API design**: Virtual Folder API is simpler than Collection item management
5. **Thread safety**: Always copy shared state before passing to async operations
6. **Interface design**: Local interfaces for dependency injection avoid global client changes
7. **Test-driven bug finding**: Unit tests discovered 3 bugs before manual testing
8. **Validation importance**: Always check JellyfinID existence and source file availability

---

## Previous Session: Nov 4, 2025 (Session 27)

### Jellyfin Virtual Folder (Library) API Research - COMPLETED ✅

**Work Completed:**
- ✅ Researched Jellyfin Virtual Folder API from official source code
- ✅ Documented all 6 API endpoints for library management
- ✅ Analyzed Janitorr's symlink library implementation approach
- ✅ Compared Collections (current) vs Virtual Folders (alternative)
- ✅ Evaluated implementation complexity and user setup requirements
- ✅ Created comprehensive research document (SESSION_27_JELLYFIN_LIBRARY_API.md)
- ✅ Made architectural decision: Keep collections for v1.0

**Research Findings:**

**Virtual Folder API Endpoints Discovered:**
1. `GET /Library/VirtualFolders` - List all libraries
2. `POST /Library/VirtualFolders` - Create library (with collectionType: movies/tvshows/music/etc.)
3. `DELETE /Library/VirtualFolders` - Delete library
4. `POST /Library/VirtualFolders/Name` - Rename library
5. `POST /Library/VirtualFolders/Paths` - Add path to library
6. `DELETE /Library/VirtualFolders/Paths` - Remove path from library

**Key Insights:**
- Virtual Folders (Libraries) appear in sidebar (better visibility than Collections)
- Janitorr uses symlinks to create "Leaving Soon" as a library (not collection)
- Requires filesystem access and complex Docker volume mapping
- Path translation needed between OxiCleanarr/Jellyfin/Radarr/Sonarr containers
- Collections API already working perfectly (Sessions 7, 20)

**Decision Made:**
- **v1.0**: Keep current Collections approach (production-ready, simple setup)
- **v2.0**: Consider symlink libraries as optional enhancement
- **Rationale**: Avoid filesystem complexity, easier user setup, proven stable

**Files Created:**
- `SESSION_27_JELLYFIN_LIBRARY_API.md` - Complete research documentation with implementation plan

**Current State:**
- Running: Yes (backend + frontend)
- Tests passing: 381/381 ✅
- Known issues: None
- Collections feature: ✅ Working and stable
- Symlink libraries: 🔬 Researched, deferred to v2.0

**Next Session TODO:**
- [ ] Move to next priority feature: user-based cleanup or mobile responsiveness
- [ ] Consider user feedback on Collections visibility before v2.0 planning
- [ ] Statistics/charts for disk space trends
- [ ] Comprehensive error handling

**Key Lessons:**
1. **Research before coding**: Investigated API fully before implementation decision
2. **Simplicity wins**: Current Collections approach is simpler and production-ready
3. **Defer complexity**: Symlink libraries add filesystem/path mapping complexity
4. **User setup burden**: Virtual folders require complex Docker volume configuration
5. **Feature completeness**: Collections provide 100% functionality with better maintainability

---

## Previous Session: Nov 4, 2025 (Session 20)

### Jellyfin Collections Dry-Run Bug Fix - COMPLETED ✅

**Work Completed:**
- ✅ Fixed Jellyfin collections to respect config hot-reload for dry_run setting
- ✅ Removed dryRun field from JellyfinCollectionManager struct
- ✅ Implemented dynamic config reading at runtime with nil-safety (defaults to dry_run=true)
- ✅ Improved test safety by adding SetTestConfig() for in-memory test configs
- ✅ Eliminated live credential loading in tests (was using config.test.yaml)
- ✅ All 381 tests passing (13 collection tests + 368 others)
- ✅ Live tested collections creation with dry_run: false
- ✅ Collections created successfully: 11 movies + 6 TV shows

**Problem Fixed:**
- Collection manager stored dry_run as a field at construction time
- Config hot-reload updated in-memory config but collections kept old dry_run value
- Collections always operated in the mode set at startup, ignoring config changes
- User reported collections not being created despite dry_run: false in config

**Solution Implemented:**
1. **Dynamic Config Reading**:
   - Removed `dryRun bool` field from `JellyfinCollectionManager`
   - Added runtime config reads: `cfg := config.Get(); dryRun := cfg.App.DryRun`
   - Safety default: `dryRun = true` when config is nil (prevents accidental operations)
   - Applied to both `SyncCollections()` and `syncCollection()` methods

2. **Test Safety Improvements**:
   - Added `SetTestConfig(cfg *Config)` function to config package
   - Created `setupTestConfig(t)` helper for in-memory test configs
   - Tests no longer load config.test.yaml (no live credentials)
   - 6 tests updated to use safe in-memory config

**Files Modified & Committed:**
- `internal/config/config.go` (+6 lines) - Added SetTestConfig() for test injection
- `internal/services/jellyfin_collections.go` (~30 lines) - Dynamic config reading with nil safety
- `internal/services/sync.go` (-1 line) - Removed dryRun parameter from constructor
- `internal/services/jellyfin_collections_test.go` (+50 lines) - Safe test config setup

**Commits:**
1. `cfb4a68` - fix: make Jellyfin collections respect config hot-reload for dry_run setting

**Current State:**
- Running: Yes (backend PID: 597610)
- Tests passing: 381/381 ✅
- Known issues: 1 movie (Red Dawn) missing Jellyfin ID (skipped from collections)
- Collections verified: ✅ Created successfully with 11 movies + 6 TV shows
- Config hot-reload: ✅ Working correctly for collections

**Testing Results:**
- ✅ Collections deleted when empty (`hide_when_empty: true`)
- ✅ Collections created when items scheduled (retention changed to 10d)
- ✅ Logs show `dry_run: false` correctly applied
- ✅ Dynamic config reading works at runtime
- ✅ Safety default (dry_run=true) when config is nil

**Key Lessons:**
1. **State at construction vs runtime**: Values stored at construction time don't respect hot-reload
2. **Dynamic config reading**: Always read from `config.Get()` at runtime for hot-reload support
3. **Nil safety**: Always provide safe defaults when config might be nil
4. **Test isolation**: Never load real config files in tests - use in-memory configs
5. **ReapplyRetentionRules limitation**: Only re-evaluates retention, doesn't sync collections
6. **Full sync needed**: Collection changes require full sync, not just retention rule re-application

**Next Session TODO:**
- [ ] Investigate "Red Dawn" missing Jellyfin ID (separate sync debugging)
- [ ] Consider adding collection sync to ReapplyRetentionRules() for faster UI updates
- [ ] User-based cleanup with watch tracking integration
- [ ] Mobile responsiveness improvements
- [ ] Statistics/charts for disk space trends

---

## Previous Session: Nov 4, 2025 (Session 18)

### Part 1: Scheduled Deletions Data Source Refactoring - COMPLETED ✅

**Work Completed:**
- ✅ Resumed from Session 17 (auto-sync optimization completed)
- ✅ Refactored Scheduled Deletions page to query media API directly instead of job summaries
- ✅ Added config query to dynamically fetch dry-run mode
- ✅ Implemented client-side filtering for overdue items (deletion_date < now)
- ✅ All 109 test functions passing (380 test runs with subtests)

**Problem Identified:**
- Scheduled Deletions page was querying jobs endpoint (`would_delete` from job summaries)
- This created timing issues: empty → stale → correct data flow
- Confusing UX: delays between config changes and UI updates
- Different architecture from Library/Timeline pages (inconsistent)

**Decision Process:**
- Initially attempted to fix by populating `would_delete` in `ReapplyRetentionRules()`
- User correctly identified this was "too complex" and likely browser caching issue
- Chose **Option B**: Change to query media API directly (consistent with other pages)

**Solution Implemented:**
- Changed data source from `jobs` endpoint to `movies`/`shows` endpoints
- Added `config` query to fetch dry-run mode dynamically (replaced hardcoded TODO)
- Filter overdue items client-side: `deletion_date < now && !excluded && deletion_date != zero`
- Map `MediaItem → DeletionCandidate` on the fly with calculated `days_overdue`
- Benefits from Session 17's TanStack Query invalidation for immediate updates

**Why This is Better:**
1. **Consistent architecture** - All pages query media directly (Library, Timeline, Scheduled Deletions)
2. **Immediate updates** - Benefits from auto-invalidation after config/rule changes
3. **Simpler** - No dual data sources or complex job summary logic
4. **More accurate** - Always shows real-time state, not historical snapshots
5. **Eliminates lag** - No 60-second delay between config changes and UI updates

**Files Modified:**
- `web/src/pages/ScheduledDeletionsPage.tsx` (+70 lines, -17 lines) - Query media APIs, filter overdue, fetch dry-run from config

**Testing Results:**
- ✅ All 109 test functions passing (380 test runs with subtests)
- ✅ Frontend builds successfully (hot-reload working)
- ✅ Manual API testing: 254 movies with valid deletion dates
- ✅ Config API returns dry_run mode correctly
- ✅ Media items have proper deletion_date fields with overdue calculations

### Part 2: Sync Scheduler Auto-Start Fix - COMPLETED ✅

**Work Completed:**
- ✅ Fixed sync scheduler not starting automatically on backend startup
- ✅ Added `StartScheduler()` call to main.go when `sync.auto_start: true`
- ✅ All 109 test functions passing

**Problem Identified:**
- `sync.auto_start: true` in config but scheduler never started
- User had to manually trigger syncs via UI
- Scheduler initialization code existed but was never called

**Solution:**
- Added scheduler initialization in main.go after service setup
- Checks `config.Sync.AutoStart` before starting
- Logs "Sync scheduler started" when enabled
- Logs "Sync scheduler disabled" when `auto_start: false`

**Commits:**
2. `5b3cefa` - fix: ensure sync scheduler starts automatically when auto_start is enabled

### Part 3: Retention Rules Investigation - RESOLVED ✅

**Work Completed:**
- ✅ Investigated user report: retention rule changes didn't update Dashboard/Timeline
- ✅ Added debug logging to rules engine and config reload
- ✅ Verified system working correctly end-to-end
- ✅ Identified file watcher limitation with `sed -i` edits
- ✅ All 109 test functions passing

**Problem Reported:**
- User changed retention from `10d` to `0d` via Configuration UI
- Dashboard and Timeline pages still showed old data with `10d` retention
- Expected immediate update (from Session 17's auto-refresh feature)

**Investigation Results:**
- ✅ Config hot-reload works correctly (`config.Get()` returns updated values)
- ✅ Rules engine uses correct retention values from config
- ✅ Auto-sync triggers within 1-2 seconds after config API update (Session 17 feature)
- ✅ TanStack Query invalidation triggers UI refresh (Session 17 feature)
- ✅ Debug logs confirm: `use_global: true`, retention values match config file

**Root Cause Identified:**
- **SYSTEM IS WORKING AS DESIGNED** ✅
- Issue likely: Browser cache or user checked UI before auto-sync completed (~1-2s delay)
- File watcher limitation discovered: `sed -i` doesn't trigger fsnotify (creates new file)
- **Workaround**: Use config API endpoint for updates (works perfectly)

**Files Modified:**
- `internal/services/rules.go` (+9 lines) - Debug logging at rules evaluation
- `internal/config/config.go` (+4 lines) - Enhanced config reload logging

**Commits:**
3. `2c3a67e` - debug: add retention policy logging for troubleshooting

**Testing Evidence:**
- Manual test: `0d` retention → 0 scheduled deletions (correct)
- Manual test: `10d` retention → 359 scheduled deletions (correct)
- Manual test: `5d` retention → Rules engine evaluates with `5d` values (correct)
- Leaving-soon API: 18 items with "10d" in deletion reasons (correct)
- Auto-sync triggered within 1 second after config API updates (correct)

### Part 4: Frontend Cache Issue Resolution - COMPLETED ✅

**Work Completed:**
- ✅ Investigated user report: Frontend showing 359 items despite 0d retention
- ✅ Identified dual-cause issue: Frontend cache + backend stale in-memory data
- ✅ Fixed TanStack Query configuration (refetchOnWindowFocus)
- ✅ Restarted backend with fresh sync after retention changes
- ✅ All 381 test runs passing

**Problem Identified:**
- User reported frontend showing **359 scheduled deletions** despite config set to `0d` retention
- Backend API correctly returned **1-2 items** (tag rule exceptions only)
- Root causes:
  1. **Frontend**: `refetchOnWindowFocus: false` prevented automatic cache refresh
  2. **Backend**: Old process had in-memory data from previous higher retention

**Investigation Process:**
- Verified backend APIs working correctly (leaving-soon: 1 item, config: 0d retention)
- Identified TanStack Query cache showing stale data
- Discovered backend process started before retention changes (still had old in-memory cache)
- Config hot-reload updates config but doesn't re-evaluate existing media

**Solution Implemented:**
1. **Frontend Fix** (`web/src/App.tsx`):
   - Changed `refetchOnWindowFocus: false` → `true`
   - Added `staleTime: 30000` (30 seconds)
   - Enables automatic refetch when switching browser tabs
   - Improves cross-tab synchronization

2. **Backend Fix**:
   - Stopped old backend process (PID 473594)
   - Rebuilt binary: `go build -o oxicleanarr-test`
   - Started fresh backend (PID 491951) with clean sync
   - Full sync completed: 0 scheduled deletions (correct!)

**Files Modified:**
- `web/src/App.tsx` (+2 lines, -1 line) - QueryClient refetch configuration

**Commits:**
4. `25b7711` - fix: enable refetchOnWindowFocus for cross-tab query updates

**Testing Results:**
- ✅ All 381 test runs passing (109 test functions)
- ✅ Frontend builds successfully (442.24 kB, gzipped: 131.24 kB)
- ✅ Backend API: Leaving-soon returns 1 item (tag rule exception)
- ✅ Backend API: Movies endpoint shows 1 scheduled deletion
- ✅ Config API: Returns 0d retention correctly
- ✅ Full sync: 255 movies, 123 TV shows, 0 standard deletions

**Current State:**
- Running: Yes (backend PID 491951 + frontend dev server)
- Tests passing: 109/109 functions ✅ (381 test runs with subtests)
- Known issues: None

### Part 5: Dashboard "Leaving Soon" Navigation Fix - COMPLETED ✅

**Work Completed:**
- ✅ Fixed Dashboard "Leaving Soon" section "View All" button navigation
- ✅ Changed navigation from `/scheduled-deletions` to `/timeline` (correct page)
- ✅ Changed button condition from `scheduledDeletionsCount` to `leavingSoon.total`
- ✅ All tests still passing (381 test runs)

**Problem Identified:**
- Dashboard "Leaving Soon" section shows future deletions (8 items)
- "View All" button incorrectly navigated to `/scheduled-deletions` (overdue items)
- Button condition used wrong count: `scheduledDeletionsCount` (368) instead of `leavingSoon.total` (8)
- Semantic mismatch: leaving-soon section should link to Timeline page

**Root Cause:**
- Code location: `web/src/pages/DashboardPage.tsx` lines 216-223
- Button used `scheduledDeletionsCount > 10` check (wrong metric)
- Button navigated to wrong page for the data being displayed
- Dashboard has two distinct sections:
  - **Scheduled Deletions Card**: Overdue items → `/scheduled-deletions` ✅
  - **Leaving Soon Section**: Future items → `/timeline` ✅ (NOW FIXED)

**Solution Implemented:**
```typescript
// OLD: Wrong navigation and count
{scheduledDeletionsCount > 10 && (
  <Button onClick={() => navigate('/scheduled-deletions')}>
    View All {scheduledDeletionsCount} Items
  </Button>
)}

// NEW: Correct navigation to Timeline
{leavingSoon.total > 10 && (
  <Button onClick={() => navigate('/timeline')}>
    View All {leavingSoon.total} Items
  </Button>
)}
```

**Files Modified:**
- `web/src/pages/DashboardPage.tsx` (3 lines changed) - Fixed button navigation and count

**Commits:**
5. `a2d378c` - fix: correct Dashboard 'Leaving Soon' button to navigate to Timeline page

**Testing Results:**
- ✅ All tests passing (cached)
- ✅ Frontend hot-reloaded successfully
- ✅ Button now shows correct count (8 items)
- ✅ Button navigates to correct page (Timeline)
- ✅ Button condition uses correct metric (leavingSoon.total)

**Page Navigation Map (Corrected):**
| Dashboard Section | Data Type | Count | Navigates To |
|-------------------|-----------|-------|--------------|
| Scheduled Deletions Card | Overdue (`deletion_date < now`) | 368 | `/scheduled-deletions` ✅ |
| Leaving Soon Section | Future (`deletion_date > now`) | 8 | `/timeline` ✅ (FIXED) |

**Current State:**
- Running: Yes (backend PID 528481 + frontend dev server)
- Tests passing: 109/109 functions ✅ (381 test runs with subtests)
- Known issues: None
- Session 18: COMPLETE ✅ (Parts 1-5)

**Key Lessons:**
1. **In-memory state matters**: Config hot-reload updates config but doesn't re-evaluate existing data
2. **Process restart needed**: When testing retention changes, restart backend for clean slate
3. **TanStack Query defaults**: `refetchOnWindowFocus: false` is too aggressive for multi-tab apps
4. **Cross-component invalidation**: Query invalidation only works for active queries or with refetchOnWindowFocus
5. **Debug logging value**: Helps verify system behavior during troubleshooting
6. **File watcher caveat**: `sed -i` creates new files, breaking fsnotify watch
7. **API-first approach**: Config API endpoint more reliable than direct file edits
8. **System integrity**: Sessions 13 + 17 features working correctly together
9. **Navigation consistency**: Each dashboard section should link to the page showing the same data type
10. **Button conditions**: UI controls should use the metric from the section they're in, not unrelated counts

**Next Session TODO:**
- [ ] User-based cleanup with watch tracking integration
- [ ] Mobile responsiveness improvements
- [ ] Statistics/charts for disk space trends
- [ ] Consider file watcher improvements (detect file replacement vs modification)
- [ ] Consider adding "Refresh Data" button in UI for manual cache clearing

---

## Previous Session: Nov 4, 2025 (Session 17)

### Part 1: Config Auto-Sync Performance Optimization - COMPLETED ✅

**Work Completed:**
- ✅ Resumed from Session 16 (tag-based rules UI completed)
- ✅ User identified inefficiency in auto-sync behavior
- ✅ Optimized config updates to use `ReapplyRetentionRules()` instead of `FullSync()`
- ✅ Removed unused `context` import from config handler
- ✅ All 109 test functions passing (380 test runs with subtests)

**Problem Identified:**
- Session 13 implemented auto-sync on config changes using `FullSync()`
- `FullSync()` re-fetches ALL data from Radarr/Sonarr/Jellyfin (~12 seconds)
- When only retention rules change, no external data fetch needed
- Inefficient: causing unnecessary API calls and slower updates

**Solution Implemented:**
- Changed config handler to call `ReapplyRetentionRules()` instead of `FullSync()`
- `ReapplyRetentionRules()` only re-evaluates existing in-memory media (instant)
- No external API calls needed for rule-only changes
- Deletion dates update within 1-2 seconds after config save
- Rules engine already uses global config (hot-reload support from Session 13)

**Files Modified:**
- `internal/api/handlers/config.go` (-5 lines net) - Use `ReapplyRetentionRules()`, remove unused import

**Commits:**
1. `f845a21` - perf: optimize config updates to use ReapplyRetentionRules instead of FullSync

**Testing Results:**
- ✅ All 109 test functions passing
- ✅ Manual testing: Changed tag rule retention 90d → 1d → 365d via API
- ✅ Deletion dates updated correctly: May 31, 2025 → May 31, 2025 (overdue) → May 30, 2026
- ✅ No external API calls observed in logs (verified with Radarr/Sonarr/Jellyfin grep)
- ✅ Update time: ~1-2 seconds (was ~12 seconds with FullSync)
- ✅ Log message confirms: "Re-applying retention rules to existing media (no external API calls needed)"

**Performance Impact:**
- **Before**: Config update → FullSync → Re-fetch all data (~12s)
- **After**: Config update → ReapplyRetentionRules → Re-evaluate in-memory (~instant)
- **Improvement**: ~12x faster for rule-only changes

### Part 2: Automatic UI Refresh After Config Changes - COMPLETED ✅

**Work Completed:**
- ✅ Added TanStack Query invalidation for media queries after config/rule updates
- ✅ ConfigurationPage now auto-refreshes UI after saving changes
- ✅ RulesPage now auto-refreshes UI after create/update/delete/toggle operations
- ✅ All tests passing (109 test functions)

**Problem Identified:**
- Backend re-applied retention rules instantly via `ReapplyRetentionRules()`
- Config/rules saved successfully with toast notification
- **Gap**: Frontend didn't know media data changed
- UI showed stale deletion dates until manual page refresh
- Poor UX: changes appeared to work but UI didn't reflect them

**Solution Implemented:**
- Added `queryClient.invalidateQueries()` calls for media-related queries
- Invalidated query keys: `movies`, `shows`, `leaving-soon`, `leaving-soon-all`, `jobs`
- ConfigurationPage: Invalidates on successful config update
- RulesPage: Invalidates on create/update/delete/toggle success
- TanStack Query automatically refetches invalidated queries that are currently in use
- Open pages (Library, Timeline, Scheduled Deletions) auto-update within 1-2 seconds

**Files Modified:**
- `web/src/pages/ConfigurationPage.tsx` (+4 lines) - Added media query invalidations
- `web/src/pages/RulesPage.tsx` (+20 lines) - Added media query invalidations to all 4 mutations

**Commits:**
2. `dfd87b4` - feat: add automatic UI refresh after config and rule changes

**Testing Results:**
- ✅ All 109 test functions passing
- ✅ Frontend built successfully (441.28 kB, gzipped: 130.98 kB)
- ✅ Manual API testing: Config update 0d → 365d → 1d
- ✅ Leaving-soon count updated correctly: 0 → 232 → 1 items
- ✅ Advanced rule create/delete tested successfully
- ✅ No external API calls during re-apply (verified in logs)

**How It Works:**
1. User saves config/rule via UI
2. Backend receives request → Updates config → Calls `ReapplyRetentionRules()` (~1-2s)
3. Frontend mutation succeeds → Invalidates media queries
4. TanStack Query marks queries as stale
5. All active pages (Library/Timeline/Scheduled Deletions) automatically refetch
6. UI updates with new deletion dates within 1-2 seconds
7. User sees changes immediately without manual refresh

**Current State:**
- Running: Yes (PID: 388993)
- Tests passing: 109/109 functions ✅ (380 test runs with subtests)
- Known issues: None
- Config auto-sync: Optimized ✅
- UI auto-refresh: Implemented ✅
- Session 17: COMPLETE ✅

**Next Session TODO:**
- [ ] Consider reducing duplicate triggers (API handler + file watcher both call ReapplyRetentionRules)
- [ ] User-based cleanup with watch tracking integration
- [ ] Mobile responsiveness improvements
- [ ] Statistics/charts for disk space trends
- [ ] Comprehensive error handling

---

## Previous Session: Nov 4, 2025 (Session 16)

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
│   └── config.yaml          # Main configuration (hot-reload enabled)
├── data/
│   ├── exclusions.json       # User "Keep" exclusions
│   └── jobs.json             # Job history (circular buffer)
└── logs/
    └── oxicleanarr.log           # Structured JSON logs
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
- **Example**: `config/config.yaml.example` - Template with defaults
- **Test Config**: `config/config.test.yaml` - Testing configuration
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
./oxicleanarr-test --config config/config.test.yaml

# Check logs
tail -f /tmp/oxicleanarr-debug.log

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
./oxicleanarr --config config/config.test.yaml

# Check logs
tail -f /tmp/oxicleanarr-debug.log | jq
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
2. **Check OXICLEANARR_SPEC.md** for detailed architecture
3. **Review recent commits** (`git log --oneline -10`)
4. **Check test coverage** (`make test`)

### When Making Changes
1. **Always run tests** after backend changes
2. **Preserve exclusions** - Never break `applyExclusions()` logic
3. **Maintain data source hierarchy** - Radarr/Sonarr = truth, Jellyfin = watch data only
4. **Update this file** when completing major features
5. **Document in OXICLEANARR_SPEC.md** when fixing bugs or adding features

### ⚠️ CRITICAL: Git Commit Policy (DETAILED)
**ABSOLUTE RULE: DO NOT create git commits unless explicitly told to do so by the user.**

**This means:**
- ❌ NEVER commit just because files are staged
- ❌ NEVER commit just because tests are passing
- ❌ NEVER commit just because a previous summary said "ready to commit"
- ❌ NEVER commit just because a previous summary said "waiting for approval"
- ❌ NEVER commit just because work seems complete
- ❌ NEVER assume the user wants you to commit
- ✅ ALWAYS ask: "Should I commit these changes now?" before ANY git commit command

**When to commit**: ONLY when the user explicitly uses words like:
- "commit this"
- "create a commit"
- "commit these changes"
- "please commit"
- "go ahead and commit"

**When NOT to commit**: ALL OTHER TIMES, including:
- "this looks good" (does NOT mean commit)
- "that's perfect" (does NOT mean commit)
- "ready to commit" in a summary (does NOT mean commit)
- Completing work (does NOT mean commit)

**STOP AND ASK CHECKLIST** - Before ANY `git commit` command:
1. ⚠️ Did the user EXPLICITLY say to commit in THIS session?
2. ⚠️ Did they use the word "commit" directly?
3. ⚠️ If NO to either: STOP and ASK first

**Example workflow:**
```bash
# ✅ CORRECT: Make changes and verify
make test
go build -o oxicleanarr

# ✅ CORRECT: Report completion and ASK
# You: "Changes complete. Tests passing. Should I commit these changes?"
# User: "yes commit it"
# Then: git add . && git commit -m "..."

# ❌ WRONG: NEVER do this
# git add .
# git commit -m "feat: add new feature"  # NO! User didn't say to commit!

# ❌ WRONG: NEVER assume from summary
# Previous summary: "waiting for approval to commit"
# git commit ...  # NO! Must ask in THIS session!
```

**Rationale**: The user may want to:
- Review changes first
- Test manually
- Group multiple changes into one commit
- Amend an existing commit
- Use a different commit message
- Not commit at all

**Violation of this policy is a critical failure.** User MUST have final control over ALL commits.

### When Debugging
1. **Enable debug logging** (`LOG_LEVEL=debug`)
2. **Use test config** with real services
3. **Check job history** to see sync results
4. **Verify API responses** with curl or test-api.sh

---

## Quick Reference

### File Locations
- Config: `config/config.yaml`
- Data: `data/exclusions.json`, `data/jobs.json`
- Logs: `/tmp/oxicleanarr-debug.log` (test mode)
- Binary: `./oxicleanarr` or `./oxicleanarr-test`

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
- `config/config.yaml.example` - Documented new config with clear comments
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
- [ ] Configuration UI page (edit config.yaml via web)
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
- [ ] Configuration UI page (edit config.yaml via web)
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
- ✅ Successfully created "OxiCleanarr - Movies Leaving Soon" collection with 8 movies

**Files Modified & Committed:**
- `internal/services/jellyfin_collections.go` - NEW: 174 lines
- `internal/clients/jellyfin.go` - +231 lines (URL encoding, collection methods)
- `internal/clients/types.go` - +13 lines (collection types)
- `internal/config/types.go` - +20 lines (collection config)
- `internal/config/validation.go` - +16 lines (collection validation)
- `internal/services/sync.go` - +21 lines (integrate collection manager)
- `config/config.yaml.example` - +10 lines (collection config docs)

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
- Running: Yes (OxiCleanarr + Frontend dev server)
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
- [ ] Configuration UI page (allow editing config.yaml via web)
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
- ✅ Removed Plex-related fields (OxiCleanarr is Jellyfin-only)
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
- Integration tests require `OXICLEANARR_INTEGRATION_TEST=1` environment variable

**Next Session TODO:**
- [ ] Configuration UI page (allow editing config.yaml via web)
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
- ✅ Updated OXICLEANARR_SPEC.md with clarified matching strategy
- ✅ Updated config/config.yaml.example with clear examples

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
