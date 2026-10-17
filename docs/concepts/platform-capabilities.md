# Shelf and Platform Capabilities

!!! note "Design, partly implemented"
    This page is the agreed design for [PSW-141](https://patchouli.atlassian.net/browse/PSW-141).
    The [shelf capability fields](#shelf-capabilities) are served since
    PSW-145; the platform interface is not implemented yet.
    It fixes the capability fields the server reports before `1.0.0-rc1`,
    because they become part of the API's compatibility promise then. It
    builds on [Shelf Sources and Credentials](shelf-sources.md) (PSW-140).

The frontend reaches its data through one `BookshelfProvider`. Today that
interface also carries every client's special cases as optional methods: the
desktop app's "open in Finder", the Android app's downloads, the TypeScript
pCloud reader's "I have no server". A screen that wants to know whether a
feature exists asks the provider with a `supportsXxx()` call, so every new
source grows another one.

This design splits those methods by **who knows the answer**:

- **What a shelf can do** is known by the engine that opened it. The server
  reports it as fields on each shelf, the same on every client.
- **What the device can do** is known by the host the frontend runs in (Wails,
  Capacitor, the wasm page, a browser). The host provides it as a separate
  platform interface, never over the HTTP API.

Once pCloud moves into Go (PSW-138, PSW-139), every client reads every shelf
through the same Go API, and the provider is left with one job: reading and
writing a shelf.

---

## Where each optional method goes

Every optional method on `BookshelfProvider` today, in
`frontend/src/providers/bookshelfProvider.ts`:

| Method | Implemented by | Goes to |
|---|---|---|
| `getSourceAsset` | server, mobile, pCloud | **Required** on the shelf interface |
| `getSourceAssetsBundle` | server, pCloud | **Required** on the shelf interface |
| `supportsShelfRefresh` | server, mobile, pCloud | Shelf capability `rescan` |
| `refreshShelf` | server, mobile, pCloud | Shelf interface, gated by `rescan` |
| `supportsCharCountListing` | mobile, pCloud | Shelf capability `char_count` |
| `getShelfFetchedAt` | mobile, pCloud | Shelf field `offline.snapshot_at` |
| `downloadBook`, `removeDownload`, `getDownloadState`, `listDownloadedBookEntries` | mobile | Shelf interface, backed by the Go API (PSW-145), gated by `download` |
| `getPersistedShelfID` | mobile | Remote-server connection only (see below) |
| `supportsServerMode` | mobile, pCloud | **Deleted** after PSW-139 |
| `filtersNsfwOnDevice`, `localNsfwMarks` | mobile, pCloud | **Deleted** after PSW-139: the engine applies NSFW marks |
| `coversMustBeFetchedAsBlob` | mobile, pCloud | Platform capability, until PSW-136 shows whether it is still needed |
| `getStorageEstimate` | mobile | Platform interface |
| `openLocalBookFiles`, `importBookFromLocalPath` | desktop | Platform interface |
| `openDesktopFolder`, `openDesktopBookFolder`, `openDesktopShelfFolder`, `openDesktopShelfDirectory` | desktop | Platform interface, renamed without `Desktop` |
| `openDesktopReader` | desktop | Platform interface |
| `saveBookContentToFile` | desktop, mobile | Platform interface |
| `previewDesktopShelfID`, `addDesktopShelf`, `getDesktopShelfDetails`, `modifyDesktopShelf`, `removeDesktopShelf` | desktop | Platform interface as `previewShelfID`, `addShelf`, `getShelfDetails`, `modifyShelf`, `removeShelf` |
| *(new)* `openExternalUrl` | — | Platform interface: opens the system browser for pCloud authorization |

No method is left unassigned. "Mobile" here is `MobileBookshelfProvider`, the
wrapper that adds offline downloads to a remote PlainShelf server connection.

---

## Shelf capabilities

`GET /api/shelves` returns `id`, `name` and `read_only` today. It gains:

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

| Today | After |
|---|---|
| `composables/useShelfRefresh.ts` asks `supportsShelfRefresh` and `getShelfFetchedAt` | reads `capabilities.rescan` and `offline.snapshot_at` |
| `features/library/pages/LibraryPage.vue`, `features/maintenance/pages/SimilarContentPage.vue` and `utils/bookFilters/registry.ts` ask `supportsCharCountListing` | read `capabilities.char_count` |
| `features/maintenance/pages/SimilarContentPage.vue` calls `getFingerprintStatus` and handles an empty answer | checks `capabilities.fingerprint` first |
| `features/mobile/pages/DownloadsPage.vue` and the book actions use the mobile download methods | check `capabilities.download`, then call the shelf interface |
| `composables/useServerMode.ts` asks `supportsServerMode` | asks the server, which every shelf now has |

The fields live in `server/contract/shelves`, beside `ShelfInfo`
(`server/handle_shelf.go`). The source configuration itself is never part of the
response, as [the source design](shelf-sources.md#rules) requires.

---

## The platform interface

```ts
interface PlatformCapabilities {
  // Shelf management: present only where the host can manage shelves.
  shelves?: {
    previewShelfID(name: string): Promise<ShelfIDPreview>;
    addShelf(params: AddShelfParams): Promise<void>;
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

The host builds this object when the app starts and the frontend reads it
through one accessor, beside `getBookshelfProvider()`. A missing member means the
platform cannot do it, which is the same rule the optional methods follow today,
in one place instead of spread over the provider.

| Host | `shelves` | Files and folders | `openExternalUrl` |
|---|---|---|---|
| Server (browser) | absent: shelves come only from the YAML file | absent | browser `window.open` |
| Desktop (Wails) | Wails bindings | Wails bindings | Wails `BrowserOpenURL` |
| Android (Capacitor) | Capacitor plugin into Go | Capacitor plugins | Capacitor `Browser` |
| wasm demo | host routes around the engine, once the demo gets a shelf list | absent | `window.open` |

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

That wrapper is applied **only** to a remote-server connection. A local shelf and
a virtual shelf opened by the embedded Go engine get their offline behaviour
from the engine (PSW-145) and are not wrapped.

Because those downloads are the wrapper's own, the wrapper reports
`capabilities.download: true` on every shelf it wraps, whatever the remote
server says. A remote server reports `download: false` for its local shelves,
and an older server reports nothing, so the UI would otherwise hide downloads
that work today. The rule is that the layer that owns the offline copy answers
`download`: the engine for shelves it opens, the wrapper for a remote
connection. `getPersistedShelfID` stays with
the wrapper, which needs it to scope the remote connection's downloads.

Once virtual shelves cover reading another PlainShelf (PSW-146, PSW-149), the
remote-server connection and its wrapper can be retired; that is a separate
decision and not part of this design.

---

## What stays the same

- `WailsBookshelfProvider` keeps extending `ServerBookshelfProvider`: the
  desktop is already the target shape. Its desktop-only methods move to the
  platform object; the shelf methods do not change.
- `read_only` on `GET /api/shelves` keeps its meaning and its place.
- An older client against a newer server ignores `kind`, `capabilities` and
  `offline`. A newer client against an older server sees them missing and falls
  back to today's behaviour: `rescan` true, the rest true, and `download` as
  the remote-server wrapper reports it (true) or false where nothing wraps the
  shelf.

---

## What this changes in the follow-up tickets

- **PSW-138 (pCloud in Go).** A pCloud shelf reports `kind: virtual`,
  `download: true` and `offline.snapshot_at`, so the UI that reads them needs no
  pCloud branch. `PCloudBookshelfProvider` is no longer constructed for pCloud,
  and the screens listed in [Who reads them](#who-reads-them) move to the new
  fields in the same change.
- **PSW-139 (remove the TS pCloud reader).** Done for the reader itself. The
  provider interface is unchanged: `filtersNsfwOnDevice` and `localNsfwMarks`
  are still what `EmbeddedPCloudReader` answers, and no implementation returns
  `false` from `supportsServerMode` any more, so its removal, with making
  `getSourceAsset` and `getSourceAssetsBundle` required, is left to its own
  change.
- **PSW-145 (offline download).** Implements `capabilities.download` and
  `offline.snapshot_at`, and the shelf-interface download methods call its API.
- **PSW-147, PSW-136.** The desktop and Android hosts expose shelf management
  through `PlatformCapabilities.shelves`, with the generic names above.
