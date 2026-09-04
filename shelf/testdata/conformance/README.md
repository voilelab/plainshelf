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
| `books[].id`, `title`, `format`, `authors`, `tags`, `star`, `cover` | `Book.GetMeta` | `parseBookJson` |
| `books[].cover_present` | `Book.OpenCover` | `findCoverFile` |
| `books[].schema_version_on_disk` | `readBookMeta` | `BookJson.schema_version` |
| `books[].read_only` | `Book.EnsureWritable` | `isSchemaNewerThanSupported` |
| `books[].current_source_field` | `Book.CurrentSource` | `BookJson.current_source` |
| `books[].current_source` | `Book.ResolveCurrentSource` | `findCurrentSource` |
| `books[].sources[]` | `Book.ListSource`, `Source.GetMeta` | `BookSourceRef` |
| `books[].sources[].assets` | `Source.AssetPath` / `OpenAsset` | `BookSourceRef.assets` |
| `book_caches[]` | `Shelf.readBookCacheFile` | `parseBookCacheFile` |

A case's `shelf.json`, when it has one, is part of the input rather than a field
of the reading: both harnesses read it before walking (`loadIgnoreRules`,
`parseShelfConfig`), so its effect shows up in `folders` and `books`. A case
whose `shelf.json` names its own directories no longer skips the defaults, so
such a case is where a name like `lost+found` legitimately appears as a folder.

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

## Adding a case

1. Build the shelf under `cases/<name>/shelf/`, keeping files small — a fixture
   is read by people more often than by tests.
2. Write `expected.json` by hand, from the format documentation
   (`docs/concepts/data-model.md`) rather than from a test run: an expectation
   copied out of an implementation cannot disagree with it.
3. Add the case to `manifest.json`.
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
- a file broken by hand: the Go side reads `book.json`, `meta.json`,
  `trash.json` and `shelf.json` with `encoding/json/v2`, which refuses a
  duplicate member and invalid UTF-8, while the pCloud reader uses `JSON.parse`,
  where the last member wins and undecodable bytes have already become U+FFFD.
  A case here would have to record one reading, and there is no reading both
  produce.

Fixtures for those belong in the unit tests on either side, where the intended
difference can be stated. If one of them is ever unified, move it here.
