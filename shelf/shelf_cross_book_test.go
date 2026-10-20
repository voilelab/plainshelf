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

func twoShelves(t *testing.T) (source, target *Shelf, targetLib string) {
	t.Helper()
	sourceLib := path.Join(t.TempDir(), "source")
	targetLib = path.Join(t.TempDir(), "target")
	source = newTestShelf(t, &ShelfConf{LibRoot: sourceLib})
	target = newTestShelf(t, &ShelfConf{LibRoot: targetLib})
	return source, target, targetLib
}

func seedBook(t *testing.T, s *Shelf, folder FolderPath, title string) *Book {
	t.Helper()

	book, err := s.NewBook(folder, title)
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

func TestShelfCopyBookFromGivesNewIDAndKeepsSource(t *testing.T) {
	source, target, targetLib := twoShelves(t)
	original := seedBook(t, source, FolderPath{"fiction"}, "Cross Copy")
	originalSourceID := original.CurrentSource()

	copied, err := target.CopyBookFrom(source, original.ID(), FolderPath{"imported"}, false)
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
	if !copiedListing.Folders.Equal(FolderPath{"imported"}) {
		t.Errorf("copy layers = %v, want [imported]", copiedListing.Folders)
	}

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

func TestShelfMoveBookFromPreservesIDAndRemovesSource(t *testing.T) {
	source, target, targetLib := twoShelves(t)
	original := seedBook(t, source, FolderPath{"fiction"}, "Cross Move")
	originalID := original.ID()

	moved, err := target.MoveBookFrom(source, originalID, FolderPath{"archive"})
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
	if !movedListing.Folders.Equal(FolderPath{"archive"}) {
		t.Errorf("moved layers = %v, want [archive]", movedListing.Folders)
	}

	if _, err := target.GetBook(originalID); err != nil {
		t.Fatalf("target GetBook after move: %v", err)
	}

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

func TestShelfMoveBookFromRefusesIDConflict(t *testing.T) {
	source, target, targetLib := twoShelves(t)
	original := seedBook(t, source, FolderPath{"fiction"}, "Conflicting")
	id := original.ID()

	if _, err := target.MoveBookFrom(source, id, FolderPath{"archive"}); err != nil {
		t.Fatalf("initial MoveBookFrom: %v", err)
	}
	if err := source.RestoreTrashedBook(id); err != nil {
		t.Fatalf("RestoreTrashedBook: %v", err)
	}

	_, err := target.MoveBookFrom(source, id, FolderPath{"again"})
	if !errors.Is(err, ErrBookIDConflict) {
		t.Fatalf("MoveBookFrom into a conflict = %v, want ErrBookIDConflict", err)
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

	assertStagingClean(t, targetLib)
}

func TestShelfCopyBookFromRefusesReadOnlyTarget(t *testing.T) {
	sourceLib := path.Join(t.TempDir(), "source")
	targetLib := path.Join(t.TempDir(), "target")
	source := newTestShelf(t, &ShelfConf{LibRoot: sourceLib})
	original := seedBook(t, source, FolderPath{"fiction"}, "No Room")

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

	if _, err := target.MoveBookFrom(source, original.ID(), FolderPath{"archive"}); !errors.Is(err, fsutil.ErrReadOnly) {
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

func TestShelfTransferSameShelfRefused(t *testing.T) {
	s := newTestShelf(t, &ShelfConf{LibRoot: path.Join(t.TempDir(), "shelf")})
	book := seedBook(t, s, FolderPath{"fiction"}, "Here")

	if _, err := s.CopyBookFrom(s, book.ID(), FolderPath{"elsewhere"}, false); !errors.Is(err, ErrSameShelfTransfer) {
		t.Fatalf("CopyBookFrom onto the same shelf = %v, want ErrSameShelfTransfer", err)
	}
}

func plantBookOnDisk(t *testing.T, lib string, folder FolderPath, bookID, title string) {
	t.Helper()

	dir := filepath.Join(append([]string{lib, booksFolder}, folder...)...)
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

func TestShelfMoveBookFromRefusesReadOnlySource(t *testing.T) {
	sourceLib := path.Join(t.TempDir(), "source")

	seed, err := NewShelf(&ShelfConf{LibRoot: sourceLib})
	if err != nil {
		t.Fatalf("seed source shelf: %v", err)
	}
	if err := seed.WaitReady(t.Context()); err != nil {
		t.Fatalf("WaitReady seed: %v", err)
	}
	original := seedBook(t, seed, FolderPath{"fiction"}, "Locked")
	id := original.ID()
	if err := seed.Close(); err != nil {
		t.Fatalf("close seed: %v", err)
	}

	source := newTestShelf(t, &ShelfConf{LibRoot: sourceLib, ReadOnly: true})
	target := newTestShelf(t, &ShelfConf{LibRoot: path.Join(t.TempDir(), "target")})

	if _, err := target.MoveBookFrom(source, id, FolderPath{"archive"}); !errors.Is(err, fsutil.ErrReadOnly) {
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

func TestShelfMoveBookFromDetectsUnscannedTargetConflict(t *testing.T) {
	source, target, targetLib := twoShelves(t)
	original := seedBook(t, source, FolderPath{"fiction"}, "Racer")
	id := original.ID()

	plantBookOnDisk(t, targetLib, FolderPath{"existing"}, id, "Planted")

	if _, err := target.MoveBookFrom(source, id, FolderPath{"archive"}); !errors.Is(err, ErrBookIDConflict) {
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

func TestShelfCopyBookFromNotFound(t *testing.T) {
	source, target, targetLib := twoShelves(t)

	if _, err := target.CopyBookFrom(source, "does-not-exist", FolderPath{"imported"}, false); !errors.Is(err, ErrBookNotFound) {
		t.Fatalf("CopyBookFrom of a missing book = %v, want ErrBookNotFound", err)
	}
	assertStagingClean(t, targetLib)
}
