package shelf

import (
	"errors"
	"io/fs"
	"log/slog"
	"path"
	"slices"
	"strings"

	"github.com/voilelab/plainshelf/internal/fsutil"
	"github.com/voilelab/plainshelf/internal/logutil"
	"github.com/voilelab/plainshelf/internal/util"
)

var ErrNotAShelf = util.NewError("folder is not a shelf: it has no books/ directory")

func IsShelfRoot(dirs []string) bool {
	return slices.Contains(dirs, booksFolder)
}

func IsBookPackage(name string) bool {
	return strings.HasSuffix(name, bookExtension)
}

func CountBookPackages(root fsutil.ReadFS) (int, error) {
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
