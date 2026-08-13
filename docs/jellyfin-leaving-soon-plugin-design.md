# DESIGN - Generic "Leaving Soon" symlink library plugin for Jellyfin (provider-pull model)

> **Status:** design proposal, not yet implemented.
> **Where to use:** paste into an AI coding agent when implementing either side. The
> Maintainerr side builds in the Maintainerr monorepo (branch off `main`); the
> plugin side builds in a NEW generic Jellyfin plugin repo; OxiCleanarr gets a
> small rewrite that drops its symlink-library responsibility.
>
> **This doc supersedes** `maintainerr-plugin-implementation-prompt.md` for the
> "leaving soon symlink library" feature. That prompt described a PUSH model
> (Maintainerr calls the plugin). This design inverts it to a PULL model (the
> plugin polls one or more provider apps). The push model is dead; do not
> implement it.

---

## 1. Motivation

Maintainerr developers asked: could a Jellyfin plugin *check the leaving-soon
collections API from Maintainerr*, instead of Maintainerr pushing symlink
commands to the plugin?

Yes - and it is the better architecture. Reasons:

1. **The data already exists.** `GET /api/collections/overlay-data` on Maintainerr
   returns collections with full media membership. The Jellyfin item id is
   already in the payload (`CollectionMedia.mediaServerId` == the Jellyfin item
   GUID). Nothing new needs to be computed server-side to answer "what is leaving
   soon".

2. **Paths resolve inside the plugin.** Maintainerr's `MediaItem`/`MediaSource`
   contracts carry no on-disk path, and the Jellyfin adapter fetches
   `ItemFields.Path` internally but discards it into the mapped type. A plugin
   that runs inside Jellyfin can resolve `mediaServerId -> BaseItem.Path` from
   Jellyfin's own metadata store - authoritatively, for any provider. The push
   model forced Maintainerr to ship paths; the pull model does not.

3. **Decoupled, provider-agnostic.** The plugin owns the sync loop, virtual
   folder lifecycle, and symlink management. It polls whatever provider(s) are
   configured: Maintainerr, OxiCleanarr, and future apps. One plugin, N
   sources.

4. **No per-source timing coupling.** Maintainerr runs rules on its own schedule;
   OxiCleanarr on its own. The plugin reconciles on its own poll interval. Each
   side degrades independently (plugin keeps last-known state if a provider is
   briefly down).

### Scope of this redesign

- **New generic Jellyfin plugin** (new repo, generic name) with a provider
  abstraction. Builds on the current `jellyfin-plugin-oxicleanarr` symlink +
  virtual-folder code.
- **Maintainerr:** one new read-only endpoint, `GET /api/collections/leaving-soon`
  (Option B from the discussion). No other Maintainerr changes.
- **OxiCleanarr:** drop its symlink-library service; keep its media/deletion
  engine; expose a leaving-soon endpoint that satisfies the same provider
  contract. It already has `GET /api/media/leaving-soon` - normalize it.

---

## 2. Architecture

```
 +----------------+        poll       +--------------------------------+
 |  Maintainerr   |  GET /api/       |   jellyfin-plugin-leaving-soon |
 |  (provider)    |  collections/    |                                |
 +----------------+  leaving-soon     |  ProviderManager               |
                                      |   MaintainerrProvider  <-------+
 +----------------+        poll       |   OxiCleanarrProvider   <-------+
 |  OxiCleanarr   |  GET /api/        |   FutureProvider       <-------+
 |  (provider)    |  media/           |                                |
 +----------------+  leaving-soon     |  SyncService (poll + reconcile)|
                                      |  SymlinkManager                |
 +----------------+   path lookup     |  VirtualFolderManager         |
 |  Jellyfin      | <---------------- |  (BaseItem -> Path)           |
 |  (plugin host) |                   |                                |
 +----------------+                   +--------------------------------+
```

Roles:

| Component | Responsibility |
|---|---|
| Plugin | Poll providers, normalize to one contract, resolve paths from Jellyfin, reconcile symlink libraries + virtual folders, double-refresh |
| Maintainerr | Expose current scheduled-deletion collections + their media (read-only) |
| OxiCleanarr | Expose its leaving-soon list in the same normalized shape |
| Jellyfin | Provide item metadata / paths to the plugin (plugin is in-process) |

---

## 3. Normalized provider contract (shared by every provider)

Both providers return the SAME shape, so the plugin has one consumer path.

### 3.1 Response envelope

```jsonc
{
  "items": [
    {
      "mediaServerId": "jellyfin-item-guid-here",  // REQUIRED - Jellyfin item id
      "type": "movie" | "show",                    // REQUIRED - library type
      "title": "The Matrix",                       // optional
      "deletionDate": "2026-09-01T00:00:00Z",      // optional - for sorting/UI
      "sourcePath": "/media/movies/The.Matrix.mkv" // optional - plugin prefers
                                                   //   its own Jellyfin resolution
    }
  ]
}
```

### 3.2 The one hard requirement

`mediaServerId` MUST be the id Jellyfin knows for the item (a Jellyfin item GUID,
not a TMDB/IMDB/arr id). Both providers already hold it:

- Maintainerr: `CollectionMedia.mediaServerId` is the media-server item id; for
  Jellyfin that IS the item GUID (`collections.service.ts:1847`,
  `getCollectionMediaByCollection`).
- OxiCleanarr: `models.Media.JellyfinID` (`internal/models/media.go`).

Path resolution, sorting, and dedupe all key off this id.

---

## 4. The new generic plugin

### 4.1 Naming (decide before creating the repo)

Goal: generic, provider-agnostic, survives future apps.

- **Proposal:** `jellyfin-plugin-leaving-soon`
- Alternatives: `jellyfin-plugin-deletion-bridge`, `jellyfin-plugin-symlink-libraries`

Recommend `jellyfin-plugin-leaving-soon`: describes the feature, not the
provider. The current repo `jellyfin-plugin-oxicleanarr` stays as-is for
backward compat; the new plugin is the forward path. Confirm the name before
creating the repo.

### 4.2 Module layout (C#, mirrors current plugin)

```
Jellyfin.Plugin.LeavingSoon/
  Plugin.cs
  PluginServiceRegistrator.cs
  Configuration/
    PluginConfiguration.cs
    configPage.html
  Providers/
    ILeavingSoonProvider.cs
    LeavingSoonItem.cs              # normalized contract (section 3)
    MaintainerrProvider.cs
    OxiCleanarrProvider.cs
    ProviderRegistry.cs             # construct providers from config
  Services/
    SymlinkManager.cs               # ported from jellyfin-plugin-oxicleanarr
    VirtualFolderManager.cs         # ported from OxiCleanarr Go jellyfin.go
    SyncService.cs                  # poll + reconcile loop (IScheduledTask)
  Api/
    LeavingSoonController.cs        # status/debug endpoints, health
```

### 4.3 Provider interface

```csharp
public interface ILeavingSoonProvider
{
    string Name { get; }                    // "maintainerr", "oxicleanarr"
    Task<IReadOnlyList<LeavingSoonItem>> GetLeavingSoonItemsAsync(
        CancellationToken ct);
}
```

`ProviderRegistry` builds providers from `PluginConfiguration` (see 4.5). A
provider that fails on a poll returns an empty list + logs; it never crashes the
sync.

### 4.4 Sync algorithm (ported from OxiCleanarr `internal/services/symlink_library.go`)

`SyncLibraries` -> `filterScheduledMedia` -> `syncLibrary` -> `ensureVirtualFolder`
+ `createSymlinks` + `cleanupSymlinks`, then `RefreshLibraryByID`/`RefreshLibrary`.

In the plugin, `filterScheduledMedia` is replaced by "union of all configured
providers' items, deduped by `mediaServerId`". Per library (movies / tv):

1. **Resolve paths:** for each `mediaServerId`, resolve `BaseItem.Path` from
   Jellyfin's `ILibraryManager`; fall back to the provider's `sourcePath` if the
   provider sent one and Jellyfin resolution fails.
2. **Ensure the target host directory exists** (under configured `basePath`).
3. **Ensure the Jellyfin virtual folder exists** (`CreateVirtualFolder` with
   `collectionType` movies/tvshows + path); add the path if the folder exists
   without it (`AddPathToVirtualFolder`).
4. **Create symlinks** for items not yet linked.
5. **Clean stale symlinks** (`ListSymlinks` then remove those not in the desired
   set).
6. **Refresh:** scoped by library id first, fall back to global.
7. **Hide-when-empty:** when a library has zero items, clean symlinks, delete the
   virtual folder, then **double-refresh ~5s apart** (Jellyfin needs two
   refreshes after a library deletion to update `/Users/{userId}/Views` - this
   is empirical and already in the OxiCleanarr code with comments).

The plugin owns this loop on `IScheduledTask` (user-configurable interval, e.g.
15 min). No provider is required to be up; empty results just reconcile toward
empty libraries.

### 4.5 Plugin configuration

```jsonc
{
  "basePath": "/data/leaving-soon",
  "moviesLibraryName": "Leaving Soon - Movies",
  "tvLibraryName": "Leaving Soon - TV Shows",
  "hideWhenEmpty": true,
  "syncIntervalMinutes": 15,
  "providers": [
    {
      "name": "maintainerr",
      "enabled": true,
      "url": "http://maintainerr:6246",
      "apiKey": "",                // optional, sent as Bearer if set
      "includeCollections": []     // optional; empty = all scheduled-deletion collections
    },
    {
      "name": "oxicleanarr",
      "enabled": true,
      "url": "http://oxicleanarr:8080",
      "apiKey": ""
    }
  ]
}
```

This replaces the current stateless plugin's "paths come via API" model. The
current `OxiCleanarrController` endpoints (`api/oxicleanarr/*`) can be kept on
the old plugin for backward compat, but the new plugin does not need them.

### 4.6 Auth

- Maintainerr's collections API is currently unauthenticated (no guard on
  `CollectionsController`). For a LAN plugin this works, but recommend adding
  optional token auth to the new endpoint (see 5.4). The plugin sends
  `Authorization: Bearer <apiKey>` when configured.
- OxiCleanarr has admin auth; the leaving-soon endpoint must be reachable with
  the configured `apiKey` (or auth disabled) so the plugin can poll it.

---

## 5. Maintainerr: new `GET /api/collections/leaving-soon` endpoint

### 5.1 Route and controller

Add to `CollectionsController` (`apps/server/src/modules/collections/collections.controller.ts`),
alongside `overlay-data` (line ~377):

```
GET /api/collections/leaving-soon?libraryId=&typeId=
```

```ts
@Get('/leaving-soon')
@ApiOperation({
  summary: 'Get collections scheduled for deletion with their leaving-soon media',
})
@ApiQuery({ name: 'libraryId', required: false })
@ApiQuery({ name: 'typeId', required: false, enum: MediaItemTypes })
getLeavingSoonCollections(
  @Query('libraryId') libraryId?: string,
  @Query('typeId') typeId?: MediaItemType,
) {
  return this.collectionService.getLeavingSoonCollections(
    libraryId || undefined,
    typeId || undefined,
  );
}
```

### 5.2 Service method (in `CollectionsService`)

```ts
async getLeavingSoonCollections(libraryId?: string, typeId?: MediaItemType) {
  const collections = (await this.findCollections(libraryId, typeId))
    .filter((c) => c.isActive && c.deleteAfterDays != null);

  if (collections.length === 0) return { collections: [], total: 0 };

  const mediaByCollection =
    await this.getCollectionMediaByCollection(collections.map((c) => c.id));

  const collectionsOut = collections.map((collection) => ({
    id: collection.id,
    title: collection.title,
    type: collection.type,              // 'movie' | 'show'
    libraryId: collection.libraryId,
    mediaServerId: collection.mediaServerId,
    deleteAfterDays: collection.deleteAfterDays,
    arrAction: collection.arrAction,
    media: (mediaByCollection.get(collection.id) ?? []).map((m) => ({
      mediaServerId: m.mediaServerId,   // == Jellyfin item GUID
      addDate: m.addDate,
      deletionDate: collection.deleteAfterDays != null
        ? new Date(
            new Date(m.addDate).getTime() +
              +collection.deleteAfterDays * 86400000,
          )
        : null,
      tmdbId: m.tmdbId ?? null,
    })),
  }));

  return { collections: collectionsOut, total: collectionsOut.length };
}
```

Notes:
- Reuses `findCollections` and `getCollectionMediaByCollection` - the exact same
  queries `overlay-data` uses. Minimal new code, one DB query for membership.
- Filters to `isActive && deleteAfterDays != null` server-side: only collections
  that actually schedule deletion are "leaving soon" material. This is the
  server-side filter Option B asked for.
- `deletionDate` is precomputed as `addDate + deleteAfterDays`, matching
  `getCollectionDangerDate` semantics (fixed-ms arithmetic, `collections.service.ts:114`).
- Membership = "currently in the collection" = "in its grace period". The worker
  removes an item when it handles it, so the next poll naturally drops it. No
  per-item window filtering needed - the collection IS the leaving-soon set.

### 5.3 Contracts

Add a Zod schema + DTO in `packages/contracts/src/collections/`:

```ts
export const leavingSoonMediaItemSchema = z.object({
  mediaServerId: z.string(),
  addDate: z.date(),
  deletionDate: z.date().nullable(),
  tmdbId: z.number().nullable(),
});

export const leavingSoonCollectionSchema = z.object({
  id: z.number(),
  title: z.string(),
  type: z.enum(['movie', 'show']),
  libraryId: z.string(),
  mediaServerId: z.string().nullable(),
  deleteAfterDays: z.number().nullable(),
  arrAction: z.number(),
  media: z.array(leavingSoonMediaItemSchema),
});

export const leavingSoonResponseSchema = z.object({
  collections: z.array(leavingSoonCollectionSchema),
  total: z.number(),
});

export type LeavingSoonCollection = z.infer<typeof leavingSoonCollectionSchema>;
```

### 5.4 Auth on the new endpoint (recommendation)

The existing collections API is LAN-open. For a server that exposes itself to a
plugin on the network, recommend gating the new endpoint behind an optional
token:

- Option 1 (cheap): a `LEAVING_SOON_API_TOKEN` env var; when set, the endpoint
  requires `Authorization: Bearer <token>`. ~20 lines with a small guard.
- Option 2 (consistent): an API-key settings block like Streamystats.

Document it; do not let the plugin integration introduce a backdoor into a
server that is otherwise reachable from untrusted networks.

### 5.5 Maintainerr response mapping to the plugin contract (section 3)

| Plugin contract | Maintainerr field |
|---|---|
| `mediaServerId` | `media[].mediaServerId` |
| `type` | `collection.type` ('movie'/'show' - plugin maps to Jellyfin collectionType) |
| `title` | `collection.title` (optional; plugin may use Jellyfin's own title) |
| `deletionDate` | `media[].deletionDate` |

The plugin flattens collections into one `items[]` list (or keeps collections as
a filter knob via `includeCollections`).

---

## 6. OxiCleanarr rewrite

### 6.1 What moves out

- `internal/services/symlink_library.go` (`SymlinkLibraryManager`, all of it):
  virtual-folder lifecycle, symlink creation/cleanup, double-refresh. This logic
  now lives in the plugin.
- `internal/clients/jellyfin.go` plugin + virtual-folder client methods.
- `internal/config/types.go` `SymlinkLibraryConfig` block (can stay as a
  deprecated no-op for config backward compat, or be removed).
- The OxiCleanarr README's "OxiCleanarr Bridge Plugin is required" wording
  becomes "the generic leaving-soon plugin is required".

### 6.2 What stays

- The sync engine (`internal/services/sync.go`) - rules, retention, arr-stack
  matching.
- The web UI, storage, notifications, config hot-reload.

### 6.3 What changes

- Normalize `GET /api/media/leaving-soon` (`internal/api/handlers/media.go:99`)
  to emit the section-3 contract. It already filters `DaysUntilDue` into the
  `leaving_soon_days` window and excludes excluded items - keep that logic, just
  reshape the JSON to `{ "items": [ { mediaServerId, type, title, deletionDate } ] }`
  (map `JellyfinID -> mediaServerId`, `DeleteAfter -> deletionDate`,
  `Type -> "movie"|"show"`).
- Add the optional `Authorization: Bearer` check on that endpoint when an API
  key is configured (admin auth may already cover it - verify).
- OxiCleanarr no longer needs to know `base_path`/library names. Those live in
  the plugin config now. If an operator already uses OxiCleanarr symlink
  libraries, document the migration: recreate the same names in the plugin
  config; the plugin reconciles to the same symlinks.

---

## 7. Migration path

1. **Land the Maintainerr endpoint** (section 5) - small, isolated, no behavior
   change to existing collections.
2. **Create the generic plugin repo** (section 4) - port symlink + virtual
   folder code from `jellyfin-plugin-oxicleanarr`, add provider abstraction +
   `MaintainerrProvider` + `OxiCleanarrProvider`, `IScheduledTask` sync.
3. **Rewrite OxiCleanarr** (section 6) - remove symlink-library service, expose
   normalized leaving-soon endpoint.
4. **Flip operators:** existing OxiCleanarr symlink users point the new plugin at
   OxiCleanarr; Maintainerr users point it at Maintainerr (or both). Retire the
   old push-based prompt/plugin path once the new plugin covers both.
5. **Future apps:** add a new `XxxProvider` + config entry. No plugin core change.

---

## 8. Edge cases and decisions

| Topic | Decision |
|---|---|
| Item already symlinked | `CreateSymlinkAsync` removes an existing file/dir at target first (already in plugin code) - idempotent |
| Provider down during poll | Return empty list, log; reconcile toward empty but DO NOT delete libraries on first failure (needs a consecutive-failure counter or "stale grace" - recommend: only delete empty libraries after the provider has been up, or expose a `forceEmptyAfterFailures` knob) |
| Provider returns an item Jellyfin no longer has | Path resolution returns null; skip the item, log it |
| Same item from two providers | Dedupe by `mediaServerId`; Maintainerr wins if a tie exists (configurable priority) |
| TV shows | Plugin resolves the show folder path (`BaseItem.Path` of the show), symlinks the directory; same as OxiCleanarr today |
| `hideWhenEmpty` false | Keep the empty library + virtual folder, no delete/refresh dance |
| Double-refresh | Only on library deletion (empty case), 5s apart - empirical Jellyfin behavior, keep the OxiCleanarr comments |
| Maintainerr collection later deactivated | Drops out of the endpoint filter (`isActive`), next poll removes its symlinks |
| API versioning | Maintainerr endpoint is additive; contract version field in envelope if providers diverge later (`"version": 1`) |

---

## 9. Tests

- **Plugin (C#):** provider parsing (Maintainerr/OxiCleanarr JSON -> `LeavingSoonItem`),
  dedupe, path-resolution fallback, symlink create/cleanup idempotency, virtual
  folder create/delete/add-path, hide-when-empty double-refresh. Port the
  existing plugin test fixtures.
- **Maintainerr (Jest):** `getLeavingSoonCollections` - filter (active + has
  `deleteAfterDays`), deletionDate computation, media mapping, empty result.
  Controller spec for the route + query params. Contracts: schema defaults/invalid.
- **OxiCleanarr (Go):** normalized leaving-soon response contract, auth gate.
- **End-to-end with mocks:** extend `tools/dev/fake-jellyfin.mjs` in Maintainerr
  to serve the plugin's in-process expectations and the leaving-soon endpoint;
  or drive the plugin against a stub provider. OxiCleanarr already has
  `internal/api/handlers/media_test.go` fixtures to reuse.

---

## 10. Definition of done

- [ ] Maintainerr: `GET /api/collections/leaving-soon` + contracts + tests; no
      change to existing collection behavior.
- [ ] New generic plugin repo created (name agreed), `MaintainerrProvider` +
      `OxiCleanarrProvider` both return section-3 items.
- [ ] Plugin sync loop (add/cleanup/virtual-folder/hide-when-empty/double-refresh)
      working via `IScheduledTask`, configured interval.
- [x] OxiCleanarr normalized leaving-soon endpoint; symlink-library service
      removed.
- [ ] Auth (optional Bearer) honored by both providers' endpoints and sent by
      the plugin. (OxiCleanarr side done: `admin.api_key` accepted on every
      protected endpoint; Maintainerr pending.)
- [ ] Unit + mock e2e coverage on all three sides.
- [ ] Existing Maintainerr BoxSet behaviour unchanged; existing OxiCleanarr
      deletion engine unchanged.
