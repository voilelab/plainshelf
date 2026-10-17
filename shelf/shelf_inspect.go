package shelf

import (
	"errors"
	"io/fs"
	"log/slog"
	"path"
	"strings"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
)

// These answer "is this a shelf" from names alone, for a client choosing a
// shelf root before any shelf is opened, so it need not know the layout.

// ErrNotAShelf is returned for a folder without a books/ directory.
var ErrNotAShelf = util.NewError("folder is not a shelf: it has no books/ directory")

// IsShelfRoot reports whether a folder whose subdirectories are named dirs is
// a shelf root.
func IsShelfRoot(dirs []string) bool {
	for _, name := range dirs {
		if name == booksFolder {
			return true
		}
	}
	return false
}

// IsBookPackage reports whether a directory named name is a book package,
// which can never be a shelf root.
func IsBookPackage(name string) bool {
	return strings.HasSuffix(name, bookExtension)
}

// CountBookPackages counts the book packages under root's books/, skipping the
// directories shelf.json ignores. Books are not opened, so on a remote root it
// costs a listing, not a read per book.
func CountBookPackages(root fsutil.ReadFS) (int, error) {
	// The root's own failure (a missing folder, no network) is not "not a shelf".
	if _, err := root.Stat("."); err != nil {
		return 0, util.Errorf("%w", err)
	}
	info, err := root.Stat(booksFolder)
	if err != nil || !info.IsDir() {
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return 0, util.Errorf("%w", err)
		}
		return 0, ErrNotAShelf
	}
	logger := logutil.Logger{Logger: slog.New(slog.DiscardHandler)}
	ignore := loadShelfRules(root, logger).ignore

	var count func(dir string) (int, error)
	count = func(dir string) (int, error) {
		entries, err := root.ReadDir(dir)
		if err != nil {
			return 0, util.Errorf("%w", err)
		}
		n := 0
		for _, entry := range entries {
			if !entry.IsDir() || ignore.IsIgnoredDir(entry.Name()) {
				continue
			}
			if IsBookPackage(entry.Name()) {
				n++
				continue
			}
			sub, err := count(path.Join(dir, entry.Name()))
			if err != nil {
				return 0, err
			}
			n += sub
		}
		return n, nil
	}
	return count(booksFolder)
}
