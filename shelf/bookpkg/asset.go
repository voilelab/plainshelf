package bookpkg

import (
	"errors"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf/internal/shelfutil"
)

const SourceAssetsFolder = "assets"

var ErrAssetNotFound = util.NewError("asset not found")

var ErrInvalidAssetName = util.NewError("invalid asset name")

var imageExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true,
}

func IsSupportedImageExt(ext string) bool {
	return imageExtensions[strings.ToLower(ext)]
}

func validateAssetName(name string) error {
	if strings.HasPrefix(name, ".") {
		return util.Errorf("%w %q: must not start with a dot", ErrInvalidAssetName, name)
	}
	if err := shelfutil.ValidatePathSegment(name); err != nil {
		return util.Errorf("%w %q: %w", ErrInvalidAssetName, name, err)
	}
	if !IsSupportedImageExt(path.Ext(name)) {
		return util.Errorf("%w %q: not a supported image extension", ErrInvalidAssetName, name)
	}
	return nil
}

func (r *Source) AssetPath(name string) (string, error) {
	if err := validateAssetName(name); err != nil {
		return "", util.Errorf("%w", err)
	}
	return path.Join(r.folderPath, SourceAssetsFolder, name), nil
}

func (r *Source) WriteAsset(name string, data []byte) error {
	if err := r.EnsureWritable(); err != nil {
		return util.Errorf("%w", err)
	}
	root, err := r.writeRoot()
	if err != nil {
		return util.Errorf("%w", err)
	}
	assetPath, err := r.AssetPath(name)
	if err != nil {
		return util.Errorf("%w", err)
	}

	if err := root.MkdirAll(path.Join(r.folderPath, SourceAssetsFolder)); err != nil {
		return util.Errorf("%w", err)
	}

	if err := fsutil.WriteFileAtomic(root, assetPath, data); err != nil {
		return util.Errorf("%w", err)
	}

	return nil
}

func (r *Source) DeleteAsset(name string) error {
	if err := r.EnsureWritable(); err != nil {
		return util.Errorf("%w", err)
	}
	root, err := r.writeRoot()
	if err != nil {
		return util.Errorf("%w", err)
	}
	assetPath, err := r.AssetPath(name)
	if err != nil {
		return util.Errorf("%w", err)
	}

	if err := root.Remove(assetPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return util.Errorf("%w: %s", ErrAssetNotFound, name)
		}
		return util.Errorf("%w", err)
	}

	return nil
}

func (r *Source) AssetETag(name string) string {
	assetPath, err := r.AssetPath(name)
	if err != nil {
		return ""
	}
	return shelfutil.FileETag(r.root, assetPath)
}

type Asset struct {
	File fs.File
	Info fs.FileInfo
	Ext  string
}

func (r *Source) OpenAsset(name string) (*Asset, error) {
	assetPath, err := r.AssetPath(name)
	if err != nil {
		return nil, util.Errorf("%w", err)
	}

	assetFile, err := r.root.Open(assetPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, util.Errorf("%w: %s", ErrAssetNotFound, name)
		}
		return nil, util.Errorf("%w", err)
	}

	info, err := assetFile.Stat()
	if err != nil {
		assetFile.Close()
		return nil, util.Errorf("%w", err)
	}
	if !info.Mode().IsRegular() {
		assetFile.Close()
		return nil, util.Errorf("%w: %s", ErrAssetNotFound, name)
	}

	return &Asset{File: assetFile, Info: info, Ext: path.Ext(name)}, nil
}

func (r *Source) ListAssets() ([]string, error) {
	entries, err := r.root.ReadDir(path.Join(r.folderPath, SourceAssetsFolder))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, util.Errorf("%w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if validateAssetName(entry.Name()) != nil {
			continue
		}
		names = append(names, entry.Name())
	}
	slices.Sort(names)
	return names, nil
}
