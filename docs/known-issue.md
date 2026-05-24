# Known Issues

## Shelf cache design limitations by use case

This document summarizes known limitations of the current shelf cache behavior, based on the current implementation in `shelf/shelf_cache.go`, `shelf/book.go`, `shelf/filestate.go`, and `shelf/shelf.go`.

---

### 1) Desktop (single-machine usage)

1. **New books may appear with delay (up to `scan_interval`)**
   - Within `scan_interval`, refresh may only reopen books already in cache, instead of doing a full library scan.
   - Newly added books from external file operations may not show up immediately.

2. **Staleness detection is based only on `book.json` file stat (`mtime` + `size`)**
   - Content changes can be missed if they happen to preserve tracked stat values.

3. **Cache refresh decisions are driven by `book.json`**
   - Staleness checks focus on `book.json`, so metadata-derived cached book state can remain stale when only cover/source files change; this should not be interpreted as cover/source file contents themselves being cached.

---

### 2) Personal Tailscale (single server on one host, multiple personal clients)

> Scope clarification: this scenario means one shelf server process running on one machine, with personal devices accessing that same server over Tailscale.

1. **No multi-server cache divergence in this mode**
   - Because there is only one server process, clients share one authoritative in-memory cache.

2. **Still has external file change visibility delay**
   - If the library folder is modified outside shelf (sync tool/manual operation), discovery is still bounded by refresh/full-scan behavior.

3. **Staleness precision limitation still applies**
   - `book.json` stat-based validation can still miss certain edits.

---

### 3) Sync file app workflow (Dropbox/Google Drive/Syncthing/iCloud-like)

1. **Transient partial-sync states can cause temporary read/refresh failures**
   - During in-progress sync (rename/copy/write not complete), reopening a stale entry may fail and be skipped temporarily.

2. **Timestamp-preserving sync behavior can reduce change detectability**
   - If sync preserves/normalizes metadata and tracked stat values do not differ, stale detection may miss content-level changes.

3. **New/deleted books may not be reflected immediately**
   - During scan throttling windows, refresh focuses on existing cache entries.

---

## Notes

- These are design trade-offs in the current cache strategy (scan throttling + per-book stale checks).
- For personal Tailscale with one server, the main concerns are usually external folder mutations and scan interval tuning, not distributed cache coherence.
