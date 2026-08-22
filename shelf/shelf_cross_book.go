package shelf

import (
	"errors"
	"fmt"
	"os"
	"path"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf/bookpkg"
)

// ErrBookIDConflict is returned when a cross-shelf move would land a book on a
// target shelf that already holds a book with the same ID. A move keeps the
// source ID so that reading progress and external references survive, so it is
// the one transfer that can collide; the target is never overwritten.
var ErrBookIDConflict = util.NewError("target shelf already holds a book with this ID")

// ErrSameShelfTransfer is returned when a cross-shelf transfer names one shelf as
// both source and target. The same-shelf copy (CopyBook) and layer move
// (MoveBook) are the operations for that, and taking one shelf's lock twice here
// would deadlock.
var ErrSameShelfTransfer = util.NewError("source and target are the same shelf")

// publishBookCopy stages sourceBook's package into this (the target) shelf under
// targetLayer and publishes it, returning the published book.
//
// It is the shared core behind both the same-shelf copy (CopyBook) and the
// cross-shelf transfer: sourceRoot is the filesystem sourceBook lives on, which
// is this shelf's own root for a same-shelf copy and the other shelf's root for a
// transfer. The whole package is reproduced verbatim into the target's app temp
// folder and published with a single Rename, so a failure part way leaves the
// target holding only temp data (wiped on next startup) and never touches
// sourceRoot beyond reading it.
//
// preserveID selects the ID the published book carries:
//   - false: a fresh ID is drawn from this shelf, so an original and its copy
//     coexist. book.json is rewritten to stamp it, so the caller must have
//     ensured the source book is writable by this build.
//   - true: the source book's ID is kept verbatim, so a moved book keeps its
//     reading progress and external references. book.json is not rewritten, so a
//     book this build cannot rewrite still transfers faithfully.
//
// The caller must hold this shelf's exclusive lock (and, for a cross-shelf
// transfer, a read lock on the source shelf), exactly as CopyBook does.
func (target *Shelf) publishBookCopy(sourceRoot fsutil.ReadFS, sourceBook *Book, targetLayer Layers, preserveID bool) (*Book, error) {
	root, err := target.writeRoot()
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	bookPath, err := createTempDir(root, path.Join(appFolder, appTmpFolder, "book"))
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer root.RemoveAll(bookPath)

	if err := copyTreeAcross(sourceRoot, sourceBook.FolderPath(), root, bookPath); err != nil {
		return nil, util.Errorf("%w", err)
	}

	stagedBook, err := bookpkg.Open(root, target.Logger, bookPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	stagedMeta := stagedBook.GetMeta()

	if !preserveID {
		newID, err := target.drawUnusedBookID()
		if err != nil {
			return nil, util.Errorf("%w", err)
		}
		stagedMeta.ID = newID
		if err := stagedBook.SetMeta(stagedMeta); err != nil {
			return nil, util.Errorf("%w", err)
		}
	}

	layerPath := path.Join(booksFolder, path.Join(targetLayer...))
	if err := root.MkdirAll(layerPath); err != nil {
		return nil, util.Errorf("%w", err)
	}

	target.addLayersToBookCache(targetLayer)

	folderName := titleToFolderName(stagedMeta.Title)
	for i := 1; ; i++ {
		finalBookPath := path.Join(layerPath, folderName)
		if _, err := target.dbRoot.Stat(finalBookPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				break
			}
			return nil, util.Errorf("%w", err)
		}
		folderName = titleToFolderName(fmt.Sprintf("%s-%d", stagedMeta.Title, i))
	}

	finalBookPath := path.Join(layerPath, folderName)
	if err := root.Rename(bookPath, finalBookPath); err != nil {
		return nil, util.Errorf("%w", err)
	}

	newBook, err := bookpkg.Open(target.dbRoot, target.Logger, finalBookPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	target.updateBookCacheEntry(targetLayer, finalBookPath, newBook)

	return newBook, nil
}

// CopyBookFrom copies the book identified by bookID in source into this (the
// target) shelf under targetLayer, returning the copy. It never modifies source:
// the copy is fully staged and published on the target before this returns, so an
// interruption leaves the source completely intact and the target holding nothing
// but temp data.
//
// preserveID is false for a plain copy - the copy is minted a fresh ID drawn from
// the target so the original and the copy coexist - and true for the target half
// of a move, where the copy keeps the source book's ID. A move that would collide
// with a book the target already holds under that ID is refused with
// ErrBookIDConflict rather than overwriting it. Use MoveBookFrom to also remove
// the source once the copy is published.
//
// The two shelves must be distinct: a transfer within one shelf is CopyBook or
// MoveBook, and locking the same shelf as both source and target would deadlock.
func (target *Shelf) CopyBookFrom(source *Shelf, bookID string, targetLayer Layers, preserveID bool) (*Book, error) {
	if err := validateLayers(targetLayer); err != nil {
		return nil, util.Errorf("%w", err)
	}

	if source == target {
		return nil, util.Errorf("%w", ErrSameShelfTransfer)
	}

	// Refuse a read-only target before taking any lock; publishBookCopy asks the
	// same question again once staging begins.
	if _, err := target.writeRoot(); err != nil {
		return nil, util.Errorf("%w", err)
	}

	// A read lock on the source keeps its book still while it is read, the way
	// GetBook reads one; the exclusive target lock is what every target mutation
	// takes. The background worker runs transfers one at a time, so no second
	// goroutine holds a shelf lock while this one holds two.
	if err := source.shelfLock.RLock(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer source.shelfLock.Unlock()

	if err := target.shelfLock.Lock(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer target.shelfLock.Unlock()

	sourceBook, _, err := source.getUpdatedBookFromBookID(bookID)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	if preserveID {
		// A move keeps the ID, so the target must not already hold it - in its
		// listing or in its trash, where a later restore would collapse the two
		// books that share it into one.
		if err := target.ensureBookIDFree(bookID); err != nil {
			return nil, util.Errorf("%w", err)
		}
	} else {
		// A copy rewrites book.json to stamp the fresh ID, so refuse a book this
		// build cannot rewrite before touching the disk.
		if err := sourceBook.EnsureWritable(); err != nil {
			return nil, util.Errorf("%w", err)
		}
	}

	newBook, err := target.publishBookCopy(source.dbRoot, sourceBook, targetLayer, preserveID)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	return newBook, nil
}

// MoveBookFrom moves the book identified by bookID out of source and into this
// (the target) shelf under targetLayer, preserving its ID so that reading
// progress and external references carry over, and returns the moved book.
//
// The book is fully published on the target before it is removed from the source,
// so an interruption at any point leaves the source intact: at worst the book
// ends up copied to the target and still present on the source, never lost from
// both. Removal routes the source book through its own trash, like any other
// delete, so a move is still recoverable on the source side until that trash is
// emptied.
func (target *Shelf) MoveBookFrom(source *Shelf, bookID string, targetLayer Layers) (*Book, error) {
	// A move ends by deleting the source, so refuse a read-only source before
	// copying anything. Otherwise the copy is published on the target and only
	// the trailing delete fails, stranding the book on both shelves.
	if _, err := source.writeRoot(); err != nil {
		return nil, util.Errorf("%w", err)
	}

	moved, err := target.CopyBookFrom(source, bookID, targetLayer, true)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	if err := source.DeleteBook(bookID); err != nil {
		// The copy is already published on the target, so the book is safe; the
		// source simply still holds it. Report the failure rather than hide a
		// half-finished move.
		return nil, util.Errorf("book copied to the target but removing it from the source failed: %w", err)
	}

	return moved, nil
}

// ensureBookIDFree reports ErrBookIDConflict when this shelf already holds a book
// with bookID, whether it is listed or sitting in the trash. The caller must hold
// the shelf lock, exactly as drawUnusedBookID's own probes assume.
//
// A move forces the destination ID rather than drawing a free one, so the check
// forces a full rescan first: a cache miss alone only re-reads books already
// cached until the scan interval expires, which would miss a book another
// instance or a file manager added since the last scan and let the move publish a
// second package under the same ID.
func (s *Shelf) ensureBookIDFree(bookID string) error {
	if err := s.refreshBookCacheIfNeeded(true); err != nil {
		return util.Errorf("%w", err)
	}

	if _, _, err := s.getUpdatedBookFromBookID(bookID); err == nil {
		return util.Errorf("%w: %q", ErrBookIDConflict, bookID)
	} else if !errors.Is(err, ErrBookNotFound) {
		return util.Errorf("%w", err)
	}

	inTrash, err := s.isBookIDInTrash(bookID)
	if err != nil {
		return util.Errorf("%w", err)
	}
	if inTrash {
		return util.Errorf("%w: %q is in the target shelf trash", ErrBookIDConflict, bookID)
	}

	return nil
}
