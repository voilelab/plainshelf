# Shelf and platform capabilities

!!! note "Implemented"
    Shelf capability fields are served by Go and read by the frontend.
    Device operations are exposed through `PlatformCapabilities`, accessed with
    `getPlatformCapabilities()` beside `getBookshelfProvider()`.

The frontend reads and writes shelf data through `BookshelfProvider`. The
host supplies file pickers, native readers, file exports and shelf management
through a separate platform object.

This design splits those methods by **who knows the answer**:

- **What a shelf can do** is known by the engine that opened it. The server
  reports it as fields on each shelf, the same on every client.
- **What the device can do** is known by the host the frontend runs in (Wails,
  Capacitor, the wasm page, a browser). The host provides it as a separate
  platform interface, never over the HTTP API.

Since pCloud moved into Go and the TypeScript reader was removed, every client reads every shelf through the same Go
API. The capability split leaves the provider with one job:
reading and writing a shelf.

---

## Where each optional method goes

The implemented migration from the optional methods formerly on `BookshelfProvider`, in
`frontend/src/providers/bookshelfProvider.ts`:

| Method | Implemented by | Goes to |
|---|---|---|
| `getSourceAsset` (now required) | server, mobile, pCloud | **Required** on the shelf interface |
| `getSourceAssetsBundle` (now required) | server, mobile, pCloud | **Required** on the shelf interface |
| `supportsShelfRefresh` | server, mobile, pCloud | Shelf capability `rescan` |
| `refreshShelf` | server, mobile, pCloud | Shelf interface, gated by `rescan` |
| `supportsCharCountListing` | mobile, pCloud | Shelf capability `char_count` |
| `getShelfFetchedAt` | mobile, pCloud | Shelf field `offline.snapshot_at` |
| `downloadBook`, `removeDownload`, `getDownloadState`, `listDownloadedBookEntries` | mobile | Shelf interface, backed by the Go API, gated by `download` |
| `getPersistedShelfID` | mobile | Offline wrapper for remote-server entries (see below) |
| `supportsServerMode` (removed) | — | Every backend now has a Go server to answer `/api/mode` |
| `filtersNsfwOnDevice`, `localNsfwMarks` | mobile, pCloud | Still used by `EmbeddedPCloudReader` for the device preference and offline filtering |
| `coversMustBeFetchedAsBlob` | mobile | Platform boolean, retained on Android: direct WebView image requests bypass CapacitorHttp and can be blocked as mixed content |
| `getStorageEstimate` | mobile | Platform interface |
| `openLocalBookFiles`, `importBookFromLocalPath` | desktop | Platform interface |
| `openDesktopFolder`, `openDesktopBookFolder`, `openDesktopShelfFolder`, `openDesktopShelfDirectory` | desktop | Platform interface, renamed without `Desktop` |
| `openDesktopReader` | desktop | Platform interface |
| `saveBookContentToFile` | desktop, mobile | Platform interface |
| `previewDesktopShelfID`, `addDesktopShelf`, `getDesktopShelfDetails`, `modifyDesktopShelf`, `removeDesktopShelf` | desktop | Platform interface as `previewShelfID`, `addShelf`, `getShelfDetails`, `modifyShelf`, `removeShelf` |
| *(new)* `openExternalUrl` | — | Platform interface: opens the system browser for pCloud authorization |

No method is left unassigned. "Mobile" here is `MobileBookshelfProvider`, the
wrapper that adds offline downloads to remote PlainShelf server connections.
Embedded pCloud/WebDAV entries use Go downloads directly; local shelves are not wrapped.

---

## Shelf capabilities

`GET /api/shelves` returns `kind`, `capabilities` and `offline` alongside `id`,
`name` and `read_only`:

```json
[
  {
    "id": "living_room",
    "name": "Living room",
    "read_only": true,
    "kind": "virtual",
    "capabilities": {
      "rescan": true,
      "download": true,
      "fingerprint": false,
      "char_count": false
    },
    "offline": {
      "snapshot_at": "2026-10-16T09:12:44Z"
    }
  }
]
```

| Field | Meaning |
|---|---|
| `kind` | `local` or `virtual`, as defined in [Two kinds of shelf](shelf-sources.md#two-kinds-of-shelf). |
| `capabilities.rescan` | The shelf can be rescanned on request (`POST /api/shelves/{id}/scans`). |
| `capabilities.download` | Books can be downloaded to the device's offline layer. The engine reports true for a virtual shelf and false for a local one, which is already local. A client that keeps its own offline copy overrides it (see [the remote-server connection](#the-remote-server-connection-stays-a-wrapper)). |
| `capabilities.fingerprint` | Fingerprints and similar-book pairs are available. False on a virtual shelf: computing them reads every source, a request per book. |
| `capabilities.char_count` | Listings carry character counts, so sorting and filtering by length work. False on a virtual shelf, whose listing comes from the exported book cache and carries none. |
| `offline.snapshot_at` | When the shelf listing was last fetched from the source. Present only on a virtual shelf that has a snapshot; the UI shows it as "last updated" when the source is unreachable. |

`capabilities` is an object rather than a flat set of `can_*` booleans so that a
capability can grow detail later without a new top-level key, and so that a
client can treat an unknown member as absent. Every field is **added**: an older
client ignores members it does not know, and `read_only` keeps its meaning.

The server fills these from the opened shelf. `read_only` is still the one
answer to "may I show write controls", and `kind` never stands in for it: a
local shelf can be read-only too.

### Who reads them

| Former caller | Implemented behavior |
|---|---|
| `composables/useShelfRefresh.ts` asks `supportsShelfRefresh` and `getShelfFetchedAt` | reads `capabilities.rescan` and `offline.snapshot_at` |
| `features/library/pages/LibraryPage.vue`, `features/maintenance/pages/SimilarContentPage.vue` and `utils/bookFilters/registry.ts` ask `supportsCharCountListing` | read `capabilities.char_count` |
| `features/maintenance/pages/SimilarContentPage.vue` calls `getFingerprintStatus` and handles an empty answer | checks `capabilities.fingerprint` first |
| `features/mobile/pages/DownloadsPage.vue` and the book actions use the mobile download methods | check `capabilities.download`, then call the shelf interface |
| `composables/useServerMode.ts` asks `/api/mode` directly | Already implemented; pCloud entries query the embedded server |

`/api/mode` reports the whole installation's `read_only`, not the selected
shelf's. The embedded server can be writable while its pCloud shelf stays
read-only: `useWriteAccess` also requires a provider with a write surface, which
the mobile pCloud reader does not have. Its API transport rejects writes too.
The mode query does not wait for the pCloud shelf to open.

The fields live in `server/contract/shelves`, beside `ShelfInfo`
(`server/handle_shelf.go`). The source configuration itself is never part of the
response, as [the source design](shelf-sources.md#rules) requires.

---

## The platform interface

The interface is implemented in `frontend/src/providers/platformCapabilities.ts`.
Wails supplies its host operations in `providers/wailsBookshelfProvider.ts`;
Capacitor supplies its object in `shells/mobile/platform.ts`. Web and the wasm
app use the browser implementation. The wasm toolbar continues to manage its
own engine outside the shared app; it does not expose `shelves` to that app.

```ts
interface PlatformCapabilities {
  // Shelf management: present only where the host can manage shelves.
  shelves?: {
    previewShelfID(name: string): Promise<ShelfIDPreview>;
    addShelf(params: AddShelfParams): Promise<void>;
    checkShelfSource(params: ShelfSourceParams): Promise<ShelfSourceCheck>;
    listShelfDetails?(): Promise<ShelfDetails[]>; // Includes failed virtual entries.
    retryShelf?(shelfID: string): Promise<void>;
    getShelfDetails(shelfID: string): Promise<ShelfDetails>;
    modifyShelf(params: ModifyShelfParams): Promise<void>;
    removeShelf(shelfID: string): Promise<void>;
  };

  // Files and folders on the device.
  openLocalBookFiles?(): Promise<string[] | null>;
  importBookFromLocalPath?(localPath: string, folderPath: string): Promise<ImportBookResult | null>;
  openFolder?(folderPath: string): Promise<void>;
  openBookFolder?(bookId: string): Promise<void>;
  openShelfFolder?(shelfID: string): Promise<void>;
  chooseShelfDirectory?(): Promise<string | null>;
  saveBookContentToFile?(bookId: string, suggestedName: string): Promise<string | void>;

  // Other apps and the system.
  openReader?(bookId: string, section?: number): Promise<void>;
  openExternalUrl?(url: string): Promise<void>;
  getStorageEstimate?(): Promise<StorageEstimateResult>;

  // Rendering workarounds.
  coversMustBeFetchedAsBlob?: boolean;
}
```

The host builds this object during bootstrap (lazily for desktop), and the frontend
reads it through `getPlatformCapabilities()`, beside `getBookshelfProvider()`. A missing member means the
platform cannot do it, which is the same rule the optional methods follow today,
in one place instead of spread over the provider.

| Host | `shelves` | Files and folders | `openExternalUrl` |
|---|---|---|---|
| Server (browser) | absent: shelves come only from the YAML file | absent | browser `window.open` |
| Desktop (Wails) | Wails bindings | Wails bindings | Wails `BrowserOpenURL` |
| Android (Capacitor) | absent from the shared desktop management form; the mobile entry editor uses its existing Go plugin | Documents export, storage estimate, blob covers | Capacitor `Browser` (browser preview uses `window.open`) |
| wasm demo | absent in the shared app; the host toolbar manages the engine | absent | `window.open` |

**None of this goes over the HTTP API.** Shelf management stays a host
capability, as [Who holds the shelf list](shelf-sources.md#who-holds-the-shelf-list)
requires: `server.App`'s handler has no shelf-management route, so "can this
client manage shelves" is answered by whether `shelves` is present, never by a
server field.

---

## The remote-server connection stays a wrapper

The Android app can still connect to another PlainShelf server's full API, as
it does today. That connection is client-side state, not a shelf source (see
[Compatibility](shelf-sources.md#compatibility)), and its offline downloads
remain the job of `MobileBookshelfProvider` wrapping a `ServerBookshelfProvider`.

In the Android shell (`frontend/src/shells/mobile/index.ts`), only remote-server
entries use `MobileBookshelfProvider`. Embedded pCloud and WebDAV entries use
`EmbeddedPCloudReader` directly: Go owns their offline content and snapshots.
Local shelves use `ServerBookshelfProvider` without a wrapper.

The capability migration keeps downloads available on a wrapped
remote connection, whatever the remote server reports. A remote server reports
`download: false` for its local shelves, and an older server reports nothing,
so the mobile shell overrides `download` to true for its wrapped entries. The
layer that owns the offline copy answers `download`: the
engine for shelves it opens, the wrapper for a remote connection.
`getPersistedShelfID` belongs to the remote-server wrapper.

Virtual shelves now cover reading another PlainShelf through its WebDAV export.
Retiring the full-API remote-server connection and its
wrapper remains a separate decision.

---

## What stays the same

- `WailsBookshelfProvider` keeps extending `ServerBookshelfProvider`: the
  desktop is already the target shape. Its desktop-only methods have moved to the
  platform object; the shelf methods do not change.
- `read_only` on `GET /api/shelves` keeps its meaning and its place.
- An older client against a newer server ignores `kind`, `capabilities` and
  `offline`. A newer client against an older server sees them missing and falls
  back to today's behaviour: `rescan` true, the rest true, and `download` as
  the remote-server wrapper reports it (true) or false where nothing wraps the
  shelf.

---

## How the rest of the system follows

- **pCloud in Go.** A pCloud shelf reports `kind: virtual`,
  `download: true` and `offline.snapshot_at`, so the UI that reads them needs no
  pCloud branch. `PCloudBookshelfProvider` is no longer constructed for pCloud,
  and the screens listed in [Who reads them](#who-reads-them) read the new
  fields.
- **Removing the TS pCloud reader.** Done. `EmbeddedPCloudReader`
  reads through Go and still answers `filtersNsfwOnDevice` and `localNsfwMarks`
  for the device preference and offline filtering. A follow-up removed
  `supportsServerMode` and routed pCloud's `/api/mode` query to the embedded
  server.
- **Required illustration methods.** Done. `getSourceAsset` and
  `getSourceAssetsBundle` are required on `BookshelfReader`, so a backend that
  omits either fails the type check. `MobileBookshelfProvider` forwards a bundle
  request to its remote while online and rejects it offline, since the cache
  stores figures one by one; the reader's per-image path still reads them.
- **Offline download.** Done in Go: `capabilities.download`,
  `offline.snapshot_at` and the download API exist. Android's embedded virtual
  entries now use Go downloads directly; only remote-server entries use the wrapper.
- **Desktop and Android hosts.** LAN sharing and Android
  local shelves are implemented through host bindings. Exposing shelf
  management through `PlatformCapabilities.shelves`, with the generic names
  above, is implemented for Wails. Android keeps its mobile entry editor and Go
  plugin outside the desktop management form. `coversMustBeFetchedAsBlob` is
  retained as a platform boolean for all Android shelves, including local ones.

### Reading shell metadata

`listServerShelves()` preserves validated `kind`, `capabilities` and
`offline.snapshot_at` fields. The mobile shell enriches its device-local entry
with the engine response, retaining the device's entry ID and display name.
Embedded virtual entries route `/api/shelves` to Go as well as their shelf
routes. A disconnected wrapped entry still advertises its working downloads.
The selected shelf's capabilities are reactive, so switching shelves updates
refresh, character-count and fingerprint controls without rebuilding a provider.

Missing `rescan`, `char_count` or `fingerprint` members default to true.
Missing `download` defaults to false unless the offline wrapper overrides it.
Invalid snapshot dates are ignored. This keeps older servers compatible while
honoring explicit false values from newer servers.
