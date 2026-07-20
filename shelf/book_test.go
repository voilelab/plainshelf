package shelf

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/voilelab/plainshelf/internal/fsutil"
)

func TestGetBook(t *testing.T) {
	testdataRoot, err := os.OpenRoot("testdata")
	if err != nil {
		t.Fatalf("Failed to open testdata root: %v", err)
	}
	defer testdataRoot.Close()

	rootFS := fsutil.NewRootFS(testdataRoot)
	book, err := openBook(rootFS, newLoggerForTest(), "book-a82m")
	if err != nil {
		t.Fatalf("Failed to open book: %v", err)
	}

	expectedTitle := "Book Title"
	if book.Title() != expectedTitle {
		t.Errorf("Expected book title '%s', got '%s'", expectedTitle, book.Title())
	}

	_, err = openBook(rootFS, newLoggerForTest(), "nonexistent-book")
	if err == nil {
		t.Fatalf("Expected error when getting nonexistent book, but got none")
	}
}

func TestGetBookCover(t *testing.T) {
	testdataRoot, err := os.OpenRoot("testdata")
	if err != nil {
		t.Fatalf("Failed to open testdata root: %v", err)
	}
	defer testdataRoot.Close()

	rootFS := fsutil.NewRootFS(testdataRoot)
	book, err := openBook(rootFS, newLoggerForTest(), "book-a82m")
	if err != nil {
		t.Fatalf("Failed to open book: %v", err)
	}

	coverData, _, err := book.OpenCover()
	if err != nil {
		t.Fatalf("Failed to get book cover: %v", err)
	}

	expectedCoverData := []byte{0x89, 0x50, 0x4E, 0x47} // PNG file signature
	if len(coverData) < 4 || !bytes.Equal(coverData[:4], expectedCoverData) {
		t.Errorf("Expected cover data to start with PNG signature, got %v", coverData[:4])
	}
}

func TestGetSource(t *testing.T) {
	testdataRoot, err := os.OpenRoot("testdata")
	if err != nil {
		t.Fatalf("Failed to open testdata root: %v", err)
	}
	defer testdataRoot.Close()

	rootFS := fsutil.NewRootFS(testdataRoot)
	book, err := openBook(rootFS, newLoggerForTest(), "book-a82m")
	if err != nil {
		t.Fatalf("Failed to open book: %v", err)
	}

	source, err := book.GetSource("20260315-a1")
	if err != nil {
		t.Fatalf("Failed to get source: %v", err)
	}

	expectedSourceID := "20260315-a1"
	if source.ID() != expectedSourceID {
		t.Errorf("Expected source ID '%s', got '%s'", expectedSourceID, source.ID())
	}

	_, err = book.GetSource("nonexistent-source")
	if err == nil {
		t.Fatalf("Expected error when getting nonexistent source, but got none")
	}
}

func TestGetCurrentSource(t *testing.T) {
	testdataRoot, err := os.OpenRoot("testdata")
	if err != nil {
		t.Fatalf("Failed to open testdata root: %v", err)
	}
	defer testdataRoot.Close()

	rootFS := fsutil.NewRootFS(testdataRoot)
	book, err := openBook(rootFS, newLoggerForTest(), "book-a82m")
	if err != nil {
		t.Fatalf("Failed to open book: %v", err)
	}

	currentSourceID := book.CurrentSource()
	expectedSourceID := "20260315-a1"
	if currentSourceID != expectedSourceID {
		t.Errorf("Expected current source ID '%s', got '%s'", expectedSourceID, currentSourceID)
	}
}

func TestListSources(t *testing.T) {
	testdataRoot, err := os.OpenRoot("testdata")
	if err != nil {
		t.Fatalf("Failed to open testdata root: %v", err)
	}
	defer testdataRoot.Close()

	rootFS := fsutil.NewRootFS(testdataRoot)
	book, err := openBook(rootFS, newLoggerForTest(), "book-a82m")
	if err != nil {
		t.Fatalf("Failed to open book: %v", err)
	}

	sources, err := book.ListSource()
	if err != nil {
		t.Fatalf("Failed to list sources: %v", err)
	}

	if len(sources) != 1 {
		t.Fatalf("Expected 1 source, got %d", len(sources))
	}

	expectedSourceID := "20260315-a1"
	if sources[0].ID() != expectedSourceID {
		t.Errorf("Expected source ID '%s', got '%s'", expectedSourceID, sources[0].ID())
	}
}

func TestNewBook(t *testing.T) {
	tmpLib := path.Join(t.TempDir())
	tmpRoot, err := os.OpenRoot(tmpLib)
	if err != nil {
		t.Fatalf("Failed to open temporary root: %v", err)
	}
	defer tmpRoot.Close()

	bookID := "test-book-a38j"
	title := "Test Book"

	rootFS := fsutil.NewRootFS(tmpRoot)
	book, err := createBook(rootFS, newLoggerForTest(), bookID, bookID, title)
	if err != nil {
		t.Fatalf("Failed to create new book: %v", err)
	}

	if book.ID() != bookID {
		t.Errorf("Expected book ID '%s', got '%s'", bookID, book.ID())
	}

	if book.Title() != title {
		t.Errorf("Expected book title '%s', got '%s'", title, book.Title())
	}

	// Check if the book folder was created
	bookPath := path.Join(tmpLib, bookID)
	if _, err := os.Open(bookPath); err != nil {
		t.Fatalf("Expected book folder to be created, but got error: %v", err)
	}
}

func TestSetCover(t *testing.T) {
	tmpLib := path.Join(t.TempDir())
	tmpRoot, err := os.OpenRoot(tmpLib)
	if err != nil {
		t.Fatalf("Failed to open temporary root: %v", err)
	}
	defer tmpRoot.Close()

	bookID := "test-book-a38j"
	title := "Test Book"

	rootFS := fsutil.NewRootFS(tmpRoot)
	book, err := createBook(rootFS, newLoggerForTest(), bookID, bookID, title)
	if err != nil {
		t.Fatalf("Failed to create new book: %v", err)
	}

	coverData := []byte{0x89, 0x50, 0x4E, 0x47} // PNG file signature
	err = book.SetCover(coverData, ".png")
	if err != nil {
		t.Fatalf("Failed to set book cover: %v", err)
	}

	retrievedCoverData, _, err := book.OpenCover()
	if err != nil {
		t.Fatalf("Failed to get book cover: %v", err)
	}

	if !bytes.Equal(retrievedCoverData, coverData) {
		t.Errorf("Expected retrieved cover data to match set cover data, got %v", retrievedCoverData)
	}

	// Check if the cover file was created
	coverPath := path.Join(tmpLib, bookID, "cover.png")
	if _, err := os.Open(coverPath); err != nil {
		t.Fatalf("Expected cover file to be created, but got error: %v", err)
	}
}

func TestDeleteCoverAndETag(t *testing.T) {
	tmpLib := t.TempDir()
	tmpRoot, err := os.OpenRoot(tmpLib)
	if err != nil {
		t.Fatalf("OpenRoot: %v", err)
	}
	defer tmpRoot.Close()

	rootFS := fsutil.NewRootFS(tmpRoot)
	book, err := createBook(rootFS, newLoggerForTest(), "test-book", "test-book", "Test Book")
	if err != nil {
		t.Fatalf("createBook: %v", err)
	}
	if etag := book.CoverETag(); etag != "" {
		t.Fatalf("ETag without cover = %q, want empty", etag)
	}
	if err := book.SetCover([]byte("cover bytes"), ".png"); err != nil {
		t.Fatalf("SetCover: %v", err)
	}
	if etag := book.CoverETag(); etag == "" {
		t.Fatal("ETag with cover is empty")
	}

	if err := book.DeleteCover(); err != nil {
		t.Fatalf("DeleteCover: %v", err)
	}
	if book.GetMeta().Cover != "" {
		t.Fatalf("cover metadata = %q, want empty", book.GetMeta().Cover)
	}
	data, ext, err := book.OpenCover()
	if err != nil {
		t.Fatalf("OpenCover after delete: %v", err)
	}
	if data != nil || ext != "" {
		t.Fatalf("OpenCover after delete = (%v, %q), want (nil, empty)", data, ext)
	}
	if _, err := os.Stat(path.Join(tmpLib, "test-book", "cover.png")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("deleted cover stat error = %v, want os.ErrNotExist", err)
	}
	if err := book.DeleteCover(); err != nil {
		t.Fatalf("second DeleteCover: %v", err)
	}
}

func TestNewLayersFromString(t *testing.T) {
	tests := []struct {
		input string
		want  Layers
	}{
		{input: "", want: nil},
		{input: "fiction", want: Layers{"fiction"}},
		{input: " fiction / classics ", want: Layers{"fiction", "classics"}},
	}
	for _, tt := range tests {
		if got := NewLayersFromString(tt.input); !got.Equal(tt.want) {
			t.Errorf("NewLayersFromString(%q) = %#v, want %#v", tt.input, got, tt.want)
		}
	}
}

func TestNewSource(t *testing.T) {
	tmpLib := path.Join(t.TempDir())
	tmpRoot, err := os.OpenRoot(tmpLib)
	if err != nil {
		t.Fatalf("Failed to open temporary root: %v", err)
	}
	defer tmpRoot.Close()

	bookID := "test-book-a38j"
	title := "Test Book"

	rootFS := fsutil.NewRootFS(tmpRoot)
	book, err := createBook(rootFS, newLoggerForTest(), bookID, bookID, title)
	if err != nil {
		t.Fatalf("Failed to create new book: %v", err)
	}

	srcText := "This is the content of the source."
	source, err := book.NewSource(bytes.NewReader([]byte(srcText)))
	if err != nil {
		t.Fatalf("Failed to create new source: %v", err)
	}

	retrievedSource, err := book.GetSource(source.ID())
	if err != nil {
		t.Fatalf("Failed to get source: %v", err)
	}

	getSrc, err := retrievedSource.Open()
	if err != nil {
		t.Fatalf("Failed to open source: %v", err)
	}

	retrievedSrcData, err := io.ReadAll(getSrc)
	if err != nil {
		t.Fatalf("Failed to read source data: %v", err)
	}

	if string(retrievedSrcData) != srcText {
		t.Errorf("Expected retrieved source to match original source, got '%s'", string(retrievedSrcData))
	}
}

// TestNewSourceSameSecond verifies that multiple sources created within the same
// second get distinct IDs instead of overwriting each other. The source ID is a
// second-granularity timestamp, so without collision handling these would all
// resolve to the same folder.
func TestNewSourceSameSecond(t *testing.T) {
	tmpLib := path.Join(t.TempDir())
	tmpRoot, err := os.OpenRoot(tmpLib)
	if err != nil {
		t.Fatalf("Failed to open temporary root: %v", err)
	}
	defer tmpRoot.Close()

	bookID := "test-book-a38j"
	rootFS := fsutil.NewRootFS(tmpRoot)
	book, err := createBook(rootFS, newLoggerForTest(), bookID, bookID, "Test Book")
	if err != nil {
		t.Fatalf("Failed to create new book: %v", err)
	}

	const n = 5
	wantContent := make(map[string]string, n)
	for i := range n {
		text := fmt.Sprintf("content-%d", i)
		src, err := book.NewSource(bytes.NewReader([]byte(text)))
		if err != nil {
			t.Fatalf("Failed to create source %d: %v", i, err)
		}
		if _, dup := wantContent[src.ID()]; dup {
			t.Fatalf("Duplicate source ID returned: %s", src.ID())
		}
		wantContent[src.ID()] = text
	}

	sources, err := book.ListSource()
	if err != nil {
		t.Fatalf("Failed to list sources: %v", err)
	}
	if len(sources) != n {
		t.Fatalf("Expected %d sources, got %d (sources overwrote each other)", n, len(sources))
	}

	for _, s := range sources {
		f, err := s.Open()
		if err != nil {
			t.Fatalf("Failed to open source %s: %v", s.ID(), err)
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			t.Fatalf("Failed to read source %s: %v", s.ID(), err)
		}
		if want := wantContent[s.ID()]; string(data) != want {
			t.Errorf("Source %s: expected content %q, got %q", s.ID(), want, string(data))
		}
	}
}

func TestNewSourceNil(t *testing.T) {
	tmpLib := path.Join(t.TempDir())
	tmpRoot, err := os.OpenRoot(tmpLib)
	if err != nil {
		t.Fatalf("Failed to open temporary root: %v", err)
	}
	defer tmpRoot.Close()

	bookID := "test-book-a38j"
	title := "Test Book"

	rootFS := fsutil.NewRootFS(tmpRoot)
	book, err := createBook(rootFS, newLoggerForTest(), bookID, bookID, title)
	if err != nil {
		t.Fatalf("Failed to create new book: %v", err)
	}

	source, err := book.NewSource(nil)
	if err != nil {
		t.Fatalf("Failed to create new source with nil: %v", err)
	}

	if source.ID() == "" {
		t.Error("Expected non-empty source ID")
	}

	retrievedSource, err := book.GetSource(source.ID())
	if err != nil {
		t.Fatalf("Failed to get source: %v", err)
	}

	getSrc, err := retrievedSource.Open()
	if err != nil {
		t.Fatalf("Failed to open source: %v", err)
	}

	retrievedSrcData, err := io.ReadAll(getSrc)
	if err != nil {
		t.Fatalf("Failed to read source data: %v", err)
	}

	if len(retrievedSrcData) != 0 {
		t.Errorf("Expected empty source content, got %q", string(retrievedSrcData))
	}
}

func TestDeleteSource(t *testing.T) {
	tmpLib := path.Join(t.TempDir())
	tmpRoot, err := os.OpenRoot(tmpLib)
	if err != nil {
		t.Fatalf("Failed to open temporary root: %v", err)
	}
	defer tmpRoot.Close()

	bookID := "test-book-a38j"
	title := "Test Book"

	rootFS := fsutil.NewRootFS(tmpRoot)
	book, err := createBook(rootFS, newLoggerForTest(), bookID, bookID, title)
	if err != nil {
		t.Fatalf("Failed to create new book: %v", err)
	}

	source, err := book.NewSource(bytes.NewReader([]byte("some content")))
	if err != nil {
		t.Fatalf("Failed to create new source: %v", err)
	}
	sourceID := source.ID()

	err = book.DeleteSource(sourceID)
	if err != nil {
		t.Fatalf("Failed to delete source: %v", err)
	}

	_, err = book.GetSource(sourceID)
	if err == nil {
		t.Fatal("Expected error when getting deleted source, but got none")
	}
}

func TestDeleteSourceNonexistent(t *testing.T) {
	tmpLib := path.Join(t.TempDir())
	tmpRoot, err := os.OpenRoot(tmpLib)
	if err != nil {
		t.Fatalf("Failed to open temporary root: %v", err)
	}
	defer tmpRoot.Close()

	rootFS := fsutil.NewRootFS(tmpRoot)
	book, err := createBook(rootFS, newLoggerForTest(), "test-book-a38j", "test-book-a38j", "Test Book")
	if err != nil {
		t.Fatalf("Failed to create new book: %v", err)
	}

	err = book.DeleteSource("nonexistent-source")
	if err == nil {
		t.Fatal("Expected error when deleting nonexistent source, but got none")
	}
}

func TestSetCurrentSource(t *testing.T) {
	tmpLib := path.Join(t.TempDir())
	tmpRoot, err := os.OpenRoot(tmpLib)
	if err != nil {
		t.Fatalf("Failed to open temporary root: %v", err)
	}
	defer tmpRoot.Close()

	bookID := "test-book-a38j"
	title := "Test Book"

	rootFS := fsutil.NewRootFS(tmpRoot)
	book, err := createBook(rootFS, newLoggerForTest(), bookID, bookID, title)
	if err != nil {
		t.Fatalf("Failed to create new book: %v", err)
	}

	srcText := "This is the content of the source."
	source, err := book.NewSource(bytes.NewReader([]byte(srcText)))
	if err != nil {
		t.Fatalf("Failed to create new source: %v", err)
	}

	err = book.SetCurrentSource(source.ID())
	if err != nil {
		t.Fatalf("Failed to set current source: %v", err)
	}

	if book.CurrentSource() != source.ID() {
		t.Errorf("Expected current source ID to be '%s', got '%s'", source.ID(), book.CurrentSource())
	}

	srcText2 := "This is the content of the second source."
	source2, err := book.NewSource(bytes.NewReader([]byte(srcText2)))
	if err != nil {
		t.Fatalf("Failed to create second source: %v", err)
	}

	err = book.SetCurrentSource(source2.ID())
	if err != nil {
		t.Fatalf("Failed to set current source: %v", err)
	}

	if book.CurrentSource() != source2.ID() {
		t.Errorf("Expected current source ID to be '%s', got '%s'", source2.ID(), book.CurrentSource())
	}

	// Set current source back to the first source
	err = book.SetCurrentSource(source.ID())
	if err != nil {
		t.Fatalf("Failed to set current source: %v", err)
	}

	if book.CurrentSource() != source.ID() {
		t.Errorf("Expected current source ID to be '%s', got '%s'", source.ID(), book.CurrentSource())
	}
}

func TestSetMetaMarksOtherInstanceStale(t *testing.T) {
	tmpLib := path.Join(t.TempDir())
	tmpRoot, err := os.OpenRoot(tmpLib)
	if err != nil {
		t.Fatalf("Failed to open temporary root: %v", err)
	}
	defer tmpRoot.Close()

	bookID := "test-book-a38j"
	title := "Test Book"

	rootFS := fsutil.NewRootFS(tmpRoot)
	book1, err := createBook(rootFS, newLoggerForTest(), bookID, bookID, title)
	if err != nil {
		t.Fatalf("Failed to create new book: %v", err)
	}

	book2, err := openBook(rootFS, newLoggerForTest(), bookID)
	if err != nil {
		t.Fatalf("Failed to open second book instance: %v", err)
	}

	if book1.IsStale() {
		t.Fatalf("Expected first instance to be fresh initially")
	}
	if book2.IsStale() {
		t.Fatalf("Expected second instance to be fresh initially")
	}

	meta := book1.GetMeta()
	meta.Comments = "updated by book1"

	// Ensure filesystem mtime has advanced on platforms with coarse timestamp precision.
	time.Sleep(time.Until(time.Now().Truncate(time.Second).Add(time.Second)))
	err = book1.SetMeta(meta)
	if err != nil {
		t.Fatalf("Failed to set book meta from first instance: %v", err)
	}

	if book1.IsStale() {
		t.Fatalf("Expected first instance to remain fresh after SetMeta")
	}
	if !book2.IsStale() {
		t.Fatalf("Expected second instance to become stale after first instance updates meta")
	}
}

// TestSetMetaMigratesLegacyPublishedAt verifies the lazy migration of the
// published_at field: a book.json still holding a full RFC3339 timestamp
// (as written by older versions using JSONTime) loads as a date, and the next
// SetMeta persists it back in date-only form.
func TestSetMetaMigratesLegacyPublishedAt(t *testing.T) {
	tmpLib := t.TempDir()
	bookID := "legacy-book-a38j"
	bookDir := path.Join(tmpLib, bookID)
	if err := os.MkdirAll(bookDir, 0o755); err != nil {
		t.Fatalf("Failed to create book dir: %v", err)
	}

	legacyMeta := `{
  "id": "legacy-book-a38j",
  "title": "Legacy Book",
  "cover": "",
  "authors": [],
  "language": "en",
  "comments": "",
  "star": 0,
  "published_at": "2026-03-15T08:30:00Z",
  "current_source": ""
}`
	if err := os.WriteFile(path.Join(bookDir, BookMetaFile), []byte(legacyMeta), 0o644); err != nil {
		t.Fatalf("Failed to write legacy book.json: %v", err)
	}

	tmpRoot, err := os.OpenRoot(tmpLib)
	if err != nil {
		t.Fatalf("Failed to open temporary root: %v", err)
	}
	defer tmpRoot.Close()

	rootFS := fsutil.NewRootFS(tmpRoot)
	book, err := openBook(rootFS, newLoggerForTest(), bookID)
	if err != nil {
		t.Fatalf("Failed to open legacy book: %v", err)
	}

	meta := book.GetMeta()
	if y, m, d := time.Time(meta.PublishedAt).Date(); y != 2026 || m != time.March || d != 15 {
		t.Errorf("Expected PublishedAt 2026-03-15, got %04d-%02d-%02d", y, m, d)
	}

	if err := book.SetMeta(meta); err != nil {
		t.Fatalf("Failed to set meta: %v", err)
	}

	written, err := os.ReadFile(path.Join(bookDir, BookMetaFile))
	if err != nil {
		t.Fatalf("Failed to read back book.json: %v", err)
	}
	if !strings.Contains(string(written), `"published_at": "2026-03-15"`) {
		t.Errorf("Expected date-only published_at in written book.json, got:\n%s", written)
	}
	if strings.Contains(string(written), "2026-03-15T") {
		t.Errorf("Written book.json still contains a full timestamp:\n%s", written)
	}
}

// TestIdentifiersRoundTrip verifies that Identifiers survive a SetMeta →
// persist → reopen cycle, and that GetMeta returns an independent copy of
// the map so mutating it does not leak back into the book's internal state.
func TestIdentifiersRoundTrip(t *testing.T) {
	tmpLib := path.Join(t.TempDir())
	tmpRoot, err := os.OpenRoot(tmpLib)
	if err != nil {
		t.Fatalf("Failed to open temporary root: %v", err)
	}
	defer tmpRoot.Close()

	bookID := "test-book-a38j"
	title := "Test Book"

	rootFS := fsutil.NewRootFS(tmpRoot)
	book, err := createBook(rootFS, newLoggerForTest(), bookID, bookID, title)
	if err != nil {
		t.Fatalf("Failed to create new book: %v", err)
	}

	meta := book.GetMeta()
	meta.Identifiers = map[string]string{"isbn": "978-0-13-468599-1", "douban": "12345"}
	if err := book.SetMeta(meta); err != nil {
		t.Fatalf("Failed to set book meta with identifiers: %v", err)
	}

	// Mutating the map returned by GetMeta must not affect the book's internal state.
	returned := book.GetMeta()
	returned.Identifiers["isbn"] = "mutated"
	fresh := book.GetMeta()
	if fresh.Identifiers["isbn"] != "978-0-13-468599-1" {
		t.Fatalf("Mutating the map returned by GetMeta leaked into internal state: %#v", fresh.Identifiers)
	}

	// Reopen the book from disk and confirm the identifiers persisted.
	reopened, err := openBook(rootFS, newLoggerForTest(), bookID)
	if err != nil {
		t.Fatalf("Failed to reopen book: %v", err)
	}
	reopenedMeta := reopened.GetMeta()
	if len(reopenedMeta.Identifiers) != 2 || reopenedMeta.Identifiers["isbn"] != "978-0-13-468599-1" || reopenedMeta.Identifiers["douban"] != "12345" {
		t.Fatalf("Identifiers did not round-trip through disk, got: %#v", reopenedMeta.Identifiers)
	}
}

// TestIdentifiersLegacyCompat verifies that a book.json written before the
// identifiers field existed still opens successfully (Identifiers is nil),
// and that a subsequent SetMeta without identifiers does not introduce an
// "identifiers" key into the persisted JSON.
func TestIdentifiersLegacyCompat(t *testing.T) {
	tmpLib := t.TempDir()
	bookID := "legacy-book-b91k"
	bookDir := path.Join(tmpLib, bookID)
	if err := os.MkdirAll(bookDir, 0o755); err != nil {
		t.Fatalf("Failed to create book dir: %v", err)
	}

	legacyMeta := `{
  "id": "legacy-book-b91k",
  "title": "Legacy Book",
  "cover": "",
  "authors": [],
  "language": "en",
  "comments": "",
  "star": 0,
  "current_source": ""
}`
	if err := os.WriteFile(path.Join(bookDir, BookMetaFile), []byte(legacyMeta), 0o644); err != nil {
		t.Fatalf("Failed to write legacy book.json: %v", err)
	}

	tmpRoot, err := os.OpenRoot(tmpLib)
	if err != nil {
		t.Fatalf("Failed to open temporary root: %v", err)
	}
	defer tmpRoot.Close()

	rootFS := fsutil.NewRootFS(tmpRoot)
	book, err := openBook(rootFS, newLoggerForTest(), bookID)
	if err != nil {
		t.Fatalf("Failed to open legacy book: %v", err)
	}

	meta := book.GetMeta()
	if meta.Identifiers != nil {
		t.Fatalf("Expected nil Identifiers for legacy book.json, got: %#v", meta.Identifiers)
	}

	if err := book.SetMeta(meta); err != nil {
		t.Fatalf("Failed to set meta: %v", err)
	}

	written, err := os.ReadFile(path.Join(bookDir, BookMetaFile))
	if err != nil {
		t.Fatalf("Failed to read back book.json: %v", err)
	}
	if strings.Contains(string(written), "identifiers") {
		t.Errorf("Expected written book.json to omit identifiers key, got:\n%s", written)
	}
}

// TestSetMetaRejectsEmptyIdentifierKey verifies that SetMeta rejects
// identifiers with a blank (or whitespace-only) key.
func TestSetMetaRejectsEmptyIdentifierKey(t *testing.T) {
	tmpLib := path.Join(t.TempDir())
	tmpRoot, err := os.OpenRoot(tmpLib)
	if err != nil {
		t.Fatalf("Failed to open temporary root: %v", err)
	}
	defer tmpRoot.Close()

	bookID := "test-book-a38j"
	title := "Test Book"

	rootFS := fsutil.NewRootFS(tmpRoot)
	book, err := createBook(rootFS, newLoggerForTest(), bookID, bookID, title)
	if err != nil {
		t.Fatalf("Failed to create new book: %v", err)
	}

	meta := book.GetMeta()
	meta.Identifiers = map[string]string{" ": "x"}
	if err := book.SetMeta(meta); err == nil {
		t.Fatalf("Expected error when setting identifier with empty key, got none")
	}
}
