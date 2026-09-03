package bookpkg

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
)

// testdataFS opens the committed testdata tree read-only. Write-path tests must
// use newBookFromFixture instead, which copies the fixture into a temp dir.
func testdataFS(t *testing.T) fsutil.FS {
	t.Helper()

	testdataRoot, err := os.OpenRoot("testdata")
	if err != nil {
		t.Fatalf("Failed to open testdata root: %v", err)
	}
	t.Cleanup(func() { testdataRoot.Close() })

	return fsutil.NewRootFS(testdataRoot)
}

// The getters all read one committed fixture, so they share a single open
// rather than repeating the same preamble five times.
func TestOpenBookReadsTheFixture(t *testing.T) {
	const sourceID = "20260315-a1"

	rootFS := testdataFS(t)
	book, err := Open(rootFS, newLoggerForTest(), "book-a82m")
	if err != nil {
		t.Fatalf("Failed to open book: %v", err)
	}

	t.Run("title", func(t *testing.T) {
		if expected := "Book Title"; book.Title() != expected {
			t.Errorf("Expected book title '%s', got '%s'", expected, book.Title())
		}
	})

	t.Run("a missing book is an error", func(t *testing.T) {
		if _, err := Open(rootFS, newLoggerForTest(), "nonexistent-book"); err == nil {
			t.Fatalf("Expected error when getting nonexistent book, but got none")
		}
	})

	t.Run("cover", func(t *testing.T) {
		coverData, _, err := book.OpenCover()
		if err != nil {
			t.Fatalf("Failed to get book cover: %v", err)
		}

		expectedCoverData := []byte{0x89, 0x50, 0x4E, 0x47} // PNG file signature
		if len(coverData) < 4 || !bytes.Equal(coverData[:4], expectedCoverData) {
			t.Errorf("Expected cover data to start with PNG signature, got %v", coverData[:4])
		}
	})

	t.Run("source by id", func(t *testing.T) {
		source, err := book.GetSource(sourceID)
		if err != nil {
			t.Fatalf("Failed to get source: %v", err)
		}
		if source.ID() != sourceID {
			t.Errorf("Expected source ID '%s', got '%s'", sourceID, source.ID())
		}

		if _, err := book.GetSource("nonexistent-source"); err == nil {
			t.Fatalf("Expected error when getting nonexistent source, but got none")
		}
	})

	t.Run("current source", func(t *testing.T) {
		if got := book.CurrentSource(); got != sourceID {
			t.Errorf("Expected current source ID '%s', got '%s'", sourceID, got)
		}
	})

	t.Run("list sources", func(t *testing.T) {
		sources, err := book.ListSource()
		if err != nil {
			t.Fatalf("Failed to list sources: %v", err)
		}

		if len(sources) != 1 {
			t.Fatalf("Expected 1 source, got %d", len(sources))
		}
		if sources[0].ID() != sourceID {
			t.Errorf("Expected source ID '%s', got '%s'", sourceID, sources[0].ID())
		}
	})
}

// newTestBook creates a book in a fresh temporary library. It returns the book,
// the FS rooted at that library, and the library's path; the book's directory is
// path.Join(tmpLib, bookID).
func newTestBook(t *testing.T, bookID, title string) (*Book, fsutil.FS, string) {
	t.Helper()

	tmpLib := t.TempDir()
	tmpRoot, err := os.OpenRoot(tmpLib)
	if err != nil {
		t.Fatalf("Failed to open temporary root: %v", err)
	}
	t.Cleanup(func() { tmpRoot.Close() })

	rootFS := fsutil.NewRootFS(tmpRoot)
	book, err := Create(rootFS, newLoggerForTest(), bookID, bookID, title)
	if err != nil {
		t.Fatalf("Failed to create new book: %v", err)
	}

	return book, rootFS, tmpLib
}

func TestNewBook(t *testing.T) {
	bookID := "test-book-a38j"
	title := "Test Book"

	book, _, tmpLib := newTestBook(t, bookID, title)

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

// The cover write/read round trip is covered by
// TestSetCoverKeepsFileWhenExtensionUnchanged and TestDeleteCoverAndETag.

// writableRoot narrows a book's read handle back to a writable one so a test
// can wrap it in a fault-injecting filesystem.
func writableRoot(t *testing.T, root fsutil.ReadFS) fsutil.FS {
	t.Helper()

	writable, err := fsutil.Writable(root)
	if err != nil {
		t.Fatalf("Writable: %v", err)
	}
	return writable
}

// failWriteFS fails WriteFile for paths matching a predicate, standing in for a
// write that is interrupted partway through.
type failWriteFS struct {
	fsutil.FS
	failOn func(name string) bool
}

var errInjectedWrite = errors.New("injected write failure")

func (f *failWriteFS) WriteFile(name string, data []byte) error {
	if f.failOn != nil && f.failOn(name) {
		return errInjectedWrite
	}
	return f.FS.WriteFile(name, data)
}

// assertNoTempFiles fails the test if any *.tmp file survived in dir.
func assertNoTempFiles(t *testing.T, dir string) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%q): %v", dir, err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Errorf("temp file left behind in %s: %s", dir, entry.Name())
		}
	}
}

func newBookForCoverTest(t *testing.T) (*Book, string, string) {
	t.Helper()

	const bookID = "cover-book"
	book, _, tmpLib := newTestBook(t, bookID, "Cover Book")

	return book, tmpLib, path.Join(tmpLib, bookID)
}

// A failed cover write must not damage the cover already on disk. Before the
// atomic write the destination was opened with O_TRUNC, so an interrupted write
// left a truncated image behind.
func TestSetCoverLeavesPreviousCoverIntactOnFailure(t *testing.T) {
	book, _, bookDir := newBookForCoverTest(t)

	original := []byte{0x89, 0x50, 0x4E, 0x47, 0x01, 0x02}
	if err := book.SetCover(original, ".png"); err != nil {
		t.Fatalf("SetCover: %v", err)
	}

	book.root = &failWriteFS{
		FS:     writableRoot(t, book.root),
		failOn: func(name string) bool { return strings.Contains(path.Base(name), "cover") },
	}

	err := book.SetCover([]byte("replacement image data"), ".png")
	if !errors.Is(err, errInjectedWrite) {
		t.Fatalf("SetCover error = %v, want %v", err, errInjectedWrite)
	}

	data, err := os.ReadFile(path.Join(bookDir, "cover.png"))
	if err != nil {
		t.Fatalf("ReadFile cover.png: %v", err)
	}
	if !bytes.Equal(data, original) {
		t.Errorf("cover on disk = %v, want the original bytes %v", data, original)
	}
	if got := book.GetMeta().Cover; got != "cover.png" {
		t.Errorf("meta cover = %q, want %q", got, "cover.png")
	}
	assertNoTempFiles(t, bookDir)
}

// The API converts uploads to JPEG, so replacing a PNG cover changes the file
// name. The old file is then referenced by nobody and must not linger in a
// shelf that users browse by hand.
func TestSetCoverRemovesReplacedCoverWithDifferentExtension(t *testing.T) {
	book, _, bookDir := newBookForCoverTest(t)

	if err := book.SetCover([]byte("png data"), ".png"); err != nil {
		t.Fatalf("SetCover png: %v", err)
	}

	jpegData := []byte("jpeg data")
	if err := book.SetCover(jpegData, ".jpg"); err != nil {
		t.Fatalf("SetCover jpg: %v", err)
	}

	if _, err := os.Stat(path.Join(bookDir, "cover.png")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("cover.png stat error = %v, want os.ErrNotExist", err)
	}

	data, err := os.ReadFile(path.Join(bookDir, "cover.jpg"))
	if err != nil {
		t.Fatalf("ReadFile cover.jpg: %v", err)
	}
	if !bytes.Equal(data, jpegData) {
		t.Errorf("cover.jpg = %q, want %q", data, jpegData)
	}
	if got := book.GetMeta().Cover; got != "cover.jpg" {
		t.Errorf("meta cover = %q, want %q", got, "cover.jpg")
	}
	assertNoTempFiles(t, bookDir)
}

// afterRenameFS runs a hook once, immediately after a rename to a matching
// path, so a test can interleave a concurrent writer at an exact point.
type afterRenameFS struct {
	fsutil.FS
	match func(newPath string) bool
	on    func()
	once  sync.Once
}

func (f *afterRenameFS) Rename(oldPath, newPath string) error {
	err := f.FS.Rename(oldPath, newPath)
	if err == nil && f.match(newPath) {
		f.once.Do(f.on)
	}
	return err
}

// Two overlapping cover uploads with different extensions can interleave so
// that the second one re-claims the first one's old file name. The cleanup of
// the replaced cover must not delete the image the book now points to.
func TestSetCoverKeepsReplacedCoverReclaimedByAnotherWriter(t *testing.T) {
	book, _, bookDir := newBookForCoverTest(t)

	if err := book.SetCover([]byte("original png"), ".png"); err != nil {
		t.Fatalf("SetCover png: %v", err)
	}

	// A second handle on the same book, standing in for a concurrent request.
	other, err := Open(book.root, newLoggerForTest(), book.folderPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	const reclaimed = "reclaimed png"

	// Fire the competing PNG upload right after this call's book.json rename,
	// which lands it between our metadata write and our cleanup.
	book.root = &afterRenameFS{
		FS:    writableRoot(t, book.root),
		match: func(newPath string) bool { return path.Base(newPath) == BookMetaFile },
		on: func() {
			if err := other.SetCover([]byte(reclaimed), ".png"); err != nil {
				t.Errorf("concurrent SetCover png: %v", err)
			}
		},
	}

	if err := book.SetCover([]byte("jpeg data"), ".jpg"); err != nil {
		t.Fatalf("SetCover jpg: %v", err)
	}

	data, err := os.ReadFile(path.Join(bookDir, "cover.png"))
	if err != nil {
		t.Fatalf("cover.png was deleted while the book still pointed at it: %v", err)
	}
	if string(data) != reclaimed {
		t.Errorf("cover.png = %q, want %q", data, reclaimed)
	}

	persisted, err := readBookMeta(other.root, other.folderPath)
	if err != nil {
		t.Fatalf("readBookMeta: %v", err)
	}
	if persisted.Cover != "cover.png" {
		t.Fatalf("persisted cover = %q, want %q", persisted.Cover, "cover.png")
	}
	if _, err := os.Stat(path.Join(bookDir, persisted.Cover)); err != nil {
		t.Errorf("book.json points at %q but it is missing: %v", persisted.Cover, err)
	}
}

func TestSetCoverKeepsFileWhenExtensionUnchanged(t *testing.T) {
	book, _, bookDir := newBookForCoverTest(t)

	if err := book.SetCover([]byte("first"), ".png"); err != nil {
		t.Fatalf("SetCover first: %v", err)
	}
	if err := book.SetCover([]byte("second"), ".png"); err != nil {
		t.Fatalf("SetCover second: %v", err)
	}

	data, err := os.ReadFile(path.Join(bookDir, "cover.png"))
	if err != nil {
		t.Fatalf("ReadFile cover.png: %v", err)
	}
	if string(data) != "second" {
		t.Errorf("cover.png = %q, want %q", data, "second")
	}
	if got := book.GetMeta().Cover; got != "cover.png" {
		t.Errorf("meta cover = %q, want %q", got, "cover.png")
	}
	assertNoTempFiles(t, bookDir)
}

func TestDeleteCoverAndETag(t *testing.T) {
	book, _, tmpLib := newTestBook(t, "test-book", "Test Book")

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

func TestNewSource(t *testing.T) {
	bookID := "test-book-a38j"
	title := "Test Book"

	book, rootFS, _ := newTestBook(t, bookID, title)

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
	if book.CurrentSource() != "" {
		t.Fatalf("creating a non-current source changed current source to %q", book.CurrentSource())
	}
	if _, err := rootFS.Stat(path.Join(book.PackagePath(), CurrentSourceHintFile)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("creating a non-current source wrote current pointer: %v", err)
	}
}

// TestNewSourceSameSecond verifies that multiple sources created within the same
// second get distinct IDs instead of overwriting each other. The source ID is a
// second-granularity timestamp, so without collision handling these would all
// resolve to the same folder.
func TestNewSourceSameSecond(t *testing.T) {
	book, _, _ := newTestBook(t, "test-book-a38j", "Test Book")

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
	bookID := "test-book-a38j"
	title := "Test Book"

	book, _, _ := newTestBook(t, bookID, title)

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
	bookID := "test-book-a38j"
	title := "Test Book"

	book, _, _ := newTestBook(t, bookID, title)

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
	book, _, _ := newTestBook(t, "test-book-a38j", "Test Book")

	err := book.DeleteSource("nonexistent-source")
	if err == nil {
		t.Fatal("Expected error when deleting nonexistent source, but got none")
	}
}

func TestSetCurrentSource(t *testing.T) {
	bookID := "test-book-a38j"
	title := "Test Book"

	book, _, _ := newTestBook(t, bookID, title)

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
	if book.GetMeta().Format != BookFormatText {
		t.Errorf("compatibility format = %q, want txt", book.GetMeta().Format)
	}

	srcText2 := "This is the content of the second source."
	source2, err := book.NewSourceWithOptions(bytes.NewReader([]byte(srcText2)), NewSourceOptions{Format: BookFormatMarkdown})
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
	if book.GetMeta().Format != BookFormatMarkdown {
		t.Errorf("compatibility format = %q, want md", book.GetMeta().Format)
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

// TestCurrentSourceHintFile covers the disposable pointer file: its name, that
// its content is English and points at the current source, and that a shelf
// written by an older build ends up with one hint file rather than two.
func TestCurrentSourceHintFile(t *testing.T) {
	book, rootFS, _ := newTestBook(t, "hint-book", "Hint Book")

	source, err := book.NewSource(bytes.NewBufferString("first"))
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}

	// Pretend an older build already wrote its hint under the previous name.
	legacyPath := path.Join(book.PackagePath(), LegacyCurrentSourceHintFile)
	if err := rootFS.WriteFile(legacyPath, []byte("stale pointer")); err != nil {
		t.Fatalf("Failed to seed the legacy hint file: %v", err)
	}

	if err := book.SetCurrentSource(source.ID()); err != nil {
		t.Fatalf("SetCurrentSource: %v", err)
	}

	hint, err := fs.ReadFile(rootFS, path.Join(book.PackagePath(), CurrentSourceHintFile))
	if err != nil {
		t.Fatalf("Failed to read %s: %v", CurrentSourceHintFile, err)
	}
	wantPath := path.Join(SourcesFolder, source.ID(), SourceFile)
	if !strings.Contains(string(hint), wantPath) {
		t.Errorf("%s does not point at %s, got:\n%s", CurrentSourceHintFile, wantPath, hint)
	}
	if r, found := firstCJKRune(string(hint)); found {
		t.Errorf("%s contains the non-English character %q:\n%s", CurrentSourceHintFile, r, hint)
	}
	if !strings.Contains(string(hint), "book.json") {
		t.Errorf("%s should name book.json as the source of truth, got:\n%s", CurrentSourceHintFile, hint)
	}
	if !strings.Contains(string(hint), "safely delete") {
		t.Errorf("%s should say it can be deleted, got:\n%s", CurrentSourceHintFile, hint)
	}

	if _, err := rootFS.Stat(legacyPath); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Legacy %s must be removed once the new hint is written, got err %v", LegacyCurrentSourceHintFile, err)
	}
}

func firstCJKRune(s string) (rune, bool) {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) ||
			unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hangul, r) {
			return r, true
		}
	}
	return 0, false
}

func TestListSourceSkipsUnpublishedTemporaryDirectories(t *testing.T) {
	book, _, tmpLib := newTestBook(t, "temp-source-book", "Temporary Source")
	source, err := book.NewSource(bytes.NewBufferString("published"))
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}

	tempDir := path.Join(tmpLib, "temp-source-book", SourcesFolder, "."+source.ID()+"-crash.tmp")
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		t.Fatalf("MkdirAll temp source: %v", err)
	}
	metaBytes, err := json.Marshal(source.GetMeta())
	if err != nil {
		t.Fatalf("Marshal source meta: %v", err)
	}
	if err := os.WriteFile(path.Join(tempDir, SourceMetaFile), metaBytes, 0o644); err != nil {
		t.Fatalf("write temp meta: %v", err)
	}
	if err := os.WriteFile(path.Join(tempDir, SourceFile), []byte("unpublished"), 0o644); err != nil {
		t.Fatalf("write temp content: %v", err)
	}

	sources, err := book.ListSource()
	if err != nil {
		t.Fatalf("ListSource: %v", err)
	}
	if len(sources) != 1 || sources[0].ID() != source.ID() {
		t.Fatalf("ListSource = %#v, want only published source %q", sources, source.ID())
	}
}

func TestSetMetaMarksOtherInstanceStale(t *testing.T) {
	bookID := "test-book-a38j"
	title := "Test Book"

	_, rootFS, tmpLib := newTestBook(t, bookID, title)

	// SetMeta writes through the API, so its timestamp cannot be forced after
	// the fact. Backdate the file instead and open both instances against that,
	// which leaves the write below unambiguously newer than what they cached.
	shiftModTime(t, path.Join(tmpLib, bookID, BookMetaFile), -2*time.Second)

	book1, err := Open(rootFS, newLoggerForTest(), bookID)
	if err != nil {
		t.Fatalf("Failed to open first book instance: %v", err)
	}
	book2, err := Open(rootFS, newLoggerForTest(), bookID)
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
	book, err := Open(rootFS, newLoggerForTest(), bookID)
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
	bookID := "test-book-a38j"
	title := "Test Book"

	book, rootFS, _ := newTestBook(t, bookID, title)

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
	reopened, err := Open(rootFS, newLoggerForTest(), bookID)
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
	book, err := Open(rootFS, newLoggerForTest(), bookID)
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
	bookID := "test-book-a38j"
	title := "Test Book"

	book, _, _ := newTestBook(t, bookID, title)

	meta := book.GetMeta()
	meta.Identifiers = map[string]string{" ": "x"}
	if err := book.SetMeta(meta); err == nil {
		t.Fatalf("Expected error when setting identifier with empty key, got none")
	}
}

// newBookFromFixture copies a committed book.json fixture into a fresh temp
// directory so write-path tests never mutate the repository's testdata.
// It returns the FS rooted at the temp library, the book's folder name, and
// the absolute path of the copied book.json.
func newBookFromFixture(t *testing.T, fixture string) (fsutil.FS, string, string) {
	t.Helper()

	original, err := os.ReadFile(path.Join("testdata", fixture, BookMetaFile))
	if err != nil {
		t.Fatalf("Failed to read fixture %s: %v", fixture, err)
	}

	tmpLib := t.TempDir()
	bookFolder := "fixture-book"
	bookDir := path.Join(tmpLib, bookFolder)
	if err := os.MkdirAll(bookDir, 0o755); err != nil {
		t.Fatalf("Failed to create book dir: %v", err)
	}

	metaPath := path.Join(bookDir, BookMetaFile)
	if err := os.WriteFile(metaPath, original, 0o644); err != nil {
		t.Fatalf("Failed to write fixture book.json: %v", err)
	}

	tmpRoot, err := os.OpenRoot(tmpLib)
	if err != nil {
		t.Fatalf("Failed to open temporary root: %v", err)
	}
	t.Cleanup(func() { tmpRoot.Close() })

	return fsutil.NewRootFS(tmpRoot), bookFolder, metaPath
}

// TestOpenBookNormalizesMissingSchemaVersion verifies that a pre-v1 book.json
// (written before schema_version existed) is read as v1 in memory, and that
// merely opening it does not rewrite the file on disk. The "no write on read"
// property is what keeps a large legacy shelf from being rewritten wholesale
// on first launch after an upgrade.
func TestOpenBookNormalizesMissingSchemaVersion(t *testing.T) {
	fixturePath := path.Join("testdata", "schema", "v0-full", BookMetaFile)
	before, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("Failed to read fixture: %v", err)
	}

	book, err := Open(testdataFS(t), newLoggerForTest(), path.Join("schema", "v0-full"))
	if err != nil {
		t.Fatalf("Failed to open legacy book: %v", err)
	}

	if got := book.GetMeta().SchemaVersion; got != BookMetaSchemaVersion {
		t.Errorf("Expected normalized SchemaVersion %d, got %d", BookMetaSchemaVersion, got)
	}

	after, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("Failed to re-read fixture: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("Opening a book must not rewrite book.json on disk")
	}
	if strings.Contains(string(after), "schema_version") {
		t.Errorf("Fixture must stay pre-v1 on disk, got:\n%s", after)
	}
}

// TestOpenBookSparseLegacyBookHasSchemaVersion verifies the normalization also
// applies to the minimal legacy book.json shape that omits most fields.
func TestOpenBookSparseLegacyBookHasSchemaVersion(t *testing.T) {
	book, err := Open(testdataFS(t), newLoggerForTest(), "book-a82m")
	if err != nil {
		t.Fatalf("Failed to open book: %v", err)
	}

	meta := book.GetMeta()
	if meta.SchemaVersion != BookMetaSchemaVersion {
		t.Errorf("Expected SchemaVersion %d, got %d", BookMetaSchemaVersion, meta.SchemaVersion)
	}
	if meta.Title != "Book Title" {
		t.Errorf("Expected title 'Book Title', got '%s'", meta.Title)
	}
	if meta.CurrentSource != "20260315-a1" {
		t.Errorf("Expected current_source '20260315-a1', got '%s'", meta.CurrentSource)
	}
}

// TestSetMetaStampsSchemaVersionOnLegacyBook verifies the lazy v0 → v1 upgrade:
// the version is written on the next SetMeta, lands as the first JSON key, and
// no user-visible field is lost in the process.
func TestSetMetaStampsSchemaVersionOnLegacyBook(t *testing.T) {
	rootFS, bookFolder, metaPath := newBookFromFixture(t, path.Join("schema", "v0-full"))

	book, err := Open(rootFS, newLoggerForTest(), bookFolder)
	if err != nil {
		t.Fatalf("Failed to open legacy book: %v", err)
	}

	if err := book.SetMeta(book.GetMeta()); err != nil {
		t.Fatalf("Failed to set meta: %v", err)
	}

	written, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("Failed to read back book.json: %v", err)
	}

	if !strings.HasPrefix(string(written), "{\n  \"schema_version\": 1,") {
		t.Errorf("Expected schema_version as the first key, got:\n%s", written)
	}

	// Every user-visible field from the fixture must survive the upgrade.
	for _, want := range []string{
		`"id": "schema-v0-full-a1"`,
		`"title": "Full Legacy Book"`,
		`"format": "txt"`,
		`"legacy"`,
		`"isbn": "978-0-13-468599-1"`,
		`"cover": "cover.png"`,
		`"A. Legacy"`,
		`"language": "en"`,
		`"comments": "written by PlainShelf v0.8"`,
		`"star": 4`,
		`"created_at": "2026-03-15T08:30:00Z"`,
		`"published_at": "2026-03-15"`,
		`"current_source": "20260315-a1"`,
	} {
		if !strings.Contains(string(written), want) {
			t.Errorf("Upgraded book.json lost %s, got:\n%s", want, written)
		}
	}
}

// TestCreateBookWritesSchemaVersion verifies new books are born at v1.
func TestCreateBookWritesSchemaVersion(t *testing.T) {
	tmpLib := t.TempDir()
	tmpRoot, err := os.OpenRoot(tmpLib)
	if err != nil {
		t.Fatalf("Failed to open temporary root: %v", err)
	}
	defer tmpRoot.Close()

	bookID := "fresh-book-a38j"
	rootFS := fsutil.NewRootFS(tmpRoot)
	book, err := Create(rootFS, newLoggerForTest(), bookID, bookID, "Fresh Book")
	if err != nil {
		t.Fatalf("Failed to create new book: %v", err)
	}

	if got := book.GetMeta().SchemaVersion; got != BookMetaSchemaVersion {
		t.Errorf("Expected in-memory SchemaVersion %d, got %d", BookMetaSchemaVersion, got)
	}

	written, err := os.ReadFile(path.Join(tmpLib, bookID, BookMetaFile))
	if err != nil {
		t.Fatalf("Failed to read back book.json: %v", err)
	}
	if !strings.HasPrefix(string(written), "{\n  \"schema_version\": 1,") {
		t.Errorf("Expected schema_version as the first key, got:\n%s", written)
	}
}

// TestOpenBookAcceptsFutureSchemaVersionAsReadOnly verifies that a book.json
// newer than this build is still readable. Failing the open would make the book
// vanish from listings and, worse, become impossible to restore from trash.
func TestOpenBookAcceptsFutureSchemaVersionAsReadOnly(t *testing.T) {
	book, err := Open(testdataFS(t), newLoggerForTest(), path.Join("schema", "v2-future"))
	if err != nil {
		t.Fatalf("Expected a future-version book to open, got error: %v", err)
	}

	if book.ID() != "schema-v2-c3" {
		t.Errorf("Expected ID 'schema-v2-c3', got '%s'", book.ID())
	}
	if book.Title() != "Book From The Future" {
		t.Errorf("Expected title 'Book From The Future', got '%s'", book.Title())
	}
	// The reported version must not be clamped down to what this build writes,
	// otherwise the write guard and any client-side "needs upgrade" hint break.
	if got := book.GetMeta().SchemaVersion; got != 2 {
		t.Errorf("Expected SchemaVersion 2 to be preserved, got %d", got)
	}
}

// writeFutureBookSource plants a source this build can write next to a book.json
// it cannot, so a refused DeleteSource can be shown to leave the text alone.
func writeFutureBookSource(t *testing.T, metaPath, sourceID string) {
	t.Helper()

	sourceDir := path.Join(path.Dir(metaPath), SourcesFolder, sourceID)
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("Failed to create source dir: %v", err)
	}
	if err := os.WriteFile(path.Join(sourceDir, SourceFile), []byte("source body"), 0o644); err != nil {
		t.Fatalf("Failed to write source body: %v", err)
	}

	raw, err := json.Marshal(&SourceMeta{
		SchemaVersion: SourceMetaSchemaVersion,
		ID:            sourceID,
		CreatedAt:     util.JSONTime(time.Now()),
		Format:        BookFormatText,
	})
	if err != nil {
		t.Fatalf("Failed to marshal source meta: %v", err)
	}
	if err := os.WriteFile(path.Join(sourceDir, SourceMetaFile), raw, 0o644); err != nil {
		t.Fatalf("Failed to write source meta: %v", err)
	}
}

// Every write path must refuse a book.json produced by a newer build, and the
// refusal must leave that file byte-for-byte intact. What differs per path is
// only what else must stay untouched, so each case carries its own setup and
// its own extra check; the shared skeleton is stated once here.
func TestWritePathsRejectFutureSchemaVersion(t *testing.T) {
	const coverName = "cover.png"
	const futureSourceID = "20260315-a1"
	originalCover := []byte("original-cover-bytes")

	writeCover := func(t *testing.T, metaPath string) {
		t.Helper()
		if err := os.WriteFile(path.Join(path.Dir(metaPath), coverName), originalCover, 0o644); err != nil {
			t.Fatalf("Failed to write cover: %v", err)
		}
	}
	assertCoverIntact := func(t *testing.T, metaPath string) {
		t.Helper()
		after, err := os.ReadFile(path.Join(path.Dir(metaPath), coverName))
		if err != nil {
			t.Fatalf("Refused write must not remove the cover: %v", err)
		}
		if !bytes.Equal(originalCover, after) {
			t.Errorf("Cover contents changed after a refused write, got %q", after)
		}
	}

	tests := []struct {
		name string
		// setup prepares state the refused write must not damage.
		setup  func(t *testing.T, metaPath string)
		mutate func(*Book) error
		// check asserts the path-specific side effect never happened.
		check func(t *testing.T, metaPath string)
	}{
		{
			name: "SetMeta",
			mutate: func(b *Book) error {
				meta := b.GetMeta()
				meta.Title = "Clobbered"
				return b.SetMeta(meta)
			},
			check: func(t *testing.T, metaPath string) {
				t.Helper()
				after, err := os.ReadFile(metaPath)
				if err != nil {
					t.Fatalf("Failed to re-read book.json: %v", err)
				}
				// A key this build does not understand must survive untouched.
				if !strings.Contains(string(after), "reading_direction") {
					t.Errorf("Unknown key must survive a rejected write, got:\n%s", after)
				}
				if _, err := os.Stat(metaPath + ".tmp"); !os.IsNotExist(err) {
					t.Errorf("Rejected write must not leave a temp meta file behind")
				}
			},
		},
		{
			// setMeta is the single chokepoint, so the CURRENT_SOURCE.txt
			// hint must not be written either.
			name:   "SetCurrentSource",
			mutate: func(b *Book) error { return b.SetCurrentSource("20260315-a1") },
			check: func(t *testing.T, metaPath string) {
				t.Helper()
				hintPath := path.Join(path.Dir(metaPath), CurrentSourceHintFile)
				if _, err := os.Stat(hintPath); !os.IsNotExist(err) {
					t.Errorf("Rejected SetCurrentSource must not write %s", CurrentSourceHintFile)
				}
			},
		},
		{
			// Deleting the current source hands the pointer over first, which
			// writes book.json, so the whole operation has to be refused — and
			// refusing it must leave the source's text where it was.
			name: "DeleteSource",
			setup: func(t *testing.T, metaPath string) {
				t.Helper()
				writeFutureBookSource(t, metaPath, futureSourceID)

				raw, err := os.ReadFile(metaPath)
				if err != nil {
					t.Fatalf("Failed to read book.json: %v", err)
				}
				withCurrent := strings.Replace(string(raw), `"current_source": ""`,
					`"current_source": "`+futureSourceID+`"`, 1)
				if withCurrent == string(raw) {
					t.Fatalf("Fixture did not contain an empty current_source field to patch")
				}
				if err := os.WriteFile(metaPath, []byte(withCurrent), 0o644); err != nil {
					t.Fatalf("Failed to write book.json: %v", err)
				}
			},
			mutate: func(b *Book) error { return b.DeleteSource(futureSourceID) },
			check: func(t *testing.T, metaPath string) {
				t.Helper()
				sourcePath := path.Join(path.Dir(metaPath), SourcesFolder, futureSourceID, SourceFile)
				if _, err := os.Stat(sourcePath); err != nil {
					t.Errorf("Rejected DeleteSource must not remove the source: %v", err)
				}
			},
		},
		{
			// SetCover truncates the target on open, so a guard that only ran
			// at SetMeta would destroy an existing cover and then report failure.
			name:   "SetCover",
			setup:  writeCover,
			mutate: func(b *Book) error { return b.SetCover([]byte("replacement-bytes"), ".png") },
			check:  assertCoverIntact,
		},
		{
			// DeleteCover deletes first, so a late guard would lose the image
			// and leave book.json pointing at a missing file.
			name: "DeleteCover",
			setup: func(t *testing.T, metaPath string) {
				t.Helper()
				writeCover(t, metaPath)

				// Point the future-version book at that cover.
				raw, err := os.ReadFile(metaPath)
				if err != nil {
					t.Fatalf("Failed to read book.json: %v", err)
				}
				withCover := strings.Replace(string(raw), `"cover": ""`, `"cover": "`+coverName+`"`, 1)
				if withCover == string(raw) {
					t.Fatalf("Fixture did not contain an empty cover field to patch")
				}
				if err := os.WriteFile(metaPath, []byte(withCover), 0o644); err != nil {
					t.Fatalf("Failed to write book.json: %v", err)
				}
			},
			mutate: func(b *Book) error { return b.DeleteCover() },
			check:  assertCoverIntact,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootFS, bookFolder, metaPath := newBookFromFixture(t, path.Join("schema", "v2-future"))

			if tt.setup != nil {
				tt.setup(t, metaPath)
			}

			// Read after setup so a case that patches book.json still compares
			// against what was actually on disk when the book was opened.
			before, err := os.ReadFile(metaPath)
			if err != nil {
				t.Fatalf("Failed to read copied book.json: %v", err)
			}

			book, err := Open(rootFS, newLoggerForTest(), bookFolder)
			if err != nil {
				t.Fatalf("Failed to open future book: %v", err)
			}

			err = tt.mutate(book)
			if err == nil {
				t.Fatalf("Expected %s to reject a future schema version, got none", tt.name)
			}
			if !errors.Is(err, ErrUnsupportedBookSchemaVersion) {
				t.Errorf("Expected ErrUnsupportedBookSchemaVersion, got %v", err)
			}

			after, err := os.ReadFile(metaPath)
			if err != nil {
				t.Fatalf("Failed to re-read book.json: %v", err)
			}
			if !bytes.Equal(before, after) {
				t.Errorf("Rejected write must leave book.json untouched, got:\n%s", after)
			}

			tt.check(t, metaPath)
		})
	}
}

// TestSetMetaRejectsStarOutOfRange verifies that the star bounds are enforced
// here rather than by callers, and that the refusal is reported as a sentinel
// so an API layer can map it to a status without matching on the message.
func TestSetMetaRejectsStarOutOfRange(t *testing.T) {
	tmpLib := path.Join(t.TempDir())
	tmpRoot, err := os.OpenRoot(tmpLib)
	if err != nil {
		t.Fatalf("Failed to open temporary root: %v", err)
	}
	defer tmpRoot.Close()

	rootFS := fsutil.NewRootFS(tmpRoot)
	bookID := "test-book-star"
	book, err := Create(rootFS, newLoggerForTest(), bookID, bookID, "Test Book")
	if err != nil {
		t.Fatalf("Failed to create new book: %v", err)
	}

	for _, star := range []int{MinStar - 1, MaxStar + 1} {
		meta := book.GetMeta()
		meta.Star = star

		if err := book.SetMeta(meta); !errors.Is(err, ErrInvalidStar) {
			t.Fatalf("star %d: err = %v, want ErrInvalidStar", star, err)
		}

		// The rejection happens before anything is written.
		if got := book.GetMeta().Star; got == star {
			t.Fatalf("star %d was persisted despite being rejected", star)
		}
	}

	for _, star := range []int{MinStar, MaxStar} {
		meta := book.GetMeta()
		meta.Star = star

		if err := book.SetMeta(meta); err != nil {
			t.Fatalf("star %d: %v", star, err)
		}
		if got := book.GetMeta().Star; got != star {
			t.Fatalf("star = %d, want %d", got, star)
		}
	}
}

// TestSetMetaRejectsInvalidLanguageTag verifies that a malformed language tag
// is refused as a sentinel, so an API layer can answer it as a client error
// rather than as an unexplained failure.
func TestSetMetaRejectsInvalidLanguageTag(t *testing.T) {
	tmpLib := path.Join(t.TempDir())
	tmpRoot, err := os.OpenRoot(tmpLib)
	if err != nil {
		t.Fatalf("Failed to open temporary root: %v", err)
	}
	defer tmpRoot.Close()

	rootFS := fsutil.NewRootFS(tmpRoot)
	bookID := "test-book-lang"
	book, err := Create(rootFS, newLoggerForTest(), bookID, bookID, "Test Book")
	if err != nil {
		t.Fatalf("Failed to create new book: %v", err)
	}

	meta := book.GetMeta()
	meta.Language = "!!!not-a-tag"
	if err := book.SetMeta(meta); !errors.Is(err, ErrInvalidLanguageTag) {
		t.Fatalf("err = %v, want ErrInvalidLanguageTag", err)
	}
	if got := book.GetMeta().Language; got == "!!!not-a-tag" {
		t.Fatal("the rejected language tag was persisted")
	}

	// An empty tag means "unknown" and stays allowed.
	for _, lang := range []string{"", "zh-Hant"} {
		meta := book.GetMeta()
		meta.Language = lang
		if err := book.SetMeta(meta); err != nil {
			t.Fatalf("language %q: %v", lang, err)
		}
		if got := book.GetMeta().Language; got != lang {
			t.Fatalf("language = %q, want %q", got, lang)
		}
	}
}

// TestSetMetaFormat covers switching a book between the two stored formats,
// which is what makes an import's format guess correctable. The empty value has
// to stay writable: books created through the API never had a format, and
// refusing it would make every unrelated edit to such a book fail.
func TestSetMetaFormat(t *testing.T) {
	tmpLib := path.Join(t.TempDir())
	tmpRoot, err := os.OpenRoot(tmpLib)
	if err != nil {
		t.Fatalf("Failed to open temporary root: %v", err)
	}
	defer tmpRoot.Close()

	rootFS := fsutil.NewRootFS(tmpRoot)
	bookID := "test-book-format"
	book, err := Create(rootFS, newLoggerForTest(), bookID, bookID, "Test Book")
	if err != nil {
		t.Fatalf("Failed to create new book: %v", err)
	}

	for _, format := range []string{BookFormatMarkdown, BookFormatText, ""} {
		meta := book.GetMeta()
		meta.Format = format

		if err := book.SetMeta(meta); err != nil {
			t.Fatalf("format %q: %v", format, err)
		}
		if got := book.GetMeta().Format; got != format {
			t.Fatalf("format = %q, want %q", got, format)
		}
	}

	meta := book.GetMeta()
	meta.Format = "epub"
	if err := book.SetMeta(meta); !errors.Is(err, ErrInvalidBookFormat) {
		t.Fatalf("err = %v, want ErrInvalidBookFormat", err)
	}
	if got := book.GetMeta().Format; got == "epub" {
		t.Fatal("the rejected format was persisted")
	}
}

// breakCurrentSource removes the book's current source behind DeleteSource's
// back, reproducing the dangling current_source a hand edit or a sync tool can
// leave. book.json keeps naming the source that is now gone.
func breakCurrentSource(t *testing.T, book *Book, rootFS fsutil.FS) string {
	t.Helper()

	danglingID := book.CurrentSource()
	if danglingID == "" {
		t.Fatal("breakCurrentSource needs a book with a current source")
	}
	if err := rootFS.RemoveAll(path.Join(book.PackagePath(), SourcesFolder, danglingID)); err != nil {
		t.Fatalf("Failed to remove current source folder: %v", err)
	}
	if book.CurrentSource() != danglingID {
		t.Fatalf("current_source = %q, want it left dangling at %q", book.CurrentSource(), danglingID)
	}
	return danglingID
}

func TestDeleteCurrentSourcePromotesLatestSurvivor(t *testing.T) {
	book, _, _ := newTestBook(t, "delete-current-book", "Delete Current")

	var ids []string
	for i := range 3 {
		source, err := book.NewSource(bytes.NewBufferString(fmt.Sprintf("body %d", i)))
		if err != nil {
			t.Fatalf("NewSource %d: %v", i, err)
		}
		ids = append(ids, source.ID())
	}
	// Delete the middle one so "promote the newest" is distinguishable from
	// both "promote the first" and "promote whatever ReadDir returned first".
	if err := book.SetCurrentSource(ids[1]); err != nil {
		t.Fatalf("SetCurrentSource: %v", err)
	}

	if err := book.DeleteSource(ids[1]); err != nil {
		t.Fatalf("DeleteSource: %v", err)
	}

	newest := ids[len(ids)-1]
	if got := book.CurrentSource(); got != newest {
		t.Errorf("current_source = %q, want the newest survivor %q", got, newest)
	}
	if _, err := book.GetSource(book.CurrentSource()); err != nil {
		t.Errorf("Promoted source is not openable: %v", err)
	}
	if _, err := book.GetSource(ids[1]); !errors.Is(err, ErrSourceNotFound) {
		t.Errorf("GetSource(deleted) error = %v, want ErrSourceNotFound", err)
	}
}

func TestDeleteLastSourceCreatesEmptyReplacement(t *testing.T) {
	book, _, _ := newTestBook(t, "delete-last-book", "Delete Last")

	source, err := book.NewSourceWithOptions(bytes.NewBufferString("# Only\n\n## One\nBody"),
		NewSourceOptions{Format: BookFormatMarkdown})
	if err != nil {
		t.Fatalf("NewSourceWithOptions: %v", err)
	}
	if err := book.SetCurrentSource(source.ID()); err != nil {
		t.Fatalf("SetCurrentSource: %v", err)
	}

	if err := book.DeleteSource(source.ID()); err != nil {
		t.Fatalf("DeleteSource: %v", err)
	}

	sources, err := book.ListSource()
	if err != nil {
		t.Fatalf("ListSource: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("ListSource returned %d sources, want the empty replacement only", len(sources))
	}
	replacement := sources[0]
	if replacement.ID() == source.ID() {
		t.Errorf("Replacement reused the deleted source ID %q", source.ID())
	}
	if got := book.CurrentSource(); got != replacement.ID() {
		t.Errorf("current_source = %q, want the replacement %q", got, replacement.ID())
	}

	content, err := replacement.Open()
	if err != nil {
		t.Fatalf("Open replacement: %v", err)
	}
	defer content.Close()
	body, err := io.ReadAll(content)
	if err != nil {
		t.Fatalf("Read replacement: %v", err)
	}
	if len(body) != 0 {
		t.Errorf("Replacement content = %q, want it empty", body)
	}
	// The replacement really is an empty plain-text source, so the book.json
	// compatibility mirror has to follow it away from Markdown.
	if got := book.GetMeta().Format; got != BookFormatText {
		t.Errorf("book format mirror = %q, want %q", got, BookFormatText)
	}
}

func TestDeleteNonCurrentSourceLeavesBookMetaUntouched(t *testing.T) {
	book, _, tmpLib := newTestBook(t, "delete-other-book", "Delete Other")

	keep, err := book.NewSource(bytes.NewBufferString("keep"))
	if err != nil {
		t.Fatalf("NewSource keep: %v", err)
	}
	drop, err := book.NewSource(bytes.NewBufferString("drop"))
	if err != nil {
		t.Fatalf("NewSource drop: %v", err)
	}
	if err := book.SetCurrentSource(keep.ID()); err != nil {
		t.Fatalf("SetCurrentSource: %v", err)
	}

	metaPath := path.Join(tmpLib, book.PackagePath(), BookMetaFile)
	before, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("Read book.json: %v", err)
	}

	if err := book.DeleteSource(drop.ID()); err != nil {
		t.Fatalf("DeleteSource: %v", err)
	}

	after, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("Re-read book.json: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("Deleting a non-current source rewrote book.json:\nbefore %s\nafter  %s", before, after)
	}
	if got := book.CurrentSource(); got != keep.ID() {
		t.Errorf("current_source = %q, want it unchanged at %q", got, keep.ID())
	}
}

func TestListSourceReturnsIDsInAscendingOrder(t *testing.T) {
	book, _, _ := newTestBook(t, "sorted-source-book", "Sorted Sources")

	for i := range 4 {
		if _, err := book.NewSource(bytes.NewBufferString(fmt.Sprintf("body %d", i))); err != nil {
			t.Fatalf("NewSource %d: %v", i, err)
		}
	}

	sources, err := book.ListSource()
	if err != nil {
		t.Fatalf("ListSource: %v", err)
	}
	for i := 1; i < len(sources); i++ {
		if sources[i-1].ID() >= sources[i].ID() {
			t.Fatalf("ListSource is not ascending: %q came before %q", sources[i-1].ID(), sources[i].ID())
		}
	}
}

func TestResolveCurrentSourceFallsBackToLatest(t *testing.T) {
	book, rootFS, tmpLib := newTestBook(t, "dangling-book", "Dangling Pointer")

	survivor, err := book.NewSource(bytes.NewBufferString("survivor body"))
	if err != nil {
		t.Fatalf("NewSource survivor: %v", err)
	}
	broken, err := book.NewSource(bytes.NewBufferString("doomed body"))
	if err != nil {
		t.Fatalf("NewSource broken: %v", err)
	}
	if err := book.SetCurrentSource(broken.ID()); err != nil {
		t.Fatalf("SetCurrentSource: %v", err)
	}

	metaPath := path.Join(tmpLib, book.PackagePath(), BookMetaFile)
	before, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("Read book.json: %v", err)
	}

	danglingID := breakCurrentSource(t, book, rootFS)

	resolved, err := book.ResolveCurrentSource()
	if err != nil {
		t.Fatalf("ResolveCurrentSource: %v", err)
	}
	if resolved.ID() != survivor.ID() {
		t.Errorf("Resolved source = %q, want the surviving source %q", resolved.ID(), survivor.ID())
	}

	// A read must never repair the shelf; only an explicit write may.
	after, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("Re-read book.json: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("ResolveCurrentSource rewrote book.json:\nbefore %s\nafter  %s", before, after)
	}
	if got := book.CurrentSource(); got != danglingID {
		t.Errorf("current_source = %q, want it left at %q", got, danglingID)
	}
}

func TestResolveCurrentSourceFallsBackOnUnreadableSource(t *testing.T) {
	book, rootFS, _ := newTestBook(t, "corrupt-source-book", "Corrupt Source")

	survivor, err := book.NewSource(bytes.NewBufferString("survivor body"))
	if err != nil {
		t.Fatalf("NewSource survivor: %v", err)
	}
	broken, err := book.NewSource(bytes.NewBufferString("broken body"))
	if err != nil {
		t.Fatalf("NewSource broken: %v", err)
	}
	if err := book.SetCurrentSource(broken.ID()); err != nil {
		t.Fatalf("SetCurrentSource: %v", err)
	}

	// A half-removed folder fails in openSource, which does not report
	// ErrSourceNotFound; the fallback must not depend on that sentinel.
	if err := rootFS.Remove(path.Join(broken.FolderPath(), SourceMetaFile)); err != nil {
		t.Fatalf("Remove source meta: %v", err)
	}

	resolved, err := book.ResolveCurrentSource()
	if err != nil {
		t.Fatalf("ResolveCurrentSource: %v", err)
	}
	if resolved.ID() != survivor.ID() {
		t.Errorf("Resolved source = %q, want the readable source %q", resolved.ID(), survivor.ID())
	}
}

func TestResolveCurrentSourceWithoutAnySource(t *testing.T) {
	// A freshly created book has no sources/ folder at all, so listing reports
	// fs.ErrNotExist rather than an empty list.
	book, _, _ := newTestBook(t, "empty-book", "No Sources")

	if _, err := book.ResolveCurrentSource(); !errors.Is(err, ErrSourceNotFound) {
		t.Errorf("ResolveCurrentSource error = %v, want ErrSourceNotFound", err)
	}
}

func TestResolveCurrentSourceWithUnsetPointer(t *testing.T) {
	book, _, _ := newTestBook(t, "unset-current-book", "Unset Current")

	source, err := book.NewSource(bytes.NewBufferString("body"))
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	if book.CurrentSource() != "" {
		t.Fatalf("Expected an unset current_source, got %q", book.CurrentSource())
	}

	// An empty pointer fails path validation rather than reporting a missing
	// source, so it has to be answered before GetSource is asked.
	resolved, err := book.ResolveCurrentSource()
	if err != nil {
		t.Fatalf("ResolveCurrentSource: %v", err)
	}
	if resolved.ID() != source.ID() {
		t.Errorf("Resolved source = %q, want %q", resolved.ID(), source.ID())
	}
}

// TestCreateWritesEmptyAuthorsArray pins the encoder decision PSW-35 deferred:
// a book with no authors carries "authors": [] on disk, not "authors": null.
// The field is not omitempty — the shelf format keeps it present — so before
// json/v2 the nil slice reached the file as null and every reader had to accept
// two spellings of "nobody".
func TestCreateWritesEmptyAuthorsArray(t *testing.T) {
	bookID := "empty-authors-a38j"
	_, _, tmpLib := newTestBook(t, bookID, "No Authors")

	written, err := os.ReadFile(path.Join(tmpLib, bookID, BookMetaFile))
	if err != nil {
		t.Fatalf("Failed to read book.json: %v", err)
	}
	if !strings.Contains(string(written), `"authors": []`) {
		t.Errorf("Expected an empty authors array in book.json, got:\n%s", written)
	}
	if strings.Contains(string(written), `"authors": null`) {
		t.Errorf("book.json still writes a null authors field:\n%s", written)
	}
}

// TestSetMetaWritesEmptyAuthorsArray is the same claim for the update path,
// which marshals a BookMeta that has been through GetMeta rather than a fresh
// one. GetMeta used to clone Authors with append precisely to keep the nil.
func TestSetMetaWritesEmptyAuthorsArray(t *testing.T) {
	bookID := "empty-authors-b91k"
	book, _, tmpLib := newTestBook(t, bookID, "No Authors")

	meta := book.GetMeta()
	meta.Authors = nil
	if err := book.SetMeta(meta); err != nil {
		t.Fatalf("Failed to set meta: %v", err)
	}

	written, err := os.ReadFile(path.Join(tmpLib, bookID, BookMetaFile))
	if err != nil {
		t.Fatalf("Failed to read book.json: %v", err)
	}
	if !strings.Contains(string(written), `"authors": []`) {
		t.Errorf("Expected an empty authors array in book.json, got:\n%s", written)
	}
}

// TestSetMetaWritesHTMLCharactersLiterally covers the other visible half of the
// json/v2 defaults. book.json's whole selling point is that a text editor shows
// what you typed, and v1 wrote &, < and > as \u0026, \u003c and \u003e — legal
// JSON, unreadable to the person the format is for.
func TestSetMetaWritesHTMLCharactersLiterally(t *testing.T) {
	const title = `Sense & Sensibility <annotated> "quoted"`

	bookID := "html-chars-c47m"
	book, rootFS, tmpLib := newTestBook(t, bookID, "Placeholder")

	meta := book.GetMeta()
	meta.Title = title
	meta.Comments = "a > b && b < c"
	if err := book.SetMeta(meta); err != nil {
		t.Fatalf("Failed to set meta: %v", err)
	}

	written, err := os.ReadFile(path.Join(tmpLib, bookID, BookMetaFile))
	if err != nil {
		t.Fatalf("Failed to read book.json: %v", err)
	}
	for _, want := range []string{`"title": "Sense & Sensibility <annotated> \"quoted\""`, `"comments": "a > b && b < c"`} {
		if !strings.Contains(string(written), want) {
			t.Errorf("Expected %s in book.json, got:\n%s", want, written)
		}
	}
	for _, escape := range []string{`\u0026`, `\u003c`, `\u003e`} {
		if strings.Contains(string(written), escape) {
			t.Errorf("book.json still escapes %s:\n%s", escape, written)
		}
	}

	// The characters are only written literally if they also read back, which is
	// the half a byte assertion cannot see.
	reopened, err := Open(rootFS, newLoggerForTest(), bookID)
	if err != nil {
		t.Fatalf("Failed to reopen book: %v", err)
	}
	if got := reopened.GetMeta().Title; got != title {
		t.Errorf("Title did not round-trip: got %q, want %q", got, title)
	}
}

// TestReadBookMetaMatchesFieldNamesCaseInsensitively holds the read path where
// v1 left it. json/v2 matches member names exactly by default, which would make
// a hand-edited "Title" read as absent — and setMeta rewrites the whole file, so
// the next save through the UI would delete it. Tightening this is PSW-99's
// call, once unknown members survive a write.
func TestReadBookMetaMatchesFieldNamesCaseInsensitively(t *testing.T) {
	tmpLib := t.TempDir()
	bookID := "hand-edited-d52p"
	bookDir := path.Join(tmpLib, bookID)
	if err := os.MkdirAll(bookDir, 0o755); err != nil {
		t.Fatalf("Failed to create book dir: %v", err)
	}

	handEdited := `{
  "schema_version": 1,
  "id": "hand-edited-d52p",
  "Title": "Hand Edited",
  "authors": []
}`
	if err := os.WriteFile(path.Join(bookDir, BookMetaFile), []byte(handEdited), 0o644); err != nil {
		t.Fatalf("Failed to write book.json: %v", err)
	}

	tmpRoot, err := os.OpenRoot(tmpLib)
	if err != nil {
		t.Fatalf("Failed to open temporary root: %v", err)
	}
	defer tmpRoot.Close()

	book, err := Open(fsutil.NewRootFS(tmpRoot), newLoggerForTest(), bookID)
	if err != nil {
		t.Fatalf("Failed to open hand-edited book: %v", err)
	}
	if got := book.GetMeta().Title; got != "Hand Edited" {
		t.Errorf("Title read from a capitalized member: got %q, want %q", got, "Hand Edited")
	}
}
