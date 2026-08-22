// Package readingprogress models the cross-process reading-progress document
// and the reader -> desktop projection that folds standalone-reader progress
// back into the desktop library.
//
// Two independent processes share one reading_progress.json: the desktop app
// (which keys progress by a book's real shelf id) and the standalone reader. A
// reader launched from the desktop is handed the book's real shelf id and keys
// progress directly under it, so its writes need no projection. A reader run on
// its own has no real shelf and keys everything under the synthetic shelf id
// "book"; those entries are what the projection folds onto real shelves. This
// package owns the document shape both sides agree on, the pure projection that
// maps the reader's "book" entries onto real shelves by stable book id, and the
// namespace-scoped merges that keep one process from clobbering the other's
// entries. The file-backed, cross-process-locked store lives in store.go.
//
// The document shape mirrors frontend/src/storage/readingProgress/document.ts,
// which owns the single client-side implementation. This package only reads and
// writes what that format defines; it never invents new fields (there is
// deliberately no timestamp — see the design note referenced from the ticket).
package readingprogress

import (
	"encoding/json"
	"math"
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
)

// DocumentVersion matches READING_PROGRESS_DOCUMENT_VERSION in the frontend. A
// document of any other version is treated as absent rather than upgraded.
const DocumentVersion = 1

// ReaderShelfID is the synthetic shelf key the standalone reader writes progress
// under. It is the canonical value; reader/readerapi.ShelfID must equal it (a
// guard test in the reader package enforces this).
const ReaderShelfID = "book"

// maxSafeInteger mirrors JavaScript's Number.MAX_SAFE_INTEGER: the frontend
// stores offsets as JSON numbers, so anything larger could not round-trip.
const maxSafeInteger = 1<<53 - 1

// Document is the on-device reading-progress document: a shelf key maps to a set
// of book ids, each carrying a UTF-16 character offset. It never reaches the
// server and is not authoritative shelf data.
type Document struct {
	Version int                         `json:"version"`
	Shelves map[string]map[string]int64 `json:"shelves"`
}

// New returns an empty, current-version document.
func New() Document {
	return Document{Version: DocumentVersion, Shelves: map[string]map[string]int64{}}
}

// Parse reads a stored document. A missing, corrupt, wrongly shaped, or
// future-version document yields an empty document rather than an error, so a
// bad file can never keep a reader from opening — this mirrors the frontend
// parser's leniency.
func Parse(text string) Document {
	doc := New()

	if strings.TrimSpace(text) == "" {
		return doc
	}

	var raw struct {
		Version int                           `json:"version"`
		Shelves map[string]map[string]float64 `json:"shelves"`
	}
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
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
		normalized := map[string]int64{}
		for bookID, offset := range books {
			trimmedID := strings.TrimSpace(bookID)
			value, ok := normalizeOffset(offset)
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
	if strings.TrimSpace(text) != "" && !json.Valid([]byte(text)) {
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
		doc.Shelves = map[string]map[string]int64{}
	}
	bs, err := json.Marshal(doc)
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	return string(bs), nil
}

// Clone returns a deep copy so callers can mutate freely without touching the
// input — the projection and merges all build their result this way, which is
// what makes them safe to call repeatedly on the same document.
func (d Document) Clone() Document {
	out := Document{Version: d.Version, Shelves: make(map[string]map[string]int64, len(d.Shelves))}
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
// real shelf holds that stable book id and, when one does, copies the offset to
// shelves[realShelfID][bookID] taking the max — reader progress can advance the
// desktop's position but never drag it backwards. Entries under readerShelfID
// are always kept: an unresolved book (removed, or a loose book outside every
// shelf) stays there untouched, so a later import can pick it up on the next
// projection. The projection is therefore idempotent: running it again changes
// nothing once every resolvable book has been folded in.
func Project(doc Document, readerShelfID string, resolve ResolveShelf) Document {
	out := doc.Clone()
	readerShelfID = strings.TrimSpace(readerShelfID)
	readerBooks := out.Shelves[readerShelfID]
	if readerShelfID == "" || len(readerBooks) == 0 || resolve == nil {
		return out
	}

	for bookID, offset := range readerBooks {
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
			dst = map[string]int64{}
			out.Shelves[realShelfID] = dst
		}
		if offset > dst[bookID] {
			dst[bookID] = offset
		}
	}

	return out
}

// MergeReaderBookWrite folds a single reader-updated book into the on-disk
// document under the reader's namespace. Only the one book the reader has open
// (bookID) is taken from the write; every other entry is preserved from disk —
// the desktop's real shelves, and any other reader book, including one a second
// reader process wrote concurrently. The book is set to the incoming offset, or
// removed when the write cleared it (a progress reset). Merging at book
// granularity is what keeps two reader processes from clobbering each other
// within the shared "book" namespace even though each rewrites the whole file.
func MergeReaderBookWrite(disk, write Document, readerShelfID, bookID string) Document {
	out := disk.Clone()
	readerShelfID = strings.TrimSpace(readerShelfID)
	bookID = strings.TrimSpace(bookID)
	if readerShelfID == "" || bookID == "" {
		return out
	}

	books := out.Shelves[readerShelfID]
	if books == nil {
		books = map[string]int64{}
	}
	if offset, ok := write.Shelves[readerShelfID][bookID]; ok {
		books[bookID] = offset
	} else {
		delete(books, bookID)
	}

	if len(books) == 0 {
		delete(out.Shelves, readerShelfID)
	} else {
		out.Shelves[readerShelfID] = books
	}
	return out
}

// MergeDesktopWrite folds a desktop-side write into the on-disk document. Every
// shelf key except the reader's is taken from the write; the reader namespace is
// kept from disk. This is how the desktop process mutates the shared file
// without ever clobbering standalone-reader progress it has not yet projected.
func MergeDesktopWrite(disk, write Document, readerShelfID string) Document {
	out := write.Clone()
	setShelf(out, readerShelfID, disk.Shelves[strings.TrimSpace(readerShelfID)])
	return out
}

func setShelf(doc Document, shelfID string, books map[string]int64) {
	shelfID = strings.TrimSpace(shelfID)
	if shelfID == "" {
		return
	}
	if len(books) == 0 {
		delete(doc.Shelves, shelfID)
		return
	}
	doc.Shelves[shelfID] = cloneBooks(books)
}

func cloneBooks(books map[string]int64) map[string]int64 {
	out := make(map[string]int64, len(books))
	for bookID, offset := range books {
		out[bookID] = offset
	}
	return out
}

// normalizeOffset keeps only the offsets the frontend would keep: positive,
// integral, and within the JSON-safe integer range.
func normalizeOffset(value float64) (int64, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	if value <= 0 || value > maxSafeInteger {
		return 0, false
	}
	if value != math.Trunc(value) {
		return 0, false
	}
	return int64(value), true
}
