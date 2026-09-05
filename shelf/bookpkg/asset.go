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

/*
{source-folder}/
├─ meta.json
├─ source.txt
└─ assets/
   ├─ img-0001.jpg
   └─ img-0002.png
*/

// SourceAssetsFolder holds the illustrations a source's text references.
//
// Assets sit beside the text, so `![](assets/img-0001.jpg)` in source.txt
// resolves the same way in the reader as in any editor opened on that file. A
// source thus owns its images, and DeleteSource leaves no orphans behind.
//
// The directory is deliberately flat, which makes "contains no separator" a
// complete traversal defense on its own, and nothing records its contents: the
// filesystem is the list, so no metadata schema had to change.
const SourceAssetsFolder = "assets"

var ErrAssetNotFound = util.NewError("asset not found")

// ErrInvalidAssetName is reported before the filesystem is touched, so callers
// can treat it as a request error.
var ErrInvalidAssetName = util.NewError("invalid asset name")

var imageExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true,
}

// IsSupportedImageExt takes an extension including the leading dot, in any case.
func IsSupportedImageExt(ext string) bool {
	return imageExtensions[strings.ToLower(ext)]
}

func validateAssetName(name string) error {
	// Checked before shelfutil.ValidatePathSegment, which rejects dot-prefixed
	// segments too but reports them in the vocabulary of the shelf scanner.
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

// AssetPath returns the shelf-relative path of one of this source's assets.
func (r *Source) AssetPath(name string) (string, error) {
	if err := validateAssetName(name); err != nil {
		return "", util.Errorf("%w", err)
	}
	return path.Join(r.folderPath, SourceAssetsFolder, name), nil
}

// WriteAsset stores one illustration, replacing one already under that name.
// The name goes through the validation the read path uses, so a file the server
// could never serve cannot be written in the first place.
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

// DeleteAsset reports ErrAssetNotFound rather than succeeding quietly, since an
// asset is addressed by name and a silent success would hide a typo.
//
// The text is not touched: a link to a deleted file renders as its alt text,
// which beats rewriting someone's prose.
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

// AssetETag returns an empty string when the asset cannot be stat'd.
func (r *Source) AssetETag(name string) string {
	assetPath, err := r.AssetPath(name)
	if err != nil {
		return ""
	}
	return shelfutil.FileETag(r.root, assetPath)
}

// Asset is an open handle to one of a source's illustrations. The caller owns
// File and must close it. Info comes from that same handle, so a caller can
// size the response without a second lookup.
type Asset struct {
	File fs.File
	Info fs.FileInfo
	Ext  string
}

// OpenAsset hands the file back open rather than buffered: unlike a cover,
// which the API caps on upload, an asset is placed on the shelf by hand and can
// be arbitrarily large.
//
// A missing asset and a name resolving to a non-regular file both report
// ErrAssetNotFound; to a reader they are the same outcome.
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

	// Stat the open handle rather than the path: no extra lookup, and a
	// directory named like an image does not fail as a read error.
	info, err := assetFile.Stat()
	if err != nil {
		assetFile.Close() //nolint:errcheck // best-effort cleanup; stat error is returned
		return nil, util.Errorf("%w", err)
	}
	if !info.Mode().IsRegular() {
		assetFile.Close() //nolint:errcheck // best-effort cleanup; the asset is unusable
		return nil, util.Errorf("%w: %s", ErrAssetNotFound, name)
	}

	return &Asset{File: assetFile, Info: info, Ext: path.Ext(name)}, nil
}

// ListAssets returns exactly the set a per-name request could open: anything
// the read path could not serve is skipped, and a missing assets/ directory is
// an empty result rather than an error.
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
