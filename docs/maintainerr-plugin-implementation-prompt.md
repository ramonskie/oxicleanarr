# IMPLEMENTATION PROMPT - Optional "Leaving Soon" symlink library for Jellyfin

> **⚠️ SUPERSEDED — DO NOT IMPLEMENT THIS.** The push-based symlink design below was
> replaced by the provider-pull model: a standalone Jellyfin plugin
> ([jellyfin-plugin-leaving-soon](https://github.com/ramonskie/jellyfin-plugin-leaving-soon))
> polls provider apps for scheduled-deletion media and manages the symlink libraries
> itself. See `docs/jellyfin-leaving-soon-plugin-design.md` (authoritative) for the
> current design. This document is kept for historical reference only.

> **Where to use:** paste this prompt into an AI coding agent working inside the
> Maintainerr monorepo (branch off `main`). The repo already contains the full
> Plex/Jellyfin/Emby media-server abstraction this feature builds on.
>
> **Goal:** add an *optional* Jellyfin integration that surfaces "leaving soon"
> media as a standalone symlink-backed library on the Jellyfin server, gated so
> every existing behaviour is unchanged. Each collection keeps working exactly as
> it does today (native BoxSet collection) unless the operator opts the
> collection into the plugin-driven symlink library.

---

## 1. Context

Maintainerr already implements a "leaving soon" grace period: rules match media
into a `Collection` (a Jellyfin BoxSet), the item sits in the collection for
`deleteAfterDays`, then the collection handler applies the configured action
(delete / unmonitor / quality profile). See:

- `apps/server/src/modules/collections/entities/collection.entities.ts` - `Collection` entity.
- `apps/server/src/modules/collections/collection-handler.ts` - `CollectionHandler.handleMedia()`.
- `apps/server/src/modules/api/media-server/jellyfin/jellyfin-adapter.service.ts` - `JellyfinAdapterService` (`implements IMediaServerService`).
- `packages/contracts/src/media-server/enums.ts` - `MediaServerFeature` capability flags.
- `packages/contracts/src/media-server/features.ts` - `MEDIA_SERVER_FEATURES` matrix + `supportsFeature()`.

The feature we are adding gives Jellyfin users the option to see the same
upcoming-deletion media as a **separate library** (e.g. `Leaving Soon - Movies`)
instead of a BoxSet collection. Because Jellyfin's native API cannot create
symlinks on the host filesystem, this relies on a small, **stateless**, open
source Jellyfin plugin: `jellyfin-plugin-oxicleanarr`
(https://github.com/ramonskie/jellyfin-plugin-oxicleanarr).

Reference implementation of the whole sync algorithm exists in OxiCleanarr:
`internal/services/symlink_library.go` (Go) - port the *behaviour*, not the
code, to the TypeScript/Nest stack.

---

## 2. Design decision (do NOT deviate)

Add a **per-collection switch** with a global default:

- New field on `Collection`: `leavingSoonMethod: 'collection' | 'symlink'`
  - `'collection'` = current behaviour, native BoxSet. **Default**, unchanged for
    all existing collections.
  - `'symlink'` = plugin-driven standalone library. Only available when the
    media server is Jellyfin, the plugin is installed/reachable, and the
    integration settings are configured. Expose it in the UI only under those
    conditions.
- New **Jellyfin settings block** `symlinkLibrary` (contracts + settings UI),
  mirroring how Streamystats settings are structured:
  - `enabled: boolean`
  - `basePath: string` (host path, e.g. `/data/leaving-soon`)
  - `moviesLibraryName: string` (default `Leaving Soon - Movies`)
  - `tvLibraryName: string` (default `Leaving Soon - TV Shows`)
  - `hideWhenEmpty: boolean` (default `true`)
- New `MediaServerFeature.SYMLINK_LIBRARY` flag: present for JELLYFIN only.
  The UI and the collection form gate the `'symlink'` option behind
  `supportsFeature(serverType, SYMLINK_LIBRARY)` AND the settings being enabled
  AND a plugin health check succeeding.

Rationale: makes the feature strictly opt-in, keeps the media-server abstraction
server-agnostic, and matches the existing Streamystats pattern for a
Jellyfin-only integration.

---

## 3. Plugin API contract (implement this client)

Plugin is stateless, auth via the already-stored Jellyfin API key
(`X-Emby-Token`). Base URL = configured Jellyfin URL.

| Method | Path | Auth | Body / Query | Response (PascalCase) |
|---|---|---|---|---|
| status | `GET /api/oxicleanarr/status` | no | - | `{ "Version": "2.0.0.0" }` |
| add symlinks | `POST /api/oxicleanarr/symlinks/add` | yes | `{ "items": [{ "sourcePath": string, "targetDirectory": string }] }` | `{ "Success": bool, "CreatedSymlinks": string[], "Errors": string[] }` |
| remove symlinks | `POST /api/oxicleanarr/symlinks/remove` | yes | `{ "symlinkPaths": string[] }` | `{ "Success": bool, "RemovedSymlinks": string[], "Errors": string[] }` |
| list symlinks | `GET /api/oxicleanarr/symlinks/list?directory=/path` | yes | query `directory` | `{ "Symlinks": [{ "Path", "Target", "Name" }], "Count": int, "SymlinkNames": string[], "Message": string }` |
| create directory | `POST /api/oxicleanarr/directories/create` | yes | `{ "directory": string }` | `{ "Success": bool, "Directory": string, "Created": bool, "Message": string }` |
| remove directory | `DELETE /api/oxicleanarr/directories/remove` | yes | `{ "directory": string, "force": bool }` | `{ "Success": bool, "Directory": string, "Message": string }` |

Notes:
- Body field for remove is `symlinkPaths` (camelCase); response fields are
  PascalCase (`Success`, `CreatedSymlinks`, ...). Check both, do not assume.
- Partial success: `Success: true` with a non-empty `Errors` array is normal.
- Directories are auto-created by `symlinks/add` as a fallback; the explicit
  `directories/create` call is used first so a Jellyfin VirtualFolder creation
  does not 400 on a missing path.
- Jellyfin virtual-folder/library lifecycle is NOT the plugin's job. Use the
  Jellyfin API for: `GET /Library/VirtualFolders`,
  `POST /Library/VirtualFolders`, `POST /Library/Refresh`,
  `POST /Library/Refresh` scoped by library id. See `JellyfinVirtualFolderClient`
  in OxiCleanarr `internal/services/symlink_library.go` and `internal/clients/jellyfin.go`.

---

## 4. Implementation steps

### 4.1 Contracts (`packages/contracts`)

1. `packages/contracts/src/media-server/enums.ts`: add
   `SYMLINK_LIBRARY = 'symlink_library'` to `MediaServerFeature`.
2. `packages/contracts/src/media-server/features.ts`: add
   `MediaServerFeature.SYMLINK_LIBRARY` to the JELLYFIN set only.
3. New `packages/contracts/src/media-server/jellyfin/symlinkLibrarySetting.ts`:
   Zod schema for the `symlinkLibrary` settings block (fields listed in
   section 2). Follow the shape/style of the existing `jellyfinSetting.ts` in
   the same directory.
4. `packages/contracts/src/collections/...`: add `leavingSoonMethod` enum
   (`'collection' | 'symlink'`, default `'collection'`) to the collection DTO /
   create / update schemas. Reuse the existing Zod pattern used for fields like
   `cleanupLeftoverFolders`.

### 4.2 Jellyfin settings (`apps/server/src/modules/settings` + UI)

1. Extend the Jellyfin settings entity/service/controller (see
   `settings-data.service.ts`, `settings.controller.ts`, the `Jellyfin*` schemas
   in contracts) with the `symlinkLibrary` block.
2. `apps/ui/src/components/Settings/Jellyfin/index.tsx`: add the form section,
   reusing shared settings components. Only render when server type is Jellyfin.
3. Add a **plugin status / connection test** endpoint (e.g.
   `POST /api/settings/jellyfin/plugin-test`) that calls
   `GET /api/oxicleanarr/status` and returns the version. Reuse the existing
   pattern from Streamystats' connection test (auth with the stored Jellyfin
   key; never send the key back).

### 4.3 Plugin HTTP client (new module)

Create a thin service, e.g.
`apps/server/src/modules/api/jellyfin-plugin/jellyfin-plugin.service.ts`
(register in `JellyfinModule` and the settings module where needed). Use the
repo's existing axios conventions (`apps/server/src/modules/api/` clients),
`MaintainerrLogger`, and `X-Emby-Token` header from the stored Jellyfin API key.
Methods:

- `checkStatus(): Promise<PluginStatus>`
- `addSymlinks(items: { sourcePath, targetDirectory }[]): Promise<AddResult>`
- `removeSymlinks(paths: string[]): Promise<RemoveResult>`
- `listSymlinks(directory: string): Promise<Symlink[]>`
- `createDirectory(directory: string): Promise<void>`
- `removeDirectory(directory: string, force: boolean): Promise<void>`

Type the responses explicitly (PascalCase fields). Do not use `any`.

### 4.4 Collection entity + migration

1. `collection.entities.ts`: add
   `@Column({ type: 'varchar', default: 'collection' }) leavingSoonMethod: 'collection' | 'symlink'`.
2. Generate a TypeORM migration (repo convention: `typeorm_instructions.txt`
   in the repo root; migrations generated through TypeORM, safe and reversible).

### 4.5 Symlink library sync service (new module)

Create `apps/server/src/modules/collections/symlink-library.service.ts`
(port the algorithm from OxiCleanarr `internal/services/symlink_library.go`:

`SyncLibraries` -> `filterScheduledMedia` -> `syncLibrary` -> `ensureVirtualFolder`
+ `createSymlinks` + `cleanupSymlinks`, then `RefreshLibraryByID`/`RefreshLibrary`).

Behaviour to replicate:
1. Select media from collections whose `leavingSoonMethod === 'symlink'`,
   keyed by scheduled deletion (media currently in their grace period).
2. `syncLibrary` per type (movies / tv):
   a. Ensure the target host directory exists (`directories/create`).
   b. Ensure the Jellyfin virtual folder exists
      (`POST /Library/VirtualFolders`, name + `collectionType` movies/tvshows +
      target path); add path if the folder exists without it.
   c. `symlinks/add` for each scheduled item (source = item's media-server path,
      target = the per-type subdir under `basePath`).
   d. `symlinks/list` then `symlinks/remove` for symlinks that are no longer
      scheduled (stale cleanup).
   e. Trigger a Jellyfin library refresh (scoped by library id first, fall back
      to global). After an empty library is removed, Jellyfin needs **two**
      refreshes ~5s apart to update user views - replicate this.
3. `hideWhenEmpty`: when a library has zero scheduled items, clean symlinks,
   delete the virtual folder, double-refresh.

Integration points:
- **On collection rule run / membership change:** when an item joins or leaves a
  `'symlink'` collection, enqueue/re-run the affected library sync (mirror how
  `collection-handler.ts` and the rule executor interact; keep it async/batched,
  do not block rule execution).
- **Collection handler** (`handleMedia`): when a `'symlink'` collection handles
  an item (action fires), the item's symlink must be removed during the next
  sync. Do not add new blocking filesystem work inside `handleMedia`.

### 4.6 Media-server abstraction touchpoints

- Keep `modules/api/media-server/` server-agnostic. The plugin client and the
  virtual-folder logic live under `jellyfin/` (or the new
  `jellyfin-plugin`/`collections` modules), never in the shared interface.
- The `'symlink'` option must be **unavailable** for Plex and Emby: gate on
  `supportsFeature(serverType, MediaServerFeature.SYMLINK_LIBRARY)` in the UI
  and validate server-side when creating/updating a collection.

### 4.7 UI

1. Collection create/edit form (`apps/ui/src/components/Rules/RuleGroup/AddModal/index.tsx`
   and any collection edit views): a "Leaving soon method" selector
   (`Collection` / `Symlink library`) with a helper tooltip. Disable
   `Symlink library` when not supported/unconfigured (tooltip explains why).
2. Settings: the `symlinkLibrary` block + "Test plugin" button
   (`apps/ui/src/components/Settings/Jellyfin/index.tsx`).
3. Follow the repo UI conventions (shared Button/LoadingSpinner/feedback
   components; no new bespoke primitives).

### 4.8 Logging & errors

- Use `MaintainerrLogger` with a context set in the constructor.
- For paired caught-error logs: keep warn/error messages plain, put the
  throwable on `logger.debug(error)`. Only use `logger.error('message', error)`
  when the higher-level log intentionally carries the throwable.
- Never log the Jellyfin API key.

---

## 5. Tests

- **Unit (server):** `jellyfin-plugin.service.spec.ts` - each plugin method maps
  request/response correctly (camelCase body, PascalCase response parsing),
  partial-success handling, auth header set, error propagation.
- **Unit (server):** `symlink-library.service.spec.ts` - filterScheduledMedia
  (movies vs tv), ensureVirtualFolder (create vs existing vs add-path),
  createSymlinks, stale cleanup, hideWhenEmpty path, double-refresh logic.
  Use mocked plugin + jellyfin clients.
- **Unit (contracts):** schema tests for `leavingSoonMethod` and
  `symlinkLibrarySetting` (invalid values rejected, defaults applied).
- **Unit (UI):** selector disabled state when feature unsupported/unconfigured.
- **Test with mocks:** `tools/dev/fake-jellyfin.mjs` exists; extend it to
  emulate `GET/POST /Library/VirtualFolders`, `/Library/Refresh`, and the
  plugin's `/api/oxicleanarr/*` endpoints so the full flow is testable without a
  real Jellyfin. See `tools/dev/` and the seeding docs in AGENTS.md.

---

## 6. Docs & conventions

- Document the feature (per-collection switch, plugin install, settings,
  hideWhenEmpty, caveats) in the Maintainerr docs repo referenced by
  ARCHITECTURE.md - feature documentation lives there, not in this repo.
- Commit messages: Conventional Commits (`feat:`, `fix:`). Use plain ASCII
  hyphen, never em/en dashes, in any committed artifact.
- Keep `modules/api/media-server/` free of Plex/Jellyfin type leaks.
- All new external integration tokens use existing settings/storage; never
  hard-code.

---

## 7. Definition of done

- [ ] `MediaServerFeature.SYMLINK_LIBRARY` added, Jellyfin-only.
- [ ] `symlinkLibrary` settings block (contracts + server + UI) with plugin test.
- [ ] `jellyfin-plugin.service.ts` with all 6 endpoints, typed, tested.
- [ ] `leavingSoonMethod` on `Collection` + migration; `'collection'` default;
      `'symlink'` rejected for Plex/Emby.
- [ ] `symlink-library.service.ts` implementing sync (add/stale-remove/virtual
      folder/hideWhenEmpty/double-refresh), wired into rule runs and the
      collection handler.
- [ ] UI selector + settings section, gated by capability.
- [ ] Unit tests (server + contracts + UI) and fake-jellyfin/plugin mock
      coverage for the full flow.
- [ ] Existing BoxSet behaviour unchanged when `leavingSoonMethod` is
      `'collection'` (default) - full regression suite green.
