package shelf

import (
	"errors"
	"io"
	"io/fs"
	"path"
	"strings"

	"github.com/voilelab/plainshelf/internal/util"
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
// Assets sit beside the text that references them, so `![](assets/img-0001.jpg)`
// in source.txt resolves the same way in the reader as it does in any ordinary
// editor opened on that file. It also means a source owns its images:
// DeleteSource already removes the whole source folder, so there are no
// orphans to collect.
//
// The directory is deliberately flat. An asset name is a single path segment,
// which makes "contains no separator" a complete traversal defense rather than
// one rule among several.
//
// Nothing records the contents of this directory: the filesystem is the list.
// That is what keeps book.json's schema unchanged, so a build without asset
// support reads such a shelf as it always did.
const SourceAssetsFolder = "assets"

// ErrAssetNotFound is returned when a source has no asset under the given name.
var ErrAssetNotFound = util.NewError("asset not found")

// ErrInvalidAssetName is returned when an asset name is not a safe, servable
// file name. Every asset lookup validates the name before touching the
// filesystem, so callers can treat it as a request error.
var ErrInvalidAssetName = util.NewError("invalid asset name")

// imageExtensions are the image file extensions the shelf stores and the read
// path serves with a correct content type.
var imageExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true,
}

// IsSupportedImageExt reports whether ext — a file extension including the
// leading dot, in any case — is one the read path serves with a correct
// content type.
func IsSupportedImageExt(ext string) bool {
	return imageExtensions[strings.ToLower(ext)]
}

func validateAssetName(name string) error {
	if err := validatePathSegment(name); err != nil {
		return util.Errorf("%w %q: %w", ErrInvalidAssetName, name, err)
	}
	// A leading dot would let a request name a hidden file. The shelf never
	// writes one under assets/, so nothing legitimate is refused here.
	if strings.HasPrefix(name, ".") {
		return util.Errorf("%w %q: must not start with a dot", ErrInvalidAssetName, name)
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

// AssetETag returns a weak ETag derived from the asset's mtime and size, or an
// empty string when the name is invalid or the file cannot be stat'd.
func (r *Source) AssetETag(name string) string {
	assetPath, err := r.AssetPath(name)
	if err != nil {
		return ""
	}
	return fileETag(r.root, assetPath)
}

// OpenAsset reads one of this source's assets, returning its bytes and its
// file extension including the leading dot.
//
// A missing asset, and a name that resolves to something other than a regular
// file, both report ErrAssetNotFound: from a reader's point of view they are
// the same outcome, and neither is a server fault.
func (r *Source) OpenAsset(name string) ([]byte, string, error) {
	assetPath, err := r.AssetPath(name)
	if err != nil {
		return nil, "", util.Errorf("%w", err)
	}

	assetFile, err := r.root.Open(assetPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, "", util.Errorf("%w: %s", ErrAssetNotFound, name)
		}
		return nil, "", util.Errorf("%w", err)
	}
	defer assetFile.Close()

	// Stat the open handle rather than the path: it costs no extra lookup and
	// keeps a directory named like an image from failing as a read error.
	info, err := assetFile.Stat()
	if err != nil {
		return nil, "", util.Errorf("%w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, "", util.Errorf("%w: %s", ErrAssetNotFound, name)
	}

	data, err := io.ReadAll(assetFile)
	if err != nil {
		return nil, "", util.Errorf("%w", err)
	}

	return data, path.Ext(name), nil
}
