package shelf

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
)

// ErrTargetFolderExists is returned by a cross-shelf folder transfer when the
// target shelf already holds the destination folder. A folder transfer builds the
// whole subtree fresh on the target, so it refuses to land on an existing folder
// rather than merging into it - the same rule RenameFolder enforces for a
// same-shelf rename.
var ErrTargetFolderExists = util.NewError("target shelf already holds a folder with this name")

// ErrSourceFolderNotFound is returned when a cross-shelf folder transfer names a
// source folder that does not exist on the source shelf.
var ErrSourceFolderNotFound = util.NewError("source shelf has no such folder")

// FolderBookIDConflictError reports that a cross-shelf folder move would land on
// book IDs the target shelf already holds. Pre-flight collects every colliding
// ID before touching a single file, so the whole set is reported at once rather
// than surfacing them one failed move at a time.
//
// It satisfies errors.Is(err, ErrBookIDConflict) so a caller that only cares
// that a move collided can test for the same sentinel the single-book move
// returns, while one that wants the full list type-asserts to *FolderBookIDConflictError.
type FolderBookIDConflictError struct {
	BookIDs []string
}

func (e *FolderBookIDConflictError) Error() string {
	return fmt.Sprintf("target shelf already holds books with these IDs: %s", strings.Join(e.BookIDs, ", "))
}

// Is lets errors.Is match the shared ErrBookIDConflict sentinel, so callers do
// not have to special-case the folder-level aggregate.
func (e *FolderBookIDConflictError) Is(target error) bool {
	return target == ErrBookIDConflict
}

// folderTransferPlan is what pre-flight resolves before any file is touched: the
// books a transfer will carry and the folder directories it will reproduce. Both
// are gathered from a single fresh scan of the source so the plan reflects books
// and folders another instance or a file manager added since the last scan.
type folderTransferPlan struct {
	// books is every book under sourceFolder (the folder itself and all
	// descendants) paired with the source folder it sits in, sorted by ID for a
	// deterministic transfer order. Pairing the folder here fixes each book's
	// destination from the pre-flight snapshot rather than re-reading it per book.
	books []plannedBook
	// sourceFolders is sourceFolder and every descendant folder, so empty folders
	// are reproduced on the target too, not only the ones that hold a book.
	sourceFolders []FolderPath
}

type plannedBook struct {
	id    string
	folder FolderPath
}

// CopyFolderFrom copies the whole folder sourceFolder - the folder and every book
// and sub-folder beneath it - out of source and into this (the target) shelf
// under targetFolder, returning the books it copied. Each book is copied through
// CopyBookFrom, so every copy is minted a fresh ID drawn from the target and
// carries no reading progress, exactly like a new import: the original folder on
// the source is left completely intact.
//
// A book at source folder sourceFolder/x/y lands on the target at
// targetFolder/x/y, so the subtree's shape is preserved under its new root.
//
// Pre-flight runs to completion before any file is touched: the transfer is
// refused if the two shelves are the same, if the target is read-only, if
// sourceFolder does not exist, or if targetFolder already exists on the target. A
// copy draws fresh IDs, so it never checks for a book-ID conflict. When
// pre-flight refuses the transfer, neither shelf has been modified.
//
// There is deliberately no rollback once execution begins, and a transaction
// folder is out of scope. Pre-flight has already ruled out every foreseeable
// conflict, so a failure mid-transfer is a rare I/O or space error. The copies
// published so far are returned; the rest stay on the source. Because the first
// published book creates the destination folder, calling this again then hits
// ErrTargetFolderExists, so finishing a partial copy is not an automatic re-run:
// resolve it by copying the remaining source books individually, or by removing
// the partial destination and starting over.
func (target *Shelf) CopyFolderFrom(source *Shelf, sourceFolder, targetFolder FolderPath) ([]*Book, error) {
	return target.transferFolderFrom(source, sourceFolder, targetFolder, false)
}

// MoveFolderFrom moves the whole folder sourceFolder out of source and into this
// (the target) shelf under targetFolder, returning the books it moved. Each book
// is moved through MoveBookFrom, so every book keeps its ID and reading progress
// and is published on the target before it is removed from the source; the
// removed source books route through the source's trash, so a move stays
// recoverable on the source side until that trash is emptied. Once every book has
// moved, the now-empty source folder subtree is removed.
//
// A book at source folder sourceFolder/x/y lands on the target at
// targetFolder/x/y, so the subtree's shape is preserved under its new root.
//
// Pre-flight runs to completion before any file is touched: the transfer is
// refused if the two shelves are the same, if either shelf is read-only, if
// sourceFolder does not exist, or if targetFolder already exists on the target. A
// move keeps each book's ID, so pre-flight also refuses the transfer when the
// target already holds any of those IDs, reporting the whole colliding set at
// once through *FolderBookIDConflictError rather than stopping at the first. When
// pre-flight refuses the transfer, neither shelf has been modified.
//
// There is deliberately no rollback once execution begins, and a transaction
// folder is out of scope. Pre-flight has already ruled out every foreseeable
// conflict, so a failure mid-transfer is a rare I/O or space error. The books
// moved so far are returned and, because a moved book is no longer on the
// source, the source keeps only what still has to move. Completing it is not an
// automatic re-run of this call, though: the first moved book creates the
// destination folder, so a second call hits ErrTargetFolderExists. Move the
// remaining source books into the now-existing destination folder individually,
// or remove the partial destination and start over.
func (target *Shelf) MoveFolderFrom(source *Shelf, sourceFolder, targetFolder FolderPath) ([]*Book, error) {
	return target.transferFolderFrom(source, sourceFolder, targetFolder, true)
}

// transferFolderFrom is the shared body of CopyFolderFrom and MoveFolderFrom. isMove
// selects between the two: a move preserves book IDs, checks for ID conflicts,
// requires a writable source, and clears the source subtree at the end, while a
// copy does none of those.
func (target *Shelf) transferFolderFrom(source *Shelf, sourceFolder, targetFolder FolderPath, isMove bool) ([]*Book, error) {
	if err := validateFolderPath(sourceFolder); err != nil {
		return nil, util.Errorf("invalid source folder: %w", err)
	}
	if err := validateFolderPath(targetFolder); err != nil {
		return nil, util.Errorf("invalid target folder: %w", err)
	}
	if len(sourceFolder) == 0 {
		return nil, util.Errorf("cannot transfer the root folder")
	}
	if len(targetFolder) == 0 {
		return nil, util.Errorf("cannot transfer onto the root folder")
	}
	if source == target {
		return nil, util.Errorf("%w", ErrSameShelfTransfer)
	}

	plan, err := target.preflightFolderTransfer(source, sourceFolder, targetFolder, isMove)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	// Reproduce the folder structure first - including folders that hold no book -
	// so an empty sub-folder survives the transfer. Folders that do hold a book are
	// re-created here too, harmlessly, since publishing a book MkdirAll's its own
	// folder as well.
	for _, sl := range plan.sourceFolders {
		mapped := mapFolderPath(sl, sourceFolder, targetFolder)
		if len(mapped) == 0 {
			continue
		}
		if err := target.NewFolder(mapped[:len(mapped)-1], mapped[len(mapped)-1]); err != nil {
			return nil, util.Errorf("%w", err)
		}
	}

	transferred := make([]*Book, 0, len(plan.books))
	for _, planned := range plan.books {
		dstFolder := mapFolderPath(planned.folder, sourceFolder, targetFolder)

		var book *Book
		var err error
		if isMove {
			book, err = target.MoveBookFrom(source, planned.id, dstFolder)
		} else {
			book, err = target.CopyBookFrom(source, planned.id, dstFolder, false)
		}
		if err != nil {
			return transferred, util.Errorf("%w", err)
		}
		transferred = append(transferred, book)
	}

	if isMove {
		// Every book has moved out into the source's trash, so the source subtree
		// now holds only empty folders; drop it so the moved folder no longer shows
		// on the source. The trash lives outside booksFolder, so this does not touch
		// the recoverable copies.
		if err := source.removeEmptyFolderTree(sourceFolder); err != nil {
			return transferred, util.Errorf("book move finished but clearing the source folder failed: %w", err)
		}
	}

	return transferred, nil
}

// preflightFolderTransfer resolves the transfer plan and refuses the transfer for
// every condition that can be known up front, all before a single file is
// touched. It reads both shelves under their locks - a read lock on the source
// and the exclusive lock on the target, the same order CopyBookFrom takes - and
// releases them before returning, so the per-book delegation that follows can
// take them again without deadlocking.
func (target *Shelf) preflightFolderTransfer(source *Shelf, sourceFolder, targetFolder FolderPath, isMove bool) (*folderTransferPlan, error) {
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

	sourceFolderPath := path.Join(booksFolder, path.Join(sourceFolder...))
	if _, err := source.dbRoot.Stat(sourceFolderPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, util.Errorf("%w: %q", ErrSourceFolderNotFound, sourceFolder.String())
		}
		return nil, util.Errorf("%w", err)
	}

	// The target builds the subtree fresh, so its destination folder must not
	// already exist - checked on disk, the authority RenameFolder trusts too.
	targetFolderPath := path.Join(booksFolder, path.Join(targetFolder...))
	if _, err := target.dbRoot.Stat(targetFolderPath); err == nil {
		return nil, util.Errorf("%w: %q", ErrTargetFolderExists, targetFolder.String())
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, util.Errorf("%w", err)
	}

	plan := &folderTransferPlan{}
	for _, folder := range source.listFoldersFromCache() {
		if folderHasPrefix(folder, sourceFolder) {
			plan.sourceFolders = append(plan.sourceFolders, folder)
		}
	}
	for _, listing := range source.listBookListingsFromCache() {
		if folderHasPrefix(listing.Folders, sourceFolder) {
			plan.books = append(plan.books, plannedBook{id: listing.Book.ID(), folder: listing.Folders})
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
			return nil, &FolderBookIDConflictError{BookIDs: conflicts}
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

// removeEmptyFolderTree drops the folders a move has emptied out of a folder
// subtree and marks the cache tree dirty. It takes the exclusive lock itself, so
// the caller must not already hold it.
//
// It prunes only empty directories, never a book package or a file: a move
// carries only the books pre-flight could resolve, and an unreadable package -
// one bookpkg.Open failed on during the scan, so it was never in the plan - is
// still physically present. A blanket RemoveAll would delete it permanently,
// bypassing the trash a delete routes through, so such a package (and the
// folders above it) is deliberately left in place.
func (s *Shelf) removeEmptyFolderTree(folder FolderPath) error {
	root, err := s.writeRoot()
	if err != nil {
		return util.Errorf("%w", err)
	}

	if err := s.shelfLock.Lock(); err != nil {
		return util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	folderPath := path.Join(booksFolder, path.Join(folder...))
	if _, err := pruneEmptyDirs(root, folderPath); err != nil {
		return util.Errorf("%w", err)
	}

	s.markBookCacheTreeDirty()
	return nil
}

// pruneEmptyDirs removes dirPath and every descendant directory that is empty,
// bottom-up, and reports whether dirPath itself was removed. It neither descends
// into nor deletes a book package (a .bookpkg directory) or a file, so anything
// a move left behind - most importantly a package that was unreadable at scan
// time and so never transferred - survives, along with the folders above it.
func pruneEmptyDirs(root fsutil.FS, dirPath string) (bool, error) {
	entries, err := root.ReadDir(dirPath)
	if err != nil {
		return false, util.Errorf("%w", err)
	}

	remaining := 0
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasSuffix(entry.Name(), bookExtension) {
			childRemoved, err := pruneEmptyDirs(root, path.Join(dirPath, entry.Name()))
			if err != nil {
				return false, util.Errorf("%w", err)
			}
			if !childRemoved {
				remaining++
			}
		} else {
			remaining++
		}
	}

	if remaining > 0 {
		return false, nil
	}

	if err := root.Remove(dirPath); err != nil {
		return false, util.Errorf("%w", err)
	}
	return true, nil
}

// folderHasPrefix reports whether folder is prefix itself or sits beneath it.
func folderHasPrefix(folder, prefix FolderPath) bool {
	if len(folder) < len(prefix) {
		return false
	}
	for i := range prefix {
		if folder[i] != prefix[i] {
			return false
		}
	}
	return true
}

// mapFolderPath rewrites a source folder that sits under sourceFolder into its
// place under targetFolder, preserving the tail below sourceFolder. sourceFolder
// must be a prefix of srcFolder, which folderHasPrefix has already guaranteed for
// every folder the plan carries.
func mapFolderPath(srcFolder, sourceFolder, targetFolder FolderPath) FolderPath {
	tail := srcFolder[len(sourceFolder):]
	mapped := append(append(FolderPath(nil), targetFolder...), tail...)
	return mapped
}
