# OxiCleanarr Development Sessions

## Session 1: Deletion Lifecycle Test - Fixed Critical Bug & Improved Reliability

### Issues Fixed

#### 1. Critical Bug in DeleteMedia Function (internal/services/sync.go)
**Problem:** OxiCleanarr was incorrectly deleting items from Jellyfin directly using `jellyfinClient.DeleteItem()`. This doesn't delete actual files since Jellyfin is just a media indexer, not a file manager.

**Correct Flow:**
1. Delete from Radarr/Sonarr (with `deleteFiles=true`) - removes files from disk
2. Trigger Jellyfin library scan - Jellyfin detects missing files and updates its database
3. Remove from internal media library

**Fix Applied:**
- Removed direct Jellyfin deletion via `DeleteItem()`
- Added `RefreshLibrary()` call after Radarr/Sonarr deletion
- Made Jellyfin refresh non-fatal (logs warning if it fails, but deletion still succeeds)
- Added detailed logging for each step

#### 2. Test Reliability - Fixed Race Conditions
**Problem:** Tests used fixed sleep timers which caused intermittent failures.

**Fix Applied:**
- Added `WaitForJobCompletion()` helper in `test/integration/helpers.go`
- Polls job status every 500ms until completed (configurable timeout)
- Tracks job ID changes and status transitions
- Updated `test/integration/deletion_lifecycle_test.go` Phase 2 to use new wait helper

#### 3. YAML Configuration Management - ✅ COMPLETED

**Problem:** Helper functions (`UpdateRetentionPolicy`, `UpdateDryRun`, `UpdateEnableDeletion`) were using string manipulation to update config values, causing indentation issues that broke YAML parsing.

**Root Causes:**
- `test/assets/config/config.yaml` had inconsistent indentation (2 spaces vs 4 spaces)
- String-based replacement didn't preserve original indentation
- OxiCleanarr failed to restart after config changes (30s timeout)

**Fixes Applied:** ✅
1. Fixed source config file (`test/assets/config/config.yaml`):
   - Changed `movie_retention` from 2-space to 4-space indentation to match other fields
   - Changed default from `7d` to `20d`
2. **Completely refactored all config helpers to use `gopkg.in/yaml.v3`:**
   - `UpdateRetentionPolicy` - Now uses proper YAML parsing
   - `UpdateDryRun` - Now uses proper YAML parsing
   - `UpdateEnableDeletion` - Now uses proper YAML parsing
   - Added import: `gopkg.in/yaml.v3`
3. **Result:** OxiCleanarr now restarts successfully every time, no more 30s timeout failures!

**Test Results:**
- ✅ Config updates work perfectly: "Updated movie_retention from 7d to 999d"
- ✅ Restarts succeed consistently
- ✅ No more YAML indentation corruption

### Test Results After Fixes

#### Phase 2 Deletion - ✅ WORKING
- `deleted_count: 1` appears in job summary
- `deleted_items` contains Pulp Fiction with correct details
- Movie count reduced from 7 to 6
- Total media count shows 6 (confirms internal library updated)

#### Current Issues (Business Logic)

1. **Test Logic Issue - Retention vs Advanced Rules** (NEEDS ANALYSIS)
   - **Phase 1 Failure:** Expected `scheduled_deletions: 1`, got `0`
   - **Phase 1 Failure:** Expected 0 items in leaving-soon, got 6
   - **Phase 2 Failure:** Expected `scheduled_deletions: 1`, got `0`
   - **Root Cause:** Test sets retention to `999d` (keep forever) to prevent untagged movies from being scheduled, BUT Pulp Fiction has an advanced tag rule (`test-deletion` tag)
   - **Confusion:** Does the advanced rule work independently of retention? Or does retention override advanced rules?
   - **Test Expectation:** Pulp Fiction should be scheduled for deletion because it has the `test-deletion` tag, regardless of the `999d` retention setting
   - **Actual Behavior:** Pulp Fiction is NOT scheduled (scheduled_deletions: 0)
   - **Leaving-soon mystery:** 6 other movies are in leaving-soon (from previous test runs?)
   
2. **Next Steps:**
   - Investigate how advanced rules interact with retention settings in sync.go
   - Determine if this is a bug in the business logic or in the test expectations
   - Check if the "test-deletion" tag rule is being applied correctly
   - Understand why 6 movies are in leaving-soon when retention is 999d

### Files Modified (Not Committed)
- ✏️ `internal/services/sync.go` - Fixed DeleteMedia to use Jellyfin refresh instead of direct deletion
- ✏️ `test/integration/helpers.go` - Added WaitForJobCompletion, refactored UpdateRetentionPolicy/UpdateDryRun/UpdateEnableDeletion to use YAML parsing
- ✏️ `test/integration/deletion_lifecycle_test.go` - Uses new wait helper, added debug logging
- ✏️ `test/assets/config/config.yaml` - Fixed indentation from 2-space to 4-space, changed default retention from 7d to 20d

### Architecture Insight
The deletion flow is now architecturally correct:
- Radarr/Sonarr delete files → Jellyfin scans library → Jellyfin auto-detects missing files
- Jellyfin is a media indexer, not a file manager - it should never directly delete files
- This is the proper separation of concerns

### TODO
- [x] Complete YAML refactor for all config update helpers ✅
- [ ] **CRITICAL:** Investigate advanced rule logic - why isn't Pulp Fiction scheduled when tagged with "test-deletion"?
- [ ] Understand retention vs advanced rules interaction in sync.go
- [ ] Fix test logic or business logic based on investigation findings
- [ ] Investigate why 6 movies are in leaving-soon when retention is 999d
- [ ] Apply WaitForJobCompletion to other tests (SymlinkLifecycle, ExclusionLifecycle)
- [ ] Run full test suite to confirm all tests pass

### Key Learnings
1. **Always use proper parsing libraries** (YAML, JSON) instead of string manipulation for structured data
2. **Indentation bugs are silent killers** - they cause app crashes on restart without clear error messages
3. **Test reliability matters** - Fixed sleep timers should be replaced with polling mechanisms
4. **Architecture matters** - Jellyfin is a media indexer, not a file manager. Deletion must happen at the *arr level.

---

## Session 2: Advanced Rules - Watched-Based Rules Implementation & Mock Services

### Overview
Implemented `type: watched` advanced rules feature to enable automatic cleanup of watched media based on configurable retention periods. Added mock HTTP servers to replace Docker services for Jellyseerr and Jellystat, significantly improving test speed and reliability.

### Features Implemented

#### 1. Watched-Based Advanced Rules ✅
**New Feature:** Added `type: watched` to advanced rules system

**Core Implementation (`internal/services/rules.go`):**
- Added `evaluateWatchedRules()` function (lines 333-411)
- Supports `require_watched` flag at rule level to protect unwatched media
- Uses `LastWatched` timestamp for watched media, falls back to `AddedAt` for unwatched
- Integrated into rule evaluation chain with priority: Tag > User > Watched > Standard
- Added watched rule parsing in `GenerateDeletionReason()` (lines 621-669)

**Configuration Format:**
```yaml
advanced_rules:
  - name: "Auto Clean Watched Content"
    type: watched
    enabled: true
    retention: 30d              # Delete 30 days after last watch
    require_watched: true       # Only delete watched media (protects unwatched)
```

**Documentation:**
- Added "Example 2: Watched-Based Cleanup" section to `config/config.yaml.example`
- Documents `type: watched` configuration with `require_watched` flag

#### 2. Mock HTTP Servers for Integration Tests ✅
**Replaced Docker containers with lightweight mock servers**

**Files Created:**
- `test/integration/mock_jellyseerr.go` - Mock Jellyseerr API
- `test/integration/mock_jellystat.go` - Mock Jellystat API

**Mock Jellyseerr Features:**
- 3 test users: Trial (ID:100), Premium (ID:200), VIP (ID:300)
- 6 movie requests with varying dates
- Implements `/api/v1/status` and `/api/v1/request` endpoints
- Uses `httptest.Server` for dynamic port allocation

**Mock Jellystat Features:**
- Returns watch history for 6/7 movies (Schindler's List unwatched)
- Implements `/api/getWatchHistory` endpoint
- Configurable movie ID mapping for test flexibility

**Benefits:**
- ⚡ Faster test execution (no Docker container startup)
- 🎯 Deterministic test data
- 🧹 Simpler test infrastructure
- 💾 Reduced resource usage

#### 3. Jellystat Watch Count Fix ✅
**Made Jellystat authoritative for watch history**

**Changes (`internal/services/sync.go`):**
- Track watch count per media item from Jellystat history
- Override Jellyfin's `WatchCount` with Jellystat data during sync
- Jellystat is now the single source of truth for watch history
- Improved comments clarifying authoritative data flow

#### 4. Test Infrastructure Cleanup ✅
**Removed unnecessary Docker services**

**Docker Compose Changes (`test/assets/docker-compose.yml`):**
- ❌ Removed `jellyseerr` service
- ❌ Removed `postgres` service (only needed for Jellystat)
- ❌ Removed `jellystat` service
- ❌ Removed unused volumes: `jellyseerr-config`, `postgres-data`, `jellystat-data`
- ✅ Added `extra_hosts` configuration for mock server communication
- ✅ Improved `test-media-init` container with explicit user and safer copy

**Test Improvements:**
- Added `GITHUB_TOKEN` support to avoid API rate limiting when downloading OxiCleanarr plugin
- Documented token usage in `test/README.md`
- Fixed `symlink_library_test.go` to handle new `SyncLibraries()` return value
- Updated test config with placeholder values for mock services

### Integration Tests ✅

**Test Files Created:**
- `test/integration/advanced_rules_user_test.go` - User-based rules tests
- `test/integration/advanced_rules_watched_test.go` - Watched-based rules tests

**Test Coverage:**

**User-Based Rules (AdvancedRulesUser):**
- Match by user ID
- Match by username
- Match by email
- Retention periods per user tier

**Watched-Based Rules (AdvancedRulesWatched):**
- `RequireWatched_ProtectsUnwatched` ✅ - Validates unwatched protection
- `WatchTimestamp_AffectsTiming` ✅ - Tests retention timing
- `WatchedDate_TakesPrecedence` ✅ - LastWatched overrides AddedAt
- `WatchedContent_HasRetention` ✅ - Watched content respects retention

**Test Results:**
- ✅ All integration tests passing (119.5s runtime)
- ✅ 4/4 watched rules sub-tests pass
- ✅ Mock services work flawlessly

### Helper Function Enhancements

**Updated Test Helpers (`test/integration/helpers.go`):**
- `GetMovies()` - Fetch all movies from API
- `UpdateConfigAPIKeysWithExtras()` - Update all service API keys including Jellyseerr/Jellystat

### Git Commits Created ✅

**6 logical commits organized by concern:**

1. **`4946cc8`** - `test: add mock HTTP servers for Jellyseerr and Jellystat`
   - Added mock servers + helper enhancements
   
2. **`c4d8e0a`** - `feat: implement watched-based advanced rules with require_watched flag`
   - Core feature implementation + config documentation
   
3. **`70c7230`** - `test: add integration tests for user and watched-based advanced rules`
   - Comprehensive test coverage for advanced rules
   
4. **`224a10d`** - `test: remove Jellyseerr and Jellystat Docker services in favor of mocks`
   - Docker compose cleanup + GitHub token support
   
5. **`21c9d5b`** - `fix: use Jellystat as authoritative source for watch counts`
   - Sync engine improvement for accurate watch counts
   
6. **`9d7eb4b`** - `test: update config placeholders for mock services`
   - Test config with placeholder values

### Architecture Improvements

**Rule Evaluation Priority:**
1. Tag-based rules (highest priority)
2. User-based rules
3. **Watched-based rules** ← NEW
4. Standard retention rules (lowest priority)

**Data Flow:**
- Jellystat is authoritative for watch history
- Mock services enable isolated, deterministic testing
- No external Docker dependencies for Jellyseerr/Jellystat

### Key Learnings

1. **Mock services > Docker containers for tests** - Faster, more reliable, easier to maintain
2. **Priority-based rule evaluation** - Clear precedence prevents conflicts
3. **Authoritative data sources matter** - Jellystat overrides Jellyfin for watch data
4. **Sensible commit organization** - Group by concern: test infrastructure, features, tests, cleanup
5. **Test config should use placeholders** - Makes it clear values are set at runtime

### TODO

- [ ] Document watched rules in `OXICLEANARR_SPEC.md`
- [ ] Add unit tests for `evaluateWatchedRules()` function
- [ ] Consider adding UI for configuring watched rules in Rules page
- [ ] Test with real Jellystat integration (manual testing)

---
