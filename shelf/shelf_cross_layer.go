package shelf

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
)

// ErrTargetLayerExists is returned by a cross-shelf layer transfer when the
// target shelf already holds the destination layer. A layer transfer builds the
// whole subtree fresh on the target, so it refuses to land on an existing layer
// rather than merging into it - the same rule RenameLayer enforces for a
// same-shelf rename.
var ErrTargetLayerExists = util.NewError("target shelf already holds a layer with this name")

// ErrSourceLayerNotFound is returned when a cross-shelf layer transfer names a
// source layer that does not exist on the source shelf.
var ErrSourceLayerNotFound = util.NewError("source shelf has no such layer")

// LayerBookIDConflictError reports that a cross-shelf layer move would land on
// book IDs the target shelf already holds. Pre-flight collects every colliding
// ID before touching a single file, so the whole set is reported at once rather
// than surfacing them one failed move at a time.
//
// It satisfies errors.Is(err, ErrBookIDConflict) so a caller that only cares
// that a move collided can test for the same sentinel the single-book move
// returns, while one that wants the full list type-asserts to *LayerBookIDConflictError.
type LayerBookIDConflictError struct {
	BookIDs []string
}

func (e *LayerBookIDConflictError) Error() string {
	return fmt.Sprintf("target shelf already holds books with these IDs: %s", strings.Join(e.BookIDs, ", "))
}

// Is lets errors.Is match the shared ErrBookIDConflict sentinel, so callers do
// not have to special-case the layer-level aggregate.
func (e *LayerBookIDConflictError) Is(target error) bool {
	return target == ErrBookIDConflict
}

// layerTransferPlan is what pre-flight resolves before any file is touched: the
// books a transfer will carry and the layer directories it will reproduce. Both
// are gathered from a single fresh scan of the source so the plan reflects books
// and folders another instance or a file manager added since the last scan.
type layerTransferPlan struct {
	// books is every book under sourceLayer (the layer itself and all
	// descendants) paired with the source layer it sits in, sorted by ID for a
	// deterministic transfer order. Pairing the layer here fixes each book's
	// destination from the pre-flight snapshot rather than re-reading it per book.
	books []plannedBook
	// sourceLayers is sourceLayer and every descendant layer, so empty folders
	// are reproduced on the target too, not only the ones that hold a book.
	sourceLayers []Layers
}

type plannedBook struct {
	id    string
	layer Layers
}

// CopyLayerFrom copies the whole layer sourceLayer - the folder and every book
// and sub-folder beneath it - out of source and into this (the target) shelf
// under targetLayer, returning the books it copied. Each book is copied through
// CopyBookFrom, so every copy is minted a fresh ID drawn from the target and
// carries no reading progress, exactly like a new import: the original layer on
// the source is left completely intact.
//
// A book at source layer sourceLayer/x/y lands on the target at
// targetLayer/x/y, so the subtree's shape is preserved under its new root.
//
// Pre-flight runs to completion before any file is touched: the transfer is
// refused if the two shelves are the same, if the target is read-only, if
// sourceLayer does not exist, or if targetLayer already exists on the target. A
// copy draws fresh IDs, so it never checks for a book-ID conflict. When
// pre-flight refuses the transfer, neither shelf has been modified.
//
// There is deliberately no rollback once execution begins. Pre-flight has
// already ruled out every foreseeable conflict, so a failure mid-transfer is a
// rare I/O or space error; the copies published so far are returned and the
// caller can re-run to finish the rest.
func (target *Shelf) CopyLayerFrom(source *Shelf, sourceLayer, targetLayer Layers) ([]*Book, error) {
	return target.transferLayerFrom(source, sourceLayer, targetLayer, false)
}

// MoveLayerFrom moves the whole layer sourceLayer out of source and into this
// (the target) shelf under targetLayer, returning the books it moved. Each book
// is moved through MoveBookFrom, so every book keeps its ID and reading progress
// and is published on the target before it is removed from the source; the
// removed source books route through the source's trash, so a move stays
// recoverable on the source side until that trash is emptied. Once every book has
// moved, the now-empty source layer subtree is removed.
//
// A book at source layer sourceLayer/x/y lands on the target at
// targetLayer/x/y, so the subtree's shape is preserved under its new root.
//
// Pre-flight runs to completion before any file is touched: the transfer is
// refused if the two shelves are the same, if either shelf is read-only, if
// sourceLayer does not exist, or if targetLayer already exists on the target. A
// move keeps each book's ID, so pre-flight also refuses the transfer when the
// target already holds any of those IDs, reporting the whole colliding set at
// once through *LayerBookIDConflictError rather than stopping at the first. When
// pre-flight refuses the transfer, neither shelf has been modified.
//
// There is deliberately no rollback once execution begins. Pre-flight has
// already ruled out every foreseeable conflict, so a failure mid-transfer is a
// rare I/O or space error; the books moved so far are returned and, because a
// moved book is no longer on the source, the caller can simply re-run to finish
// the rest.
func (target *Shelf) MoveLayerFrom(source *Shelf, sourceLayer, targetLayer Layers) ([]*Book, error) {
	return target.transferLayerFrom(source, sourceLayer, targetLayer, true)
}

// transferLayerFrom is the shared body of CopyLayerFrom and MoveLayerFrom. isMove
// selects between the two: a move preserves book IDs, checks for ID conflicts,
// requires a writable source, and clears the source subtree at the end, while a
// copy does none of those.
func (target *Shelf) transferLayerFrom(source *Shelf, sourceLayer, targetLayer Layers, isMove bool) ([]*Book, error) {
	if err := validateLayers(sourceLayer); err != nil {
		return nil, util.Errorf("invalid source layer: %w", err)
	}
	if err := validateLayers(targetLayer); err != nil {
		return nil, util.Errorf("invalid target layer: %w", err)
	}
	if len(sourceLayer) == 0 {
		return nil, util.Errorf("cannot transfer the root layer")
	}
	if len(targetLayer) == 0 {
		return nil, util.Errorf("cannot transfer onto the root layer")
	}
	if source == target {
		return nil, util.Errorf("%w", ErrSameShelfTransfer)
	}

	plan, err := target.preflightLayerTransfer(source, sourceLayer, targetLayer, isMove)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	// Reproduce the folder structure first - including layers that hold no book -
	// so an empty sub-folder survives the transfer. Layers that do hold a book are
	// re-created here too, harmlessly, since publishing a book MkdirAll's its own
	// layer as well.
	for _, sl := range plan.sourceLayers {
		if err := target.NewLayer(mapLayerPath(sl, sourceLayer, targetLayer)); err != nil {
			return nil, util.Errorf("%w", err)
		}
	}

	transferred := make([]*Book, 0, len(plan.books))
	for _, planned := range plan.books {
		dstLayer := mapLayerPath(planned.layer, sourceLayer, targetLayer)

		var book *Book
		var err error
		if isMove {
			book, err = target.MoveBookFrom(source, planned.id, dstLayer)
		} else {
			book, err = target.CopyBookFrom(source, planned.id, dstLayer, false)
		}
		if err != nil {
			return transferred, util.Errorf("%w", err)
		}
		transferred = append(transferred, book)
	}

	if isMove {
		// Every book has moved out into the source's trash, so the source subtree
		// now holds only empty folders; drop it so the moved layer no longer shows
		// on the source. The trash lives outside booksFolder, so this does not touch
		// the recoverable copies.
		if err := source.removeEmptyLayerTree(sourceLayer); err != nil {
			return transferred, util.Errorf("book move finished but clearing the source layer failed: %w", err)
		}
	}

	return transferred, nil
}

// preflightLayerTransfer resolves the transfer plan and refuses the transfer for
// every condition that can be known up front, all before a single file is
// touched. It reads both shelves under their locks - a read lock on the source
// and the exclusive lock on the target, the same order CopyBookFrom takes - and
// releases them before returning, so the per-book delegation that follows can
// take them again without deadlocking.
func (target *Shelf) preflightLayerTransfer(source *Shelf, sourceLayer, targetLayer Layers, isMove bool) (*layerTransferPlan, error) {
	// Refuse a read-only target before any lock, and a read-only source too for a
	// move, which ends by deleting from the source. publishBookCopy and DeleteBook
	// ask again once they run; this only moves the refusal ahead of any work.
	if _, err := target.writeRoot(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	if isMove {
		if _, err := source.writeRoot(); err != nil {
			return nil, util.Errorf("%w", err)
		}
	}

	if err := source.shelfLock.RLock(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer source.shelfLock.Unlock()

	if err := target.shelfLock.Lock(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer target.shelfLock.Unlock()

	// A full rescan of the source, like the single-book move's own conflict check,
	// so the plan sees books and folders added since the last scan rather than a
	// stale cache.
	if err := source.refreshBookCacheIfNeeded(true); err != nil {
		return nil, util.Errorf("%w", err)
	}

	sourceLayerPath := path.Join(booksFolder, path.Join(sourceLayer...))
	if _, err := source.dbRoot.Stat(sourceLayerPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, util.Errorf("%w: %q", ErrSourceLayerNotFound, sourceLayer.String())
		}
		return nil, util.Errorf("%w", err)
	}

	// The target builds the subtree fresh, so its destination layer must not
	// already exist - checked on disk, the authority RenameLayer trusts too.
	targetLayerPath := path.Join(booksFolder, path.Join(targetLayer...))
	if _, err := target.dbRoot.Stat(targetLayerPath); err == nil {
		return nil, util.Errorf("%w: %q", ErrTargetLayerExists, targetLayer.String())
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, util.Errorf("%w", err)
	}

	plan := &layerTransferPlan{}
	for _, layer := range source.listLayersFromCache() {
		if layerHasPrefix(layer, sourceLayer) {
			plan.sourceLayers = append(plan.sourceLayers, layer)
		}
	}
	for _, listing := range source.listBookListingsFromCache() {
		if layerHasPrefix(listing.Layers, sourceLayer) {
			plan.books = append(plan.books, plannedBook{id: listing.Book.ID(), layer: listing.Layers})
		}
	}

	if isMove {
		// A move keeps every book's ID, so the target must hold none of them - in
		// its listing or its trash. A fresh target scan first, for the same reason
		// ensureBookIDFree forces one: a cache miss alone would not see a book
		// another instance or a file manager added since the last scan. Collect the
		// whole colliding set rather than stopping at the first.
		if err := target.refreshBookCacheIfNeeded(true); err != nil {
			return nil, util.Errorf("%w", err)
		}
		var conflicts []string
		for _, planned := range plan.books {
			present, err := target.bookIDPresent(planned.id)
			if err != nil {
				return nil, util.Errorf("%w", err)
			}
			if present {
				conflicts = append(conflicts, planned.id)
			}
		}
		if len(conflicts) > 0 {
			return nil, &LayerBookIDConflictError{BookIDs: conflicts}
		}
	}

	return plan, nil
}

// bookIDPresent reports whether this shelf already holds a book with bookID,
// whether it is listed or sitting in the trash. Unlike ensureBookIDFree it does
// not force its own rescan, so a caller checking many IDs can refresh the cache
// once and then probe each. The shelf lock must be held.
func (s *Shelf) bookIDPresent(bookID string) (bool, error) {
	if _, _, err := s.getUpdatedBookFromBookID(bookID); err == nil {
		return true, nil
	} else if !errors.Is(err, ErrBookNotFound) {
		return false, util.Errorf("%w", err)
	}

	inTrash, err := s.isBookIDInTrash(bookID)
	if err != nil {
		return false, util.Errorf("%w", err)
	}
	return inTrash, nil
}

// removeEmptyLayerTree drops a layer subtree that a move has already emptied of
// books and marks the cache tree dirty. It takes the exclusive lock itself, so
// the caller must not already hold it.
func (s *Shelf) removeEmptyLayerTree(layer Layers) error {
	root, err := s.writeRoot()
	if err != nil {
		return util.Errorf("%w", err)
	}

	if err := s.shelfLock.Lock(); err != nil {
		return util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	layerPath := path.Join(booksFolder, path.Join(layer...))
	if err := root.RemoveAll(layerPath); err != nil {
		return util.Errorf("%w", err)
	}

	s.markBookCacheTreeDirty()
	return nil
}

// layerHasPrefix reports whether layer is prefix itself or sits beneath it.
func layerHasPrefix(layer, prefix Layers) bool {
	if len(layer) < len(prefix) {
		return false
	}
	for i := range prefix {
		if layer[i] != prefix[i] {
			return false
		}
	}
	return true
}

// mapLayerPath rewrites a source layer that sits under sourceLayer into its
// place under targetLayer, preserving the tail below sourceLayer. sourceLayer
// must be a prefix of srcLayer, which layerHasPrefix has already guaranteed for
// every layer the plan carries.
func mapLayerPath(srcLayer, sourceLayer, targetLayer Layers) Layers {
	tail := srcLayer[len(sourceLayer):]
	mapped := append(append(Layers(nil), targetLayer...), tail...)
	return mapped
}
