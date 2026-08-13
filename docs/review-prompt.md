# REVIEW PROMPT - Leaving Soon refactor (OxiCleanarr + jellyfin-plugin-leaving-soon)

Paste this into a fresh agent session working inside the `oxicleanarr` repo to review
the current state of the "Leaving Soon" refactor before committing/merging.

---

## Context

We replaced the old push-based "Leaving Soon symlink library" design with a
**provider-pull** model: a new standalone Jellyfin plugin polls provider apps
(Maintainerr, OxiCleanarr, future sources) for scheduled-deletion media and manages
the symlink libraries itself. OxiCleanarr no longer owns any symlink logic.

**Design doc:** `docs/jellyfin-leaving-soon-plugin-design.md` (authoritative).
The old push design doc `docs/maintainerr-plugin-implementation-prompt.md` is
superseded - do NOT implement the push model.

**New plugin repo:** `jellyfin-plugin-leaving-soon` (sibling checkout, or
https://github.com/ramonskie/jellyfin-plugin-leaving-soon). It has a `v1.0.0`
release on GitHub. C#/.NET 9, Jellyfin 10.11.

## The normalized leaving-soon contract (shared by every provider)

Both Maintainerr and OxiCleanarr return this shape to the plugin:

```jsonc
{
  "items": [
    {
      "mediaServerId": "jellyfin-item-guid",   // REQUIRED - Jellyfin item GUID
      "type": "movie" | "show",
      "title": "The Matrix",                    // optional
      "deletionDate": "2026-09-01T00:00:00Z",  // optional
      "sourcePath": "/media/.../file.mkv"       // optional - plugin prefers its own Jellyfin path resolution
    }
  ]
}
```

- Maintainerr side: a new `GET /api/collections/leaving-soon` endpoint was designed
  (Option B) but **NOT yet implemented in the Maintainerr repo**. It is future work.
- OxiCleanarr side: `GET /api/media/leaving-soon` was **normalized to this contract**.

## Changes made in this repo (oxicleanarr)

1. **`internal/api/handlers/media.go`** - `ListLeavingSoon` rewritten: filters
   `DaysUntilDue` into the `leaving_soon_days` window, drops excluded items AND
   items without a Jellyfin id, maps `tv_show` -> `show`, emits
   `{ version: 1, items: [...] }` using new `models.LeavingSoonItem` /
   `models.LeavingSoonResponse`. No longer returns full `Media` objects.
2. **`internal/models/media.go`** - added `LeavingSoonItem` and
   `LeavingSoonResponse`.
3. **Removed entirely** (feature moved to the plugin):
   - `internal/services/symlink_library.go` + `symlink_library_test.go`
   - Jellyfin client plugin/virtual-folder methods in `internal/clients/jellyfin.go`
     (kept `RefreshLibrary`, used by post-deletion refresh) and `jellyfin_plugin_test.go`
   - All `Plugin*` types + `JellyfinVirtualFolder*` types in `internal/clients/types.go`
   - `SymlinkLibraryConfig` + field from `internal/config/types.go`, defaults in
     `defaults.go`, validation in `validation.go`, handler fields in
     `internal/api/handlers/config.go`
   - Web UI: `symlink_library` block in `web/src/lib/types.ts`, the Symlink Library
     section + handler + nav item in `web/src/pages/ConfigurationPage.tsx` and
     `web/src/components/AppLayout.tsx`
4. **`internal/services/sync.go`** - removed `symlinkLibraryManager` field, init,
   the `SyncLibraries` call, and the `leaving_soon_count` job-summary line.
5. **`config/config.yaml.example`** - replaced symlink_library docs with a note that
   leaving-soon symlinks are now handled by the plugin.
6. **Integration tests** (`test/integration/`):
   - Deleted `symlink_lifecycle_test.go`, `exclusion_lifecycle_test.go`
   - Removed plugin install/verify helpers (`InstallOxiCleanarrPlugin*`,
     `VerifyOxiCleanarrPlugin*`) from `jellyfin_setup.go` and setup_test.go
   - Removed `CheckSymlinks` / `WaitForSymlinkCount` / `GetHideWhenEmpty` /
     `GetMoviesLibraryName` from `helpers.go`; removed symlink assertions from
     `deletion_lifecycle_test.go`
   - Moved shared consts to `constants_test.go`
   - NEW: `leaving_soon_lifecycle_test.go` + `leaving_soon_constants.go`
   - NEW: `InstallLeavingSoonPluginToContainer` in `jellyfin_setup.go` - mirrors the
     old OxiCleanarr-bridge installer: fetch latest GitHub release, download zip
     (in-memory, zip-slip guarded), extract, write `meta.json` + `config.xml`
     (pointing at `http://oxicleanarr-test:9709`), `docker cp` into
     `/config/plugins/Leaving Soon/`, restart Jellyfin.
   - `setup_test.go`: added `LeavingSoonPluginLifecycle` subtest.

## What to review / verify

### Correctness of the OxiCleanarr API change
- `ListLeavingSoon` matches the contract exactly (field names, casing, version).
- Filtering is right: within `leaving_soon_days`, not excluded, HAS a Jellyfin id.
- `Type` is `movie`/`show` (never `tv_show`).
- The `sortMedia` helper for `LeavingSoonItem` (bubble sort) is correct and the
  DeletionDate-nil handling is safe.
- `media_test.go` updated for the new shape (check both subtests).

### Completeness of the symlink removal
- `rg -in 'symlink' --glob '*.go' --glob '*.tsx' --glob '*.ts'` should only hit
  comments/docs, nothing functional.
- No leftover references to `SymlinkLibrary`, `PluginAddSymlinks`,
  `CheckPluginStatus`, `InstallOxiCleanarrPlugin` etc.
- `go build ./...`, `go vet ./...`, `go test ./internal/...`, and web
  `tsc --noEmit` + `vite build` all green.
- `internal/clients/jellyfin.go` still has `RefreshLibrary` (used at
  sync.go post-deletion) and `ProxyImage` (used by media poster proxying).

### The new integration test
- Does the plugin folder name/`meta.json`/`config.xml` layout match what Jellyfin
  actually expects (PluginManager: folder under `/config/plugins/`, optional
  `meta.json`, config via Jellyfin XmlSerializer)?
- Auth gap: OxiCleanarr's `/api/media/leaving-soon` sits behind `mw.Auth` (JWT).
  The plugin polls without credentials by default, so the test likely needs
  `admin.disable_auth: true` in `test/assets/config/config.yaml` - OR the plugin
  must send a Bearer key. Decide which and make it consistent.
- `WaitForContainerSymlinkCount` uses `docker exec ls -la` and counts lines whose
  first char is `l` - verify that's reliable on the CI host.
- The plugin sync is fire-and-forget (202 Accepted); the test polls the container
  dir, so ordering is handled, but confirm the timeout (120s) is sane.
- The plugin resolves item paths via `ILibraryManager.GetItemById`, so items must
  already exist in Jellyfin with the same GUIDs OxiCleanarr reports.

### Plugin-side consistency (sibling repo, review too if you have it checked out)
- `Providers/MaintainerrProvider.cs` and `OxiCleanarrProvider.cs` parse the exact
  JSON each provider emits (Maintainerr one is future work - only OxiCleanarr is
  live now).
- `Services/SyncService.cs` resolves paths from Jellyfin, reconciles symlinks,
  hide-when-empty, double-refresh ~5s.
- The plugin `status` endpoint returns `{ Status: "ok" }` (not `{ Version }`) -
  confirm any consumer checks the right field.

## Known open items (flag, do not silently fix)
- Maintainerr `GET /api/collections/leaving-soon` endpoint is NOT implemented yet.
- OxiCleanarr auth for the plugin poll (disable_auth vs API key) is undecided.
- The plugin release zip contains only the DLL; `meta.json`/`config.xml` are
  written by the test. Consider whether that should move into the plugin build.

## Definition of done for this review
- List concrete findings with `file:line` references and severity
  (blocker / should-fix / nit).
- Do NOT edit files unless asked - this is a review.
- End with a one-line verdict: ready to commit / needs fixes before commit.
