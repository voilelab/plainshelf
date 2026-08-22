package shelf

import (
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/util"
	"github.com/voilelab/plainshelf/shelf/internal/shelfutil"
	"go.rtnl.ai/x/slugify"
)

const MaxTempDirCreationAttempts = 10

// MaxBookIDCreationAttempts bounds the retry loop that draws a fresh random
// book ID when the drawn one is already taken. A v4 UUID's 122 random bits make
// a single retry already unreachable in practice; the bound only keeps a
// filesystem that answers every probe with "taken" from spinning forever.
const MaxBookIDCreationAttempts = 10

func createTempDir(root fsutil.FS, prefix string) (string, error) {
	for range MaxTempDirCreationAttempts {
		tmpDirName := fmt.Sprintf("%s-%s-%s", prefix, time.Now().Format("20060102-150405"), shelfutil.RandomString(6))
		err := root.Mkdir(tmpDirName)
		if err == nil {
			return tmpDirName, nil
		}
	}

	return "", util.NewError("failed to create temp directory after multiple attempts")
}

// copyTreeAcross recursively copies the tree rooted at src in srcRoot onto dst in
// dstRoot, reproducing every file and subdirectory. dst is created if it does not
// exist. The two roots may be the same filesystem (a same-shelf copy) or two
// different ones: a book copied whole stays self-contained, so the relative asset
// paths a source records need no rewriting, which is what lets a book move between
// two shelves - including across a filesystem boundary that os.Rename cannot
// cross.
//
// Whether a child is a directory is decided by Stat, not by the directory
// entry's own type, so that a symlinked directory is descended into and copied
// as a real one - the same way the shelf scanner (scancache.ChildIsDir) treats
// it. A
// listing reports a symlink as a non-directory, but opening it as a file fails,
// so keying the copy on the entry type would break a package that holds one.
func copyTreeAcross(srcRoot fsutil.ReadFS, src string, dstRoot fsutil.FS, dst string) error {
	info, err := srcRoot.Stat(src)
	if err != nil {
		return util.Errorf("%w", err)
	}

	if !info.IsDir() {
		return copyFileAcross(srcRoot, src, dstRoot, dst)
	}

	if err := dstRoot.MkdirAll(dst); err != nil {
		return util.Errorf("%w", err)
	}

	entries, err := srcRoot.ReadDir(src)
	if err != nil {
		return util.Errorf("%w", err)
	}

	for _, entry := range entries {
		if err := copyTreeAcross(srcRoot, path.Join(src, entry.Name()), dstRoot, path.Join(dst, entry.Name())); err != nil {
			return util.Errorf("%w", err)
		}
	}

	return nil
}

// copyFileAcross copies a single regular file from src in srcRoot to dst in
// dstRoot, creating or truncating dst.
func copyFileAcross(srcRoot fsutil.ReadFS, src string, dstRoot fsutil.FS, dst string) error {
	in, err := srcRoot.Open(src)
	if err != nil {
		return util.Errorf("%w", err)
	}
	defer in.Close()

	out, err := dstRoot.OpenWriter(dst)
	if err != nil {
		return util.Errorf("%w", err)
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return util.Errorf("%w", err)
	}

	if err := out.Close(); err != nil {
		return util.Errorf("%w", err)
	}

	return nil
}

// ErrInvalidLayer is returned when a layer name is not a usable path segment.
// Every operation that accepts caller-supplied layers checks them before
// touching the filesystem, so callers can treat it as a request error.
var ErrInvalidLayer = util.NewError("invalid layer name")

// ErrIgnoredLayerName is the ErrInvalidLayer case where the name is well formed
// but names a directory the scanners skip. It wraps ErrInvalidLayer, so callers
// that only classify layer errors keep matching it, while the API can tell this
// reason apart and explain it: a user filing an existing "@eaDir" under
// PlainShelf is not making a typo, they are hitting a deliberate rule.
var ErrIgnoredLayerName = util.Errorf("%w: hidden or system directory name", ErrInvalidLayer)

func validateLayers(layers Layers) error {
	for _, layer := range layers {
		if err := shelfutil.ValidatePathSegment(layer); err != nil {
			if errors.Is(err, shelfutil.ErrIgnoredPathSegment) {
				return util.Errorf("%w %q: %w", ErrIgnoredLayerName, layer, err)
			}
			return util.Errorf("%w %q: %w", ErrInvalidLayer, layer, err)
		}
		if strings.Contains(layer, bookExtension) {
			return util.Errorf("%w %q: must not contain %q", ErrInvalidLayer, layer, bookExtension)
		}
	}
	return nil
}

// ValidateLayers reports whether every layer path segment is safe to use.
// API handlers use this before scheduling background work so malformed batch
// requests fail synchronously rather than becoming failed worker tasks.
func ValidateLayers(layers Layers) error {
	return validateLayers(layers)
}

// newBookID draws a random book ID as a version 4 UUID.
//
// The ID is opaque: generated once at creation, persisted in book.json, and
// never recomputed, so renaming the title, moving the book, or restoring it
// from trash all leave it alone. Older builds derived it from layers and title,
// which read as if it could be recomputed and gave two books the same ID
// whenever they shared a layer path and title — routine on a shared shelf.
//
// A v4 UUID's 122 random bits make the ID unique on its own rather than by
// agreement: the creation-time collision probe cannot see a book another
// machine just wrote into a shared shelf, or one copied in with a file manager,
// so the ID has to stand alone. Its canonical form is lowercase hex with
// hyphens, which survives a case-insensitive filesystem (the trash names a
// folder after the book ID) and sits in a URL path without escaping.
func newBookID() (string, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", util.Errorf("%w", err)
	}
	return id.String(), nil
}

// validateBookID reports whether a caller-supplied ID is usable as one.
//
// It deliberately says nothing about the shape of the ID beyond path safety.
// This build writes v4 UUIDs, but shelves written by older builds carry
// 8-character hex IDs (some with a "-1" de-duplication suffix) and 16-character
// base32 IDs, and those stay valid forever alongside the UUIDs; a shelf holds
// them all at once and none is migrated.
func validateBookID(bookID string) error {
	if err := shelfutil.ValidatePathSegment(bookID); err != nil {
		return util.Errorf("invalid book id %q: %w", bookID, err)
	}
	return nil
}

func titleToFolderName(title string) string {
	folderName := strings.ReplaceAll(title, " ", "-")
	return slugify.Slugify(folderName) + bookExtension
}
