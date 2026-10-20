package shelf

import (
	"path"
	"strings"
	"testing"
	"time"
)

func newCountingTestShelf(t *testing.T) *Shelf {
	t.Helper()

	return newTestShelf(t, &ShelfConf{
		LibRoot:      path.Join(t.TempDir(), "shelf_test"),
		ScanInterval: "1h",
	})
}

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

	if got := listedCharCount(t, s, book.ID()); got != 5 {
		t.Errorf("char count before the refresh = %d, want the cached 5", got)
	}

	s.RefreshBookCharCount(book.ID())

	if got := listedCharCount(t, s, book.ID()); got != 8 {
		t.Errorf("char count after the refresh = %d, want 8", got)
	}
}

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

func TestScanKeepsACharCountObservedAfterItsOwnRead(t *testing.T) {
	s := newCountingTestShelf(t)

	book := newBookWithContent(t, s, "Raced", "abcde")

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

func TestRefreshBookCharCountAppliesToAReplacedEntry(t *testing.T) {
	s := newCountingTestShelf(t)

	book := newBookWithContent(t, s, "Replaced", "abcde")

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
