package shelf

import (
	"encoding/json/v2"
	"fmt"
	"os"
	"path"
	"testing"
	"time"
)

const testWriterID = "testwriter01"

// waitForBookCacheExport polls for the exported cache file, which the initial
// scan writes after the shelf reports ready so that readiness is never blocked
// on a write (see initCache).
func waitForBookCacheExport(t *testing.T, libRoot, writerID string) BookCacheFile {
	t.Helper()

	filePath := path.Join(libRoot, appFolder, bookCacheFilePrefix+writerID+bookCacheFileSuffix)
	deadline := time.Now().Add(5 * time.Second)
	for {
		cache, err := readBookCacheFileAt(filePath)
		if err == nil {
			return *cache
		}
		if time.Now().After(deadline) {
			t.Fatalf("book cache was not exported to %s: %v", filePath, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func readBookCacheFileAt(filePath string) (*BookCacheFile, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var cache BookCacheFile
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	return &cache, nil
}

func writeRawBookCache(t *testing.T, libRoot, name string, body any) string {
	t.Helper()

	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("Failed to marshal raw book cache: %v", err)
	}
	filePath := path.Join(libRoot, appFolder, name)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		t.Fatalf("Failed to write raw book cache: %v", err)
	}
	return filePath
}

func TestBookCacheExportAfterInitialScan(t *testing.T) {
	libRoot := t.TempDir()
	shelf := newTestShelf(t, &ShelfConf{
		LibRoot:           libRoot,
		LockMode:          "none",
		BookCacheWriterID: testWriterID,
	})

	if err := shelf.NewFolder(FolderPath{"Fiction"}, "Classics"); err != nil {
		t.Fatalf("NewFolder: %v", err)
	}
	// An empty folder is the case a per-book listing cannot reconstruct, which is
	// why the file records folders separately.
	if err := shelf.NewFolder(FolderPath{}, "Empty"); err != nil {
		t.Fatalf("NewFolder: %v", err)
	}
	book, err := shelf.NewBook(FolderPath{"Fiction", "Classics"}, "Dune")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}

	if _, err := shelf.ExportBookCache(); err != nil {
		t.Fatalf("ExportBookCache: %v", err)
	}
	cache := waitForBookCacheExport(t, libRoot, testWriterID)

	if cache.SchemaVersion != BookCacheSchemaVersion {
		t.Errorf("schema_version = %d, want %d", cache.SchemaVersion, BookCacheSchemaVersion)
	}
	if cache.WriterID != testWriterID {
		t.Errorf("writer_id = %q, want %q", cache.WriterID, testWriterID)
	}
	if cache.Timestamp <= 0 {
		t.Errorf("timestamp = %d, want a positive Unix time", cache.Timestamp)
	}
	if cache.Generator == "" {
		t.Error("generator is empty")
	}

	entry, ok := cache.Books[book.ID()]
	if !ok {
		t.Fatalf("book %q missing from the exported cache: %v", book.ID(), cache.Books)
	}
	if entry.Path != book.PackagePath() {
		t.Errorf("path = %q, want %q", entry.Path, book.PackagePath())
	}
	if entry.Meta == nil || entry.Meta.Title != "Dune" {
		t.Errorf("exported meta = %+v, want the book.json of Dune", entry.Meta)
	}

	// "/" for the top level matches what the API and the pCloud client use.
	want := map[string]bool{"/": true, "Fiction": true, "Fiction/Classics": true, "Empty": true}
	got := map[string]bool{}
	for _, folder := range cache.Folders {
		got[folder] = true
	}
	for folder := range want {
		if !got[folder] {
			t.Errorf("layer %q missing from %v", folder, cache.Folders)
		}
	}
	if len(cache.Folders) != len(want) {
		t.Errorf("layers = %v, want exactly %d entries", cache.Folders, len(want))
	}
}

func TestBookCacheExportDisabledWithoutWriterID(t *testing.T) {
	libRoot := t.TempDir()
	shelf := newTestShelf(t, &ShelfConf{LibRoot: libRoot, LockMode: "none"})

	if _, err := shelf.NewBook(nil, "Untracked"); err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	if _, err := shelf.ListBooks(); err != nil {
		t.Fatalf("ListBooks: %v", err)
	}
	if err := shelf.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	entries, err := os.ReadDir(path.Join(libRoot, appFolder))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, entry := range entries {
		if len(entry.Name()) >= len(bookCacheFilePrefix) && entry.Name()[:len(bookCacheFilePrefix)] == bookCacheFilePrefix {
			t.Fatalf("exported %s without a writer ID configured", entry.Name())
		}
	}

	// ExportBookCache is also refused rather than silently doing nothing, so the
	// API can tell the user why no file appeared.
	if _, err := shelf.ExportBookCache(); err == nil {
		t.Fatal("ExportBookCache succeeded without a writer ID")
	}
}

func TestBookCacheExportSkipsUnchangedContent(t *testing.T) {
	libRoot := t.TempDir()
	shelf := newTestShelf(t, &ShelfConf{
		LibRoot:           libRoot,
		LockMode:          "none",
		BookCacheWriterID: testWriterID,
	})

	if _, err := shelf.NewBook(nil, "Stable"); err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	if _, err := shelf.ExportBookCache(); err != nil {
		t.Fatalf("ExportBookCache: %v", err)
	}

	filePath := path.Join(libRoot, appFolder, bookCacheFilePrefix+testWriterID+bookCacheFileSuffix)
	before, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	// Backdate so a rewrite is unmistakable even at one-second granularity.
	shiftModTime(t, filePath, -time.Hour)

	// Not forced: the shelf has not changed, so nothing should reach the disk.
	if err := shelf.exportBookCache(false); err != nil {
		t.Fatalf("exportBookCache: %v", err)
	}
	after, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !after.ModTime().Before(before.ModTime()) {
		t.Error("unchanged shelf rewrote the exported cache")
	}

	// A real change must still get through.
	if _, err := shelf.NewBook(nil, "Added Later"); err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	if err := shelf.exportBookCache(false); err != nil {
		t.Fatalf("exportBookCache: %v", err)
	}
	cache, err := readBookCacheFileAt(filePath)
	if err != nil {
		t.Fatalf("readBookCacheFileAt: %v", err)
	}
	if len(cache.Books) != 2 {
		t.Errorf("exported %d books after adding one, want 2", len(cache.Books))
	}
}

// TestBookCacheExportOfManyBooksSkipsUnchangedContent is the same claim as the
// test above with enough books for it to mean something. The export is skipped
// when bookCacheDigest matches the last one, and that digest hashes a payload
// holding map[string]BookCacheEntry — so the skip only holds while the encoder
// sorts map keys, which json/v2 does not do unless jsonopt says so. One book
// has no order to vary and passes either way; twelve do not. What a missing
// Deterministic costs is not a wrong answer but a re-upload of an unchanged
// listing on every scan, on a shelf its owner keeps on pCloud or SMB.
func TestBookCacheExportOfManyBooksSkipsUnchangedContent(t *testing.T) {
	libRoot := t.TempDir()
	shelf := newTestShelf(t, &ShelfConf{
		LibRoot:           libRoot,
		LockMode:          "none",
		BookCacheWriterID: testWriterID,
	})

	for i := range 12 {
		if _, err := shelf.NewBook(nil, fmt.Sprintf("Stable %02d", i)); err != nil {
			t.Fatalf("NewBook: %v", err)
		}
	}
	if _, err := shelf.ExportBookCache(); err != nil {
		t.Fatalf("ExportBookCache: %v", err)
	}

	filePath := path.Join(libRoot, appFolder, bookCacheFilePrefix+testWriterID+bookCacheFileSuffix)
	want, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// Repeated because a random map order agrees with the previous one now and
	// then; eight rounds of twelve books do not agree by luck.
	for round := range 8 {
		shiftModTime(t, filePath, -time.Hour)
		before, err := os.Stat(filePath)
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}

		if err := shelf.exportBookCache(false); err != nil {
			t.Fatalf("exportBookCache: %v", err)
		}

		after, err := os.Stat(filePath)
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}
		if !after.ModTime().Equal(before.ModTime()) {
			t.Fatalf("round %d rewrote the exported cache for an unchanged shelf", round)
		}
		got, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		if string(got) != string(want) {
			t.Fatalf("round %d changed the exported cache on disk", round)
		}
	}

	// Forcing an export re-encodes the same content, which is where an
	// unstable encoder would show up in the bytes rather than in a mtime.
	if err := shelf.exportBookCache(true); err != nil {
		t.Fatalf("exportBookCache(force): %v", err)
	}
	forced, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(forced) != string(want) {
		t.Error("a forced re-export of unchanged content produced different bytes")
	}
}

func TestBookCacheExportOnClose(t *testing.T) {
	libRoot := t.TempDir()
	shelf, err := NewShelf(&ShelfConf{
		LibRoot:           libRoot,
		LockMode:          "none",
		BookCacheWriterID: testWriterID,
		// Long enough that only Close can produce the second export.
		BookCacheInterval: "24h",
	})
	if err != nil {
		t.Fatalf("NewShelf: %v", err)
	}
	if err := shelf.WaitReady(t.Context()); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}
	waitForBookCacheExport(t, libRoot, testWriterID)

	if _, err := shelf.NewBook(nil, "Written On Close"); err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	if err := shelf.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	cache := waitForBookCacheExport(t, libRoot, testWriterID)
	if len(cache.Books) != 1 {
		t.Fatalf("exported %d books, want the one added before Close", len(cache.Books))
	}
}

func TestBookCacheExportPreservesNewerSchemaVersion(t *testing.T) {
	libRoot := t.TempDir()
	bookDir := path.Join(libRoot, booksFolder, "from-the-future.bookpkg")
	if err := os.MkdirAll(bookDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	meta := map[string]any{
		"schema_version": BookMetaSchemaVersion + 98,
		"id":             "future01",
		"title":          "From The Future",
	}
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := os.WriteFile(path.Join(bookDir, BookMetaFile), data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	shelf := newTestShelf(t, &ShelfConf{
		LibRoot:           libRoot,
		LockMode:          "none",
		BookCacheWriterID: testWriterID,
	})
	if _, err := shelf.ExportBookCache(); err != nil {
		t.Fatalf("ExportBookCache: %v", err)
	}

	cache := waitForBookCacheExport(t, libRoot, testWriterID)
	entry, ok := cache.Books["future01"]
	if !ok {
		t.Fatalf("too-new book missing from the exported cache: %v", cache.Books)
	}
	// A reader must be able to tell this book is not fully understood here, so
	// the version has to survive the round trip rather than be normalized down.
	if entry.Meta.SchemaVersion != BookMetaSchemaVersion+98 {
		t.Errorf("schema_version = %d, want %d", entry.Meta.SchemaVersion, BookMetaSchemaVersion+98)
	}
}

func TestBookCachePrunesOnlyStaleForeignFiles(t *testing.T) {
	libRoot := t.TempDir()
	shelf := newTestShelf(t, &ShelfConf{
		LibRoot:           libRoot,
		LockMode:          "none",
		BookCacheWriterID: testWriterID,
	})

	stale := time.Now().Add(-bookCacheRetention - time.Hour).Unix()
	fresh := time.Now().Add(-time.Hour).Unix()

	staleForeign := writeRawBookCache(t, libRoot, bookCacheFilePrefix+"oldmachine"+bookCacheFileSuffix, BookCacheFile{
		SchemaVersion: BookCacheSchemaVersion,
		WriterID:      "oldmachine",
		Timestamp:     stale,
	})
	freshForeign := writeRawBookCache(t, libRoot, bookCacheFilePrefix+"otherlaptop"+bookCacheFileSuffix, BookCacheFile{
		SchemaVersion: BookCacheSchemaVersion,
		WriterID:      "otherlaptop",
		Timestamp:     fresh,
	})
	// Old, but from an unrecognized format: deleting a file we cannot read is a
	// worse failure than keeping a stale one.
	unreadable := path.Join(libRoot, appFolder, bookCacheFilePrefix+"mystery"+bookCacheFileSuffix)
	if err := os.WriteFile(unreadable, []byte("{not json"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	// Same shape but a future schema version: also unrecognized, also kept.
	futureFormat := writeRawBookCache(t, libRoot, bookCacheFilePrefix+"newbuild"+bookCacheFileSuffix, map[string]any{
		"schema_version": BookCacheSchemaVersion + 1,
		"writer_id":      "newbuild",
		"timestamp":      stale,
	})

	if _, err := shelf.ExportBookCache(); err != nil {
		t.Fatalf("ExportBookCache: %v", err)
	}
	waitForBookCacheExport(t, libRoot, testWriterID)

	if _, err := os.Stat(staleForeign); !os.IsNotExist(err) {
		t.Errorf("stale foreign cache was not pruned (stat err: %v)", err)
	}
	for _, keep := range []string{freshForeign, unreadable, futureFormat} {
		if _, err := os.Stat(keep); err != nil {
			t.Errorf("%s should have been kept: %v", keep, err)
		}
	}
}

func TestExportBookCachePicksUpBooksAddedOnDisk(t *testing.T) {
	libRoot := t.TempDir()
	shelf := newTestShelf(t, &ShelfConf{
		LibRoot:           libRoot,
		LockMode:          "none",
		BookCacheWriterID: testWriterID,
		// Without a forced rescan the shelf would not look at the tree again.
		ScanInterval: "24h",
	})
	waitForBookCacheExport(t, libRoot, testWriterID)

	bookDir := path.Join(libRoot, booksFolder, "added-behind-our-back.bookpkg")
	if err := os.MkdirAll(bookDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	data, err := json.Marshal(map[string]any{"schema_version": BookMetaSchemaVersion, "id": "onda1skb", "title": "On Disk"})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := os.WriteFile(path.Join(bookDir, BookMetaFile), data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	scannedAt, err := shelf.ExportBookCache()
	if err != nil {
		t.Fatalf("ExportBookCache: %v", err)
	}
	if scannedAt.IsZero() {
		t.Error("ExportBookCache returned a zero scan time")
	}

	cache := waitForBookCacheExport(t, libRoot, testWriterID)
	if _, ok := cache.Books["onda1skb"]; !ok {
		t.Fatalf("manual export did not rescan; books = %v", cache.Books)
	}
	// The timestamp reports the walk, not the write, so a reader can compare it
	// against each book.json's modification time.
	if cache.Timestamp != scannedAt.Unix() {
		t.Errorf("timestamp = %d, want the scan time %d", cache.Timestamp, scannedAt.Unix())
	}
}

func TestValidateBookCacheWriterID(t *testing.T) {
	valid := []string{"", "a1b2c3d4e5f6a7b8", "desktop-home", "laptop_2"}
	for _, writerID := range valid {
		if err := validateBookCacheWriterID(writerID); err != nil {
			t.Errorf("validateBookCacheWriterID(%q) = %v, want nil", writerID, err)
		}
	}

	// The ID becomes part of a file name under app/, so separators and dots must
	// not be able to move the write somewhere else.
	invalid := []string{"../escape", "with/slash", "with\\slash", "with.dot", "with space", "üñïçø∂é"}
	for _, writerID := range invalid {
		if err := validateBookCacheWriterID(writerID); err == nil {
			t.Errorf("validateBookCacheWriterID(%q) = nil, want an error", writerID)
		}
	}

	if _, err := NewShelf(&ShelfConf{LibRoot: t.TempDir(), BookCacheWriterID: "../escape"}); err == nil {
		t.Fatal("NewShelf accepted an unsafe book cache writer ID")
	}
}

// An edit made through *Book changes the cached book in place, without going
// anywhere near the export bookkeeping. Nothing marks it, so the export has to
// notice by comparing content — which is why there is no dirty flag.
func TestBookCacheExportPicksUpInPlaceMetadataEdits(t *testing.T) {
	libRoot := t.TempDir()
	shelf, err := NewShelf(&ShelfConf{
		LibRoot:           libRoot,
		LockMode:          "none",
		BookCacheWriterID: testWriterID,
		// Long enough that only Close can produce the second export, and long
		// enough that no rescan can quietly do the job for it.
		BookCacheInterval: "24h",
		ScanInterval:      "24h",
	})
	if err != nil {
		t.Fatalf("NewShelf: %v", err)
	}
	if err := shelf.WaitReady(t.Context()); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}

	created, err := shelf.NewBook(nil, "Before The Edit")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	if _, err := shelf.ExportBookCache(); err != nil {
		t.Fatalf("ExportBookCache: %v", err)
	}

	// Through GetBook, which is how the API reaches a book: it hands back the
	// instance the cache holds, so SetMeta edits the cached book in place.
	book, err := shelf.GetBook(created.ID())
	if err != nil {
		t.Fatalf("GetBook: %v", err)
	}
	meta := book.GetMeta()
	meta.Title = "After The Edit"
	if err := book.SetMeta(meta); err != nil {
		t.Fatalf("SetMeta: %v", err)
	}

	if err := shelf.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	cache := waitForBookCacheExport(t, libRoot, testWriterID)
	entry, ok := cache.Books[created.ID()]
	if !ok {
		t.Fatalf("book missing from the exported cache: %v", cache.Books)
	}
	if entry.Meta.Title != "After The Edit" {
		t.Errorf("exported title = %q, want the edited title", entry.Meta.Title)
	}
}

// Timestamp must be the moment the walk started. Stamping the end would cover
// a book edited while the walk was still running, after it had been read.
func TestBookCacheExportTimestampIsTheScanStart(t *testing.T) {
	libRoot := t.TempDir()
	shelf := newTestShelf(t, &ShelfConf{
		LibRoot:           libRoot,
		LockMode:          "none",
		BookCacheWriterID: testWriterID,
	})
	if _, err := shelf.NewBook(nil, "Timed"); err != nil {
		t.Fatalf("NewBook: %v", err)
	}

	scanStart, err := shelf.ExportBookCache()
	if err != nil {
		t.Fatalf("ExportBookCache: %v", err)
	}

	shelf.bookCache.RLock()
	scanEnd := shelf.bookCache.lastFullScan
	shelf.bookCache.RUnlock()

	if scanStart.After(scanEnd) {
		t.Errorf("reported scan start %v is after the scan end %v", scanStart, scanEnd)
	}

	cache := waitForBookCacheExport(t, libRoot, testWriterID)
	if cache.Timestamp != scanStart.Unix() {
		t.Errorf("timestamp = %d, want the scan start %d", cache.Timestamp, scanStart.Unix())
	}
}
