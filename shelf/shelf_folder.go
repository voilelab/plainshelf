package shelf

import (
	"errors"
	"os"
	"path"

	"github.com/voilelab/plainshelf/internal/util"
)

func (s *Shelf) GetAllFolders() ([]FolderPath, error) {
	if !s.IsReady() {
		return nil, s.notReady()
	}

	if err := s.shelfLock.RLock(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	s.scheduleBookCacheRefreshIfNeeded()

	return s.listFoldersFromCache(), nil
}

func (s *Shelf) NewFolder(parent FolderPath, name string) error {
	folder := append(append(FolderPath(nil), parent...), name)
	if err := s.ValidateFolderPath(folder); err != nil {
		return util.Errorf("%w", err)
	}
	if err := validateNewFolderName(name); err != nil {
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

	s.addFolderToBookCache(folder)

	return nil
}

func (s *Shelf) DeleteFolder(folder FolderPath) error {
	return s.deleteFolder(folder, nil)
}

func (s *Shelf) PruneFolder(folder FolderPath, carriedMeta string) error {
	return s.deleteFolder(folder, func(data []byte) bool {
		return carriedMeta != "" && folderMetaVersion(data) == carriedMeta
	})
}

func (s *Shelf) deleteFolder(folder FolderPath, dropMeta func([]byte) bool) error {
	if err := s.ValidateFolderPath(folder); err != nil {
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

	for _, entry := range entries {
		if entry.Name() != FolderMetaFile || entry.IsDir() {
			return util.Errorf("cannot delete non-empty folder")
		}
	}
	if len(entries) > 0 && dropMeta != nil {
		data, err := readFolderMetaFile(s.dbRoot, path.Join(folderPath, FolderMetaFile))
		if err != nil || !dropMeta(data) {
			return util.Errorf("cannot delete a folder whose folder.json was not carried")
		}
	}

	if len(entries) > 0 {
		if err := root.Remove(path.Join(folderPath, FolderMetaFile)); err != nil {
			return util.Errorf("%w", err)
		}
	}
	if err := root.Remove(folderPath); err != nil {
		return util.Errorf("%w", err)
	}

	s.removeFolderFromBookCache(folder)

	return nil
}

func (s *Shelf) RenameFolder(folder FolderPath, newName string) error {
	if len(folder) == 0 {
		return util.Errorf("cannot rename root folder")
	}

	newFolder := append(append(FolderPath(nil), folder[:len(folder)-1]...), newName)

	if err := s.ValidateFolderPath(folder); err != nil {
		return util.Errorf("invalid old folder: %w", err)
	}
	if err := s.ValidateFolderPath(newFolder); err != nil {
		return util.Errorf("invalid new folder: %w", err)
	}
	if err := validateNewFolderName(newName); err != nil {
		return util.Errorf("invalid new folder: %w", err)
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
			return util.Errorf("old folder does not exist")
		}
		return util.Errorf("%w", err)
	}

	if _, err := s.dbRoot.Stat(newFolderPath); err == nil {
		return util.Errorf("new folder already exists")
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

func (s *Shelf) MoveFolder(folder FolderPath, targetParent FolderPath) error {
	if err := s.ValidateFolderPath(folder); err != nil {
		return util.Errorf("invalid folder: %w", err)
	}
	if err := s.ValidateFolderPath(targetParent); err != nil {
		return util.Errorf("invalid target folder: %w", err)
	}
	if len(folder) == 0 {
		return util.Errorf("cannot move root folder")
	}
	if err := validateNewFolderName(folder[len(folder)-1]); err != nil {
		return util.Errorf("invalid folder: %w", err)
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
			return util.Errorf("folder does not exist")
		}
		return util.Errorf("%w", err)
	}

	if _, err := s.dbRoot.Stat(targetParentPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return util.Errorf("target folder does not exist")
		}
		return util.Errorf("%w", err)
	}

	for i := range folder {
		if targetParent.Equal(folder[:i+1]) {
			return util.Errorf("cannot move folder under itself")
		}
	}

	newFolder := append(append(FolderPath(nil), targetParent...), folder[len(folder)-1])
	newFolderPath := path.Join(booksFolder, path.Join(newFolder...))
	if _, err := s.dbRoot.Stat(newFolderPath); err == nil {
		return util.Errorf("target child folder already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return util.Errorf("%w", err)
	}

	if err := root.Rename(oldFolderPath, newFolderPath); err != nil {
		return util.Errorf("%w", err)
	}

	s.markBookCacheTreeDirty()

	return nil
}
