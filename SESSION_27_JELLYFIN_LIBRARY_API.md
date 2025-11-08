# Session 27: Jellyfin Virtual Folder (Library) API Research

## Executive Summary

**DISCOVERY**: Found complete Jellyfin Virtual Folder API documentation from official Jellyfin source code. Virtual Folders (Libraries) are different from Collections and provide better visibility for "Leaving Soon" items.

## Key Findings

### API Endpoints Discovered

All endpoints are under the `/Library/VirtualFolders` route and require elevated permissions.

#### 1. List All Virtual Folders
```
GET /Library/VirtualFolders
Authorization: Required (FirstTimeSetupOrElevated policy)
Response: Array of VirtualFolderInfo objects
```

#### 2. Create Virtual Folder (Library)
```
POST /Library/VirtualFolders
Query Parameters:
  - name: string (library name)
  - collectionType: CollectionTypeOptions enum (movies, tvshows, music, etc.)
  - paths: string[] (comma-delimited array of folder paths)
  - refreshLibrary: bool (default: false)
Body: AddVirtualFolderDto (optional library options)
Response: 204 No Content on success
```

#### 3. Delete Virtual Folder
```
DELETE /Library/VirtualFolders
Query Parameters:
  - name: string (library name to remove)
  - refreshLibrary: bool (default: false)
Response: 204 No Content on success
```

#### 4. Rename Virtual Folder
```
POST /Library/VirtualFolders/Name
Query Parameters:
  - name: string (current library name)
  - newName: string (new library name)
  - refreshLibrary: bool (default: false)
Response: 204 No Content, 404 if not found, 409 if name conflict
```

#### 5. Add Path to Virtual Folder
```
POST /Library/VirtualFolders/Paths
Body: MediaPathDto (name, path/pathInfo)
Query Parameters:
  - refreshLibrary: bool (default: false)
Response: 204 No Content
```

#### 6. Remove Path from Virtual Folder
```
DELETE /Library/VirtualFolders/Paths
Query Parameters:
  - name: string (library name)
  - path: string (path to remove)
  - refreshLibrary: bool (default: false)
Response: 204 No Content
```

## Collection Type Options

Based on Jellyfin source code, the `collectionType` parameter accepts:
- `movies` - Movie library
- `tvshows` - TV Show library
- `music` - Music library
- `books` - Book library
- `photos` - Photo library
- `homevideos` - Home videos
- `musicvideos` - Music videos
- `mixed` - Mixed content (not recommended)

For OxiCleanarr's "Leaving Soon" feature, we need:
- **Separate libraries** for movies (`movies`) and TV shows (`tvshows`)
- This follows Jellyfin best practices and Janitorr's proven approach

## Architecture Comparison

### Current: Jellyfin Collections
- **Location**: Collections section (requires navigation)
- **Visibility**: Hidden by default, users must open Collections
- **API**: `/Collections` endpoints (already implemented)
- **Type**: BoxSet entities (groups of media)
- **Status**: ✅ Working in OxiCleanarr (Session 7, Session 20)

### Alternative: Jellyfin Virtual Folders (Libraries)
- **Location**: Sidebar (always visible)
- **Visibility**: First-class navigation item
- **API**: `/Library/VirtualFolders` endpoints (needs implementation)
- **Type**: Separate libraries with their own views
- **Status**: 🔬 Researched (this session)

### Janitorr's Approach
- Uses **symlinks** to create virtual libraries
- Creates separate "Leaving Soon" libraries for movies and TV shows
- Symlinks point to actual media files in Radarr/Sonarr directories
- Empty file trick: Creates `empty-file.media` to prevent scan failures
- Rebuilds symlinks on each run (configurable with `from-scratch: true`)

## Implementation Requirements

### Filesystem Access
```yaml
# Required Docker volume mappings
volumes:
  # Media directories (read-only)
  - /data/media/movies:/media/movies:ro
  - /data/media/tv:/media/tv:ro
  
  # Symlink directory (read-write)
  - /data/media/leaving-soon:/leaving-soon:rw
```

### Path Resolution Challenge
**Problem**: OxiCleanarr, Jellyfin, Radarr, and Sonarr may see different filesystem paths

**Example Scenario**:
- **Radarr** sees: `/movies/Avatar (2009)/Avatar.mkv`
- **OxiCleanarr container** sees: `/media/movies/Avatar (2009)/Avatar.mkv`
- **Jellyfin container** sees: `/data/movies/Avatar (2009)/Avatar.mkv`

**Solution**: Radarr/Sonarr APIs provide actual filesystem paths in item responses
- Radarr: `GET /api/v3/movie` returns `path` field
- Sonarr: `GET /api/v3/series` returns `path` field
- These are the **container filesystem paths** as seen by Radarr/Sonarr
- Must be translated to paths visible to OxiCleanarr and Jellyfin

## Proposed Implementation Plan

### Option A: Full Symlink Library Feature (Janitorr-style)

**Pros**:
- ✅ Better UX (sidebar visibility)
- ✅ Proven approach (Janitorr production use)
- ✅ Separate movie/TV libraries (cleaner)
- ✅ Non-destructive (symlinks, not moves)

**Cons**:
- ❌ Complex path mapping configuration
- ❌ Requires filesystem access (security concern)
- ❌ Docker volume configuration complexity
- ❌ Additional failure points (symlink creation, path translation)

**Estimated Effort**: 8-12 hours (medium complexity)

**Files to Create**:
1. `internal/services/symlink_library.go` - Symlink manager service (~300 lines)
2. Unit tests for symlink service (~200 lines)

**Files to Modify**:
1. `internal/config/types.go` - Add SymlinkLibraryConfig struct
2. `internal/config/validation.go` - Validate symlink config
3. `internal/config/defaults.go` - Default symlink settings
4. `internal/clients/jellyfin.go` - Add virtual folder API methods (~150 lines)
5. `internal/clients/types.go` - Add VirtualFolderInfo struct
6. `internal/services/sync.go` - Integrate symlink sync
7. `config/oxicleanarr.yaml.example` - Document symlink config
8. `README.md` - Document Docker volume requirements

**Config Example**:
```yaml
integrations:
  jellyfin:
    leaving_soon_library:
      enabled: true
      method: "symlinks"  # or "collections"
      symlink_dir: "/leaving-soon"
      create_library: true  # Auto-create virtual folders
      movie_library_name: "Leaving Soon - Movies"
      tv_library_name: "Leaving Soon - TV Shows"
      path_mappings:
        # Map Radarr/Sonarr paths to Jellyfin paths
        - source: "/movies"
          target: "/data/movies"
        - source: "/tv"
          target: "/data/tv"
```

### Option B: Keep Current Collections Approach

**Pros**:
- ✅ Already implemented and working
- ✅ No filesystem access needed
- ✅ Simpler configuration
- ✅ No path mapping complexity

**Cons**:
- ❌ Less visible (Collections section)
- ❌ Requires user to navigate to Collections

**Current Status**: ✅ Complete (Sessions 7, 20)

## Decision Matrix

| Factor | Collections (Current) | Symlink Libraries (Option A) |
|--------|----------------------|------------------------------|
| **Visibility** | Medium (Collections section) | High (Sidebar) |
| **Complexity** | Low | High |
| **Security** | Safe (API only) | Filesystem access required |
| **Configuration** | Simple | Complex (volume mapping) |
| **User Setup** | None | Docker volumes required |
| **Maintenance** | Low | Medium |
| **Feature Parity** | 100% | 100% + better UX |

## Recommendation

**For OxiCleanarr v1.0**: Keep current Collections approach (Option B)
- Current implementation is **production-ready**
- Avoids filesystem complexity
- Easier for users to set up
- Proven stable (Sessions 7, 20)

**For OxiCleanarr v2.0** (future enhancement): Consider Option A
- Survey users to see if library visibility is a major concern
- Document comprehensive setup guide for Docker volumes
- Provide path mapping helper/validator
- Make it an **optional feature** with collections as fallback

## Next Steps

### Immediate (This Session)
1. ✅ Document findings (this file)
2. ⏳ Update AGENTS.md with Session 27 summary
3. ⏳ Commit research documentation
4. ⏳ Move to next priority feature (user-based cleanup, mobile polish, etc.)

### Future (If Option A Pursued)
1. Create feature branch: `feature/symlink-libraries`
2. Implement virtual folder API methods in Jellyfin client
3. Create SymlinkLibraryManager service
4. Add path mapping configuration and validation
5. Implement symlink creation/cleanup logic
6. Write comprehensive tests (unit + integration)
7. Document Docker setup requirements
8. Create migration guide from collections to libraries

## References

- **Jellyfin Source**: `LibraryStructureController.cs` - Virtual folder API implementation
- **Janitorr Source**: `BaseMediaServerService.kt` - Abstract library management
- **Jellyfin Docs**: https://jellyfin.org/docs/general/server/libraries
- **OxiCleanarr Collections**: `internal/services/jellyfin_collections.go` (current implementation)

---

**Session**: 27
**Date**: Nov 4, 2025
**Status**: Research Complete - Decision Deferred to Future Version
**Next Session**: Move to user-based cleanup or mobile responsiveness
