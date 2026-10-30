# Similarity fingerprints

The [Similar Books](../finding-similar-books.md) page compares books by a
fingerprint of each source's normalized text. This page records how that
fingerprint is built and compared. The code is in `internal/sketch`; where the
fingerprints are stored is in
[Data format versioning](data-format-versioning.md#fingerprint-cache).

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
