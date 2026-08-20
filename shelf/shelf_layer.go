package shelf

import (
	"errors"
	"os"
	"path"

	"github.com/voilelab/plainshelf/internal/util"
)

// GetAllLayers returns a sorted list of all unique layers present in the library.
//
// The list comes from the book cache, which records layers during the same walk
// it builds the book listing from, so this is throttled by scan_interval like
// any other listing instead of walking books/ on every request. Layers created,
// renamed, moved or deleted through this process update the cache immediately,
// so only a change made outside PlainShelf waits for the next scan.
func (s *Shelf) GetAllLayers() ([]Layers, error) {
	if !s.IsReady() {
		return nil, util.Errorf("%w", ErrShelfInitializing)
	}

	if err := s.shelfLock.RLock(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	s.scheduleBookCacheRefreshIfNeeded()

	return s.listLayersFromCache(), nil
}

// NewLayer creates a new layer in the library. It validates the layer name to ensure it does not contain invalid characters and then creates the necessary directory structure for the layer.
func (s *Shelf) NewLayer(layer Layers) error {
	if err := validateLayers(layer); err != nil {
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

	layerPath := path.Join(booksFolder, path.Join(layer...))
	err = root.MkdirAll(layerPath)
	if err != nil {
		return util.Errorf("%w", err)
	}

	// Recorded rather than left to the next scan: a layer the user just created
	// has to appear in the very next listing, and an empty one holds no book to
	// rebuild it from.
	s.addLayersToBookCache(layer)

	return nil
}

// DeleteLayer removes a layer from the library. It checks if the layer is empty (i.e., contains no books) before deleting it. If the layer is not empty, it returns an error.
func (s *Shelf) DeleteLayer(layer Layers) error {
	if err := validateLayers(layer); err != nil {
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

	layerPath := path.Join(booksFolder, path.Join(layer...))

	entries, err := s.dbRoot.ReadDir(layerPath)
	if err != nil {
		return util.Errorf("%w", err)
	}

	if len(entries) > 0 {
		return util.Errorf("cannot delete non-empty layer")
	}

	err = root.RemoveAll(layerPath)
	if err != nil {
		return util.Errorf("%w", err)
	}

	// The layer was verified empty above, so it has no descendants in the cache
	// either and dropping this one entry is enough.
	s.removeLayerFromBookCache(layer)

	return nil
}

// RenameLayer renames an existing layer without changing its parent layer.
func (s *Shelf) RenameLayer(oldLayer Layers, newLayer Layers) error {
	if err := validateLayers(oldLayer); err != nil {
		return util.Errorf("invalid old layer: %w", err)
	}
	if err := validateLayers(newLayer); err != nil {
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

	if len(oldLayer) == 0 || len(newLayer) == 0 {
		return util.Errorf("cannot rename root layer")
	}

	oldParent := oldLayer[:len(oldLayer)-1]
	newParent := newLayer[:len(newLayer)-1]
	if !oldParent.Equal(newParent) {
		return util.Errorf("rename cannot move layer")
	}

	oldLayerPath := path.Join(booksFolder, path.Join(oldLayer...))
	newLayerPath := path.Join(booksFolder, path.Join(newLayer...))

	// Check if old layer exists
	if _, err := s.dbRoot.Stat(oldLayerPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return util.Errorf("old layer does not exist")
		}
		return util.Errorf("%w", err)
	}

	// Check if new layer already exists
	if _, err := s.dbRoot.Stat(newLayerPath); err == nil {
		return util.Errorf("new layer already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return util.Errorf("%w", err)
	}

	newLayerParent := path.Dir(newLayerPath)
	if err := root.MkdirAll(newLayerParent); err != nil {
		return util.Errorf("%w", err)
	}

	err = root.Rename(oldLayerPath, newLayerPath)
	if err != nil {
		return util.Errorf("%w", err)
	}

	s.markBookCacheTreeDirty()

	return nil
}

// MoveLayer moves an existing layer under an existing target parent layer without renaming it.
func (s *Shelf) MoveLayer(layer Layers, targetParent Layers) error {
	if err := validateLayers(layer); err != nil {
		return util.Errorf("invalid layer: %w", err)
	}
	if err := validateLayers(targetParent); err != nil {
		return util.Errorf("invalid target layer: %w", err)
	}
	if len(layer) == 0 {
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

	oldLayerPath := path.Join(booksFolder, path.Join(layer...))
	targetParentPath := path.Join(booksFolder, path.Join(targetParent...))

	if _, err := s.dbRoot.Stat(oldLayerPath); err != nil {
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

	for i := range layer {
		if targetParent.Equal(layer[:i+1]) {
			return util.Errorf("cannot move layer under itself")
		}
	}

	newLayer := append(append(Layers(nil), targetParent...), layer[len(layer)-1])
	newLayerPath := path.Join(booksFolder, path.Join(newLayer...))
	if _, err := s.dbRoot.Stat(newLayerPath); err == nil {
		return util.Errorf("target child layer already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return util.Errorf("%w", err)
	}

	if err := root.Rename(oldLayerPath, newLayerPath); err != nil {
		return util.Errorf("%w", err)
	}

	s.markBookCacheTreeDirty()

	return nil
}
