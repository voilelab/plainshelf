package shelf

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"sort"
	"testing"

	"github.com/voilelab/plainshelf/internal/fsutil"
)

// twoShelvesWithLibs is twoShelves but also hands back the source library path,
// which the folder tests need to assert the source subtree is gone after a move.
func twoShelvesWithLibs(t *testing.T) (source, target *Shelf, sourceLib, targetLib string) {
	t.Helper()
	sourceLib = path.Join(t.TempDir(), "source")
	targetLib = path.Join(t.TempDir(), "target")
	source = newTestShelf(t, &ShelfConf{LibRoot: sourceLib})
	target = newTestShelf(t, &ShelfConf{LibRoot: targetLib})
	return source, target, sourceLib, targetLib
}

func folderDirExists(t *testing.T, lib string, folder FolderPath) bool {
	t.Helper()
	segs := append([]string{lib, booksFolder}, folder...)
	_, err := os.Stat(filepath.Join(segs...))
	if err == nil {
		return true
	}
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	t.Fatalf("stat layer dir %v: %v", folder, err)
	return false
}

// bookIDsByFolder returns the IDs of the books the target lists directly under
// folder, sorted, so a test can assert the exact set landed there.
func bookIDsByFolder(t *testing.T, s *Shelf, folder FolderPath) []string {
	t.Helper()
	books, err := s.GetBooksByFolder(folder)
	if err != nil {
		t.Fatalf("GetBooksByFolder %v: %v", folder, err)
	}
	ids := make([]string, 0, len(books))
	for _, b := range books {
		ids = append(ids, b.ID())
	}
	sort.Strings(ids)
	return ids
}

// A cross-shelf folder copy reproduces the whole subtree - nested folders, an
// empty sub-folder, and every book - on the target, gives each book a fresh ID,
// and leaves the source completely intact.
func TestShelfCopyFolderFromNestedGivesNewIDs(t *testing.T) {
	source, target, _, targetLib := twoShelvesWithLibs(t)

	top := seedBook(t, source, FolderPath{"fiction"}, "Top")
	mid := seedBook(t, source, FolderPath{"fiction", "sci-fi"}, "Mid")
	deep := seedBook(t, source, FolderPath{"fiction", "sci-fi", "hard"}, "Deep")
	// A sub-folder that holds no book, to prove the structure carries across even
	// where there is nothing to publish.
	if err := source.NewFolder(FolderPath{"fiction"}, "empty"); err != nil {
		t.Fatalf("NewFolder: %v", err)
	}

	copied, err := target.CopyFolderFrom(source, FolderPath{"fiction"}, FolderPath{"imported"})
	if err != nil {
		t.Fatalf("CopyFolderFrom: %v", err)
	}
	if len(copied) != 3 {
		t.Fatalf("CopyFolderFrom copied %d books, want 3", len(copied))
	}

	// Every original book lands under its remapped folder, each with a new ID.
	for _, tc := range []struct {
		original *Book
		folder    FolderPath
	}{
		{top, FolderPath{"imported"}},
		{mid, FolderPath{"imported", "sci-fi"}},
		{deep, FolderPath{"imported", "sci-fi", "hard"}},
	} {
		ids := bookIDsByFolder(t, target, tc.folder)
		if len(ids) != 1 {
			t.Fatalf("target layer %v holds %v, want one book", tc.folder, ids)
		}
		if ids[0] == tc.original.ID() {
			t.Errorf("copy under %v reused the original ID %q", tc.folder, tc.original.ID())
		}
	}

	// The empty sub-folder is reproduced on the target.
	if !folderDirExists(t, targetLib, FolderPath{"imported", "empty"}) {
		t.Errorf("empty sub-layer was not reproduced on the target")
	}

	// The source keeps all three books under their original IDs and folders.
	for _, b := range []*Book{top, mid, deep} {
		if _, err := source.GetBook(b.ID()); err != nil {
			t.Errorf("source lost %q after a copy: %v", b.ID(), err)
		}
	}
	sourceBooks, err := source.ListBooks()
	if err != nil {
		t.Fatalf("source ListBooks: %v", err)
	}
	if len(sourceBooks) != 3 {
		t.Errorf("source holds %d books after a copy, want 3", len(sourceBooks))
	}

	assertStagingClean(t, targetLib)
}

// A cross-shelf folder move keeps every book's ID, publishes the subtree on the
// target, empties the source folder into its trash, and removes the now-empty
// source subtree.
func TestShelfMoveFolderFromNestedPreservesIDs(t *testing.T) {
	source, target, sourceLib, targetLib := twoShelvesWithLibs(t)

	top := seedBook(t, source, FolderPath{"fiction"}, "Top")
	mid := seedBook(t, source, FolderPath{"fiction", "sci-fi"}, "Mid")
	topID, midID := top.ID(), mid.ID()

	moved, err := target.MoveFolderFrom(source, FolderPath{"fiction"}, FolderPath{"archive"})
	if err != nil {
		t.Fatalf("MoveFolderFrom: %v", err)
	}
	if len(moved) != 2 {
		t.Fatalf("MoveFolderFrom moved %d books, want 2", len(moved))
	}

	// The target lists both books, under their original IDs, at the remapped folders.
	if got := bookIDsByFolder(t, target, FolderPath{"archive"}); len(got) != 1 || got[0] != topID {
		t.Errorf("target [archive] = %v, want [%s]", got, topID)
	}
	if got := bookIDsByFolder(t, target, FolderPath{"archive", "sci-fi"}); len(got) != 1 || got[0] != midID {
		t.Errorf("target [archive sci-fi] = %v, want [%s]", got, midID)
	}

	// The source no longer lists either book but keeps both recoverable in trash.
	for _, id := range []string{topID, midID} {
		if _, err := source.GetBook(id); !errors.Is(err, ErrBookNotFound) {
			t.Errorf("source still lists moved book %q, err = %v", id, err)
		}
		inTrash, err := source.isBookIDInTrash(id)
		if err != nil {
			t.Fatalf("isBookIDInTrash: %v", err)
		}
		if !inTrash {
			t.Errorf("moved book %q is not recoverable in the source trash", id)
		}
	}

	// The emptied source subtree is gone.
	if folderDirExists(t, sourceLib, FolderPath{"fiction"}) {
		t.Errorf("source layer subtree survived the move")
	}

	assertStagingClean(t, targetLib)
}

// plantBrokenPackageOnDisk writes a .bookpkg directory whose book.json is
// unparseable, so bookpkg.Open fails on it and the scan skips it - the shape of
// a corrupt package a move must not silently destroy. It returns the package's
// on-disk path.
func plantBrokenPackageOnDisk(t *testing.T, lib string, folder FolderPath, folderName string) string {
	t.Helper()
	dir := filepath.Join(append([]string{lib, booksFolder}, folder...)...)
	dir = filepath.Join(dir, folderName+bookExtension)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll broken package: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, BookMetaFile), []byte("{ not valid json"), 0o644); err != nil {
		t.Fatalf("write broken book.json: %v", err)
	}
	return dir
}

// A move clears the emptied source folders but must not delete a package the
// scan could not read: it was never in the transfer plan, so blowing away the
// subtree would destroy it permanently, bypassing the trash a delete routes
// through. The unreadable package - and the folder holding it - survive.
func TestShelfMoveFolderFromPreservesUnreadablePackage(t *testing.T) {
	source, target, sourceLib, targetLib := twoShelvesWithLibs(t)

	good := seedBook(t, source, FolderPath{"fiction"}, "Readable")
	goodID := good.ID()
	// An empty sub-folder that should be pruned, and a corrupt package that must not.
	if err := source.NewFolder(FolderPath{"fiction"}, "empty"); err != nil {
		t.Fatalf("NewFolder: %v", err)
	}
	brokenPath := plantBrokenPackageOnDisk(t, sourceLib, FolderPath{"fiction"}, "corrupt")

	moved, err := target.MoveFolderFrom(source, FolderPath{"fiction"}, FolderPath{"archive"})
	if err != nil {
		t.Fatalf("MoveFolderFrom: %v", err)
	}
	if len(moved) != 1 || moved[0].ID() != goodID {
		t.Fatalf("moved = %v, want just the readable book %s", moved, goodID)
	}

	// The corrupt package is still on disk, untouched.
	if _, err := os.Stat(brokenPath); err != nil {
		t.Errorf("move destroyed the unreadable package: %v", err)
	}
	// Its containing folder is kept because it is not empty...
	if !folderDirExists(t, sourceLib, FolderPath{"fiction"}) {
		t.Errorf("move removed a source folder that still held a package")
	}
	// ...while the genuinely empty sub-folder beside it is pruned.
	if folderDirExists(t, sourceLib, FolderPath{"fiction", "empty"}) {
		t.Errorf("move left an empty source sub-folder behind")
	}

	// The readable book still moved across and is recoverable on the source.
	if _, err := target.GetBook(goodID); err != nil {
		t.Errorf("target missing the moved book: %v", err)
	}
	assertStagingClean(t, targetLib)
}

// A move whose books partly collide with the target's is refused, and the whole
// colliding set is reported at once - not just the first ID. Neither shelf is
// modified: the source keeps every book and the target gains no subtree.
func TestShelfMoveFolderFromReportsAllIDConflicts(t *testing.T) {
	source, target, sourceLib, targetLib := twoShelvesWithLibs(t)

	a := seedBook(t, source, FolderPath{"fiction"}, "Alpha")
	b := seedBook(t, source, FolderPath{"fiction", "sci-fi"}, "Bravo")
	c := seedBook(t, source, FolderPath{"fiction", "sci-fi"}, "Charlie")

	// Two of the three IDs already sit on the target's disk, added the way another
	// instance would - so pre-flight must rescan to see them and must report both.
	plantBookOnDisk(t, targetLib, FolderPath{"existing-a"}, a.ID(), "Planted A")
	plantBookOnDisk(t, targetLib, FolderPath{"existing-c"}, c.ID(), "Planted C")

	_, err := target.MoveFolderFrom(source, FolderPath{"fiction"}, FolderPath{"archive"})
	if !errors.Is(err, ErrBookIDConflict) {
		t.Fatalf("MoveFolderFrom into a conflict = %v, want ErrBookIDConflict", err)
	}
	var conflict *FolderBookIDConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("error %v is not a *FolderBookIDConflictError", err)
	}
	got := append([]string(nil), conflict.BookIDs...)
	sort.Strings(got)
	want := []string{a.ID(), c.ID()}
	sort.Strings(want)
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("reported conflicts = %v, want all of %v", got, want)
	}

	// The source is untouched: all three books still listed, none trashed.
	for _, bk := range []*Book{a, b, c} {
		if _, err := source.GetBook(bk.ID()); err != nil {
			t.Errorf("source lost %q after a refused move: %v", bk.ID(), err)
		}
		inTrash, err := source.isBookIDInTrash(bk.ID())
		if err != nil {
			t.Fatalf("isBookIDInTrash: %v", err)
		}
		if inTrash {
			t.Errorf("a refused move trashed source book %q", bk.ID())
		}
	}
	if folderDirExists(t, sourceLib, FolderPath{"fiction"}) == false {
		t.Errorf("refused move removed the source layer")
	}

	// The target gained no destination subtree - pre-flight failed before any file
	// was touched.
	if folderDirExists(t, targetLib, FolderPath{"archive"}) {
		t.Errorf("refused move left a destination layer on the target")
	}

	assertStagingClean(t, targetLib)
}

// A read-only target refuses a folder transfer before it stages or creates
// anything: the source is untouched and no destination subtree appears.
func TestShelfCopyFolderFromRefusesReadOnlyTarget(t *testing.T) {
	sourceLib := path.Join(t.TempDir(), "source")
	targetLib := path.Join(t.TempDir(), "target")
	source := newTestShelf(t, &ShelfConf{LibRoot: sourceLib})
	original := seedBook(t, source, FolderPath{"fiction"}, "Locked Out")

	// Lay the target's structure down writable, then reopen it read-only.
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

	if _, err := target.CopyFolderFrom(source, FolderPath{"fiction"}, FolderPath{"imported"}); !errors.Is(err, fsutil.ErrReadOnly) {
		t.Fatalf("CopyFolderFrom into a read-only target = %v, want fsutil.ErrReadOnly", err)
	}
	if _, err := target.MoveFolderFrom(source, FolderPath{"fiction"}, FolderPath{"imported"}); !errors.Is(err, fsutil.ErrReadOnly) {
		t.Fatalf("MoveFolderFrom into a read-only target = %v, want fsutil.ErrReadOnly", err)
	}

	if _, err := source.GetBook(original.ID()); err != nil {
		t.Errorf("source modified by a refused read-only transfer: %v", err)
	}
	if folderDirExists(t, targetLib, FolderPath{"imported"}) {
		t.Errorf("refused read-only transfer created a destination layer")
	}
}

// A move from a read-only source is refused before anything is copied: the
// target gains nothing and the source keeps its folder.
func TestShelfMoveFolderFromRefusesReadOnlySource(t *testing.T) {
	sourceLib := path.Join(t.TempDir(), "source")

	seed, err := NewShelf(&ShelfConf{LibRoot: sourceLib})
	if err != nil {
		t.Fatalf("seed source shelf: %v", err)
	}
	if err := seed.WaitReady(t.Context()); err != nil {
		t.Fatalf("WaitReady seed: %v", err)
	}
	original := seedBook(t, seed, FolderPath{"fiction"}, "Frozen")
	id := original.ID()
	if err := seed.Close(); err != nil {
		t.Fatalf("close seed: %v", err)
	}

	source := newTestShelf(t, &ShelfConf{LibRoot: sourceLib, ReadOnly: true})
	target := newTestShelf(t, &ShelfConf{LibRoot: path.Join(t.TempDir(), "target")})

	if _, err := target.MoveFolderFrom(source, FolderPath{"fiction"}, FolderPath{"archive"}); !errors.Is(err, fsutil.ErrReadOnly) {
		t.Fatalf("MoveFolderFrom from a read-only source = %v, want fsutil.ErrReadOnly", err)
	}

	targetBooks, err := target.ListBooks()
	if err != nil {
		t.Fatalf("target ListBooks: %v", err)
	}
	if len(targetBooks) != 0 {
		t.Errorf("a refused read-only-source move published %d books, want none", len(targetBooks))
	}
	if _, err := source.GetBook(id); err != nil {
		t.Errorf("read-only source lost the book: %v", err)
	}
}

// A transfer onto a folder the target already holds is refused, and nothing is
// staged or moved.
func TestShelfCopyFolderFromRefusesExistingTargetFolder(t *testing.T) {
	source, target, _, targetLib := twoShelvesWithLibs(t)
	seedBook(t, source, FolderPath{"fiction"}, "Original")

	if err := target.NewFolder(FolderPath{}, "imported"); err != nil {
		t.Fatalf("NewFolder: %v", err)
	}

	if _, err := target.CopyFolderFrom(source, FolderPath{"fiction"}, FolderPath{"imported"}); !errors.Is(err, ErrTargetFolderExists) {
		t.Fatalf("CopyFolderFrom onto an existing layer = %v, want ErrTargetFolderExists", err)
	}

	targetBooks, err := target.ListBooks()
	if err != nil {
		t.Fatalf("target ListBooks: %v", err)
	}
	if len(targetBooks) != 0 {
		t.Errorf("a refused transfer published %d books, want none", len(targetBooks))
	}
	assertStagingClean(t, targetLib)
}

// A missing source folder is refused before any work, for both copy and move.
func TestShelfCopyFolderFromMissingSourceFolder(t *testing.T) {
	source, target, _, _ := twoShelvesWithLibs(t)
	seedBook(t, source, FolderPath{"fiction"}, "Elsewhere")

	if _, err := target.CopyFolderFrom(source, FolderPath{"nope"}, FolderPath{"imported"}); !errors.Is(err, ErrSourceFolderNotFound) {
		t.Fatalf("CopyFolderFrom of a missing layer = %v, want ErrSourceFolderNotFound", err)
	}
}

// Naming one shelf as both source and target is refused rather than deadlocking.
func TestShelfFolderTransferSameShelfRefused(t *testing.T) {
	s := newTestShelf(t, &ShelfConf{LibRoot: path.Join(t.TempDir(), "shelf")})
	seedBook(t, s, FolderPath{"fiction"}, "Here")

	if _, err := s.CopyFolderFrom(s, FolderPath{"fiction"}, FolderPath{"elsewhere"}); !errors.Is(err, ErrSameShelfTransfer) {
		t.Fatalf("CopyFolderFrom onto the same shelf = %v, want ErrSameShelfTransfer", err)
	}
}
