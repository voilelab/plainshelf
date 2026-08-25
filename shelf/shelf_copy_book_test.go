package shelf

import (
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

// Copying a book must hand the copy a fresh ID so that the original and the copy
// can both be listed. This is the exact case a hand-made `cp -r` loses: the book
// cache keys on book.json's id, so two books sharing one collapse to a single
// listing entry and the other silently disappears.
func TestShelfCopyBookGivesNewIDAndCoexists(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	s := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	original, err := s.NewBook(FolderPath{"fiction"}, "Copy Me")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}
	if err := original.SetCover([]byte("cover bytes"), ".png"); err != nil {
		t.Fatalf("SetCover: %v", err)
	}
	source, err := original.NewSource(strings.NewReader("chapter body reading ![](assets/img-0001.png)\n"))
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	if err := source.WriteAsset("img-0001.png", []byte("asset bytes")); err != nil {
		t.Fatalf("WriteAsset: %v", err)
	}
	if err := original.SetCurrentSource(source.ID()); err != nil {
		t.Fatalf("SetCurrentSource: %v", err)
	}

	copied, err := s.CopyBook(original.ID(), FolderPath{"fiction"})
	if err != nil {
		t.Fatalf("CopyBook: %v", err)
	}

	if copied.ID() == original.ID() {
		t.Fatalf("the copy reuses the original ID %q", original.ID())
	}
	if err := validateBookID(copied.ID()); err != nil {
		t.Errorf("copy ID %q is not usable as one: %v", copied.ID(), err)
	}

	// A copy into the source's own folder under the same title must not overwrite
	// it: the folder name gets a suffix, exactly as a second new book would.
	if path.Base(copied.PackagePath()) == path.Base(original.PackagePath()) {
		t.Errorf("copy folder %q collides with the original", copied.PackagePath())
	}

	books, err := s.ListBooks()
	if err != nil {
		t.Fatalf("ListBooks: %v", err)
	}
	ids := map[string]bool{}
	for _, b := range books {
		ids[b.ID()] = true
	}
	if len(books) != 2 || !ids[original.ID()] || !ids[copied.ID()] {
		t.Fatalf("ListBooks holds %v, want exactly the original %q and the copy %q",
			ids, original.ID(), copied.ID())
	}

	// The copy is self-contained. Re-read it through the shelf so the checks run
	// against what reached disk, not the handle CopyBook returned.
	reopenedListing, err := s.GetBookListing(copied.ID())
	if err != nil {
		t.Fatalf("GetBookListing copy: %v", err)
	}
	reopened := reopenedListing.Book
	if reopened.Title() != "Copy Me" {
		t.Errorf("copy title = %q, want %q", reopened.Title(), "Copy Me")
	}
	if !reopenedListing.Folders.Equal(FolderPath{"fiction"}) {
		t.Errorf("copy layers = %v, want [fiction]", reopenedListing.Folders)
	}

	copiedSource, err := reopened.ResolveCurrentSource()
	if err != nil {
		t.Fatalf("ResolveCurrentSource copy: %v", err)
	}
	if copiedSource.ID() != source.ID() {
		t.Errorf("copy current source = %q, want the copied-over %q", copiedSource.ID(), source.ID())
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
		t.Errorf("copy source content = %q, want the original body", raw)
	}

	asset, err := copiedSource.OpenAsset("img-0001.png")
	if err != nil {
		t.Fatalf("OpenAsset copy: %v", err)
	}
	assetBytes, err := io.ReadAll(asset.File)
	asset.File.Close()
	if err != nil {
		t.Fatalf("read copy asset: %v", err)
	}
	if string(assetBytes) != "asset bytes" {
		t.Errorf("copy asset = %q, want %q", assetBytes, "asset bytes")
	}

	cover, ext, err := reopened.OpenCover()
	if err != nil {
		t.Fatalf("OpenCover copy: %v", err)
	}
	if string(cover) != "cover bytes" || ext != ".png" {
		t.Errorf("copy cover = %q, %q, want %q, .png", cover, ext, "cover bytes")
	}

	// Independent IDs mean independent files: writing to the copy leaves the
	// original untouched.
	meta := reopened.GetMeta()
	meta.Title = "Renamed Copy"
	if err := reopened.SetMeta(meta); err != nil {
		t.Fatalf("SetMeta copy: %v", err)
	}
	originalAfter, err := s.GetBook(original.ID())
	if err != nil {
		t.Fatalf("GetBook original: %v", err)
	}
	if originalAfter.Title() != "Copy Me" {
		t.Errorf("original title = %q, want it left as %q", originalAfter.Title(), "Copy Me")
	}

	// The copy is published by renaming it out of staging, so a successful copy
	// must leave nothing behind under app/tmp.
	staged, err := os.ReadDir(path.Join(tmpLib, appFolder, appTmpFolder))
	if err == nil && len(staged) != 0 {
		t.Errorf("staging folder not clean after a copy: %d entries", len(staged))
	}
}

// A copy can land in a different folder than the source, and the destination
// folder becomes visible without waiting for the next scan.
func TestShelfCopyBookIntoAnotherFolder(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	s := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	original, err := s.NewBook(FolderPath{"origin"}, "Move And Copy")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}

	copied, err := s.CopyBook(original.ID(), FolderPath{"elsewhere", "deep"})
	if err != nil {
		t.Fatalf("CopyBook: %v", err)
	}
	copiedListing, err := s.GetBookListing(copied.ID())
	if err != nil {
		t.Fatalf("GetBookListing: %v", err)
	}
	if !copiedListing.Folders.Equal(FolderPath{"elsewhere", "deep"}) {
		t.Fatalf("copy layers = %v, want [elsewhere deep]", copiedListing.Folders)
	}

	books, err := s.GetBooksByFolder(FolderPath{"elsewhere", "deep"})
	if err != nil {
		t.Fatalf("GetBooksByFolder: %v", err)
	}
	if len(books) != 1 || books[0].ID() != copied.ID() {
		t.Fatalf("GetBooksByFolder returned %d books, want just the copy %q", len(books), copied.ID())
	}
}

// A symlinked directory inside a book package must be copied as a real
// directory. os.Root lists a symlink as a non-directory but stats through it as
// a directory, so a copy that keyed on the directory entry alone would try to
// copy the link as a file and fail with "is a directory".
func TestShelfCopyBookFollowsSymlinkedDir(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	s := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	original, err := s.NewBook(FolderPath{"fiction"}, "Linked")
	if err != nil {
		t.Fatalf("NewBook: %v", err)
	}

	bookAbs := filepath.Join(tmpLib, original.PackagePath())
	if err := os.MkdirAll(filepath.Join(bookAbs, "extra"), 0o755); err != nil {
		t.Fatalf("MkdirAll extra: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bookAbs, "extra", "note.txt"), []byte("linked bytes"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	// A relative link to a sibling keeps the target inside the shelf root, so
	// os.Root follows it rather than refusing it as an escape.
	if err := os.Symlink("extra", filepath.Join(bookAbs, "linked")); err != nil {
		t.Skipf("symlinks unavailable on this platform: %v", err)
	}

	copied, err := s.CopyBook(original.ID(), FolderPath{"fiction"})
	if err != nil {
		t.Fatalf("CopyBook: %v", err)
	}

	copyAbs := filepath.Join(tmpLib, copied.PackagePath())
	got, err := os.ReadFile(filepath.Join(copyAbs, "linked", "note.txt"))
	if err != nil {
		t.Fatalf("read copied linked file: %v", err)
	}
	if string(got) != "linked bytes" {
		t.Errorf("copied linked file = %q, want %q", got, "linked bytes")
	}
}

// Copying an unknown book reports ErrBookNotFound and leaves nothing staged.
func TestShelfCopyBookNotFound(t *testing.T) {
	tmpLib := path.Join(t.TempDir(), "shelf_test")
	s := newTestShelf(t, &ShelfConf{LibRoot: tmpLib})

	if _, err := s.CopyBook("does-not-exist", FolderPath{"fiction"}); !errors.Is(err, ErrBookNotFound) {
		t.Fatalf("CopyBook of a missing book error = %v, want ErrBookNotFound", err)
	}

	staged, err := os.ReadDir(path.Join(tmpLib, appFolder, appTmpFolder))
	if err == nil && len(staged) != 0 {
		t.Errorf("staging folder not clean after a failed copy: %d entries", len(staged))
	}
}
