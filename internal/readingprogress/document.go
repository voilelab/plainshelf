// Package readingprogress models the cross-process reading-progress document
// and the reader -> desktop projection that folds standalone-reader progress
// back into the desktop library.
//
// Two independent processes share one reading_progress.json: the desktop app
// (which keys progress by a book's real shelf id) and the standalone reader. A
// reader launched from the desktop is handed the book's real shelf id and keys
// progress directly under it; a reader run on its own has no real shelf and keys
// everything under the synthetic shelf id "book", which the projection folds onto
// real shelves.
//
// Every entry carries the wall-clock time it was written, and all reconciliation
// is newest-wins per book: a concurrent write from either process is arbitrated
// by recency rather than by namespace ownership, so the reader and desktop can
// both write the same real shelf without losing each other's updates. A reset is
// a timestamped tombstone (offset 0) rather than a deletion, so it too wins by
// recency instead of being silently resurrected by an older advance. This package
// owns the document shape both sides agree on, the newest-wins merge, and the
// newest-wins projection. The file-backed, cross-process-locked store lives in
// store.go.
//
// The document shape mirrors frontend/src/storage/readingProgress/document.ts,
// which owns the single client-side implementation. This package only reads and
// writes what that format defines.
package readingprogress

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"math"
	"strings"

	"github.com/voilelab/plainshelf/internal/jsonopt"
	"github.com/voilelab/plainshelf/internal/util"
)

// DocumentVersion matches READING_PROGRESS_DOCUMENT_VERSION in the frontend. A
// document of any other version is treated as absent rather than upgraded: the
// timestamped v2 format deliberately does not read the untimestamped v1 one.
// This file is device-local state, outside the on-disk format commitments, and
// dropping v1 shipped in v0.10.0 as a documented breaking change (CHANGELOG.md,
// docs/installation.md) — not a pattern to follow for shelf data.
const DocumentVersion = 2

// ReaderShelfID is the synthetic shelf key the standalone reader writes progress
// under. It is the canonical value; reader/readerapi.ShelfID must equal it (a
// guard test in the reader package enforces this).
const ReaderShelfID = "book"

// maxSafeInteger mirrors JavaScript's Number.MAX_SAFE_INTEGER: the frontend
// stores offsets and timestamps as JSON numbers, so anything larger could not
// round-trip.
const maxSafeInteger = 1<<53 - 1

// Entry is one book's stored progress: a UTF-16 character offset and the epoch
// milliseconds it was written. At arbitrates concurrent writes (newest wins); a
// zero Offset with a non-zero At is a reset tombstone.
type Entry struct {
	Offset int64 `json:"offset"`
	At     int64 `json:"at"`
}

// Document is the on-device reading-progress document: a shelf key maps to a set
// of book ids, each carrying an Entry. It never reaches the server and is not
// authoritative shelf data.
type Document struct {
	Version int                         `json:"version"`
	Shelves map[string]map[string]Entry `json:"shelves"`
}

// New returns an empty, current-version document.
func New() Document {
	return Document{Version: DocumentVersion, Shelves: map[string]map[string]Entry{}}
}

// Parse reads a stored document. A missing, corrupt, wrongly shaped, or
// non-current-version document yields an empty document rather than an error, so
// a bad file can never keep a reader from opening — this mirrors the frontend
// parser's leniency.
func Parse(text string) Document {
	doc := New()

	if strings.TrimSpace(text) == "" {
		return doc
	}

	var raw struct {
		Version int `json:"version"`
		Shelves map[string]map[string]struct {
			Offset float64 `json:"offset"`
			At     float64 `json:"at"`
		} `json:"shelves"`
	}
	if err := json.Unmarshal([]byte(text), &raw, jsonopt.Read()); err != nil {
		return doc
	}
	if raw.Version != DocumentVersion {
		return doc
	}

	for shelfKey, books := range raw.Shelves {
		trimmedKey := strings.TrimSpace(shelfKey)
		if trimmedKey == "" {
			continue
		}
		normalized := map[string]Entry{}
		for bookID, entry := range books {
			trimmedID := strings.TrimSpace(bookID)
			value, ok := normalizeEntry(entry.Offset, entry.At)
			if trimmedID != "" && ok {
				normalized[trimmedID] = value
			}
		}
		if len(normalized) > 0 {
			doc.Shelves[trimmedKey] = normalized
		}
	}

	return doc
}

// ParseStrict is Parse but rejects text that is not valid JSON. Write paths use
// it: lenient-parsing corrupt input into an empty document, then merging, would
// silently wipe the writer's stored namespace. An empty string is allowed and
// yields an empty document.
func ParseStrict(text string) (Document, error) {
	if strings.TrimSpace(text) != "" && !jsontext.Value(text).IsValid() {
		return New(), util.NewError("reading progress document is not valid JSON")
	}
	return Parse(text), nil
}

// Serialize renders the document as the compact JSON the frontend also produces.
func Serialize(doc Document) (string, error) {
	if doc.Version == 0 {
		doc.Version = DocumentVersion
	}
	if doc.Shelves == nil {
		doc.Shelves = map[string]map[string]Entry{}
	}
	bs, err := json.Marshal(doc, jsonopt.DiskCompact())
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	return string(bs), nil
}

// Clone returns a deep copy so callers can mutate freely without touching the
// input — the projection and merge both build their result this way, which is
// what makes them safe to call repeatedly on the same document.
func (d Document) Clone() Document {
	out := Document{Version: d.Version, Shelves: make(map[string]map[string]Entry, len(d.Shelves))}
	if out.Version == 0 {
		out.Version = DocumentVersion
	}
	for shelfKey, books := range d.Shelves {
		out.Shelves[shelfKey] = cloneBooks(books)
	}
	return out
}

// ResolveShelf maps a stable book id to the real shelf id that owns it, or
// reports ok=false when no real shelf in the desktop library holds that book.
type ResolveShelf func(bookID string) (realShelfID string, ok bool)

// Project folds the reader's synthetic-shelf progress onto real shelves.
//
// For every book the reader recorded under readerShelfID, it asks resolve which
// real shelf holds that stable book id and, when one does, copies the entry to
// shelves[realShelfID][bookID] when the entry is newer (by At) than whatever is
// there. Entries under readerShelfID are always kept: an unresolved book (removed,
// or a loose book outside every shelf) stays there untouched, so a later import
// can pick it up on the next projection. The projection is therefore idempotent:
// running it again changes nothing once every resolvable book has been folded in.
func Project(doc Document, readerShelfID string, resolve ResolveShelf) Document {
	out := doc.Clone()
	readerShelfID = strings.TrimSpace(readerShelfID)
	readerBooks := out.Shelves[readerShelfID]
	if readerShelfID == "" || len(readerBooks) == 0 || resolve == nil {
		return out
	}

	for bookID, entry := range readerBooks {
		realShelfID, ok := resolve(bookID)
		if !ok {
			continue
		}
		realShelfID = strings.TrimSpace(realShelfID)
		if realShelfID == "" || realShelfID == readerShelfID {
			continue
		}
		dst := out.Shelves[realShelfID]
		if dst == nil {
			dst = map[string]Entry{}
			out.Shelves[realShelfID] = dst
		}
		if cur, exists := dst[bookID]; !exists || entry.At >= cur.At {
			dst[bookID] = entry
		}
	}

	return out
}

// MergeNewest overlays one process's write onto the on-disk document, keeping the
// newer entry (by At) for every book present in both. Books only on disk are
// preserved; books only in the write are added. This is the single reconciliation
// both the desktop and the reader use: because every entry is timestamped and a
// reset is a tombstone rather than a deletion, neither process can clobber the
// other's concurrent update, whichever shelf namespace it wrote — so both may
// write the same real shelf safely. On an equal timestamp the incoming write
// wins, which only ever happens when it carries the same value it just read.
func MergeNewest(disk, write Document) Document {
	out := disk.Clone()
	for shelfKey, books := range write.Shelves {
		shelfKey = strings.TrimSpace(shelfKey)
		if shelfKey == "" || len(books) == 0 {
			continue
		}
		dst := out.Shelves[shelfKey]
		if dst == nil {
			dst = map[string]Entry{}
			out.Shelves[shelfKey] = dst
		}
		for bookID, entry := range books {
			bookID = strings.TrimSpace(bookID)
			if bookID == "" {
				continue
			}
			if cur, exists := dst[bookID]; !exists || entry.At >= cur.At {
				dst[bookID] = entry
			}
		}
	}
	return out
}

// cloneBooks deliberately keeps the make+loop rather than maps.Clone. Document.Clone
// promises callers may mutate the returned maps freely, and a shelf decoded from a
// JSON "shelf": null yields a nil books map here — maps.Clone(nil) returns nil, so a
// caller that writes into the cloned shelf without a nil guard would panic. The
// make guarantees a non-nil map and preserves that contract unconditionally.
func cloneBooks(books map[string]Entry) map[string]Entry {
	out := make(map[string]Entry, len(books))
	for bookID, entry := range books {
		out[bookID] = entry
	}
	return out
}

// normalizeEntry keeps only the entries the frontend would keep. The offset must
// be a non-negative integer within the JSON-safe range; a non-integer or
// out-of-range timestamp is treated as unknown (0, the oldest) rather than
// dropping the entry. An entry with a zero offset survives only when it carries a
// timestamp — that is a reset tombstone; a bare {0,0} is nothing and is dropped.
func normalizeEntry(offset, at float64) (Entry, bool) {
	normalizedOffset, ok := normalizeNumber(offset)
	if !ok {
		return Entry{}, false
	}
	normalizedAt, ok := normalizeNumber(at)
	if !ok {
		normalizedAt = 0
	}
	if normalizedOffset == 0 && normalizedAt == 0 {
		return Entry{}, false
	}
	return Entry{Offset: normalizedOffset, At: normalizedAt}, true
}

// normalizeNumber accepts a non-negative, integral, JSON-safe value.
func normalizeNumber(value float64) (int64, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	if value < 0 || value > maxSafeInteger {
		return 0, false
	}
	if value != math.Trunc(value) {
		return 0, false
	}
	return int64(value), true
}
