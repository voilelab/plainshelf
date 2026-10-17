# Shelf Sources and Credentials

!!! note "Design, partly implemented"
    This page is the agreed design for [PSW-140](https://patchouli.atlassian.net/browse/PSW-140).
    The engine can open a virtual shelf (PSW-137) and keep it readable
    offline ([Offline copies](#offline-copies), PSW-145). The server reads the
    `source` block and resolves `env:` and `file:` credentials, and opens
    `plainshelf` and `webdav` sources (PSW-149); the desktop and Android hosts
    do not take a `source` entry or resolve `host:` yet. It fixes the
    configuration format and the credential model before `1.0.0-rc1`, because
    both become compatibility promises at that point. The implementation lands
    through the tickets listed in [What this changes in the follow-up
    tickets](#what-this-changes-in-the-follow-up-tickets).

A shelf is opened from a **source**: where its files are and how PlainShelf
reaches them. Today every shelf is a local directory named by `lib_root`, and
the Android client reads pCloud through the engine embedded in the app. This page
defines one source format that the server, the desktop app, the Android app and
the wasm demo all share, and how a source that needs a secret gets it.

---

## Two kinds of shelf

| | Local shelf | Virtual shelf |
|---|---|---|
| Source | A directory the engine can open as a path: a server or desktop path, the Android app's private directory, the wasm demo's OPFS tree | A remote file tree: pCloud, another PlainShelf's [read-only export](#open-decisions), any WebDAV server |
| Writes | Read-write, or `read_only: true` | Always read-only |
| Opened by | `os.OpenRoot`, atomic writes, the shelf lock | A remote read-only `fsutil.ReadFS` that reads `app/book-cache-*.json` instead of walking every book |
| Offline | Already local | Books can be downloaded to a device-local offline layer outside the shelf |
| Lock | `flock` on the server and desktop; `none` where only one process can reach the files | None: nothing is written |

Both kinds are implemented once, in the Go engine, so a shelf behaves the same
on every client. Clients differ only in which sources their platform can reach
(see [Availability by client](#availability-by-client)).

A virtual shelf is a storage backend, not synchronisation: PlainShelf never
writes to its source.

### Offline copies

A virtual shelf opened with an offline directory keeps two things on the device,
outside the shelf, so it stays readable while its source is unreachable:

- **the last listing**, saved whenever a listing read from the source differs
  from the saved one, together with a copy of the source's `shelf.json` so
  folder rules such as the [adult content mark](folders.md#marking-a-folder-as-adult-content)
  still apply. A source no installation exports a book cache for opens from it
  rather than being walked at every start; a rescan walks it. A shelf that has
  no snapshot yet and cannot reach its source when it opens answers that the
  source is unavailable, and tries again on later requests and on a rescan;
- **downloaded books**, copied whole (every source, the cover, illustrations) to
  the same `books/…` path they have on the source. A download is built in a
  temporary directory and renamed into place, so an interrupted one leaves
  nothing behind.

Reading a downloaded book never asks the source, online or not. While the
source is unreachable, a book without a copy answers `BOOK_NOT_DOWNLOADED`
instead of failing.

A copy records the book's version when it was taken: its `book.json` plus the
current source's `current_source_md5` from the
[exported book cache](shelf-cache-and-io.md#the-exported-book-cache). When the
listing later shows a different version, the book reports `update_available`;
the copy is replaced only when the user downloads it again.

| Route | Effect |
|---|---|
| `PUT /api/shelves/{shelf_id}/books/{book_id}/download` | Copy one book, replacing an earlier copy. Synchronous. |
| `DELETE /api/shelves/{shelf_id}/books/{book_id}/download` | Delete the copy only. |
| `GET /api/shelves/{shelf_id}/downloads` | Every copy with its size, and the total. |

Book responses on such a shelf carry `download_state` (`not_downloaded`,
`downloaded`, `update_available`) and `downloaded_at`. A download writes only
the offline directory, so a read-only server allows it, but it still needs the
`local_token` like any other write.

The server keeps offline directories under `<store_path>/offline/`, one per
cache scope (see [Compatibility](#compatibility)), so renaming a shelf keeps its downloads.

---

## The `source` block

A shelf entry gains an optional `source` block. `lib_root` stays, as shorthand
for a local source.

```yaml
app_conf:
  shelves:
    # Unchanged: lib_root alone is a local source.
    - id: default_shelf
      name: Default Shelf
      lib_root: ./shelf

    # The same thing, spelled out.
    - id: archive
      source:
        type: local
        path: /srv/books/archive

    # Another PlainShelf's read-only export.
    - id: living_room
      name: Living room
      source:
        type: plainshelf
        url: http://192.168.1.20:20001/dav/default_shelf/
        credential: file:/etc/plainshelf/living_room.token

    # Any WebDAV server holding a shelf directory.
    - id: nas
      source:
        type: webdav
        url: https://nas.example/dav/books/
        username: reader
        credential: env:PLAINSHELF_NAS_PASSWORD

    # pCloud (see "Open decisions": refused on the server and desktop for now).
    - id: cloud
      source:
        type: pcloud
        host: eapi.pcloud.com
        user_id: "12345678"
        path: /PlainShelf/default-shelf
        credential: env:PLAINSHELF_PCLOUD_TOKEN
```

### Fields

| Key | Types | Meaning |
|---|---|---|
| `type` | all | `local`, `plainshelf`, `webdav` or `pcloud`. Any other value fails startup. |
| `path` | `local`, `pcloud` | The shelf directory: a local path, or a folder path inside the pCloud account. |
| `url` | `plainshelf`, `webdav` | The URL of the shelf directory. |
| `host` | `pcloud` | The regional API host, learned during authorization. |
| `user_id` | `pcloud` | The pCloud account ID. Optional, because entries saved before it existed keep working without it. |
| `client_id` | `pcloud` | The user's own pCloud app key, needed only to authorize again. Optional. |
| `username` | `webdav` | The account to sign in as. Set, the credential is its password and is sent as HTTP Basic; unset, the credential is sent as a bearer token. Optional. |
| `credential` | `plainshelf`, `webdav`, `pcloud` | A [credential reference](#credentials), never the secret itself. |

A field that does not belong to the declared `type` fails startup instead of
being ignored, so a typo cannot silently open the wrong thing.

### Rules

- **`lib_root` and `source` are mutually exclusive.** An entry with both fails
  startup. An entry with neither fails startup, as an entry without `lib_root`
  does today.
- **A virtual source is read-only by definition.** It opens read-only whatever
  `read_only` says: the engine has no write path to a remote source, so the key
  has nothing to switch. `lock_mode` is forced to `none` and
  `book_cache_writer_id` is ignored, exactly as for a read-only local shelf
  today.
- **The tuning keys still apply.** `scan_interval`, `book_check_interval` and
  `scan_cache` keep their meaning; a virtual source makes them more important,
  since every listing is a network request.
- **The source never reaches the HTTP API.** `GET /api/shelves` returns `id`,
  `name` and `read_only` today and does not return `lib_root`; it will not
  return `source` either. What the UI needs to know about a shelf (local or
  virtual, whether it can download) is reported as capability fields, designed
  in [PSW-141](https://patchouli.atlassian.net/browse/PSW-141).

### In Go

```go
type ShelfConf struct {
	// ...existing fields unchanged...
	LibRoot string      `yaml:"lib_root" json:"lib_root,omitzero"`
	Source  *SourceConf `yaml:"source" json:"source,omitzero"`
}

type SourceConf struct {
	Type       string `yaml:"type" json:"type"`
	Path       string `yaml:"path" json:"path,omitzero"`
	URL        string `yaml:"url" json:"url,omitzero"`
	Host       string `yaml:"host" json:"host,omitzero"`
	UserID     string `yaml:"user_id" json:"user_id,omitzero"`
	ClientID   string `yaml:"client_id" json:"client_id,omitzero"`
	Username   string `yaml:"username" json:"username,omitzero"`
	Credential string `yaml:"credential" json:"credential,omitzero"`
}
```

One flat struct rather than a block per type: it decodes the same way from YAML
and JSON, and per-type validation rejects the fields a type does not use.
Resolving the configuration turns `lib_root` into `Source{Type: "local", Path: …}`,
so code past that point reads only `Source`.

`ShelfConfWithID` gains `json:"id"` and `json:"name"` tags beside its YAML ones.
The desktop and Android hosts pass shelf entries to Go as JSON, encoded through
`internal/jsonopt` as everything on disk is
([JSON encoding](../development/json-encoding.md)).

---

## Credentials

### A reference, never the secret

`credential` holds a reference of the form `<scheme>:<value>`:

| Scheme | Resolved by | Where the secret lives |
|---|---|---|
| `env:NAME` | the engine | the environment variable `NAME` (server, Docker) |
| `file:/path` | the engine | the file's contents with surrounding whitespace trimmed (server, Docker secrets) |
| `host:ID` | the host the engine runs in | the platform's secret store, under `ID` |

A value with no recognised scheme fails startup. That includes a bare token
pasted into the field: a configuration file is something people copy into bug
reports and back up, so it never carries the secret itself.

### Where each client keeps secrets

| Client | Scheme | Secret store |
|---|---|---|
| Server | `env:`, `file:` | the environment, or a file readable only by the server's user |
| Desktop | `host:` | the operating system keychain |
| Android | `host:` | Android Keystore, through the existing `SecureStorage` plugin |
| wasm demo | `host:` | browser storage, through a JavaScript callback |

On Android the `ID` of a `host:` reference is the shelf entry ID, so the
Keystore key stays `plainshelf.mobile.secret.<entryID>` and no stored token
moves.

### The Go interface

```go
// CredentialResolver turns a credential reference into the secret it names.
type CredentialResolver interface {
	Resolve(ctx context.Context, ref string) (string, error)
}
```

`server.AppConf` gains a resolver field that YAML never sets (`yaml:"-"`). The
engine's default resolver handles `env:` and `file:`, and refuses `host:`. A host
that keeps secrets (desktop, Android, wasm) passes a resolver that handles
`host:` itself and hands everything else to the default.

The secret is resolved when a source opens and again whenever the source client
reports an authentication failure, so a rotated token is picked up without
restarting. Each client maps its remote's own signal to that failure: HTTP 401
for WebDAV, and the authentication error codes pCloud returns inside an HTTP 200
response. It is held only by the
source client: it is never written to a log, to the settings store, or into any
API response.

### pCloud authorization

pCloud's `poll_token` flow needs no redirect URI and no app secret. Its only
platform-specific step is opening the authorization URL in the system browser,
which the host provides as a platform capability (PSW-141). Generating the
`request_id`, polling both regional hosts and recording `host` and `user_id` run
in Go; the resulting token is written to the platform's secret store through the
host, and the shelf entry stores only `host:<entryID>`.

### The `local_token` boundary does not move

Credentials never travel over the HTTP API, in either direction. They are read
from the environment or a file on the server, and set through the host
interface (a Wails binding, a Capacitor plugin call) everywhere else. No route
is added to `server.App`'s handler to receive or return one.

---

## Who holds the shelf list

| Client | Shelf list | Who can change it |
|---|---|---|
| Server | `app_conf.shelves` in the YAML file | The operator, by editing the file and restarting. The web UI lists shelves read-only. |
| Desktop | `shelves.json` in the data directory | The settings page, through Wails bindings |
| Android | The app's own settings (Capacitor Preferences) | The app, through a Capacitor plugin call into Go |
| wasm demo | Fixed in `cmd/plainshelf-wasm/main.go` | Nobody, for now |

Managing shelves is a host capability and never an HTTP route: `server.App`'s
handler has no route that adds, changes or removes a shelf, on any client. A
host may add one around that handler only when it has no listener reachable
from the network, as the wasm demo does today for `/_demo/shelf.zip`.

The source configuration and its credential references are never written into
the shelf directory. `shelf.json` inside a shelf holds scan settings for the
shelf itself and does not gain source fields; the shelf's on-disk format does
not change.

---

## Compatibility

**Server.** A configuration that only uses `lib_root` starts unchanged. An older
build given a `source` block ignores the unknown key, finds no `lib_root`, and
fails that shelf at startup instead of opening something else.

**Desktop.** Existing `shelves.json` entries carry `lib_root` and load
unchanged. `desktopShelfEntry` gains `source`, and the desktop keeps writing
`lib_root` (not `source`) for a local shelf, so a downgraded desktop still reads
every local shelf it had.

**Android.** A pCloud entry maps field for field:

| Current `PCloudShelfEntry` | `source` |
|---|---|
| `pcloudHost` | `host` |
| `pcloudShelfRoot` | `path` |
| `pcloudUserId` (optional) | `user_id` (optional) |
| `pcloudClientId` | `client_id` |
| Keystore `plainshelf.mobile.secret.<id>` | `credential: host:<id>` |

The entry list stays in Preferences, owned by the app. A `server` entry, which
connects to another PlainShelf's full API, is client-side connection state and
does not become a `ShelfConf`.

**Cache scope.** Downloaded books are keyed by where the shelf lives, not by the
entry's name (`frontend/src/providers/cacheScope.ts`). The Go offline layer for
virtual shelves derives the same identity from the same fields, so everything a
shelf keeps on a device, on either side, is filed under one identity that a
rename does not change:

| Type | Scope identity |
|---|---|
| `pcloud` | `pcloud://{host}` plus `/{user_id}` when present, and `path` |
| `plainshelf` | the normalized `url` |
| `webdav` | the normalized `url`, plus `username` when present |
| `local` | none: a local shelf has no offline layer |

---

## Availability by client

| Source | Server | Desktop | Android | wasm demo |
|---|---|---|---|---|
| `local` | yes | yes | yes, in the app's private directory | yes, in OPFS |
| `plainshelf`, `webdav` | yes | yes | yes | only if the remote sends CORS headers |
| `pcloud` | refused for now | refused for now | yes | no |

A source a client cannot open fails that shelf at startup with an error naming
the type, never by falling back to another source.

---

## Open decisions

- **pCloud on the server and desktop.** The format can express it, but
  `CLAUDE.md` currently limits pCloud to the Android client. Until that product
  decision changes, the server and desktop refuse `type: pcloud` at startup. No
  format change is needed to lift it.
- **PlainShelf as a source.** Settled: another PlainShelf is reached through its
  read-only WebDAV export, and `plainshelf` is the same client as `webdav` with
  the export's token convention. The Android app's existing full-API connection
  stays until virtual shelves replace it.

---

## What this changes in the follow-up tickets

- **PSW-136 (Android local shelf).** The app's local shelf is a `local` source
  under the app's private directory, with `lock_mode: none`. Shelf entries reach
  Go as JSON `ShelfConfWithID` through the plugin; the engine opens no listener
  for the UI and has no shelf-management route.
- **PSW-137 (remote read-only `ReadFS`).** Done as `shelf.NewShelfFromFS`, which
  opens any read-only `ReadFS` whatever `read_only` says. Building that
  `ReadFS` from `SourceConf` moved to PSW-149, the first real remote source.
- **PSW-138 (pCloud in Go).** The pCloud client reads `host`, `user_id`, `path`
  and resolves `credential` through the host resolver; the Keystore key does not
  change. The offline layer's scope identity follows the table above. Done:
  `internal/pcloud` is a
  read-only `ReadFS` over the pCloud API, and the app opens its pCloud entry
  with `mobile.OpenPCloudShelf`. Until the resolver exists the native side reads
  the token from the Keystore and passes it on every start; Go holds it in
  memory only. The entry keeps its cache scope, and its downloads, reading
  progress and history stay in the app's own cache and are not moved into the
  Go offline layer: downloads the earlier TypeScript reader took stay where
  they are. Adult content follows the device's own
  choice, given to Go per shelf (`App.SetShelfShowNSFW`) so it does not become
  the setting local shelves use.
- **PSW-139 (remove the TypeScript reader).** Done: the frontend parses no
  shelf format. The app's pCloud setup asks Go whether a folder is a shelf and
  how many books it holds (`mobile.CheckPCloudShelf`, `mobile.ListPCloudFolder`);
  only authorization and the picker's path walk still call pCloud from
  TypeScript.
- **PSW-145 (offline copies).** Done as described in
  [Offline copies](#offline-copies). The engine takes the offline directory as
  a parameter; on the server the scope that names it comes from
  `SourceConf.Scope` (PSW-149).
- **PSW-146, PSW-149.** The server-mode export is the top-level
  [`dav_export`](../reference/configuration.md#dav_export) section; its token
  file is what a `plainshelf` source's `credential: file:…` points at on the
  other machine. Done on the server: `internal/davfs` is a read-only `ReadFS`
  that lists with `PROPFIND` at Depth 1, stats at Depth 0 and reads with `GET`
  (a seek becomes a `Range` request), retrying only transient failures. The
  server builds it from `SourceConf`, resolves the credential through
  `AppConf.Credentials`, and files offline copies under the scope above. A 401
  answers `SOURCE_UNAUTHORIZED` and an unreachable remote `SOURCE_UNAVAILABLE`.
  None of them adds configuration keys of its own for the same purpose.
