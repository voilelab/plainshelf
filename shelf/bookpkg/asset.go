package bookpkg

import (
	"errors"
	"io/fs"
	"path"
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
// source thus owns its images: DeleteSource removes the whole source folder, so
// there are no orphans to collect.
//
// The directory is deliberately flat — an asset name is a single path segment,
// which makes "contains no separator" a complete traversal defense on its own.
//
// Nothing records the directory's contents: the filesystem is the list. That
// keeps book.json's schema unchanged, so a build without asset support reads
// such a shelf as it always did.
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
	// A leading dot would let a request name a hidden file. The shelf never
	// writes one under assets/, so nothing legitimate is refused here. Checked
	// before shelfutil.ValidatePathSegment, which rejects dot-prefixed segments too but
	// reports them in terms of the directory names the shelf scanner skips —
	// wrong vocabulary for an asset request.
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

// WriteAsset stores one illustration in the source's assets/ directory,
// replacing one already under that name.
//
// The name goes through the same validation the read path uses, so a file the
// server could never serve cannot be written in the first place.
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

// DeleteAsset removes one illustration from the source's assets/ directory.
//
// A name that is not stored reports ErrAssetNotFound rather than succeeding
// quietly: unlike a cover, which a book either has or does not, an asset is
// addressed by name, and a silent success would hide a typo.
//
// The text is not touched. A link left pointing at a deleted file renders as
// its alt text, and rewriting someone's prose to keep an invariant the shelf
// does not enforce would be a worse trade.
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

// AssetETag returns a weak ETag derived from the asset's mtime and size, or an
// empty string when the name is invalid or the file cannot be stat'd.
func (r *Source) AssetETag(name string) string {
	assetPath, err := r.AssetPath(name)
	if err != nil {
		return ""
	}
	return shelfutil.FileETag(r.root, assetPath)
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
// The file is handed back open rather than buffered: unlike a cover, which the
// API caps on upload, an asset is placed on the shelf by hand and can be
// arbitrarily large, so buffering one per in-flight request would let a few big
// illustrations decide the server's memory use.
//
// A missing asset and a name resolving to a non-regular file both report
// ErrAssetNotFound: to a reader they are the same outcome, and neither is a
// server fault.
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
