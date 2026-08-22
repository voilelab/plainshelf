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
// which the layer tests need to assert the source subtree is gone after a move.
func twoShelvesWithLibs(t *testing.T) (source, target *Shelf, sourceLib, targetLib string) {
	t.Helper()
	sourceLib = path.Join(t.TempDir(), "source")
	targetLib = path.Join(t.TempDir(), "target")
	source = newTestShelf(t, &ShelfConf{LibRoot: sourceLib})
	target = newTestShelf(t, &ShelfConf{LibRoot: targetLib})
	return source, target, sourceLib, targetLib
}

func layerDirExists(t *testing.T, lib string, layer Layers) bool {
	t.Helper()
	segs := append([]string{lib, booksFolder}, layer...)
	_, err := os.Stat(filepath.Join(segs...))
	if err == nil {
		return true
	}
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	t.Fatalf("stat layer dir %v: %v", layer, err)
	return false
}

// bookIDsByLayer returns the IDs of the books the target lists directly under
// layer, sorted, so a test can assert the exact set landed there.
func bookIDsByLayer(t *testing.T, s *Shelf, layer Layers) []string {
	t.Helper()
	books, err := s.GetBooksByLayer(layer)
	if err != nil {
		t.Fatalf("GetBooksByLayer %v: %v", layer, err)
	}
	ids := make([]string, 0, len(books))
	for _, b := range books {
		ids = append(ids, b.ID())
	}
	sort.Strings(ids)
	return ids
}

// A cross-shelf layer copy reproduces the whole subtree - nested layers, an
// empty sub-folder, and every book - on the target, gives each book a fresh ID,
// and leaves the source completely intact.
func TestShelfCopyLayerFromNestedGivesNewIDs(t *testing.T) {
	source, target, _, targetLib := twoShelvesWithLibs(t)

	top := seedBook(t, source, Layers{"fiction"}, "Top")
	mid := seedBook(t, source, Layers{"fiction", "sci-fi"}, "Mid")
	deep := seedBook(t, source, Layers{"fiction", "sci-fi", "hard"}, "Deep")
	// A sub-layer that holds no book, to prove the structure carries across even
	// where there is nothing to publish.
	if err := source.NewLayer(Layers{"fiction", "empty"}); err != nil {
		t.Fatalf("NewLayer: %v", err)
	}

	copied, err := target.CopyLayerFrom(source, Layers{"fiction"}, Layers{"imported"})
	if err != nil {
		t.Fatalf("CopyLayerFrom: %v", err)
	}
	if len(copied) != 3 {
		t.Fatalf("CopyLayerFrom copied %d books, want 3", len(copied))
	}

	// Every original book lands under its remapped layer, each with a new ID.
	for _, tc := range []struct {
		original *Book
		layer    Layers
	}{
		{top, Layers{"imported"}},
		{mid, Layers{"imported", "sci-fi"}},
		{deep, Layers{"imported", "sci-fi", "hard"}},
	} {
		ids := bookIDsByLayer(t, target, tc.layer)
		if len(ids) != 1 {
			t.Fatalf("target layer %v holds %v, want one book", tc.layer, ids)
		}
		if ids[0] == tc.original.ID() {
			t.Errorf("copy under %v reused the original ID %q", tc.layer, tc.original.ID())
		}
	}

	// The empty sub-folder is reproduced on the target.
	if !layerDirExists(t, targetLib, Layers{"imported", "empty"}) {
		t.Errorf("empty sub-layer was not reproduced on the target")
	}

	// The source keeps all three books under their original IDs and layers.
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

// A cross-shelf layer move keeps every book's ID, publishes the subtree on the
// target, empties the source layer into its trash, and removes the now-empty
// source subtree.
func TestShelfMoveLayerFromNestedPreservesIDs(t *testing.T) {
	source, target, sourceLib, targetLib := twoShelvesWithLibs(t)

	top := seedBook(t, source, Layers{"fiction"}, "Top")
	mid := seedBook(t, source, Layers{"fiction", "sci-fi"}, "Mid")
	topID, midID := top.ID(), mid.ID()

	moved, err := target.MoveLayerFrom(source, Layers{"fiction"}, Layers{"archive"})
	if err != nil {
		t.Fatalf("MoveLayerFrom: %v", err)
	}
	if len(moved) != 2 {
		t.Fatalf("MoveLayerFrom moved %d books, want 2", len(moved))
	}

	// The target lists both books, under their original IDs, at the remapped layers.
	if got := bookIDsByLayer(t, target, Layers{"archive"}); len(got) != 1 || got[0] != topID {
		t.Errorf("target [archive] = %v, want [%s]", got, topID)
	}
	if got := bookIDsByLayer(t, target, Layers{"archive", "sci-fi"}); len(got) != 1 || got[0] != midID {
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
	if layerDirExists(t, sourceLib, Layers{"fiction"}) {
		t.Errorf("source layer subtree survived the move")
	}

	assertStagingClean(t, targetLib)
}

// A move whose books partly collide with the target's is refused, and the whole
// colliding set is reported at once - not just the first ID. Neither shelf is
// modified: the source keeps every book and the target gains no subtree.
func TestShelfMoveLayerFromReportsAllIDConflicts(t *testing.T) {
	source, target, sourceLib, targetLib := twoShelvesWithLibs(t)

	a := seedBook(t, source, Layers{"fiction"}, "Alpha")
	b := seedBook(t, source, Layers{"fiction", "sci-fi"}, "Bravo")
	c := seedBook(t, source, Layers{"fiction", "sci-fi"}, "Charlie")

	// Two of the three IDs already sit on the target's disk, added the way another
	// instance would - so pre-flight must rescan to see them and must report both.
	plantBookOnDisk(t, targetLib, Layers{"existing-a"}, a.ID(), "Planted A")
	plantBookOnDisk(t, targetLib, Layers{"existing-c"}, c.ID(), "Planted C")

	_, err := target.MoveLayerFrom(source, Layers{"fiction"}, Layers{"archive"})
	if !errors.Is(err, ErrBookIDConflict) {
		t.Fatalf("MoveLayerFrom into a conflict = %v, want ErrBookIDConflict", err)
	}
	var conflict *LayerBookIDConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("error %v is not a *LayerBookIDConflictError", err)
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
	if layerDirExists(t, sourceLib, Layers{"fiction"}) == false {
		t.Errorf("refused move removed the source layer")
	}

	// The target gained no destination subtree - pre-flight failed before any file
	// was touched.
	if layerDirExists(t, targetLib, Layers{"archive"}) {
		t.Errorf("refused move left a destination layer on the target")
	}

	assertStagingClean(t, targetLib)
}

// A read-only target refuses a layer transfer before it stages or creates
// anything: the source is untouched and no destination subtree appears.
func TestShelfCopyLayerFromRefusesReadOnlyTarget(t *testing.T) {
	sourceLib := path.Join(t.TempDir(), "source")
	targetLib := path.Join(t.TempDir(), "target")
	source := newTestShelf(t, &ShelfConf{LibRoot: sourceLib})
	original := seedBook(t, source, Layers{"fiction"}, "Locked Out")

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

	if _, err := target.CopyLayerFrom(source, Layers{"fiction"}, Layers{"imported"}); !errors.Is(err, fsutil.ErrReadOnly) {
		t.Fatalf("CopyLayerFrom into a read-only target = %v, want fsutil.ErrReadOnly", err)
	}
	if _, err := target.MoveLayerFrom(source, Layers{"fiction"}, Layers{"imported"}); !errors.Is(err, fsutil.ErrReadOnly) {
		t.Fatalf("MoveLayerFrom into a read-only target = %v, want fsutil.ErrReadOnly", err)
	}

	if _, err := source.GetBook(original.ID()); err != nil {
		t.Errorf("source modified by a refused read-only transfer: %v", err)
	}
	if layerDirExists(t, targetLib, Layers{"imported"}) {
		t.Errorf("refused read-only transfer created a destination layer")
	}
}

// A move from a read-only source is refused before anything is copied: the
// target gains nothing and the source keeps its layer.
func TestShelfMoveLayerFromRefusesReadOnlySource(t *testing.T) {
	sourceLib := path.Join(t.TempDir(), "source")

	seed, err := NewShelf(&ShelfConf{LibRoot: sourceLib})
	if err != nil {
		t.Fatalf("seed source shelf: %v", err)
	}
	if err := seed.WaitReady(t.Context()); err != nil {
		t.Fatalf("WaitReady seed: %v", err)
	}
	original := seedBook(t, seed, Layers{"fiction"}, "Frozen")
	id := original.ID()
	if err := seed.Close(); err != nil {
		t.Fatalf("close seed: %v", err)
	}

	source := newTestShelf(t, &ShelfConf{LibRoot: sourceLib, ReadOnly: true})
	target := newTestShelf(t, &ShelfConf{LibRoot: path.Join(t.TempDir(), "target")})

	if _, err := target.MoveLayerFrom(source, Layers{"fiction"}, Layers{"archive"}); !errors.Is(err, fsutil.ErrReadOnly) {
		t.Fatalf("MoveLayerFrom from a read-only source = %v, want fsutil.ErrReadOnly", err)
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

// A transfer onto a layer the target already holds is refused, and nothing is
// staged or moved.
func TestShelfCopyLayerFromRefusesExistingTargetLayer(t *testing.T) {
	source, target, _, targetLib := twoShelvesWithLibs(t)
	seedBook(t, source, Layers{"fiction"}, "Original")

	if err := target.NewLayer(Layers{"imported"}); err != nil {
		t.Fatalf("NewLayer: %v", err)
	}

	if _, err := target.CopyLayerFrom(source, Layers{"fiction"}, Layers{"imported"}); !errors.Is(err, ErrTargetLayerExists) {
		t.Fatalf("CopyLayerFrom onto an existing layer = %v, want ErrTargetLayerExists", err)
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

// A missing source layer is refused before any work, for both copy and move.
func TestShelfCopyLayerFromMissingSourceLayer(t *testing.T) {
	source, target, _, _ := twoShelvesWithLibs(t)
	seedBook(t, source, Layers{"fiction"}, "Elsewhere")

	if _, err := target.CopyLayerFrom(source, Layers{"nope"}, Layers{"imported"}); !errors.Is(err, ErrSourceLayerNotFound) {
		t.Fatalf("CopyLayerFrom of a missing layer = %v, want ErrSourceLayerNotFound", err)
	}
}

// Naming one shelf as both source and target is refused rather than deadlocking.
func TestShelfLayerTransferSameShelfRefused(t *testing.T) {
	s := newTestShelf(t, &ShelfConf{LibRoot: path.Join(t.TempDir(), "shelf")})
	seedBook(t, s, Layers{"fiction"}, "Here")

	if _, err := s.CopyLayerFrom(s, Layers{"fiction"}, Layers{"elsewhere"}); !errors.Is(err, ErrSameShelfTransfer) {
		t.Fatalf("CopyLayerFrom onto the same shelf = %v, want ErrSameShelfTransfer", err)
	}
}
