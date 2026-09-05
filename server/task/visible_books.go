package task

import "github.com/voilelab/plainshelf/shelf"

// VisibleBooks is the set of books the request that queued a chain was allowed
// to see, snapshotted at that moment because the chain runs after its response
// and cannot ask again.
//
// It is one type rather than a field shape each sweep repeats because the
// consequence of getting it wrong is the same everywhere: a chain that walks
// the whole shelf reports its totals, and names book IDs in its failures, for
// books the requester was answered 404 for.
//
// A nil set means the request saw everything, which is every chain on a server
// hiding nothing - and leaves such a chain exactly what it was before the
// setting existed.
type VisibleBooks map[string]bool

// Allows reports whether a chain carrying this set may touch the book. A nil
// set allows everything, so a caller with nothing to hide passes nil rather
// than a set naming every book on the shelf. That nil case is the whole reason
// this exists: a plain v[bookID] answers false for it, which would be the
// opposite of what nil means.
func (v VisibleBooks) Allows(bookID string) bool {
	return v == nil || v[bookID]
}

// Only drops the books outside the set, keeping the order it was given.
func (v VisibleBooks) Only(books []*shelf.Book) []*shelf.Book {
	if v == nil {
		return books
	}

	kept := make([]*shelf.Book, 0, len(books))
	for _, book := range books {
		if v.Allows(book.ID()) {
			kept = append(kept, book)
		}
	}
	return kept
}
