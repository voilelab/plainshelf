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

var ErrBookIDConflict = util.NewError("target shelf already holds a book with this ID")

var ErrSameShelfTransfer = util.NewError("source and target are the same shelf")

func (target writeLockedShelf) publishBookCopy(sourceRoot fsutil.ReadFS, sourceBook *Book, targetFolder FolderPath, preserveID bool) (*Book, error) {
	root, err := target.writeRoot()
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	bookPath, err := createTempDir(root, path.Join(appFolder, appTmpFolder, "book"))
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer root.RemoveAll(bookPath)

	if err := copyTreeAcross(sourceRoot, sourceBook.PackagePath(), root, bookPath); err != nil {
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

	folderPath := path.Join(booksFolder, path.Join(targetFolder...))
	if err := root.MkdirAll(folderPath); err != nil {
		return nil, util.Errorf("%w", err)
	}

	target.addFolderToBookCache(targetFolder)

	folderName := titleToFolderName(stagedMeta.Title)
	for i := 1; ; i++ {
		finalBookPath := path.Join(folderPath, folderName)
		if _, err := target.dbRoot.Stat(finalBookPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				break
			}
			return nil, util.Errorf("%w", err)
		}
		folderName = titleToFolderName(fmt.Sprintf("%s-%d", stagedMeta.Title, i))
	}

	finalBookPath := path.Join(folderPath, folderName)
	if err := root.Rename(bookPath, finalBookPath); err != nil {
		return nil, util.Errorf("%w", err)
	}

	newBook, err := bookpkg.Open(target.dbRoot, target.Logger, finalBookPath)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	target.updateBookCacheEntry(targetFolder, finalBookPath, newBook)

	return newBook, nil
}

func (target *Shelf) CopyBookFrom(source *Shelf, bookID string, targetFolder FolderPath, preserveID bool) (*Book, error) {
	if err := target.ValidateFolderPath(targetFolder); err != nil {
		return nil, util.Errorf("%w", err)
	}

	if source == target {
		return nil, util.Errorf("%w", ErrSameShelfTransfer)
	}

	if _, err := target.writeRoot(); err != nil {
		return nil, util.Errorf("%w", err)
	}

	locked, unlock, err := lockForTransfer(source, target)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer unlock()

	sourceBook, _, err := source.getUpdatedBookFromBookID(bookID)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	if preserveID {
		if err := target.ensureBookIDFreeAfterRescan(bookID); err != nil {
			return nil, util.Errorf("%w", err)
		}
	} else {
		if err := sourceBook.EnsureWritable(); err != nil {
			return nil, util.Errorf("%w", err)
		}
	}

	newBook, err := locked.publishBookCopy(source.dbRoot, sourceBook, targetFolder, preserveID)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	return newBook, nil
}

func (target *Shelf) MoveBookFrom(source *Shelf, bookID string, targetFolder FolderPath) (*Book, error) {
	if _, err := source.writeRoot(); err != nil {
		return nil, util.Errorf("%w", err)
	}

	moved, err := target.CopyBookFrom(source, bookID, targetFolder, true)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	if err := source.DeleteBook(bookID); err != nil {
		return nil, util.Errorf("book copied to the target but removing it from the source failed: %w", err)
	}

	return moved, nil
}

func (s *Shelf) ensureBookIDFreeAfterRescan(bookID string) error {
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
