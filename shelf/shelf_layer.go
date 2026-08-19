package shelf

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
)

// GetAllLayers returns a sorted list of all unique layers present in the library.
func (s *Shelf) GetAllLayers() ([]Layers, error) {
	if err := s.shelfLock.RLock(); err != nil {
		return nil, util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	var layers []Layers
	seen := make(map[string]bool)
	err := s.iterateLayers(func(ls Layers) bool {
		key := strings.Join(ls, "/")
		if !seen[key] {
			layers = append(layers, ls)
			seen[key] = true
		}
		return true
	})

	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	sort.Slice(layers, func(i, j int) bool {
		return strings.Join(layers[i], "/") < strings.Join(layers[j], "/")
	})

	return layers, nil
}

// iterateLayers iterates over all unique layers in the library and applies the provided function to each layer.
// If the function returns false, the iteration will stop.
func (s *Shelf) iterateLayers(fn func(Layers) bool) error {
	skipAll := false

	var dfsFunc func(string, fs.DirEntry)

	dfsFunc = func(pth string, entry fs.DirEntry) {
		if skipAll {
			return
		}

		isDir, err := entryIsDir(s.dbRoot, pth, entry)
		if err != nil {
			return
		}

		if !isDir {
			return
		}

		folderName := path.Base(pth)
		if isIgnoredDir(folderName) {
			return
		}
		if strings.HasSuffix(folderName, bookExtension) {
			return
		}

		// path.Join always uses "/" separators on every platform, so split on
		// "/" rather than os.PathSeparator ("\" on Windows would break parsing).
		layers := strings.Split(pth, "/")[1:]
		if !fn(layers) {
			skipAll = true
			return
		}

		entries, err := s.dbRoot.ReadDir(pth)
		if err != nil {
			return
		}

		for _, child := range entries {
			fullPath := path.Join(pth, child.Name())
			dfsFunc(fullPath, child)
		}
	}

	dfsFunc(booksFolder, nil)
	return nil
}

// NewLayer creates a new layer in the library. It validates the layer name to ensure it does not contain invalid characters and then creates the necessary directory structure for the layer.
func (s *Shelf) NewLayer(layer Layers) error {
	if err := validateLayers(layer); err != nil {
		return util.Errorf("%w", err)
	}

	if err := s.shelfLock.Lock(); err != nil {
		return util.Errorf("%w", err)
	}
	defer s.shelfLock.Unlock()

	layerPath := path.Join(booksFolder, path.Join(layer...))
	err := s.dbRoot.MkdirAll(layerPath)
	if err != nil {
		return util.Errorf("%w", err)
	}

	return nil
}

// DeleteLayer removes a layer from the library. It checks if the layer is empty (i.e., contains no books) before deleting it. If the layer is not empty, it returns an error.
func (s *Shelf) DeleteLayer(layer Layers) error {
	if err := validateLayers(layer); err != nil {
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

	err = s.dbRoot.RemoveAll(layerPath)
	if err != nil {
		return util.Errorf("%w", err)
	}

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
	if err := s.dbRoot.MkdirAll(newLayerParent); err != nil {
		return util.Errorf("%w", err)
	}

	err := s.dbRoot.Rename(oldLayerPath, newLayerPath)
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

	if err := s.dbRoot.Rename(oldLayerPath, newLayerPath); err != nil {
		return util.Errorf("%w", err)
	}

	s.markBookCacheTreeDirty()

	return nil
}
