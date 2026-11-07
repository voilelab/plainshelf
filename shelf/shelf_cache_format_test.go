package shelf

import (
	"os"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/fsutil"
)

func setBookFormatMirror(t *testing.T, book *Book, format string) {
	t.Helper()

	meta := book.GetMeta()
	meta.Format = format
	if err := book.SetMeta(meta); err != nil {
		t.Fatalf("SetMeta(format=%q): %v", format, err)
	}
}

func newBookWithCurrentSource(t *testing.T, s *Shelf, title string, options NewSourceOptions, content string) *Book {
	t.Helper()

	created, err := s.NewBook(nil, title)
	if err != nil {
		t.Fatalf("NewBook(%s): %v", title, err)
	}
	book, err := s.GetBook(created.ID())
	if err != nil {
		t.Fatalf("GetBook(%s): %v", title, err)
	}
	source, err := book.NewSourceWithOptions(strings.NewReader(content), options)
	if err != nil {
		t.Fatalf("NewSourceWithOptions(%s): %v", title, err)
	}
	if err := book.SetCurrentSource(source.ID()); err != nil {
		t.Fatalf("SetCurrentSource(%s): %v", title, err)
	}
	return book
}

func TestBookCacheCarriesDriftedSourceFormat(t *testing.T) {
	libRoot := t.TempDir()
	s, err := NewShelf(&ShelfConf{
		LibRoot:           libRoot,
		LockMode:          "none",
		BookCacheWriterID: testWriterID,
		BookCacheInterval: "24h",
		ScanInterval:      "24h",
	})
	if err != nil {
		t.Fatalf("NewShelf: %v", err)
	}
	if err := s.WaitReady(t.Context()); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}

	images := NewSourceOptions{Format: BookFormatImages}
	text := NewSourceOptions{Format: BookFormatText}
	agreeing := newBookWithCurrentSource(t, s, "Agreeing Comic", images, "p-a.png\n")
	comic := newBookWithCurrentSource(t, s, "Drifted Comic", images, "p-a.png\n")
	prose := newBookWithCurrentSource(t, s, "Drifted Prose", text, "prose")
	setBookFormatMirror(t, comic, BookFormatText)
	setBookFormatMirror(t, prose, BookFormatImages)

	want := map[string]string{
		agreeing.ID(): BookFormatImages,
		comic.ID():    BookFormatImages,
		prose.ID():    BookFormatText,
	}
	for id := range want {
		s.RefreshBookCharCount(id)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	cache := waitForBookCacheExport(t, libRoot, testWriterID)
	exported := map[string]string{
		agreeing.ID(): "",
		comic.ID():    BookFormatImages,
		prose.ID():    BookFormatText,
	}
	for id, format := range exported {
		if got := cache.Books[id].CurrentSourceFormat; got != format {
			t.Errorf("%s: exported current_source_format = %q, want %q", id, got, format)
		}
	}

	rt, err := os.OpenRoot(libRoot)
	if err != nil {
		t.Fatalf("OpenRoot: %v", err)
	}
	t.Cleanup(func() { rt.Close() })
	remote, err := NewShelfFromFS(&ShelfConf{}, fsutil.NewRootFS(rt), "")
	if err != nil {
		t.Fatalf("NewShelfFromFS: %v", err)
	}
	t.Cleanup(func() { remote.Close() })
	if err := remote.WaitReady(t.Context()); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}

	listings, err := remote.ListBooksWithCharCount()
	if err != nil {
		t.Fatalf("ListBooksWithCharCount: %v", err)
	}
	for _, listing := range listings {
		if got := listing.SourceFormat; got != want[listing.Book.ID()] {
			t.Errorf("%s: remote SourceFormat = %q, want %q", listing.Book.GetMeta().Title, got, want[listing.Book.ID()])
		}
	}
	if len(listings) != len(want) {
		t.Errorf("remote listed %d books, want %d", len(listings), len(want))
	}
}
