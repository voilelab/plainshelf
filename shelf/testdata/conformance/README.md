# bookpkg format conformance dataset

The Go shelf in `shelf/` is the only reader of the on-disk book package format:
every server and desktop build, and the Android app, which reads a pCloud shelf
through the Go shelf embedded in it. The app once carried a second,
TypeScript reader, and this dataset was the fixture that kept the two in step.

It now pins the Go reading of each case to a file written by hand from the
format documentation, so a change in what the shelf reads shows up as a diff
here rather than only in behaviour. The harness is `shelf/conformance_test.go`.

The Go harness reads every case three times: through `NewShelf`, which walks
the shelf; through `NewShelfFromFS`, the remote entry, which lists a shelf from
its newest usable `app/book-cache-*.json` and walks only without one; and
through `NewShelfFromFS` over `internal/pcloud`, the Go pCloud client, against a
fake pCloud API serving the case directory. All must produce the same
`expected.json`, so the remote and pCloud paths cannot drift from the local walk.

The asset fixtures also check that illustration bundles contain the same flat
entry names as individual asset reads, including the pCloud batch zip path.
No additional on-disk metadata is needed for batching.

## Layout

```
conformance/
├─ manifest.json          versioned index of the cases
├─ README.md
└─ cases/<case>/
   ├─ shelf/              a real shelf root: books/, plus app/ and shelf.json
   │                      where relevant
   └─ expected.json       the reading the shelf must produce
```

Every case directory must be listed in `manifest.json` and every listed case
must exist; the harness fails otherwise, so a case cannot be added and then
silently skipped. `schema_version` in the manifest describes the shape of
`expected.json`; bump it when that shape changes and update the harness in the
same commit.

Git cannot commit an empty directory, so a directory that must exist without
holding a book carries a placeholder file. The shelf walks directories only,
which is what makes such a file invisible to it.

## The expected reading

`expected.json` is not a dump of the implementation. It records what a reader
of the shelf observes:

| Field | Go |
|---|---|
| `folders` | `collectExportFolders` |
| `folder_meta` | `Shelf.ReadFolderMeta` |
| `books[].path` | `Book.FolderPath` |
| `books[].folders` | `BookListing.Folders` |
| `books[].id`, `title`, `format`, `authors`, `tags`, `identifiers`, `language`, `comments`, `star`, `created_at`, `updated_at`, `published_at`, `cover` | `Book.GetMeta` |
| `books[].cover_present` | `Book.OpenCover` |
| `books[].schema_version_on_disk` | `readBookMeta` |
| `books[].read_only` | `Book.EnsureWritable` |
| `books[].nsfw` | `Shelf.IsBookNSFW` |
| `books[].current_source_field` | `Book.CurrentSource` |
| `books[].current_source` | `Book.ResolveCurrentSource` |
| `books[].sources[]` | `Book.ListSource` |
| `books[].sources[].schema_version`, `created_at`, `comment`, `format`, `md5_hash`, `line_count`, `char_count` | `Source.GetMeta` |
| `books[].sources[].assets` | `Source.AssetPath` / `OpenAsset` |
| `book_caches[]` | `Shelf.readBookCacheFile` |

A case's `shelf.json`, when it has one, is part of the input rather than a field
of the reading: the shelf reads it before walking (`loadShelfRules`), so its
effect shows up in `folders`, `books` and `books[].nsfw`. A case whose
`shelf.json` names its own directories no longer skips the defaults, so such a
case is where a name like `lost+found` legitimately appears as a folder.

Conventions of the recorded reading:

- `books` and `book_caches` are ordered by `path` and by `name`; `folders`,
  `sources` and `assets` are sorted the way the shelf sorts them.
- `schema_version_on_disk` is `0` when `book.json` carries no `schema_version`
  field, which is how a pre-v1 book is distinguished from a v1 one. The
  shelf's in-memory normalization does not show up here.
- `current_source` is `null` when no source can be resolved, while
  `current_source_field` is the raw pointer in `book.json`, so a fallback is
  visible as the two differing.
- `usable` in `book_caches` means the file parses as a cache *this build* can
  read; an unusable one costs a full scan and is never an error.
- `nsfw` is the sum of the two places a mark can be written — `nsfw` in the
  book's own `book.json` and a `content.nsfw_folders` rule in `shelf.json` —
  not either half on its own. The two add: a book's `nsfw: false` does not take
  it out of a marked folder. Folder paths are matched case-insensitively
  (`shelfutil.foldSegment`), which is why the marked folder in `nsfw-marked` is
  spelled in a different case than its rule.
- `folder_meta` lists only the folders that have a `folder.json` entry, keyed
  like `folders`; the root never has one. `icon` is what is shown
  (`DisplayIcon`, `""` for the default), `icon_on_disk` the member as written,
  and `read_only` that a write would be refused for the file's content.
- An absent value is recorded as `""`, `0`, `[]` or `{}` rather than `null`.
- Timestamps are recorded exactly as PlainShelf writes them: `created_at` and
  `updated_at` in RFC 3339 (`2026-03-15T08:30:00Z`), `published_at` as a date
  (`2026-03-15`). Go parses these into a `time.Time` and writes them back, so
  only the canonical spelling round-trips; write fixtures in that form.

## The shelf trees are append-only from `1.0.0-rc1`

The on-disk format freezes at `1.0.0-rc1` (see
[Compatibility policy](../../../docs/concepts/data-format-versioning.md#compatibility-policy)),
and this dataset is where that freeze is checkable rather than only stated. So
the two halves of a case are governed differently:

- **`cases/<name>/shelf/` is append-only.** The real files under it are bytes a
  shipped PlainShelf wrote, so from `1.0.0-rc1` on they are not edited or
  deleted. A file may be *added* — to a new case, or to an existing one where
  the addition is what the case is about — because an optional addition is
  exactly what the freeze still permits. Rewriting an existing one is not: it
  would quietly move the baseline the freeze is measured against, and the
  reading it pinned would be lost with it. Cover a format change with a new
  case instead.
- **`expected.json` may still change shape.** It is this suite's own record of
  a reading, not a file any PlainShelf writes, so a new observation may be added
  to it. Bump `schema_version` in `manifest.json` and update the harness in the
  same commit, as above. What it may not do is change what it says about an
  existing shelf tree without a change in the reader to justify it.

`v1-frozen-at-1.0.0-rc1` is the baseline case: one book carrying every field
`book.json` schema v1 defines and one source carrying every field source
`meta.json` schema v1 defines, at the values the freeze pinned. It is where a
field silently changing spelling, shape, or reading shows up as a failure.

Edits under `shelf/` before `1.0.0-rc1` are ordinary fixture maintenance; the
rule starts at the tag.

## Adding a case

1. Build the shelf under `cases/<name>/shelf/`, keeping files small — a fixture
   is read by people more often than by tests.
2. Write `expected.json` by hand, from the format documentation
   (`docs/concepts/data-model.md`) rather than from a test run: an expectation
   copied out of an implementation cannot disagree with it.
3. Add the case to `manifest.json`, appending rather than reordering.
4. Run the harness: `go test ./shelf -run Conformance`.

Keep source metadata self-consistent: `md5_hash`, `line_count` and `char_count`
must describe the `source.txt` beside them, the way `RefreshContentMetadata`
would write them.

The `folder-metadata` case includes a v2 exported book cache with `folder_meta`.
Local reads walk the source files; ReadFS, pCloud and WebDAV reads restore the
same icons and read-only metadata flags from that export.
