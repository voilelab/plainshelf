# Shelf Cache and Disk I/O

PlainShelf is filesystem-first: the shelf directory on disk is the source of truth, and the server keeps an in-memory cache so common browsing operations do not need to reopen every metadata file on every request.

This page explains what PlainShelf reads during shelf startup and browsing, why large shelves or network-mounted shelves can feel slower, and how to tune the cache intervals.

---

## Initial cache initialization

When a shelf is opened, PlainShelf initializes the book cache in the background. The initial cache build performs a full scan of the shelf's `books/` tree.

During that first scan, PlainShelf:

1. walks directories under `books/`;
2. detects book package folders ending in `.bookpkg`;
3. opens each book's `book.json` metadata file;
4. decodes the metadata into memory;
5. records the metadata file's modified time and size for later cache validation.

So yes: the first successful cache initialization reads every book metadata file once. It does **not** scan the whole operating system disk; it scans the configured shelf root, specifically the shelf book tree.

---

## What gets read per book

For each discovered book package, the important disk operations are roughly:

- a directory/path stat while walking the tree;
- an open/read/decode of `{book}.bookpkg/book.json`;
- a stat of `book.json` to remember its `mtime` and size.

The amount of data read is usually small because `book.json` is a metadata file. The cost is mostly from many small filesystem operations, not from raw throughput.

For example, a shelf with 10,000 books may only read a modest amount of metadata bytes, but it still needs thousands of directory, open, and stat operations. That is usually fine on a local SSD, but it can be noticeable on HDDs, SMB/NAS mounts, or other high-latency filesystems.

---

## Subsequent book listing behavior

After the initial cache is ready, book list operations normally return from the in-memory cache.

PlainShelf still needs to keep the cache reasonably fresh. It does this with two levels of refresh:

1. **Full scan**
   - Walks the `books/` tree again.
   - Discovers new, moved, or removed book folders.
   - Reopens book metadata for discovered books.
   - Controlled mainly by `scan_interval`.

2. **Per-book freshness check**
   - Checks known cached books without walking the whole tree.
   - Stats each cached book's `book.json` and compares the current `mtime` and size with the cached values.
   - Reopens a book only when its tracked metadata file stat changed.
   - Controlled mainly by `book_check_interval`.

Within the configured intervals, repeated browsing can be served from memory with little or no filesystem I/O.

---

## Tuning options

### `scan_interval`

`scan_interval` controls how often PlainShelf performs a full on-disk scan of the shelf book tree.

A shorter interval discovers externally added, moved, or deleted books sooner, but performs more directory traversal and metadata reads. A longer interval reduces filesystem and network I/O, but external changes may appear later.

Example:

```yaml
app_conf:
  shelves:
    - id: default_shelf
      name: Default Shelf
      lib_root: /path/to/shelf
      scan_interval: 10m
```

### `book_check_interval`

`book_check_interval` controls how often PlainShelf checks cached books for metadata changes.

A shorter interval notices external edits to existing `book.json` files sooner. A longer interval reduces repeated per-book stat operations, which can matter on high-latency filesystems.

Example:

```yaml
app_conf:
  shelves:
    - id: default_shelf
      name: Default Shelf
      lib_root: /path/to/shelf
      scan_interval: 10m
      book_check_interval: 5m
```

If `book_check_interval` is omitted, it defaults to the same duration as `scan_interval`.

---

## Storage impact by filesystem type

### Local SSD

The disk-wear impact of these reads is usually negligible. SSD lifetime is primarily affected by write volume, while shelf cache initialization mostly reads small metadata files.

### Local HDD

The main cost is seek latency from many small directory and metadata operations. This can make the first scan slower on very large shelves, but normal interval-based scanning should not be a major wear concern.

### SMB/NAS or other network mounts

Network filesystems are the most sensitive case. The expensive part is often the number of round trips for stat, directory, and small-file reads rather than the number of bytes transferred.

For SMB/NAS shelves, prefer longer intervals such as:

```yaml
app_conf:
  shelves:
    - id: default_shelf
      name: Default Shelf
      lib_root: /mnt/plainshelf/default-shelf
      scan_interval: 10m
      book_check_interval: 5m
      lock_timeout: 30s
```

If browsing is still slow, increase `book_check_interval` first. If external additions or moves do not need to appear immediately, increase `scan_interval` too.

---

## Practical guidance

- For small or medium local shelves, the defaults are usually fine.
- For large local shelves, consider increasing `scan_interval` if startup or browsing causes noticeable disk activity.
- For SMB/NAS shelves, start with `scan_interval: 10m` and `book_check_interval: 5m`, then tune upward if browsing is still slow.
- If you edit shelf files outside PlainShelf, remember that longer intervals trade immediate visibility for lower I/O.
