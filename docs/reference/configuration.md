# Configuration reference

Every key the PlainShelf server reads from its YAML config file, with its type,
its default, and what it changes.

The file is passed with `-conf`:

```bash
plainshelf-srv -conf config.yaml
```

`cmd/plainshelf-srv/conf/config.yaml` in the repository is the shipped example
and `docker/config.yaml` is the one baked into the container image. Neither
lists every key — the ones they leave out are exactly the ones this page is for.

!!! warning "Not covered by the format freeze"
    Configuration keys may still be renamed or removed between releases: the
    `1.0.0-rc1` freeze covers the shelf's on-disk format, not this file. Check
    `CHANGELOG.md` before upgrading.

The file has three top-level sections:

```yaml
logger:        # the process logger, used before the app is built
server_conf:   # the HTTP listener
app_conf:      # everything else
```

`server_conf` and `app_conf` are both required; the server refuses to start
without either. `logger` may be omitted, and then takes the defaults in
[Logger blocks](#logger-blocks).

Only the desktop and reader apps run without a config file. They build their
configuration in code, so nothing on this page applies to them; the settings
page is the only place their behavior is changed.

## `logger` (top level)

A [logger block](#logger-blocks) for the process logger. It carries startup and
shutdown lines — the ones written before and after the application itself
exists.

It sits outside `app_conf`, and two things follow from that. The **Logs** page
lists only the loggers under `app_conf`, so this one's files never appear there.
And the retention window saved in **Settings → Logs** is applied only to those
same loggers while the server runs, so this one keeps whatever `retention_days`
its own block names.

## `server_conf`

The HTTP listener. Both timeouts are required: an empty duration is a parse
error, not a default, and the server refuses to start.

| Key | Type | Default | What it does |
|---|---|---|---|
| `addr` | string | `:80`, on every interface | Listen address, `host:port`. Always set it, and keep it on loopback (`127.0.0.1:20000`) unless a real access boundary fronts the server — see the warning below. |
| `read_timeout` | duration | none (required) | Maximum time to read a request, including its body. Bounds how long a slow upload may hold a connection. |
| `write_timeout` | duration | none (required) | Maximum time to write a response. Raise it (e.g. `300s`) when large books are served over a slow mount, or transfers are cut off mid-response. |

Durations use Go syntax: `60s`, `5m`, `1h30m`.

`addr` is also read by the security check: it must be a loopback address unless
`app_conf.security.mode` is set explicitly. See
[Deployment and threat model](../deployment-and-threat-model.md).

!!! warning "An omitted `addr` is not a startup error"
    Leaving `addr` out does not fail. The Go HTTP server falls back to `:http`,
    so PlainShelf binds **port 80 on every interface** — the widest exposure
    there is. The loopback requirement above does not catch it either: an
    explicit `security.mode` satisfies that check whatever `addr` says, and an
    empty `addr` is not a loopback address, so the two together let the
    omission through. Set `addr` explicitly in every config file.

```yaml
server_conf:
  addr: "127.0.0.1:20000"
  read_timeout: 60s
  write_timeout: 60s
```

## `app_conf`

| Key | Type | Default | What it does |
|---|---|---|---|
| `logger` | [logger block](#logger-blocks) | see below | The application log: API requests, imports, background tasks. |
| `shelves` | list | empty | The shelves this server serves. See [`shelves[]`](#shelves). |
| `worker` | mapping | all defaults | Background task queue. See [`worker`](#app_confworker). |
| `store_path` | string | none (required) | Directory for the application store (Badger DB) holding server settings. Created if missing; an empty value fails startup. |
| `cover_to_jpg` | bool | `false` | Convert every cover PlainShelf stores to JPEG — both uploads and covers extracted from an EPUB — instead of keeping the original encoding. It also makes the upload endpoint accept image types it otherwise rejects. Overridden by a value saved from **Settings**. |
| `epub_import_strategy` | mapping | built-in default | Default EPUB conversion options. See [`epub_import_strategy`](#app_confepub_import_strategy). |
| `read_only` | bool | `false` | Serve every shelf read-only, whatever each shelf's own `read_only` says. Mutating API requests are refused with 403 before they reach a shelf; the one exception is the rescan, which only reads. |
| `security` | mapping | `local_token` defaults | The API access gate. See [`security`](#app_confsecurity). |

`store_path` holds server settings only — the cover-to-JPEG flag, the EPUB
import default, and the log retention window. It is not the book store: the
shelf on disk is. See [Backup and Restore](../backup-and-restore.md).

`cover_to_jpg` is one of the three settings that can be changed at runtime. A
value saved from the settings page wins over the config file; deleting it
reverts to the config file, and then to `false`.

### `app_conf.security`

Who may call the API. Read
[Deployment and threat model](../deployment-and-threat-model.md) before
changing anything here — the keys are individually simple and the combinations
are not.

| Key | Type | Default | What it does |
|---|---|---|---|
| `mode` | string | `local_token` | The access gate. See the table below. |
| `protect_read` | bool | `false` | Require the token for reads too, not only writes. The log API always requires it either way. |
| `token_header` | string | `X-PlainShelf-Token` | Header name the token is read from. `Authorization: Bearer <token>` is always accepted as well. |
| `allow_missing_origin_with_token` | bool | `true` in `local_token` mode | Accept a token-bearing request that carries no `Origin` or `Referer` — a non-browser client such as the Android app. Set `false` to require an allowed origin on every protected request. |
| `allowed_origins` | list of strings | the four loopback origins below | Browser origins allowed to make API calls. An origin is `scheme://host[:port]` with no path. |

The default `allowed_origins` in `local_token` mode:

```yaml
- "http://127.0.0.1:20000"
- "http://localhost:20000"
- "http://127.0.0.1:5173"
- "http://localhost:5173"
```

Listing any origin of your own **replaces** this list rather than adding to it,
so include the loopback entries you still need. Open the UI on a different host
port, a LAN address or a domain name and that exact origin has to be listed, or
the origin check rejects writes.

`mode` values:

| Value | Status | Effect |
|---|---|---|
| `local_token` | default | Mutating `/api` requests need the token; the server injects it into the served page so a local browser needs no setup. Reads are open unless `protect_read` is on. |
| `none` | supported | No token gate at all. Only for a deployment where a reverse proxy or VPN edge already authenticates. |
| `password` | **reserved, not implemented** | The server refuses to start: `security mode "password" is reserved but not implemented yet`. |
| `external` | **reserved, not implemented** | The server refuses to start, the same way. |

`password` and `external` are named in the code so the setting can grow into
them later. Setting either today is fail-closed, not a hole — but it fails at
startup, so do not discover it during an upgrade.

Leaving `mode` unset is only allowed when `server_conf.addr` is a loopback
address. On any other address the server refuses to start until the mode is
chosen explicitly.

### `app_conf.worker`

The queue behind long operations — imports, batch moves, emptying the trash,
fingerprinting. The whole section is optional.

| Key | Type | Default | What it does |
|---|---|---|---|
| `logger` | [logger block](#logger-blocks) | see below | Where background task logs are written. Often pointed at its own file. |
| `max_len` | int | `100` | How many task chains may wait in the queue. A submission beyond it is refused rather than queued. Zero or negative selects the default. |
| `max_keep` | int | `100` | How many finished task chains stay queryable through the task API, so the UI can still show a completed operation's result. Zero or negative selects the default. |

### `app_conf.logger`

A [logger block](#logger-blocks) for the application log — API requests,
imports, shelf operations.

The **Logs** page lists the files of every logger under `app_conf` — this one,
`app_conf.worker.logger` and each shelf's — and the retention window saved there
applies to all of them. Only the ones writing to `filename` or `filename_rotate`
have files to list.

### `shelves[]`

Each entry is one shelf. `id` and `lib_root` are the only required keys.

| Key | Type | Default | What it does |
|---|---|---|---|
| `id` | string | none (required) | Stable identifier, unique across the list. It appears in URLs, so keep it URI-safe. |
| `name` | string | same as `id` | Display name in the UI. |
| `lib_root` | string | none (required) | Path to the shelf directory. Created if missing, unless the shelf is read-only. Use a local mount path for a network share, not a `smb://` URL. |
| `read_only` | bool | `false` | Open this shelf without writing to it at all: no directory creation, no cache files, no lock file, and every mutating operation refused with 409. Forces `lock_mode: none` and disables the exported book cache. |
| `logger` | [logger block](#logger-blocks) | see below | Where this shelf's log is written. |
| `scan_interval` | duration | `1m` | Minimum time between full on-disk scans. Between scans a refresh only re-reads books already known, so new books appear at the next full scan. `0s` scans on every refresh. |
| `book_check_interval` | duration | same as `scan_interval` | How often per-book staleness checks run. Between checks, listing serves from memory with no filesystem I/O. |
| `lock_mode` | string | `flock` | Cross-instance locking. `flock` uses an OS lock on `app/library.lock`. `none` disables locking, for storage that cannot support `flock` (rclone mounts and similar) — then you must ensure only one PlainShelf instance touches the shelf. Any other value fails startup. |
| `lock_timeout` | duration | `30s` | How long to wait for the shelf lock before giving up. `0s` blocks indefinitely. Only used with `lock_mode: flock`. |
| `scan_cache` | bool | `true` | Let a scan skip listing folders whose modification time has not changed, replacing a directory listing with a single stat. Turn it off only on a mount whose directory times cannot be trusted — otherwise new books may never be discovered. |
| `book_cache_writer_id` | string | generated per installation | Names the exported book cache this installation owns, `app/book-cache-{id}.json`, so several machines can share a shelf without overwriting each other. Letters, digits, `-` and `_`, at most 64 characters. Set it to pin a predictable name. Ignored on a read-only shelf. |
| `book_cache_interval` | duration | `1h` | How quickly a change reaches the exported book cache. `0s` exports at every opportunity. Only used when the export is enabled. |

Leave `book_cache_writer_id` unset and the server supplies a stable random ID
for this installation. Explicitly setting it to `""` on a writable shelf is
therefore not the way to disable the export — the server fills it back in.

For what these tuning keys cost and buy on each kind of filesystem, see
[Shelf Cache and Disk I/O](../concepts/shelf-cache-and-io.md#tuning-options)
and [Configure an SMB shelf file source](../configuring-smb-shelf.md).

Scan settings held in the shelf's own `shelf.json` are a separate mechanism and
are not listed here; see [Data Model](../concepts/data-model.md).

### `app_conf.epub_import_strategy`

The default conversion options for an EPUB import that does not carry its own.
See [EPUB Import](../epub-import.md) for what each one produces.

| Key | Type | Default | What it does |
|---|---|---|---|
| `preset` | string | `markdown` | Output layout. `markdown` writes headings and keeps inline emphasis, storing the book as `md`. `plain` writes bare title lines and stores it as `txt`. Any other value is rejected and the built-in default is used. |
| `include_description` | bool | `true` when the section is absent | Also write the book description at the top of the text. It is always saved to the book's metadata regardless. |
| `keep_images` | bool | `true` | Store the EPUB's illustrations beside the text and link them. Ignored by the `plain` preset, whose output would show the link markup literally. |

`include_description` is the one key here whose default depends on the section
being present: with no `epub_import_strategy` block at all the built-in default
applies and the description is included, but inside a block you wrote, an
omitted `include_description` is `false`. Set it explicitly.

A strategy saved from **Settings → Import** takes precedence over this section.

## Logger blocks

`logger`, `app_conf.logger`, `app_conf.worker.logger` and each shelf's `logger`
all take the same shape. Each is an independent logger with its own destination.

| Key | Type | Default | What it does |
|---|---|---|---|
| `level` | string | `info` | `debug`, `info`, `warn` or `error`. Any other value fails startup. |
| `format` | string | `json` | `json` or `text`. Any other value fails startup. |
| `add_source` | bool | `false` | Include the Go source file and line in each entry. Useful when reporting a bug, noisy otherwise. |
| `log_file` | mapping | `stderr` | Where entries go; see below. |

### `log_file`

| Key | Type | Default | What it does |
|---|---|---|---|
| `type` | string | `stderr` | `stderr`, `stdout`, `none` (discard), `filename` (one file, appended forever) or `filename_rotate` (one file per day). Any other value fails startup. |
| `filename` | string | none | The file to append to. Used only by `type: filename`. |
| `dir` | string | none (required by `filename_rotate`) | Directory holding the rotated files, created if missing. Used only by `type: filename_rotate`, which needs it: leaving it empty fails startup, because there is no working directory the log files could safely default into. |
| `prefix` | string | `log` | Name prefix of the rotated files, which are `{prefix}-YYYY-MM-DD.log`. Used only by `type: filename_rotate`. |
| `retention_days` | int | `30` | Delete rotated files older than this many days. `0` keeps every file forever; a negative value fails startup. Used only by `type: filename_rotate`. |

Only `filename` and `filename_rotate` produce files the **Logs** page can list
and read; `stderr`, `stdout` and `none` leave it with nothing to show.

!!! warning "`filename_rotate` without `dir` fails startup"
    The rotating writer creates its directory before opening the day's file, and
    creating the empty path fails — silently, because a logger has nowhere to
    report that it cannot write. So the server refuses to start instead, with
    `missing log dir: filename_rotate requires dir`. Both shipped example
    configs set `dir`; if you write your own, set it too.

Deletion happens at rotation — the first log line written on a new day — so an
idle or stopped server deletes nothing. Nothing else ever removes these files.
A retention window saved from **Settings → Logs** overrides `retention_days`
for the application, worker and shelf loggers while the server runs; see
[Logs](../logs.md#retention).

## A complete example

Every key on this page in one file, at the value it takes by default. Two
exceptions, both because the default is not what you want written down:
`addr`, whose default binds port 80 on every interface, and the log
destinations, whose default `stderr` leaves the **Logs** page nothing to list.
Both follow the shipped example instead. Copy the parts you need rather than
running this as it stands — the paths are relative to the working directory.

```yaml
logger:
  level: info
  format: json
  add_source: false
  log_file:
    type: stderr

server_conf:
  addr: "127.0.0.1:20000"
  read_timeout: 60s
  write_timeout: 60s

app_conf:
  logger:
    level: info
    format: json
    log_file:
      type: filename_rotate
      dir: ./logs
      prefix: root
      retention_days: 30

  worker:
    max_len: 100
    max_keep: 100
    logger:
      level: info
      format: json
      log_file:
        type: filename_rotate
        dir: ./logs
        prefix: worker

  shelves:
    - id: default_shelf
      name: Default Shelf
      lib_root: "./shelf"
      read_only: false
      scan_interval: 1m
      book_check_interval: 1m
      lock_mode: flock
      lock_timeout: 30s
      scan_cache: true
      # book_cache_writer_id: my-laptop
      book_cache_interval: 1h
      logger:
        level: info
        format: json
        log_file:
          type: filename_rotate
          dir: ./logs
          prefix: shelf

  store_path: "./store"
  cover_to_jpg: false
  read_only: false

  epub_import_strategy:
    preset: markdown
    include_description: true
    keep_images: true

  security:
    mode: "local_token"
    protect_read: false
    token_header: "X-PlainShelf-Token"
    allow_missing_origin_with_token: true
    allowed_origins:
      - "http://127.0.0.1:20000"
      - "http://localhost:20000"
      - "http://127.0.0.1:5173"
      - "http://localhost:5173"
```

## Settings that are not in this file

Three settings are stored in `store_path` and changed from the UI, where they
override the config file: cover-to-JPEG conversion, the EPUB import default,
and the log retention window. Reading progress, history and reading time live
on each client, not on the server.
