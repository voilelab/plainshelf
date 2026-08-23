package shelf

import (
	"path"
	"strings"
	"testing"
	"time"
)

// newCountingTestShelf builds a shelf whose intervals are long enough that no
// scan or per-book refresh runs behind these tests: what the cache holds must
// be decided by the calls under test, not by a timer.
func newCountingTestShelf(t *testing.T) *Shelf {
	t.Helper()

	return newTestShelf(t, &ShelfConf{
		LibRoot:      path.Join(t.TempDir(), "shelf_test"),
		ScanInterval: "1h",
	})
}

// newBookWithContent creates a book whose current source holds content.
func newBookWithContent(t *testing.T, s *Shelf, title, content string) *Book {
	t.Helper()

	book, err := s.NewBookWith(nil, title, func(book *Book) error {
		source, err := book.NewSource(strings.NewReader(content))
		if err != nil {
			return err
		}
		return book.SetCurrentSource(source.ID())
	})
	if err != nil {
		t.Fatalf("NewBookWith: %v", err)
	}
	return book
}

// listedCharCount returns the character count the listing reports for one book.
func listedCharCount(t *testing.T, s *Shelf, bookID string) int {
	t.Helper()

	listings, err := s.ListBooksWithCharCount()
	if err != nil {
		t.Fatalf("ListBooksWithCharCount: %v", err)
	}

	for _, listing := range listings {
		if listing.Book.ID() == bookID {
			return listing.CharCount
		}
	}

	t.Fatalf("book %q is missing from the listing", bookID)
	return 0
}

// A book that has just been created is listed with its character count, and a
// book without a readable source is listed with 0 rather than failing the whole
// listing.
func TestListBooksWithCharCountReportsTheCurrentSource(t *testing.T) {
	s := newCountingTestShelf(t)

	counted := newBookWithContent(t, s, "Counted", "abcde")
	sourceless, err := s.NewBook(nil, "Sourceless")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}

	if got := listedCharCount(t, s, counted.ID()); got != 5 {
		t.Errorf("char count of a book with content = %d, want 5", got)
	}
	if got := listedCharCount(t, s, sourceless.ID()); got != 0 {
		t.Errorf("char count of a book without a current source = %d, want 0", got)
	}
}

// The listing answers from the book cache and never opens a source per book, so
// a count written straight through *Source stays invisible until the entry is
// refreshed. This is what every request path that rewrites source content has to
// call RefreshBookCharCount for.
func TestRefreshBookCharCountFollowsASourceContentUpdate(t *testing.T) {
	s := newCountingTestShelf(t)

	book := newBookWithContent(t, s, "Rewritten", "abcde")

	source, err := book.GetSource(book.CurrentSource())
	if err != nil {
		t.Fatalf("GetSource: %v", err)
	}
	if err := source.UpdateContent(strings.NewReader("abcdefgh")); err != nil {
		t.Fatalf("UpdateContent: %v", err)
	}

	// The count lives in the source's own meta.json, so the write above left
	// book.json untouched and Book.IsStale cannot see it.
	if got := listedCharCount(t, s, book.ID()); got != 5 {
		t.Errorf("char count before the refresh = %d, want the cached 5", got)
	}

	s.RefreshBookCharCount(book.ID())

	if got := listedCharCount(t, s, book.ID()); got != 8 {
		t.Errorf("char count after the refresh = %d, want 8", got)
	}
}

// Moving the current-source pointer changes which count the listing reports.
func TestRefreshBookCharCountFollowsTheCurrentSourcePointer(t *testing.T) {
	s := newCountingTestShelf(t)

	book := newBookWithContent(t, s, "Two Sources", "abcde")

	longer, err := book.NewSource(strings.NewReader("abcdefgh"))
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	if err := book.SetCurrentSource(longer.ID()); err != nil {
		t.Fatalf("SetCurrentSource: %v", err)
	}

	s.RefreshBookCharCount(book.ID())

	if got := listedCharCount(t, s, book.ID()); got != 8 {
		t.Errorf("char count after switching source = %d, want 8", got)
	}
}

// A book the cache does not hold has no entry to refresh, and asking for one
// must not create or resurrect it.
func TestRefreshBookCharCountIgnoresAnUncachedBook(t *testing.T) {
	s := newCountingTestShelf(t)

	book := newBookWithContent(t, s, "Deleted", "abcde")
	if err := s.DeleteBook(book.ID()); err != nil {
		t.Fatalf("DeleteBook: %v", err)
	}

	s.RefreshBookCharCount(book.ID())

	listings, err := s.ListBooksWithCharCount()
	if err != nil {
		t.Fatalf("ListBooksWithCharCount: %v", err)
	}
	for _, listing := range listings {
		if listing.Book.ID() == book.ID() {
			t.Fatalf("refreshing an uncached book put it back in the listing")
		}
	}
}

// A full scan fills the counts, so a shelf written from outside PlainShelf is
// listed with the counts its own source files carry.
func TestScanFillsCharCountsFromTheShelf(t *testing.T) {
	s := newCountingTestShelf(t)

	book := newBookWithContent(t, s, "Scanned", "abcde")

	if err := s.scanToBookCache(); err != nil {
		t.Fatalf("scanToBookCache: %v", err)
	}

	if got := listedCharCount(t, s, book.ID()); got != 5 {
		t.Errorf("char count after a scan = %d, want 5", got)
	}
}

// A full scan reads each book early in its walk and publishes the whole map at
// the end, so a source rewritten in between is already refreshed by the time
// the scan lands. The scan's older reading of that file must not undo it: a
// character count is written without touching book.json, so the per-book
// staleness check could never correct it and the shelf would answer with the
// pre-rewrite count until the next scan.
func TestScanKeepsACharCountObservedAfterItsOwnRead(t *testing.T) {
	s := newCountingTestShelf(t)

	book := newBookWithContent(t, s, "Raced", "abcde")

	// Stands in for a refresh that lands while the walk below is still running:
	// a count observed after the walk read the same book's source.
	s.bookCache.Lock()
	refreshed := *s.bookCache.cache[book.ID()]
	refreshed.charCount = 99
	refreshed.charCountAt = time.Now().Add(time.Minute)
	s.bookCache.cache[book.ID()] = &refreshed
	s.bookCache.Unlock()

	if err := s.scanToBookCache(); err != nil {
		t.Fatalf("scanToBookCache: %v", err)
	}

	if got := listedCharCount(t, s, book.ID()); got != 99 {
		t.Errorf("char count after the scan published its older reading = %d, want the newer 99", got)
	}
}

// The same overlap the other way round: the walk publishes first, so the entry
// the refresh read is gone by the time it has a count to store. The count still
// belongs to the book, so it lands on whatever entry is published instead of
// being dropped.
func TestRefreshBookCharCountAppliesToAReplacedEntry(t *testing.T) {
	s := newCountingTestShelf(t)

	book := newBookWithContent(t, s, "Replaced", "abcde")

	// What a walk that read this book before the rewrite below would publish.
	stale := newBookIDCacheEntry(bookFolderPath(book.PackagePath()), book.PackagePath(), book)

	source, err := book.GetSource(book.CurrentSource())
	if err != nil {
		t.Fatalf("GetSource: %v", err)
	}
	if err := source.UpdateContent(strings.NewReader("abcdefgh")); err != nil {
		t.Fatalf("UpdateContent: %v", err)
	}

	s.bookCache.Lock()
	s.bookCache.cache[book.ID()] = stale
	s.bookCache.Unlock()

	s.RefreshBookCharCount(book.ID())

	if got := listedCharCount(t, s, book.ID()); got != 8 {
		t.Errorf("char count after refreshing a replaced entry = %d, want 8", got)
	}
}
