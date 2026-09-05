# bookpkg format conformance dataset

Two independent readers of the on-disk book package format exist:

- the Go shelf in `shelf/`, which every server and desktop build reads through;
- `frontend/src/api/pcloud/bookpkg.ts` and `bookCacheFile.ts`, which the mobile
  client uses to read a shelf straight from pCloud, without a server in between.

The second one cannot import the first, so nothing but a shared fixture keeps
them from drifting apart. This dataset is that fixture: one tree of shelves and
one expected reading of each, run by both sides.

| Harness | File |
|---|---|
| Go | `shelf/conformance_test.go` |
| TypeScript | `frontend/src/api/pcloud/conformance.test.ts` |

## Layout

```
conformance/
├─ manifest.json          versioned index of the cases
├─ README.md
└─ cases/<case>/
   ├─ shelf/              a real shelf root: books/, plus app/ and shelf.json
   │                      where relevant
   └─ expected.json       the reading both implementations must produce
```

Every case directory must be listed in `manifest.json` and every listed case
must exist; both harnesses fail otherwise, so a case cannot be added and then
silently skipped. `schema_version` in the manifest describes the shape of
`expected.json`; bump it when that shape changes and update both harnesses in
the same commit.

Git cannot commit an empty directory, so a directory that must exist without
holding a book carries a placeholder file. Both readers walk directories only,
which is what makes such a file invisible to them.

## The expected reading

`expected.json` is not a dump of either implementation. It records the
observations both can make, in the shape both can produce:

| Field | Go | TypeScript |
|---|---|---|
| `folders` | `collectExportFolders` | `collectFolders` |
| `books[].path` | `Book.FolderPath` | `bookPackagePath` |
| `books[].folders` | `BookListing.Folders` | `BookPackageRef.folders` |
| `books[].id`, `title`, `format`, `authors`, `tags`, `identifiers`, `language`, `comments`, `star`, `created_at`, `updated_at`, `published_at`, `cover` | `Book.GetMeta` | `parseBookJson` |
| `books[].cover_present` | `Book.OpenCover` | `findCoverFile` |
| `books[].schema_version_on_disk` | `readBookMeta` | `BookJson.schema_version` |
| `books[].read_only` | `Book.EnsureWritable` | `isSchemaNewerThanSupported` |
| `books[].current_source_field` | `Book.CurrentSource` | `BookJson.current_source` |
| `books[].current_source` | `Book.ResolveCurrentSource` | `findCurrentSource` |
| `books[].sources[]` | `Book.ListSource` | `BookSourceRef` |
| `books[].sources[].schema_version`, `created_at`, `comment`, `format`, `md5_hash`, `line_count`, `char_count` | `Source.GetMeta` | `toSourceMeta` |
| `books[].sources[].assets` | `Source.AssetPath` / `OpenAsset` | `BookSourceRef.assets` |
| `book_caches[]` | `Shelf.readBookCacheFile` | `parseBookCacheFile` |

A case's `shelf.json`, when it has one, is part of the input rather than a field
of the reading: both harnesses read it before walking (`loadShelfRules`,
`parseShelfConfig`), so its effect shows up in `folders` and `books`. A case
whose `shelf.json` names its own directories no longer skips the defaults, so
such a case is where a name like `lost+found` legitimately appears as a folder.
Its `content` section has no such effect: it decides which books are marked as
adult content, which is not yet a field of the reading — see
[Out of scope](#out-of-scope).

Conventions that keep the two comparable:

- `books` and `book_caches` are ordered by `path` and by `name`; `folders`,
  `sources` and `assets` are sorted the way each implementation sorts them.
- `schema_version_on_disk` is `0` when `book.json` carries no `schema_version`
  field, which is how a pre-v1 book is distinguished from a v1 one. Neither
  side's in-memory normalization shows up here.
- `current_source` is `null` when no source can be resolved, while
  `current_source_field` is the raw pointer in `book.json`, so a fallback is
  visible as the two differing.
- `usable` in `book_caches` means the file parses as a cache *this build* can
  read; an unusable one costs a full scan and is never an error.
- An absent value is recorded as `""`, `0`, `[]` or `{}` rather than `null`,
  since one side reads a JSON member into a zero value and the other into
  `undefined`, and neither difference is a disagreement about the file.
- Timestamps are recorded exactly as PlainShelf writes them: `created_at` and
  `updated_at` in RFC 3339 (`2026-03-15T08:30:00Z`), `published_at` as a date
  (`2026-03-15`). Go parses these into a `time.Time` and writes them back,
  while the pCloud reader keeps the string it found, so the two agree on the
  canonical spelling and only on that. A fixture written any other way — an
  offset instead of `Z`, or a full timestamp in `published_at` — would fail as
  a disagreement without either reader being wrong; write the canonical form.
- A source `format` other than `txt` or `md` is likewise not a shared reading:
  the Go side keeps the string, the pCloud reader drops it to `undefined`. No
  fixture carries one.

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
  a reading, not a file any PlainShelf writes, so a new observation both
  implementations can make may be added to it. Bump `schema_version` in
  `manifest.json` and update both harnesses in the same commit, as above. What
  it may not do is change what it says about an existing shelf tree without a
  change in the readers to justify it.

`v1-frozen-at-1.0.0-rc1` is the baseline case: one book carrying every field
`book.json` schema v1 defines and one source carrying every field source
`meta.json` schema v1 defines, at the values the freeze pinned. It is where a
field silently changing spelling, shape, or reading shows up as a failure in
both harnesses at once.

Edits under `shelf/` before `1.0.0-rc1` are ordinary fixture maintenance; the
rule starts at the tag.

## Adding a case

1. Build the shelf under `cases/<name>/shelf/`, keeping files small — a fixture
   is read by people more often than by tests.
2. Write `expected.json` by hand, from the format documentation
   (`docs/concepts/data-model.md`) rather than from a test run: an expectation
   copied out of an implementation cannot disagree with it.
3. Add the case to `manifest.json`, appending rather than reordering.
4. Run both harnesses.

Keep source metadata self-consistent: `md5_hash`, `line_count` and `char_count`
must describe the `source.txt` beside them, the way `RefreshContentMetadata`
would write them.

## Out of scope

Deliberately not covered here, because the two readers answer differently and
neither answer is currently wrong:

- a package with no `book.json` — the Go walk drops it, the pCloud reader
  surfaces it with no `meta` and its caller skips it;
- a source folder with no `meta.json`, and the `.<id>.tmp` staging folder an
  interrupted import leaves behind — the Go listing skips both, the pCloud
  reader lists them;
- a non-image file under `assets/` — the Go read path refuses to serve it, the
  pCloud reader indexes it like any other asset;
- choosing between several usable caches: the pCloud client picks by the
  listing's modification time (`pickNewestBookCache`), which the Go side has no
  counterpart for because it only ever writes its own;
- whether a book is marked as adult content — the Go side assembles it from
  `content.nsfw_folders` in `shelf.json` and `nsfw` in `book.json`
  (`Shelf.IsBookNSFW`), while the pCloud reader reads neither. Recording it here
  would mean giving that reader its own copy of the folder rules; the exported
  book cache already carries the assembled answer as `nsfw` on each entry, which
  is the route a client that never reads `shelf.json` is meant to take. Until one
  of the two happens, `nsfw-marked` pins only that neither key changes anything
  else about the reading, and the rules themselves are covered by
  `shelf/shelf_nsfw_test.go`;
- a file broken by hand: the Go side reads `book.json`, `meta.json`,
  `trash.json` and `shelf.json` with `encoding/json/v2`, which refuses a
  duplicate member and invalid UTF-8, while the pCloud reader uses `JSON.parse`,
  where the last member wins and undecodable bytes have already become U+FFFD.
  A case here would have to record one reading, and there is no reading both
  produce.

Fixtures for those belong in the unit tests on either side, where the intended
difference can be stated. If one of them is ever unified, move it here.
