package shelf

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/voilelab/plainshelf/internal/fsutil"
)

// twoShelves builds a source and a target shelf on separate roots, the shape a
// cross-shelf transfer runs against. Each gets its own temp library so the
// transfer genuinely crosses a filesystem boundary rather than staying in one
// tree.
func twoShelves(t *testing.T) (source, target *Shelf, targetLib string) {
	t.Helper()
	sourceLib := path.Join(t.TempDir(), "source")
	targetLib = path.Join(t.TempDir(), "target")
	source = newTestShelf(t, &ShelfConf{LibRoot: sourceLib})
	target = newTestShelf(t, &ShelfConf{LibRoot: targetLib})
	return source, target, targetLib
}

// seedBook writes a self-contained book - cover, a source with an inline asset,
// and a current-source pointer - so a transfer can be checked for carrying the
// whole package across.
func seedBook(t *testing.T, s *Shelf, layer Layers, title string) *Book {
	t.Helper()

	book, err := s.NewBook(layer, title)
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	if err := book.SetCover([]byte("cover bytes"), ".png"); err != nil {
		t.Fatalf("SetCover: %v", err)
	}
	src, err := book.NewSource(strings.NewReader("chapter body ![](assets/img-0001.png)\n"))
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	if err := src.WriteAsset("img-0001.png", []byte("asset bytes")); err != nil {
		t.Fatalf("WriteAsset: %v", err)
	}
	if err := book.SetCurrentSource(src.ID()); err != nil {
		t.Fatalf("SetCurrentSource: %v", err)
	}
	return book
}

func assertStagingClean(t *testing.T, lib string) {
	t.Helper()
	staged, err := os.ReadDir(path.Join(lib, appFolder, appTmpFolder))
	if err == nil && len(staged) != 0 {
		t.Errorf("staging folder under %s not clean: %d entries", lib, len(staged))
	}
}

// A cross-shelf copy hands the copy a fresh ID drawn from the target, leaves the
// source untouched, and reproduces the whole package on the far shelf.
func TestShelfCopyBookFromGivesNewIDAndKeepsSource(t *testing.T) {
	source, target, targetLib := twoShelves(t)
	original := seedBook(t, source, Layers{"fiction"}, "Cross Copy")
	originalSourceID := original.CurrentSource()

	copied, err := target.CopyBookFrom(source, original.ID(), Layers{"imported"}, false)
	if err != nil {
		t.Fatalf("CopyBookFrom: %v", err)
	}

	if copied.ID() == original.ID() {
		t.Fatalf("cross-shelf copy reuses the original ID %q", original.ID())
	}
	copiedListing, err := target.GetBookListing(copied.ID())
	if err != nil {
		t.Fatalf("GetBookListing: %v", err)
	}
	if !copiedListing.Layers.Equal(Layers{"imported"}) {
		t.Errorf("copy layers = %v, want [imported]", copiedListing.Layers)
	}

	// The source shelf still holds the original, alone.
	if _, err := source.GetBook(original.ID()); err != nil {
		t.Fatalf("source lost the original: %v", err)
	}
	sourceBooks, err := source.ListBooks()
	if err != nil {
		t.Fatalf("source ListBooks: %v", err)
	}
	if len(sourceBooks) != 1 {
		t.Errorf("source holds %d books, want just the original", len(sourceBooks))
	}

	// The target shelf holds the copy, self-contained: content, asset and cover.
	reopened, err := target.GetBook(copied.ID())
	if err != nil {
		t.Fatalf("target GetBook: %v", err)
	}
	if reopened.Title() != "Cross Copy" {
		t.Errorf("copy title = %q, want %q", reopened.Title(), "Cross Copy")
	}
	copiedSource, err := reopened.ResolveCurrentSource()
	if err != nil {
		t.Fatalf("ResolveCurrentSource: %v", err)
	}
	if copiedSource.ID() != originalSourceID {
		t.Errorf("copy current source = %q, want the carried-over %q", copiedSource.ID(), originalSourceID)
	}
	content, err := copiedSource.Open()
	if err != nil {
		t.Fatalf("open copy source: %v", err)
	}
	raw, err := io.ReadAll(content)
	content.Close()
	if err != nil {
		t.Fatalf("read copy source: %v", err)
	}
	if !strings.Contains(string(raw), "chapter body") {
		t.Errorf("copy content = %q, want the original body", raw)
	}
	asset, err := copiedSource.OpenAsset("img-0001.png")
	if err != nil {
		t.Fatalf("OpenAsset: %v", err)
	}
	assetBytes, err := io.ReadAll(asset.File)
	asset.File.Close()
	if err != nil {
		t.Fatalf("read asset: %v", err)
	}
	if string(assetBytes) != "asset bytes" {
		t.Errorf("copy asset = %q, want %q", assetBytes, "asset bytes")
	}
	cover, ext, err := reopened.OpenCover()
	if err != nil {
		t.Fatalf("OpenCover: %v", err)
	}
	if string(cover) != "cover bytes" || ext != ".png" {
		t.Errorf("copy cover = %q,%q want %q,.png", cover, ext, "cover bytes")
	}

	assertStagingClean(t, targetLib)
}

// A cross-shelf move keeps the book's ID so reading progress and external
// references survive, publishes it on the target, and removes it from the source
// only afterwards - the source keeps it recoverable in its own trash.
func TestShelfMoveBookFromPreservesIDAndRemovesSource(t *testing.T) {
	source, target, targetLib := twoShelves(t)
	original := seedBook(t, source, Layers{"fiction"}, "Cross Move")
	originalID := original.ID()

	moved, err := target.MoveBookFrom(source, originalID, Layers{"archive"})
	if err != nil {
		t.Fatalf("MoveBookFrom: %v", err)
	}

	if moved.ID() != originalID {
		t.Fatalf("move changed the ID: got %q, want %q", moved.ID(), originalID)
	}
	movedListing, err := target.GetBookListing(moved.ID())
	if err != nil {
		t.Fatalf("GetBookListing: %v", err)
	}
	if !movedListing.Layers.Equal(Layers{"archive"}) {
		t.Errorf("moved layers = %v, want [archive]", movedListing.Layers)
	}

	// The target now lists the book under its original ID.
	if _, err := target.GetBook(originalID); err != nil {
		t.Fatalf("target GetBook after move: %v", err)
	}

	// The source no longer lists it, but keeps it in the trash - a move is still
	// recoverable on the source side.
	if _, err := source.GetBook(originalID); !errors.Is(err, ErrBookNotFound) {
		t.Fatalf("source still lists the moved book, err = %v", err)
	}
	inTrash, err := source.isBookIDInTrash(originalID)
	if err != nil {
		t.Fatalf("isBookIDInTrash: %v", err)
	}
	if !inTrash {
		t.Errorf("moved book is not in the source trash; a move should stay recoverable")
	}

	assertStagingClean(t, targetLib)
}

// A move that would land on an ID the target already holds is refused, and the
// source is left exactly as it was.
func TestShelfMoveBookFromRefusesIDConflict(t *testing.T) {
	source, target, targetLib := twoShelves(t)
	original := seedBook(t, source, Layers{"fiction"}, "Conflicting")
	id := original.ID()

	// Move it across so the target holds this ID under a genuine cache entry...
	if _, err := target.MoveBookFrom(source, id, Layers{"archive"}); err != nil {
		t.Fatalf("initial MoveBookFrom: %v", err)
	}
	// ...then bring the source's copy back out of its trash, so both shelves hold
	// the same ID and a second move would collide.
	if err := source.RestoreTrashedBook(id); err != nil {
		t.Fatalf("RestoreTrashedBook: %v", err)
	}

	_, err := target.MoveBookFrom(source, id, Layers{"again"})
	if !errors.Is(err, ErrBookIDConflict) {
		t.Fatalf("MoveBookFrom into a conflict = %v, want ErrBookIDConflict", err)
	}

	// The source book is untouched: still listed, not trashed.
	if _, err := source.GetBook(id); err != nil {
		t.Fatalf("source lost the book after a refused move: %v", err)
	}
	inTrash, err := source.isBookIDInTrash(id)
	if err != nil {
		t.Fatalf("isBookIDInTrash: %v", err)
	}
	if inTrash {
		t.Errorf("a refused move trashed the source book")
	}

	assertStagingClean(t, targetLib)
}

// A read-only target refuses the transfer before it stages anything, and the
// source is not modified.
func TestShelfCopyBookFromRefusesReadOnlyTarget(t *testing.T) {
	sourceLib := path.Join(t.TempDir(), "source")
	targetLib := path.Join(t.TempDir(), "target")
	source := newTestShelf(t, &ShelfConf{LibRoot: sourceLib})
	original := seedBook(t, source, Layers{"fiction"}, "No Room")

	// A read-only shelf is taken as found, so lay down its directory structure
	// with a writable open first, then reopen the same path read-only.
	seed, err := NewShelf(&ShelfConf{LibRoot: targetLib})
	if err != nil {
		t.Fatalf("seed target shelf: %v", err)
	}
	if err := seed.WaitReady(t.Context()); err != nil {
		t.Fatalf("WaitReady seed: %v", err)
	}
	if err := seed.Close(); err != nil {
		t.Fatalf("close seed: %v", err)
	}
	target := newTestShelf(t, &ShelfConf{LibRoot: targetLib, ReadOnly: true})

	if _, err := target.MoveBookFrom(source, original.ID(), Layers{"archive"}); !errors.Is(err, fsutil.ErrReadOnly) {
		t.Fatalf("MoveBookFrom into a read-only target = %v, want fsutil.ErrReadOnly", err)
	}

	if _, err := source.GetBook(original.ID()); err != nil {
		t.Fatalf("source modified by a refused read-only move: %v", err)
	}
	inTrash, err := source.isBookIDInTrash(original.ID())
	if err != nil {
		t.Fatalf("isBookIDInTrash: %v", err)
	}
	if inTrash {
		t.Errorf("a refused read-only move trashed the source book")
	}
}

// Naming one shelf as both source and target is refused rather than deadlocking
// on its own lock.
func TestShelfTransferSameShelfRefused(t *testing.T) {
	s := newTestShelf(t, &ShelfConf{LibRoot: path.Join(t.TempDir(), "shelf")})
	book := seedBook(t, s, Layers{"fiction"}, "Here")

	if _, err := s.CopyBookFrom(s, book.ID(), Layers{"elsewhere"}, false); !errors.Is(err, ErrSameShelfTransfer) {
		t.Fatalf("CopyBookFrom onto the same shelf = %v, want ErrSameShelfTransfer", err)
	}
}

// plantBookOnDisk writes a minimal book package straight onto a shelf's disk,
// bypassing its book cache, the way another instance or a file manager would add
// one between scans.
func plantBookOnDisk(t *testing.T, lib string, layer Layers, bookID, title string) {
	t.Helper()

	dir := filepath.Join(append([]string{lib, booksFolder}, layer...)...)
	dir = filepath.Join(dir, "planted"+bookExtension)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll planted book: %v", err)
	}
	meta := fmt.Sprintf(`{"schema_version":%d,"id":%q,"title":%q,"current_source":""}`,
		BookMetaSchemaVersion, bookID, title)
	if err := os.WriteFile(filepath.Join(dir, BookMetaFile), []byte(meta), 0o644); err != nil {
		t.Fatalf("write planted book.json: %v", err)
	}
}

// A move from a read-only source is refused before anything is copied, so the
// target gains nothing and the source keeps the book. Without the guard the copy
// would publish on the target and only the trailing source delete would fail.
func TestShelfMoveBookFromRefusesReadOnlySource(t *testing.T) {
	sourceLib := path.Join(t.TempDir(), "source")

	seed, err := NewShelf(&ShelfConf{LibRoot: sourceLib})
	if err != nil {
		t.Fatalf("seed source shelf: %v", err)
	}
	if err := seed.WaitReady(t.Context()); err != nil {
		t.Fatalf("WaitReady seed: %v", err)
	}
	original := seedBook(t, seed, Layers{"fiction"}, "Locked")
	id := original.ID()
	if err := seed.Close(); err != nil {
		t.Fatalf("close seed: %v", err)
	}

	source := newTestShelf(t, &ShelfConf{LibRoot: sourceLib, ReadOnly: true})
	target := newTestShelf(t, &ShelfConf{LibRoot: path.Join(t.TempDir(), "target")})

	if _, err := target.MoveBookFrom(source, id, Layers{"archive"}); !errors.Is(err, fsutil.ErrReadOnly) {
		t.Fatalf("MoveBookFrom from a read-only source = %v, want fsutil.ErrReadOnly", err)
	}

	targetBooks, err := target.ListBooks()
	if err != nil {
		t.Fatalf("target ListBooks: %v", err)
	}
	if len(targetBooks) != 0 {
		t.Errorf("a refused read-only-source move published %d books on the target, want none", len(targetBooks))
	}
	if _, err := source.GetBook(id); err != nil {
		t.Fatalf("read-only source lost the book: %v", err)
	}
}

// The move ID-conflict check forces a fresh scan, so it catches a book added to
// the target's disk since the last scan - not just one already in the cache.
func TestShelfMoveBookFromDetectsUnscannedTargetConflict(t *testing.T) {
	source, target, targetLib := twoShelves(t)
	original := seedBook(t, source, Layers{"fiction"}, "Racer")
	id := original.ID()

	plantBookOnDisk(t, targetLib, Layers{"existing"}, id, "Planted")

	if _, err := target.MoveBookFrom(source, id, Layers{"archive"}); !errors.Is(err, ErrBookIDConflict) {
		t.Fatalf("MoveBookFrom into an unscanned conflict = %v, want ErrBookIDConflict", err)
	}

	if _, err := source.GetBook(id); err != nil {
		t.Fatalf("source lost the book after a refused move: %v", err)
	}
	inTrash, err := source.isBookIDInTrash(id)
	if err != nil {
		t.Fatalf("isBookIDInTrash: %v", err)
	}
	if inTrash {
		t.Errorf("a refused move trashed the source book")
	}
}

// Copying an unknown book across shelves reports ErrBookNotFound and stages
// nothing on the target.
func TestShelfCopyBookFromNotFound(t *testing.T) {
	source, target, targetLib := twoShelves(t)

	if _, err := target.CopyBookFrom(source, "does-not-exist", Layers{"imported"}, false); !errors.Is(err, ErrBookNotFound) {
		t.Fatalf("CopyBookFrom of a missing book = %v, want ErrBookNotFound", err)
	}
	assertStagingClean(t, targetLib)
}
