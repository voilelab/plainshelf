package shelf

import (
	"errors"
	"os"
	"path"

	"github.com/voilelab/plainshelf/internal/util"
)

// GetAllFolders returns a sorted list of all unique folders present in the library.
//
// The list comes from the book cache, which records folders during the same walk
// it builds the book listing from, so this is throttled by scan_interval like
// any other listing instead of walking books/ on every request. Folders created,
// renamed, moved or deleted through this process update the cache immediately,
// so only a change made outside PlainShelf waits for the next scan.
func (s *Shelf) GetAllFolders() ([]FolderPath, error) {
	if !s.IsReady() {
		return nil, util.Errorf("%w", ErrShelfInitializing)
	}

	if err := s.shelfLock.RLock(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	s.scheduleBookCacheRefreshIfNeeded()

	return s.listFoldersFromCache(), nil
}

// NewFolder creates a new folder named name under the parent folder path. It
// validates every path segment to ensure it does not contain invalid characters
// and then creates the necessary directory structure.
func (s *Shelf) NewFolder(parent FolderPath, name string) error {
	folder := append(append(FolderPath(nil), parent...), name)
	if err := validateFolderPath(folder); err != nil {
		return util.Errorf("%w", err)
	}

	root, err := s.writeRoot()
	if err != nil {
		return util.Errorf("%w", err)
	}

	if err := s.shelfLock.Lock(); err != nil {
		return util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	folderPath := path.Join(booksFolder, path.Join(folder...))
	err = root.MkdirAll(folderPath)
	if err != nil {
		return util.Errorf("%w", err)
	}

	// Recorded rather than left to the next scan: a folder the user just created
	// has to appear in the very next listing, and an empty one holds no book to
	// rebuild it from.
	s.addFolderToBookCache(folder)

	return nil
}

// DeleteFolder removes a folder from the library. It checks if the folder is empty (i.e., contains no books) before deleting it. If the folder is not empty, it returns an error.
func (s *Shelf) DeleteFolder(folder FolderPath) error {
	if err := validateFolderPath(folder); err != nil {
		return util.Errorf("%w", err)
	}

	root, err := s.writeRoot()
	if err != nil {
		return util.Errorf("%w", err)
	}

	if err := s.shelfLock.Lock(); err != nil {
		return util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	folderPath := path.Join(booksFolder, path.Join(folder...))

	entries, err := s.dbRoot.ReadDir(folderPath)
	if err != nil {
		return util.Errorf("%w", err)
	}

	if len(entries) > 0 {
		return util.Errorf("cannot delete non-empty layer")
	}

	err = root.RemoveAll(folderPath)
	if err != nil {
		return util.Errorf("%w", err)
	}

	// The folder was verified empty above, so it has no descendants in the cache
	// either and dropping this one entry is enough.
	s.removeFolderFromBookCache(folder)

	return nil
}

// RenameFolder renames the last segment of an existing folder path to newName,
// keeping the folder under the same parent. The new path is derived from the
// old one's parent, so a rename can never move the folder to a different parent.
func (s *Shelf) RenameFolder(folder FolderPath, newName string) error {
	if len(folder) == 0 {
		return util.Errorf("cannot rename root layer")
	}

	newFolder := append(append(FolderPath(nil), folder[:len(folder)-1]...), newName)

	if err := validateFolderPath(folder); err != nil {
		return util.Errorf("invalid old layer: %w", err)
	}
	if err := validateFolderPath(newFolder); err != nil {
		return util.Errorf("invalid new layer: %w", err)
	}

	root, err := s.writeRoot()
	if err != nil {
		return util.Errorf("%w", err)
	}

	if err := s.shelfLock.Lock(); err != nil {
		return util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	oldFolderPath := path.Join(booksFolder, path.Join(folder...))
	newFolderPath := path.Join(booksFolder, path.Join(newFolder...))

	if _, err := s.dbRoot.Stat(oldFolderPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return util.Errorf("old layer does not exist")
		}
		return util.Errorf("%w", err)
	}

	if _, err := s.dbRoot.Stat(newFolderPath); err == nil {
		return util.Errorf("new layer already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return util.Errorf("%w", err)
	}

	newFolderParent := path.Dir(newFolderPath)
	if err := root.MkdirAll(newFolderParent); err != nil {
		return util.Errorf("%w", err)
	}

	err = root.Rename(oldFolderPath, newFolderPath)
	if err != nil {
		return util.Errorf("%w", err)
	}

	s.markBookCacheTreeDirty()

	return nil
}

// MoveFolder moves an existing folder under an existing target parent folder without renaming it.
func (s *Shelf) MoveFolder(folder FolderPath, targetParent FolderPath) error {
	if err := validateFolderPath(folder); err != nil {
		return util.Errorf("invalid layer: %w", err)
	}
	if err := validateFolderPath(targetParent); err != nil {
		return util.Errorf("invalid target layer: %w", err)
	}
	if len(folder) == 0 {
		return util.Errorf("cannot move root layer")
	}

	root, err := s.writeRoot()
	if err != nil {
		return util.Errorf("%w", err)
	}

	if err := s.shelfLock.Lock(); err != nil {
		return util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	oldFolderPath := path.Join(booksFolder, path.Join(folder...))
	targetParentPath := path.Join(booksFolder, path.Join(targetParent...))

	if _, err := s.dbRoot.Stat(oldFolderPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return util.Errorf("layer does not exist")
		}
		return util.Errorf("%w", err)
	}

	if _, err := s.dbRoot.Stat(targetParentPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return util.Errorf("target layer does not exist")
		}
		return util.Errorf("%w", err)
	}

	for i := range folder {
		if targetParent.Equal(folder[:i+1]) {
			return util.Errorf("cannot move layer under itself")
		}
	}

	newFolder := append(append(FolderPath(nil), targetParent...), folder[len(folder)-1])
	newFolderPath := path.Join(booksFolder, path.Join(newFolder...))
	if _, err := s.dbRoot.Stat(newFolderPath); err == nil {
		return util.Errorf("target child layer already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return util.Errorf("%w", err)
	}

	if err := root.Rename(oldFolderPath, newFolderPath); err != nil {
		return util.Errorf("%w", err)
	}

	s.markBookCacheTreeDirty()

	return nil
}
