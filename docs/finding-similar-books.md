# Finding Similar Books

The **Similar Content** page finds books that are *alike* but not byte-for-byte
identical: another edition, a format conversion, a trimmed-down copy, or two
transcripts of one recording. It sits in the maintenance navigation next to
**Duplicate Content**, and the two answer different questions — see
[Similar is not the same as duplicate](#similar-is-not-the-same-as-duplicate)
below.

It compares the **current source** of every book, so the results are about the
version of each book you are actually reading.

---

## The three levels

The bar at the top of the page offers three levels, from strictest to widest.
Each is a similarity floor: raising the level shows fewer, closer pairs, and
lowering it widens the net. Concrete examples of what each level is built to
catch:

| Level | Catches | Example |
|---|---|---|
| **Nearly identical** | The same text with only small edits. | The same book imported once as a `.txt` file and once from an EPUB — the words are the same, only a table of contents or a colophon differs. |
| **Same book, other edition** | The same work, reworded or revised. | A revised translation, or a cleaned-up re-typing of a book you already had. |
| **Possibly same source** | Two texts that clearly share an origin but diverge a lot. | Two independent transcripts of one audiobook or lecture, worded differently throughout. |

The page opens on **Same book, other edition**, the middle level, which is the
one most people want. **Advanced** replaces the three levels with a slider if you
want to set the floor by hand.

### Trimmed copies

Below the levels is a separate checkbox, **Only trimmed copies (one edited down
from the other)**. This is not a fourth level — it is a different question.

A book edited down from another — an abridgement, or a draft with a chapter or
two cut — can score low overall similarity just because it is so much shorter, so
it would hide under the wider levels even though one copy sits almost entirely
inside the other. The checkbox looks for exactly that shape: pairs where nearly
all of the shorter text is contained in the longer one. When it is on, your level
selection is ignored and only trimmed copies are shown, with the fuller copy
listed first and a note of how much content the shorter one is missing.

One limit, though: the page only ever compares pairs that clear the widest
level's floor (15% similar), and the checkbox narrows that same set rather than
widening it. For a straight excerpt that means the shorter copy has to be at
least roughly a sixth of the fuller text's length to be compared at all — so a
brief sample, such as a single chapter lifted from a long book, can be too short
to surface even with the checkbox on. Trimmed copies that kept most of the book
are what this filter reliably finds.

---

## Similar is not the same as duplicate

The **Duplicate Content** page groups books: three copies of the exact same text
become one group of three, because "identical" is all-or-nothing — if A matches B
and B matches C, then A matches C, and all three belong together.

**Similarity does not work that way.** A is *like* B and B is *like* C does not
mean A is like C: a revised edition can resemble both the original and a later
one that no longer resembles the first. So the Similar Content page shows
**pairs**, not groups. One card is one pair of books.

This is why the same book can appear on more than one card. If one edition is
similar to two others, you will see it paired with each of them — two cards, the
same book in both. That is the page telling you the truth about how the books
relate, not a bug.

---

## Reading the difference readout

Each pair shows a similarity percentage and, for most relations, a rough "**≈ N
in 100 characters differ**" readout. Treat it as a feel for how far apart the two
texts are, not a measurement:

- It is an **estimate**, derived from how much the two texts' fingerprints
  overlap. It reads truest when the differences are spread evenly through the
  book — scattered edits, transcription differences, OCR slips.
- It does **not** apply to trimmed copies. When one book is a single continuous
  cut of another, "characters differ" is the wrong lens; those pairs show **how
  much less content** the shorter copy has instead.

---

## Building fingerprints

Comparing books this way means reading every book's text in full and reducing it
to a compact fingerprint. That is too expensive to redo every time you open the
page, so PlainShelf does it once, on request, and remembers the result.

When some books have not been fingerprinted yet, a bar appears saying how many
are missing, with a **Build fingerprints** button. Pressing it reads every
source that has changed since the last run and computes what is missing; you can
watch the progress and the page refreshes when it finishes. You do this yourself
rather than having it happen automatically because it reads every `source.txt` on
the shelf — real work on a large or network-mounted library — and PlainShelf does
not spend that on its own.

The fingerprints are stored in `app/fingerprint-cache.json`. It is rebuildable
runtime state, safe to delete, and shared cleanly between machines — see
[the fingerprint cache in the data model](concepts/data-model.md#app). A
read-only shelf cannot build fingerprints; it can still compare whatever it
already has.

### Books you edited by hand

A fingerprint is tied to the exact bytes of a source. If you edit a book's text —
in PlainShelf or with an outside editor — the next **Build fingerprints** run
notices the file changed and recomputes it, so the comparison keeps up with your
edits.

There is one known blind spot. PlainShelf decides a file is unchanged from its
size and modification time before reading it, so an edit that leaves **both** the
size and the modification time exactly as they were slips through unnoticed. This
essentially only happens when an external tool restores a file's timestamp after
rewriting it; a normal edit updates the modification time and is caught.

If you suspect a book is being compared against its old text, there is no in-app
"force rebuild" button today, and re-saving the source alone will not give you
one: the missing-fingerprint count that shows the **Build fingerprints** bar is a
cheap in-memory check that does not re-read the file, so on a shelf that was
already fully fingerprinted the bar stays hidden and there is nothing to press.

Because the cache is disposable, the reliable way to force a recompute is to
delete `app/fingerprint-cache.json`. Every book then counts as unfingerprinted,
the **Build fingerprints** bar comes back, and pressing it reads and fingerprints
the whole shelf again from scratch — picking up your edited book along with the
rest.

### Several machines, one shelf

If more than one machine opens the same shelf, their fingerprints **merge** into
the one shared file rather than overwriting each other. A fingerprint depends only
on a source's content, so every machine computes the same answer for the same
book, and whichever machine fingerprints a given book contributes it to the
shared file. Building fingerprints on one machine is not undone by opening the
shelf on another.

---

## How similarity is computed

You do not need this section to use the page. It records the settings the
comparison uses, in one place, so that **if any of them ever change, this is the
paragraph to update** (along with the cache-invalidation note in
[Data Format Versioning](concepts/data-format-versioning.md#fingerprint-cache),
since changing any of these discards and rebuilds the fingerprint cache).

Before two texts are compared they are **normalized** to the characters that
carry their meaning: PlainShelf applies Unicode NFKC and drops whitespace,
punctuation, symbols, and separators. So line rewrapping, curly-vs-corner quotes,
and paragraph indentation all disappear before the comparison starts. Two
deliberate consequences:

- **Latin words run together** once their spaces are gone, so spacing differences
  in Latin text stop mattering — the same rule that makes CJK reflowing invisible.
- **Traditional and simplified Chinese stay separate.** 「三國」 and 「三国」 are
  treated as different text. Converting between them reliably needs a dictionary,
  and a wrong conversion would merge two books that are not the same edition.

The normalized text is then compared as overlapping runs of **5 characters**, and
the level floors are **85%** (nearly identical), **45%** (same book, other
edition), and **15%** (possibly same source). The trimmed-copy check looks for one
text at least **95%** contained in the other.

---

## Out of scope

This page describes what the feature does, not the mathematics behind the
fingerprints (the MinHash sketch, containment estimation, and shingle hashing).
Those live in the code and its comments, and in the evaluation notes, where they
can be reasoned about precisely.
