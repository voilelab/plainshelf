# Similarity fingerprints

The [Similar Books](../finding-similar-books.md) page compares books by a
fingerprint of each source's normalized text. This page records how that
fingerprint is built, compared and cached. The code is in `internal/sketch`.
The [page hash cache](#page-hash-cache) behind comic duplicate detection is
described at the end.

## The sketch

A fingerprint is a bottom-k MinHash sketch: the `k` smallest 64-bit hashes of
the document's distinct character n-grams ("shingles"). Keeping the `k` smallest
values of one hash function is more accurate and cheaper than the textbook
construction with `k` independent hash functions (Cohen and Kaplan, 2007).

| Parameter | Value | Why |
|---|---|---|
| Shingle width `n` | 5 runes | Wider shingles separate related from unrelated documents more sharply but weaken the signal; 5 is the measured balance point for book-length prose. |
| Retained hashes `k` | 128 | Bounds the stored size only; building cost is the shingle scan. The Jaccard error is roughly ±0.05. |
| Exact limit | 5000 runes | Below this length every shingle is kept instead of a sample: sampling noise would swamp the signal, and exact comparison costs nothing. |

Shingles are counted in runes, so the width means the same for CJK text as for
alphabetic text; each window is hashed over its bytes.

A sketch also stores the **exact** number of distinct shingles. Estimating it
from the retained hashes instead (KMV cardinality estimation) costs up to 16%
error at `k = 128`, and the exact count is what makes containment algebra rather
than estimation. Building streams the text once and keeps only the `k` smallest
hashes, plus a transient set of seen hashes for the exact count.

The stored form is a version byte, the shingle width, the distinct count, the
number of retained hashes, and the hashes, all little-endian, base64-encoded in
`app/fingerprint-cache.json`.

## Comparing two sketches

**Jaccard similarity** is the size of the shingle intersection over the size of
the union. It is exact when both sketches hold every shingle they counted.
Otherwise it is read off the merged sketches below the smaller of their two
largest retained hashes, where both still list their document's shingles
exhaustively. Sketches built with different widths, and empty sketches, compare
as 0.

**Containment** says how much of one document lies inside the other. With the
Jaccard similarity `J` and the exact distinct counts `na` and `nb`, the
intersection follows from `J = inter / (na + nb - inter)`, so containment adds no
error beyond the error already in `J`.

**Upper bound.** Two documents with `na` and `nb` distinct shingles cannot have a
Jaccard similarity above `min(na, nb) / max(na, nb)`. That is a hard bound on the
true value, so one integer comparison rules out a pair whose lengths differ too
much, which matters in a library whose documents vary widely in length.

## Fingerprint cache

`app/fingerprint-cache.json` holds the sketches above. It carries a
`schema_version`, but it is versioned the opposite way to the user data in
[Data format versioning](data-format-versioning.md), because it is a
rebuildable cache rather than your data.

The file records both its `schema_version` and an `algo` block naming the exact
rules its fingerprints were produced under (the text normalization, the shingle
size, the hash, and how many hashes each fingerprint keeps). When a build meets a
`schema_version` it does not read, **or** an `algo` block that does not match the
rules it fingerprints by, it throws the whole file away and rebuilds it. There is
no migration and, deliberately, no field-level salvage: changing how text is
normalized invalidates every fingerprint built on top of it, so keeping "the
parts that did not change" would mean trusting entries nobody can vouch for.

That is the right trade only because the file is a cache. Discarding it costs
one recomputation (the next similarity build reads the sources again) and never
loses anything, which is exactly what the migration promises for `books/` and
`trash/` exist to avoid. See [the `app/` directory](data-model.md#app)
for what the file holds.

The file has two levels of key:

- **index**, `{book_id}/{source_id}` to the size, mtime and MD5 of that
  `source.txt`. One stat decides whether the record still describes the file, so
  an unchanged source is answered without being opened. It is keyed on the book
  ID rather than the path, so a retitle or a move between folders keeps the
  fingerprint. When two machines' records disagree, the one first observed later
  (`seen_at`) wins, not the newer mtime: restoring a source from a backup gives
  new content an older mtime.
- **entries**, the MD5 of `source.txt` to the fingerprint of that content. An
  entry is never updated, only added, so two copies of a book share one and
  merging two machines' files is a set union that cannot conflict.

It lives under `app/` rather than in `book.json` or `meta.json` because a
fingerprint belongs to a source's content, an entry is about 1.4 kB of base64,
and a change of rules invalidates every fingerprint at once: one file to delete
here, a full-library rewrite there.

Saving merges what the run learned into the file on disk and writes it
atomically. It is not a transaction: under `lock_mode: none` two simultaneous
saves can both replace the same older file, which costs a later recomputation
and never a wrong answer, because entries are keyed by content. Saving also
drops records for books that are gone, but not a record for a source deleted
from a book that still exists (deciding that would need a listing per book);
such a record is about 1.5 kB and is never served.

!!! note "Building fingerprints can write one source file"
    The Similar Books page reads like pure analysis, but building fingerprints
    has one write side effect on `books/`. When it reads a source whose stored
    `md5_hash` no longer matches the source's actual bytes (content changed
    outside PlainShelf, or an older import that recorded a different hash), it
    corrects that source's `meta.json` in place (`Source.RepairContentHash`). It deliberately leaves the
    UI-visible line and character counts at their stale values rather than
    recomputing them, but the write still rebuilds the whole file from the
    fields PlainShelf knows, so, like any `meta.json` write, it drops top-level
    keys outside that set. It goes through the same write refusal as any other
    write, so it never touches a read-only shelf or a source whose schema
    version is newer than this build.

## Page hash cache

`app/page-hash-cache.json`, the cache behind comic duplicate detection on the
Duplicate Content page, is versioned the same throw-away way as the fingerprint
cache. It records a `schema_version` and the page hash it uses (`"hash":
"md5"`); a build that reads either differently discards the whole file, and the
next hashing run reads every page again.

`sources` maps `{book_id}/{source_id}` to two things:

- `list_md5`, the MD5 of the `source.txt` page list the record was built from.
  The Duplicate Content page reads the page list of each comic it holds a record
  for, and none other, and uses the record only while that list still hashes to
  it, so adding, removing or reordering pages takes the comic out of the
  comparison until it is hashed again, without a stat per page. It hashes the
  list itself rather than trusting `md5_hash` in
  `meta.json`, which an edit made outside PlainShelf leaves unchanged.
- `pages`, each listed page's asset name to its size, mtime and MD5. The hashing
  task stats every page and reads only those whose size or mtime changed. A page
  written within the last two seconds is stored without an mtime, so the next
  run reads it again instead of trusting a stat that could still change.

A comic whose page list names an image that is missing gets no record: an
incomplete page set could match a shorter comic exactly. The task reports it as
failed with `missing_pages`.

Saving starts from the file on disk and replaces only the records this run
recomputed, so a concurrent run's newer records survive. It also drops
records for books that are gone, and writes atomically only when the bytes
differ. It writes nothing under `books/`.
