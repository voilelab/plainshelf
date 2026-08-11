package shelf

import (
	"errors"
	"io/fs"
	"path"
	"strings"

	"github.com/voilelab/plainshelf/internal/fsutil"

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

// WriteAsset stores one illustration in the source's assets/ directory.
//
// This is an internal writer for import, not an API surface: no HTTP route
// reaches it, so the mutating-request boundary is untouched and assets still
// cannot be placed through the API.
//
// The name goes through the same validation the read path uses, so a file the
// server could never serve cannot be written in the first place.
func (r *Source) WriteAsset(name string, data []byte) error {
	assetPath, err := r.AssetPath(name)
	if err != nil {
		return util.Errorf("%w", err)
	}

	if err := r.root.MkdirAll(path.Join(r.folderPath, SourceAssetsFolder)); err != nil {
		return util.Errorf("%w", err)
	}

	if err := fsutil.WriteFileAtomic(r.root, assetPath, data); err != nil {
		return util.Errorf("%w", err)
	}

	return nil
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

// Asset is an open handle to one of a source's illustrations. The caller owns
// File and must close it.
//
// Info comes from the same open handle, so a caller can size the response
// without a second lookup, and Ext is the name's extension including the
// leading dot.
type Asset struct {
	File fs.File
	Info fs.FileInfo
	Ext  string
}

// OpenAsset opens one of this source's assets for reading.
//
// The file is handed back open rather than read into memory: unlike a cover,
// which the API caps when it is uploaded, an asset is a file the user placed on
// the shelf by hand and can be arbitrarily large. Buffering one per in-flight
// request would let a handful of large illustrations decide the server's memory
// use.
//
// A missing asset, and a name that resolves to something other than a regular
// file, both report ErrAssetNotFound: from a reader's point of view they are
// the same outcome, and neither is a server fault.
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

	// Stat the open handle rather than the path: it costs no extra lookup and
	// keeps a directory named like an image from failing as a read error.
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
